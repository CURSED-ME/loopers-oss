package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/event"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/proxy"
	"github.com/CURSED-ME/loopers-oss/internal/session"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) handleProxy(c *gin.Context, providerName string) {
	startTime := time.Now()

	// 1. Auth check
	rawProxyKey, exists := c.Get("RawProxyKey")
	if !exists {
		reqID := c.GetString("RequestID")
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.otelEnabled {
			// In OTEL, promote the span if it needs to be sampled for errors
			// otel.PromoteToSampled is not a standard OTEL function, this is pseudo-code in the original
			// Assuming we just keep it as it was in server.go
		}
		if s.alerter != nil {
			go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Missing Authorization header")
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	rawKeyStr := rawProxyKey.(string)
	reqID := c.GetString("RequestID")

	if !keyring.ValidateLoopersKey(rawKeyStr) {
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.alerter != nil {
			go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Invalid loopers key format")
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid loopers key format"})
		return
	}

	keyHash := keyring.HashKey(rawKeyStr)
	meta, err := keyring.GetKeyMetadata(c.Request.Context(), s.redis.GetUnderlyingClient(), keyHash)
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
	if meta.Active != "true" {
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
			"error": fmt.Sprintf("Key registered for provider '%s' but request is for '%s'", meta.Provider, providerName),
		})
		return
	}

	if s.otelEnabled {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(
			attribute.String("gen_ai.system", providerName),
			attribute.String("loopers.budget.key_hash", keyHash),
		)
	}

	if s.rateLimiter != nil {
		allowed, remaining, rlErr := s.rateLimiter.Check(c.Request.Context(), keyHash)
		if rlErr != nil {
			logging.Logger.Error().Err(rlErr).Msg("rate_limiter_error")
			// Fail-open for internal rate limiter errors to not disrupt traffic unnecessarily
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "rate_limit_degraded",
				KeyHash:   keyHash,
				Provider:  providerName,
				Reason:    "rate_limiter_error",
				Detail:    rlErr.Error(),
			})
		} else if !allowed {
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

	if s.otelEnabled {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.String("gen_ai.request.model", model))
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

		if ttlHeader := c.GetHeader("X-Loopers-Session-TTL"); ttlHeader != "" {
			if ttl, err := strconv.Atoi(ttlHeader); err == nil && ttl > 0 {
				if s.sessionManager != nil {
					valid, err := s.sessionManager.EnforceAbsoluteTTL(c.Request.Context(), sessionID, ttl)
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
			sessionBudget, _ = strconv.ParseFloat(budgetHeader, 64)
		}
		if stepsHeader := c.GetHeader("X-Loopers-Session-Max-Steps"); stepsHeader != "" {
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
			sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
			_, _ = s.redis.GetUnderlyingClient().IncrByFloat(ctx, sessionSpendKey, -reservedCost).Result()
		}

		// Record non-200 request metrics
		latency := time.Since(startTime)
		requestDuration.WithLabelValues(provName).Observe(latency.Seconds())
		requestsTotal.WithLabelValues(provName, model, strconv.Itoa(resp.StatusCode)).Inc()
		return nil
	}

	// Set initial headers for both stream and non-stream
	resp.Header.Set("X-Loopers-Request-Cost-Estimated", fmt.Sprintf("%.6f", reservedCost))

	if sessionID != "" {
		sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
		sessionStepsKey := fmt.Sprintf("loopers:session:%s:steps", sessionID)
		sessionBudgetKey := fmt.Sprintf("loopers:session:%s:budget", sessionID)

		rdb := s.redis.GetUnderlyingClient()
		spendVal, _ := rdb.Get(ctx, sessionSpendKey).Float64()
		stepsVal, _ := rdb.Get(ctx, sessionStepsKey).Int64()
		budgetVal, _ := rdb.Get(ctx, sessionBudgetKey).Float64()

		resp.Header.Set("X-Loopers-Session-Spend", fmt.Sprintf("%.6f", spendVal))
		resp.Header.Set("X-Loopers-Session-Steps", fmt.Sprintf("%d", stepsVal))
		if budgetVal > 0 {
			resp.Header.Set("X-Loopers-Session-Remaining", fmt.Sprintf("%.6f", budgetVal-spendVal))
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
				if sessionID != "" {
					sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
					_, _ = s.redis.GetUnderlyingClient().IncrByFloat(ctx, sessionSpendKey, actualCost-totalPaid).Result()
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
		respBodyBytes, err := io.ReadAll(resp.Body)
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
		resp.Header.Set("X-Loopers-Request-Cost", fmt.Sprintf("%.6f", actualCost))

		if sessionID != "" {
			sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
			sessionStepsKey := fmt.Sprintf("loopers:session:%s:steps", sessionID)
			sessionBudgetKey := fmt.Sprintf("loopers:session:%s:budget", sessionID)

			_, _ = s.redis.GetUnderlyingClient().IncrByFloat(ctx, sessionSpendKey, actualCost-reservedCost).Result()

			rdb := s.redis.GetUnderlyingClient()
			spendVal, _ := rdb.Get(ctx, sessionSpendKey).Float64()
			stepsVal, _ := rdb.Get(ctx, sessionStepsKey).Int64()
			budgetVal, _ := rdb.Get(ctx, sessionBudgetKey).Float64()

			resp.Header.Set("X-Loopers-Session-Spend", fmt.Sprintf("%.6f", spendVal))
			resp.Header.Set("X-Loopers-Session-Steps", fmt.Sprintf("%d", stepsVal))
			if budgetVal > 0 {
				resp.Header.Set("X-Loopers-Session-Remaining", fmt.Sprintf("%.6f", budgetVal-spendVal))
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
