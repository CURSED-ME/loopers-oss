package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/a2a"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/event"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/policy"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/proxy"
	"github.com/CURSED-ME/loopers-oss/internal/session"
	"github.com/CURSED-ME/loopers-oss/pkg/api"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) handleProxy(c *gin.Context, providerName string) {
	startTime := time.Now()

	// 1. Auth check
	var meta *keyring.KeyMetadata
	var keyHash string
	reqID := c.GetString("RequestID")

	if claims, exists := c.Get(JWTClaimsCtx); exists {
		meta = claims.(*keyring.KeyMetadata)
		if keyHashCtx, ok := c.Get("KeyHash"); ok {
			keyHash = keyHashCtx.(string)
		} else {
			keyHash = keyring.HashKey(meta.Name)
		}

		// DPoP Validation
		dpopHeader := c.GetHeader("DPoP")
		if dpopHeader != "" {
			requestURL := "https://" + c.Request.Host + c.Request.URL.Path
			if pub := viper.GetString("server.public_url"); pub != "" {
				requestURL = strings.TrimSuffix(pub, "/") + c.Request.URL.Path
			}
			_, err := keyring.ValidateDPoPAndReplay(c.Request.Context(), s.redis.GetUnderlyingClient(), dpopHeader, c.Request.Method, requestURL, meta.Jkt)
			if err != nil {
				if err.Error() == "DPoP token replay detected" {
					zspAuthFailuresTotal.WithLabelValues("dpop_replayed").Inc()
				} else {
					zspAuthFailuresTotal.WithLabelValues("dpop_invalid").Inc()
				}
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
		} else if meta.Jkt != "" {
			// Required DPoP but missing
			zspAuthFailuresTotal.WithLabelValues("dpop_missing").Inc()
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing DPoP header"})
			return
		}

	} else if keyHashCtx, exists := c.Get("KeyHash"); exists {
		keyHash = keyHashCtx.(string)
		var err error
		meta, err = keyring.GetKeyMetadata(c.Request.Context(), s.redis.GetUnderlyingClient(), keyHash)
		if err != nil {
			if err.Error() == "key does not exist" {
				requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
				if s.alerter != nil {
					go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Key not registered")
				}
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key not registered"})
			} else {
				logging.Logger.Error().Err(err).Msg("failed to get key metadata")
				requestsTotal.WithLabelValues(providerName, "unknown", "503").Inc()
				if s.alerter != nil {
					go s.alerter.TriggerFailClosed(detachedTraceContext(c.Request.Context()), reqID, "Failed to get key metadata")
				}
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable: internal error"})
			}
			return
		}
	} else {
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.alerter != nil {
			go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Missing Authorization header")
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	if !strings.EqualFold(meta.Active, "true") {
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.alerter != nil {
			go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Key revoked")
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key has been revoked"})
		return
	}

	if meta.Provider != providerName {
		requestsTotal.WithLabelValues(providerName, "unknown", "400").Inc()
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "key not authorized for this endpoint",
		})
		return
	}

	if allowedProviders := meta.ParseAllowedProviders(); len(allowedProviders) > 0 {
		if !allowedProviders[providerName] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "key not authorized for this provider",
				"type":  "provider_not_allowed",
			})
			return
		}
	}

	if s.otelEnabled {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(
			attribute.String("gen_ai.system", providerName),
			attribute.String("loopers.budget.key_hash", keyHash),
		)
		if meta.AgentName != "" {
			span.SetAttributes(attribute.String("loopers.agent.name", meta.AgentName))
		}
	}

	if s.rateLimiter != nil {
		allowed, remaining, rlErr := s.rateLimiter.Check(c.Request.Context(), keyHash)
		if rlErr != nil {
			logging.Logger.Error().Err(rlErr).Msg("rate_limiter_error")
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "rate_limit_degraded",
				KeyHash:   keyHash,
				Provider:  providerName,
				Reason:    "rate_limiter_error",
				Detail:    rlErr.Error(),
			})
			failClosed := true // Default to true per security strategy
			if viper.IsSet("security.rate_limit_fail_closed") {
				failClosed = viper.GetBool("security.rate_limit_fail_closed")
			}
			if failClosed {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable: rate limiter error"})
				return
			}
		} else if !allowed {
			if s.escalationBroker != nil {
				sessionID := c.GetHeader("X-Loopers-Session-ID")
				if sessionID == "" {
					sessionID = reqID
				}
				req := a2a.EscalationRequest{
					SessionID: sessionID,
					Reason:    "rate_limited",
					AgentName: meta.AgentName,
				}
				resp, escErr := s.escalationBroker.RequestEscalation(c.Request.Context(), req, 5*time.Second)
				if escErr == nil && resp.Approved {
					escalationsApprovedTotal.WithLabelValues("rate_limited").Inc()
					logging.Logger.Info().Str("session_id", sessionID).Msg("rate limit block escalated and approved")
					allowed = true
				} else if escErr != nil {
					escalationsTimeoutTotal.WithLabelValues("rate_limited").Inc()
					logging.Logger.Warn().Err(escErr).Str("session_id", sessionID).Msg("escalation request timed out or failed")
				}
			}

			if !allowed {
				event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
					EventType: "rate_limit_block",
					KeyHash:   keyHash,
					Provider:  providerName,
					Reason:    "rate_limited",
					Detail:    "Rate limit exceeded",
				})
				rateLimitBlocksTotal.WithLabelValues(providerName).Inc()
				requestsTotal.WithLabelValues(providerName, "unknown", "429").Inc()
				c.Header("X-RateLimit-Remaining", "0")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "rate limit exceeded",
					"type":  "rate_limit_exceeded",
				})
				return
			} else {
				c.Header("X-RateLimit-Remaining", "0")
			}
		} else {
			c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		}
	}

	prov, err := s.registry.Get(providerName)
	if err != nil {
		requestsTotal.WithLabelValues(providerName, "unknown", "400").Inc()
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	providerKey, _ := c.Get(proxy.ProviderKeyCtx)
	providerKeyStr, _ := providerKey.(string)

	// 2. Parse request body for model, max_tokens, and streaming
	bodyBytes, exists := c.Get(RequestBodyCtx)
	if !exists {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "RequestBodyCtx missing - did BodyBuffer run?"})
		return
	}
	body, ok := bodyBytes.([]byte)
	if !ok {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "RequestBodyCtx is not a byte slice"})
		return
	}

	model, isStream, maxTokensVal, mutatedBody, err := prov.ParseRequest(c.Request, body)
	if err != nil {
		requestsTotal.WithLabelValues(providerName, "unknown", "400").Inc()
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON request payload"})
		return
	}

	if override := c.GetHeader("X-Loopers-Model-Override"); override != "" {
		mutatedBody, err = prov.RewriteModel(c.Request, mutatedBody, override)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply model override"})
			return
		}
		model = override
	} else if modelMapHeader := c.GetHeader("X-Loopers-Model-Map"); modelMapHeader != "" {
		mappings := strings.Split(modelMapHeader, ",")
		for _, mapping := range mappings {
			parts := strings.SplitN(mapping, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == model {
				alias := strings.TrimSpace(parts[1])
				mutatedBody, err = prov.RewriteModel(c.Request, mutatedBody, alias)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply model map"})
					return
				}
				model = alias
				break
			}
		}
	}

	if s.otelEnabled {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.String("gen_ai.request.model", model))
	}

	if s.policyEngine != nil {
		// Build a stateful session context for OPA — fetch taint history if a session is active.
		// We do a best-effort early read of the session ID here; the full session enforcement
		// happens later in enforceSessionLimits, but we need the taint context for policy now.
		earlySessionID := c.GetHeader("X-Loopers-Session-ID")
		policySessionCtx := policy.SessionContext{
			ID: earlySessionID,
		}
		if earlySessionID != "" && session.IsValidID(earlySessionID) && s.sessionManager != nil {
			if taintFlags, tErr := s.sessionManager.GetTaintFlags(c.Request.Context(), keyHash, earlySessionID); tErr == nil {
				policySessionCtx.TaintFlags = taintFlags
			} else {
				logging.Logger.Warn().Err(tErr).Str("session_id", earlySessionID).Msg("proxy_failed_to_fetch_taint_flags")
			}
			if toolHistory, hErr := s.sessionManager.GetToolCallHistory(c.Request.Context(), keyHash, earlySessionID); hErr == nil {
				policySessionCtx.ToolsCalled = toolHistory
			} else {
				logging.Logger.Warn().Err(hErr).Str("session_id", earlySessionID).Msg("proxy_failed_to_fetch_tool_history")
			}
		}
		if policySessionCtx.TaintFlags == nil {
			policySessionCtx.TaintFlags = make(map[string]bool)
		}
		if policySessionCtx.ToolsCalled == nil {
			policySessionCtx.ToolsCalled = make([]string, 0)
		}

		promptText, _ := proxy.MapLLMCall(providerName, mutatedBody)

		decision, err := s.policyEngine.Evaluate(c.Request.Context(), policy.EvalInput{
			Agent: policy.AgentContext{
				KeyHash:   keyHash,
				Name:      meta.Name,
				AgentName: meta.AgentName,
				Owner:     meta.Owner,
				Provider:  meta.Provider,
				Tags:      meta.ParseTags(),
			},
			Request: policy.RequestContext{
				Provider: providerName,
				Model:    model,
				Method:   "llm_call",
				Path:     c.Request.URL.Path,
			},
			Session: policySessionCtx,
			Action: policy.ActionContext{
				Type:       "llm_call",
				Provider:   providerName,
				Model:      model,
				PromptText: promptText,
			},
		})
		if err != nil {
			logging.Logger.Error().Err(err).Msg("policy_engine_evaluation_error")
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "policy_evaluation_error",
				KeyHash:   keyHash,
				Provider:  providerName,
				Model:     model,
				Reason:    "policy_evaluation_error",
				Detail:    err.Error(),
			})
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "Policy evaluation failed",
				"type":  "internal_error",
			})
			return
		}
		if !decision.Allowed {
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "policy_block",
				KeyHash:   keyHash,
				Provider:  providerName,
				Model:     model,
				Reason:    "policy_denied",
				Detail:    decision.Reason,
			})
			c.AbortWithStatusJSON(http.StatusForbidden, api.NewPolicyDeniedResponse("", providerName, decision.Reason))
			return
		}
	}

	// 3. Pricing lookup
	inputPrice, outputPrice, defaultMaxOut := s.pricing.GetModelPrice(providerName, model)
	if maxTokensVal == 0 {
		maxTokensVal = defaultMaxOut
	}

	// 4. Token counting
	inputTokens, countErr := prov.CountInputTokens(c.Request.Context(), model, mutatedBody, providerKeyStr)
	if countErr != nil {
		requestsTotal.WithLabelValues(providerName, model, "400").Inc()
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Token counting failed: %v", countErr)})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(mutatedBody))
	c.Request.ContentLength = int64(len(mutatedBody))

	// 5. Estimate cost
	estimatedCost := pricing.EstimateCost(inputTokens, maxTokensVal, inputPrice, outputPrice)

	// 6. Check and reserve budget via Enforcement Engine
	model, estimatedCost, inputPrice, outputPrice, inputTokens, mutatedBody, err = s.enforceBudgetWithFallback(c, providerName, model, estimatedCost, inputPrice, outputPrice, inputTokens, maxTokensVal, mutatedBody, providerKeyStr, keyHash, meta, reqID)
	if err != nil {
		return // response already sent by enforceBudgetWithFallback
	}

	sessionID := c.GetHeader("X-Loopers-Session-ID")
	var sessionBudget float64
	var sessionMaxSteps int

	// 6.5 Session limits and Loop Detection
	if sessionID != "" {
		if s.otelEnabled {
			span := trace.SpanFromContext(c.Request.Context())
			span.SetAttributes(attribute.String("gen_ai.system.session.id", sessionID))
		}

		if !session.IsValidID(sessionID) {
			requestsTotal.WithLabelValues(providerName, model, "400").Inc()
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID format"})
			return
		}

		if s.sessionManager != nil {
			maxSessions := viper.GetInt("session.max_per_key")
			if maxSessions <= 0 {
				maxSessions = 100 // Default to 100 concurrent sessions per key
			}
			allowed, err := s.sessionManager.TrackAndLimitSessions(c.Request.Context(), keyHash, sessionID, maxSessions)
			if err != nil {
				logging.Logger.Error().Err(err).Msg("failed_session_track_limit")
			} else if !allowed {
				requestsTotal.WithLabelValues(providerName, model, "429").Inc()
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "maximum concurrent sessions per key exceeded",
					"type":  "session_limit_exceeded",
				})
				return
			}
		}

		if ttlHeader := c.GetHeader("X-Loopers-Session-TTL"); ttlHeader != "" {
			if ttl, err := strconv.Atoi(ttlHeader); err == nil && ttl > 0 {
				maxTtl := viper.GetInt("session.max_ttl_seconds")
				if maxTtl <= 0 {
					maxTtl = 86400
				}
				if ttl > maxTtl {
					ttl = maxTtl
				}
				if s.sessionManager != nil {
					valid, err := s.sessionManager.EnforceAbsoluteTTL(c.Request.Context(), keyHash, sessionID, ttl)
					if err != nil {
						logging.Logger.Error().Err(err).Msg("failed_session_ttl_check")
					} else if !valid {
						requestsTotal.WithLabelValues(providerName, model, "400").Inc()
						c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
							"error": "session expired",
							"type":  "session_ttl_exceeded",
						})
						return
					}
				}
			}
		}

		if budgetHeader := c.GetHeader("X-Loopers-Session-Budget"); budgetHeader != "" {
			allowOverride := viper.GetBool("session.allow_client_budget_override")
			if !viper.IsSet("session.allow_client_budget_override") {
				allowOverride = false // VULN-018: Default to false
			}
			if !allowOverride {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "client-controlled session budgets are disabled by policy",
					"type":  "invalid_request_error",
				})
				return
			}
			sessionBudget, _ = strconv.ParseFloat(budgetHeader, 64)
		}
		if stepsHeader := c.GetHeader("X-Loopers-Session-Max-Steps"); stepsHeader != "" {
			allowOverride := viper.GetBool("session.allow_client_budget_override")
			if !viper.IsSet("session.allow_client_budget_override") {
				allowOverride = false // VULN-018: Default to false
			}
			if !allowOverride {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "client-controlled session limits are disabled by policy",
					"type":  "invalid_request_error",
				})
				return
			}
			sessionMaxSteps, _ = strconv.Atoi(stepsHeader)
		}

		// Enforce session limits
		err = s.enforceSessionLimits(c, providerName, model, sessionID, sessionBudget, sessionMaxSteps, estimatedCost, keyHash, meta, reqID)
		if err != nil {
			return // response sent
		}

		// Enforce loop detection
		err = s.enforceLoopDetection(c, providerName, model, sessionID, body, estimatedCost, keyHash, meta, reqID)
		if err != nil {
			return // response sent
		}
	}

	// 7. Propagation to context for ReverseProxy
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, startTimeCtx, startTime)
	ctx = context.WithValue(ctx, proxy.ProxyKeyHashCtx, keyHash)
	ctx = context.WithValue(ctx, proxy.RequestCostCtx, estimatedCost)
	ctx = context.WithValue(ctx, proxy.ProviderKeyCtx, providerKeyStr)
	ctx = context.WithValue(ctx, proxy.ProviderCtx, providerName)
	ctx = context.WithValue(ctx, proxy.ProviderInstanceCtx, prov)
	ctx = context.WithValue(ctx, proxy.ModelCtx, model)
	ctx = context.WithValue(ctx, isStreamCtx, isStream)
	ctx = context.WithValue(ctx, inputPriceCtx, inputPrice)
	ctx = context.WithValue(ctx, outputPriceCtx, outputPrice)
	ctx = context.WithValue(ctx, inputTokensCtx, inputTokens)
	ctx = context.WithValue(ctx, keyNameCtx, meta.Name)
	if meta.AgentName != "" {
		ctx = context.WithValue(ctx, agentNameCtx, meta.AgentName)
	}
	ctx = context.WithValue(ctx, requestIDCtx, reqID)
	if sessionID != "" {
		ctx = context.WithValue(ctx, sessionIDCtx, sessionID)
		ctx = context.WithValue(ctx, sessionBudgetCtx, sessionBudget)
		ctx = context.WithValue(ctx, sessionMaxStepsCtx, sessionMaxSteps)
	}
	c.Request = c.Request.WithContext(ctx)

	// 8. Stream output using ReverseProxy
	s.proxy.ServeHTTP(c.Writer, c.Request)
}

