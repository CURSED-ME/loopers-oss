package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/a2a"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/event"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/pkg/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) enforceBudgetWithFallback(c *gin.Context, providerName, model string, estimatedCost, inputPrice, outputPrice float64, inputTokens, maxTokensVal int, mutatedBody []byte, providerKeyStr, keyHash string, meta *keyring.KeyMetadata, reqID string) (
	newModel string, newCost, newInputPrice, newOutputPrice float64, newInputTokens int, newMutatedBody []byte, err error,
) {
	newModel = model
	newCost = estimatedCost
	newInputPrice = inputPrice
	newOutputPrice = outputPrice
	newInputTokens = inputTokens
	newMutatedBody = mutatedBody

	err = s.redis.LeaseManager.Acquire(c.Request.Context(), keyHash, estimatedCost, 1.0)
	if err != nil {
		if err == budget.ErrBudgetExceeded {
			originalPath := c.Request.URL.Path
			fallbackModel := s.pricing.GetFallback(providerName, model)
			if fallbackModel != "" && fallbackModel != model {
				prov, _ := s.registry.Get(providerName)
				fallbackInputPrice, fallbackOutputPrice, _ := s.pricing.GetModelPrice(providerName, fallbackModel)
				fallbackMutatedBody, rewriteErr := prov.RewriteModel(c.Request, mutatedBody, fallbackModel)
				if rewriteErr == nil {
					fallbackInputTokens, countErr := prov.CountInputTokens(c.Request.Context(), fallbackModel, fallbackMutatedBody, providerKeyStr)
					if countErr == nil {
						fallbackCost := pricing.EstimateCost(fallbackInputTokens, maxTokensVal, fallbackInputPrice, fallbackOutputPrice)
						fallbackErr := s.redis.LeaseManager.Acquire(c.Request.Context(), keyHash, fallbackCost, 1.0)
						if fallbackErr == nil {
							logging.Logger.Info().Str("key_hash", keyHash).Str("original_model", model).Str("fallback_model", fallbackModel).Msg("fallback_routing_successful")
							newCost = fallbackCost
							newModel = fallbackModel
							newInputPrice = fallbackInputPrice
							newOutputPrice = fallbackOutputPrice
							newInputTokens = fallbackInputTokens
							newMutatedBody = fallbackMutatedBody
							c.Request.Body = io.NopCloser(bytes.NewReader(newMutatedBody))
							c.Request.ContentLength = int64(len(newMutatedBody))
							c.Writer.Header().Set("X-Loopers-Fallback", fallbackModel)
							return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, nil
						}
					}
				}
				// Restore original URL path if fallback failed or was rejected
				c.Request.URL.Path = originalPath
			}

			if s.shadowMode {
				logging.Logger.Warn().
					Str("key_hash", keyHash).
					Float64("cost", estimatedCost).
					Msg("shadow_blocked")
				shadowBlockedTotal.WithLabelValues(providerName, "budget").Inc()

				// In shadow mode, we didn't reserve the budget (it rolled back), so we set newCost to 0
				// so that Reconcile() later adds the full actual cost to the current spend.
				newCost = 0
				return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, nil
			} else {
				if s.escalationBroker != nil {
					sessionID := c.GetHeader("X-Loopers-Session-ID")
					if sessionID == "" {
						sessionID = reqID
					}
					req := a2a.EscalationRequest{
						SessionID: sessionID,
						Reason:    "budget_exceeded",
						AgentName: meta.AgentName,
					}
					resp, escErr := s.escalationBroker.RequestEscalation(c.Request.Context(), req, 5*time.Second)
					if escErr == nil && resp.Approved {
						escalationsApprovedTotal.WithLabelValues("budget_exceeded").Inc()
						logging.Logger.Info().Str("session_id", sessionID).Msg("budget block escalated and approved")
						return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, nil
					} else if escErr != nil {
						escalationsTimeoutTotal.WithLabelValues("budget_exceeded").Inc()
						logging.Logger.Warn().Err(escErr).Str("session_id", sessionID).Msg("escalation request timed out or failed")
					}
				}

				event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
					EventType: "budget_block",
					KeyHash:   keyHash,
					Provider:  providerName,
					Model:     model,
					Reason:    "budget_exceeded",
					Detail:    fmt.Sprintf("Budget reserved: %f", estimatedCost),
					RequestID: reqID,
				})
				if s.alerter != nil {
					go s.alerter.TriggerBlockAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, model, "budget", 0, 0, estimatedCost)
				}
				requestsTotal.WithLabelValues(providerName, model, "429").Inc()

				// We don't have the exact window in the new simple ErrBudgetExceeded, so just log generically
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "budget exceeded",
					"type":  "budget_exceeded",
				})
				return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, budget.ErrBudgetExceeded
			}
		}
		logging.Logger.Error().Err(err).Msg("Budget check failed closed due to backend connection error")
		requestsTotal.WithLabelValues(providerName, model, "503").Inc()
		if s.alerter != nil {
			go s.alerter.TriggerFailClosed(detachedTraceContext(c.Request.Context()), reqID, "Budget check backend connection error")
		}
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Budget enforcement service is temporarily unavailable (fail-closed)",
				"type":    "service_unavailable_error",
				"code":    "service_unavailable",
			},
		})
		return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, err
	}
	return newModel, newCost, newInputPrice, newOutputPrice, newInputTokens, newMutatedBody, nil
}