func (s *Server) modifyResponse(resp *http.Response) error {
	req := resp.Request
	ctx := req.Context()

	startTime, _ := ctx.Value(startTimeCtx).(time.Time)
	keyHash, _ := ctx.Value(proxy.ProxyKeyHashCtx).(string)
	keyName, _ := ctx.Value(keyNameCtx).(string)
	provName, _ := ctx.Value(proxy.ProviderCtx).(string)
	reservedCost, _ := ctx.Value(proxy.RequestCostCtx).(float64)
	prov, _ := ctx.Value(proxy.ProviderInstanceCtx).(provider.Provider)
	model, _ := ctx.Value(proxy.ModelCtx).(string)
	isStream, _ := ctx.Value(isStreamCtx).(bool)
	inputPrice, _ := ctx.Value(inputPriceCtx).(float64)
	outputPrice, _ := ctx.Value(outputPriceCtx).(float64)
	sessionID, _ := ctx.Value(sessionIDCtx).(string)
	reqID, _ := ctx.Value(requestIDCtx).(string)

	if resp.StatusCode != http.StatusOK {
		s.redis.LeaseManager.ReconcileSpend(ctx, keyHash, reservedCost, 0)
		if sessionID != "" {
			sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
			if err := s.redis.GetUnderlyingClient().IncrBy(ctx, sessionSpendKey, -budget.ToNano(reservedCost)).Err(); err != nil {
				logging.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("failed to update session spend in redis")
			}
		}

		// Record non-200 request metrics
		latency := time.Since(startTime)
		requestDuration.WithLabelValues(provName).Observe(latency.Seconds())
		requestsTotal.WithLabelValues(provName, model, strconv.Itoa(resp.StatusCode)).Inc()
		return nil
	}

	// Set initial headers for both stream and non-stream
	if !viper.GetBool("server.strip_budget_headers") {
		resp.Header.Set("X-Loopers-Request-Cost-Estimated", fmt.Sprintf("%.6f", reservedCost))
	}
	if agentName, ok := ctx.Value(agentNameCtx).(string); ok && agentName != "" {
		resp.Header.Set("X-Loopers-Agent-Name", agentName)
	}

	if sessionID != "" {
		sessionStepsKey := fmt.Sprintf("loopers:session:{%s}:%s:steps", keyHash, sessionID)

		rdb := s.redis.GetUnderlyingClient()
		vals, _ := rdb.MGet(ctx, sessionStepsKey).Result()
		var stepsVal int64
		if len(vals) == 1 {
			if vals[0] != nil {
				if stVal, err := strconv.ParseInt(fmt.Sprintf("%v", vals[0]), 10, 64); err == nil {
					stepsVal = stVal
				}
			}
		}

		if !viper.GetBool("server.strip_budget_headers") {
			resp.Header.Set("X-Loopers-Session-Steps", fmt.Sprintf("%d", stepsVal))
		}
	}

	if isStream {
		var totalPaid float64 = reservedCost

		resp.Body = proxy.NewSSEStreamReader(ctx, resp.Body, prov, inputPrice, outputPrice,
			func(cost float64) bool {
				delta := cost - totalPaid
				if delta <= 0 {
					return true
				}
				if s.redis.LeaseManager.TryAcquireFast(keyHash, delta) {
					totalPaid += delta
					return true
				}
				return false
			},
			func(actualCost float64, inTokens, outTokens int, forcedCut bool) {
				s.redis.LeaseManager.ReconcileSpend(ctx, keyHash, totalPaid, actualCost)
				s.checkBudgetOverdrawAsync(ctx, keyHash, provName, model)
				if sessionID != "" {
					sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
					if err := s.redis.GetUnderlyingClient().IncrBy(ctx, sessionSpendKey, budget.ToNano(actualCost-totalPaid)).Err(); err != nil {
						logging.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("failed to update session spend in redis")
					}
				}

				// Instrument stream metrics upon completion
				latency := time.Since(startTime)
				requestDuration.WithLabelValues(provName).Observe(latency.Seconds())
				requestsTotal.WithLabelValues(provName, model, "200").Inc()
				spendUSDTotal.WithLabelValues(provName, keyHash).Add(actualCost)
				tokensTotal.WithLabelValues(provName, "input").Add(float64(inTokens))
				tokensTotal.WithLabelValues(provName, "output").Add(float64(outTokens))

				// Trigger alerts
				if s.alerter != nil {
					go func() {
						statusMap, err := s.redis.GetBudgetStatus(detachedTraceContext(ctx), keyHash)
						if err == nil {
							limits := make(map[string]float64)
							spends := make(map[string]float64)
							for k, v := range statusMap {
								limits[k] = v.Limit
								spends[k] = v.CurrentSpend
							}
							s.alerter.TriggerThresholdAlerts(detachedTraceContext(ctx), reqID, keyHash, keyName, provName, spends, limits)
						}
					}()
				}
			})
	} else {
		limit := viper.GetInt64("server.max_response_bytes")
		if limit == 0 {
			limit = 32 * 1024 * 1024
		}
		respBodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, limit))
		if err != nil {
			return err
		}

		// Close original body to prevent connection and goroutine leaks in the ReverseProxy
		resp.Body.Close()

		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))

		var totalInputTokens int
		var totalOutputTokens int

		if prov != nil {
			totalInputTokens, totalOutputTokens, _ = prov.ParseNonStreamResponse(respBodyBytes)
		}

		actualCost := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
		s.redis.LeaseManager.ReconcileSpend(ctx, keyHash, reservedCost, actualCost)
		s.checkBudgetOverdrawAsync(ctx, keyHash, provName, model)
		if !viper.GetBool("server.strip_budget_headers") {
			resp.Header.Set("X-Loopers-Request-Cost", fmt.Sprintf("%.6f", actualCost))
		}

		if sessionID != "" {
			sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
			sessionStepsKey := fmt.Sprintf("loopers:session:{%s}:%s:steps", keyHash, sessionID)

			if err := s.redis.GetUnderlyingClient().IncrBy(ctx, sessionSpendKey, budget.ToNano(actualCost-reservedCost)).Err(); err != nil {
				logging.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("failed to update session spend in redis")
			}

			rdb := s.redis.GetUnderlyingClient()
			vals, _ := rdb.MGet(ctx, sessionStepsKey).Result()
			var stepsVal int64
			if len(vals) == 1 {
				if vals[0] != nil {
					if stVal, err := strconv.ParseInt(fmt.Sprintf("%v", vals[0]), 10, 64); err == nil {
						stepsVal = stVal
					}
				}
			}

			if !viper.GetBool("server.strip_budget_headers") {
				resp.Header.Set("X-Loopers-Session-Steps", fmt.Sprintf("%d", stepsVal))
			}
		}

		// Record non-stream metrics
		latency := time.Since(startTime)
		requestDuration.WithLabelValues(provName).Observe(latency.Seconds())
		requestsTotal.WithLabelValues(provName, model, "200").Inc()
		spendUSDTotal.WithLabelValues(provName, keyHash).Add(actualCost)
		tokensTotal.WithLabelValues(provName, "input").Add(float64(totalInputTokens))
		tokensTotal.WithLabelValues(provName, "output").Add(float64(totalOutputTokens))

		// Trigger alerts
		if s.alerter != nil {
			go func() {
				statusMap, err := s.redis.GetBudgetStatus(detachedTraceContext(ctx), keyHash)
				if err == nil {
					limits := make(map[string]float64)
					spends := make(map[string]float64)
					for k, v := range statusMap {
						limits[k] = v.Limit
						spends[k] = v.CurrentSpend
					}
					s.alerter.TriggerThresholdAlerts(detachedTraceContext(ctx), reqID, keyHash, keyName, provName, spends, limits)
				}
			}()
		}
	}

	return nil
}

func (s *Server) checkBudgetOverdrawAsync(ctx context.Context, keyHash, providerName, model string) {
	go func() {
		detached := detachedTraceContext(ctx)
		statusMap, err := s.redis.GetBudgetStatus(detached, keyHash)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("failed to check budget status async")
			return
		}

		for window, status := range statusMap {
			if status.Limit > 0 && status.CurrentSpend > status.Limit {
				event.EmitBlockEvent(detached, event.BlockEvent{
					EventType: "budget_overdrawn",
					KeyHash:   keyHash,
					Provider:  providerName,
					Model:     model,
					Reason:    "burst_overdraw",
					Detail:    fmt.Sprintf("Budget overdrawn in %s window: spend %.6f exceeded limit %.6f", window, status.CurrentSpend, status.Limit),
				})
				// Only emit one alert per check
				break
			}
		}
	}()
}