func (s *Server) enforceSessionLimits(c *gin.Context, providerName, model, sessionID string, sessionBudget float64, sessionMaxSteps int, estimatedCost float64, keyHash string, meta *keyring.KeyMetadata, reqID string) error {
	allowed, val1, val2, status, err := s.redis.CheckAndReserveSession(c.Request.Context(), keyHash, sessionID, estimatedCost, sessionBudget, sessionMaxSteps, 3600)
	if err != nil {
		// Refund key budget reservation
		s.redis.LeaseManager.ReconcileSpend(c.Request.Context(), keyHash, estimatedCost, 0)
		logging.Logger.Error().Err(err).Msg("failed_session_reserve")
		requestsTotal.WithLabelValues(providerName, model, "503").Inc()
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Session enforcement service is temporarily unavailable (fail-closed)",
				"type":    "service_unavailable_error",
				"code":    "service_unavailable",
			},
		})
		return err
	}

	if !allowed {
		windowName := "session_budget"
		if status == "session_steps_exceeded" {
			windowName = "session_steps"
		}

		if s.shadowMode {
			logging.Logger.Warn().
				Str("key_hash", keyHash).
				Str("session_id", sessionID).
				Str("window", windowName).
				Float64("cost", estimatedCost).
				Float64("limit", val2).
				Float64("current_spend", val1).
				Msg("shadow_blocked")
			shadowBlockedTotal.WithLabelValues(providerName, windowName).Inc()

			// Manually commit the session reservation since the script blocked it
			sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
			sessionStepsKey := fmt.Sprintf("loopers:session:{%s}:%s:steps", keyHash, sessionID)
			rdb := s.redis.GetUnderlyingClient()
			rdb.IncrByFloat(c.Request.Context(), sessionSpendKey, estimatedCost)
			rdb.IncrBy(c.Request.Context(), sessionStepsKey, 1)
		} else {
			// Refund key budget reservation
			s.redis.LeaseManager.ReconcileSpend(c.Request.Context(), keyHash, estimatedCost, 0)

			if s.alerter != nil {
				go s.alerter.TriggerBlockAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, model, windowName, val1, val2, estimatedCost)
			}

			requestsTotal.WithLabelValues(providerName, model, "429").Inc()
			budgetBlocksTotal.WithLabelValues(providerName, windowName).Inc()

			if status == "session_budget_exceeded" {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, api.NewSessionBudgetExceededResponse(sessionID, val2, val1, estimatedCost))
			} else {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, api.NewSessionStepsExceededResponse(sessionID, int(val2), int(val1)))
			}
			return fmt.Errorf("session limit exceeded")
		}
	}
	return nil
}

func (s *Server) enforceLoopDetection(c *gin.Context, providerName, model, sessionID string, body []byte, estimatedCost float64, keyHash string, meta *keyring.KeyMetadata, reqID string) error {
	if s.loopDetector == nil {
		return nil
	}

	result, err := s.loopDetector.Check(c.Request.Context(), sessionID, providerName+c.Request.URL.Path, body)
	if err != nil {
		logging.Logger.Error().Err(err).Str("session_id", sessionID).Msg("loop_detection_check_failed")

		// Refund key budget reservation
		s.redis.LeaseManager.ReconcileSpend(c.Request.Context(), keyHash, estimatedCost, 0)
		sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
		_, _ = s.redis.GetUnderlyingClient().IncrByFloat(c.Request.Context(), sessionSpendKey, -estimatedCost).Result()

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Internal loop detection error",
				"type":    "internal_server_error",
			},
		})
		return err
	} else if result != nil && result.Detected {
		if result.ShouldBlock {
			loopBlocksTotal.WithLabelValues(providerName, result.Rule).Inc()

			// Refund key budget reservation
			s.redis.LeaseManager.ReconcileSpend(c.Request.Context(), keyHash, estimatedCost, 0)
			sessionSpendKey := fmt.Sprintf("loopers:session:{%s}:%s:spend", keyHash, sessionID)
			_, _ = s.redis.GetUnderlyingClient().IncrByFloat(c.Request.Context(), sessionSpendKey, -estimatedCost).Result()

			logging.Logger.Warn().
				Str("session_id", sessionID).
				Str("rule", result.Rule).
				Str("detail", result.Detail).
				Msg("loop_detected_blocked")
			requestsTotal.WithLabelValues(providerName, model, "429").Inc()

			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "loop_block",
				KeyHash:   keyHash,
				Provider:  providerName,
				Model:     model,
				Reason:    "loop_detected",
				Detail:    fmt.Sprintf("Rule: %s, Detail: %s", result.Rule, result.Detail),
				RequestID: reqID,
				SessionID: sessionID,
			})
			if s.alerter != nil {
				go s.alerter.TriggerLoopAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, sessionID, result.Rule, result.Detail, true)
			}

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "loop detected",
				"type":       "loop_detected",
				"rule":       result.Rule,
				"session_id": sessionID,
				"detail":     result.Detail,
			})
			return fmt.Errorf("loop detected")
		}

		// ShouldBlock=false: warn-only mode (stall detection default)
		loopWarnsTotal.WithLabelValues(providerName, result.Rule).Inc()
		logging.Logger.Warn().
			Str("session_id", sessionID).
			Str("rule", result.Rule).
			Str("detail", result.Detail).
			Msg("loop_detected_warn_only")

		if s.alerter != nil {
			go s.alerter.TriggerLoopAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, sessionID, result.Rule, result.Detail, false)
		}
	}
	return nil
}
