package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/alerting"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/loop"
	"github.com/CURSED-ME/loopers-oss/internal/otel"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/provider/anthropic"
	"github.com/CURSED-ME/loopers-oss/internal/provider/azure"
	"github.com/CURSED-ME/loopers-oss/internal/provider/bedrock"
	"github.com/CURSED-ME/loopers-oss/internal/provider/cohere"
	"github.com/CURSED-ME/loopers-oss/internal/provider/deepseek"
	"github.com/CURSED-ME/loopers-oss/internal/provider/fireworks"
	"github.com/CURSED-ME/loopers-oss/internal/provider/gemini"
	"github.com/CURSED-ME/loopers-oss/internal/provider/generic"
	"github.com/CURSED-ME/loopers-oss/internal/provider/groq"
	"github.com/CURSED-ME/loopers-oss/internal/provider/mistral"
	"github.com/CURSED-ME/loopers-oss/internal/provider/ollama"
	"github.com/CURSED-ME/loopers-oss/internal/provider/openai"
	"github.com/CURSED-ME/loopers-oss/internal/provider/together"
	"github.com/CURSED-ME/loopers-oss/internal/provider/vllm"
	"github.com/CURSED-ME/loopers-oss/internal/provider/xai"
	"github.com/CURSED-ME/loopers-oss/internal/proxy"
	"github.com/CURSED-ME/loopers-oss/pkg/api"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_requests_total",
		Help: "Total number of AI requests processed by Loopers",
	}, []string{"provider", "model", "status"})

	budgetBlocksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_budget_blocks_total",
		Help: "Total number of AI requests blocked by Loopers budget enforcement",
	}, []string{"provider", "window"})

	loopBlocksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_loop_blocks_total",
		Help: "Total number of requests blocked by the loop detection engine",
	}, []string{"provider", "rule"})

	loopWarnsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_loop_warns_total",
		Help: "Total number of requests flagged with warnings by the loop detection engine",
	}, []string{"provider", "rule"})

	shadowBlockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_shadow_blocked_total",
		Help: "Total number of AI requests shadow blocked by Loopers budget enforcement",
	}, []string{"provider", "window"})

	spendUSDTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_spend_usd_total",
		Help: "Total spend in USD tracked by Loopers",
	}, []string{"provider", "key_hash"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "loopers_request_duration_seconds",
		Help:    "AI request duration in seconds",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
	}, []string{"provider"})

	tokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "loopers_tokens_total",
		Help: "Total number of tokens processed by Loopers",
	}, []string{"provider", "direction"})

	validSessionID = regexp.MustCompile(`^[a-zA-Z0-9._:@/-]{1,256}$`)
)

type serverContextKey string

const (
	isStreamCtx        serverContextKey = "IsStream"
	inputPriceCtx      serverContextKey = "InputPrice"
	outputPriceCtx     serverContextKey = "OutputPrice"
	inputTokensCtx     serverContextKey = "InputTokens"
	sessionIDCtx       serverContextKey = "SessionID"
	sessionBudgetCtx   serverContextKey = "SessionBudget"
	sessionMaxStepsCtx serverContextKey = "SessionMaxSteps"
	startTimeCtx       serverContextKey = "StartTime"
	keyNameCtx         serverContextKey = "KeyName"
	requestIDCtx       serverContextKey = "ProxyRequestID"
)

// Server coordinates the proxy operations and HTTP routing.
type Server struct {
	router       *gin.Engine
	redis        *budget.Client
	pricing      *pricing.Store
	proxy        *proxy.Proxy
	registry     *provider.Registry
	alerter      *alerting.Alerter
	loopDetector *loop.Detector
	shadowMode   bool
	proxyGroup   *gin.RouterGroup // exposed so tests can register routes with BodyBuffer applied
	otelEnabled  bool
	otelShutdown func(context.Context) error
}

// NewServer initializes and builds the HTTP server with middlewares and ReverseProxy configuration.
func NewServer(redisClient *budget.Client, pricingStore *pricing.Store) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	reg := provider.NewRegistry()
	reg.Register(openai.NewOpenAIProvider())
	reg.Register(anthropic.NewAnthropicProvider())
	reg.Register(gemini.NewGeminiProvider())
	reg.Register(bedrock.NewBedrockProvider())
	reg.Register(azure.NewAzureProvider())
	reg.Register(mistral.NewMistralProvider())
	reg.Register(groq.NewGroqProvider())
	reg.Register(cohere.NewCohereProvider())
	reg.Register(deepseek.NewDeepSeekProvider())
	reg.Register(together.NewTogetherProvider())
	reg.Register(ollama.NewOllamaProvider())
	reg.Register(fireworks.NewFireworksProvider())
	reg.Register(xai.NewXAIProvider())
	reg.Register(vllm.NewVLLMProvider())

	type GenericProviderConfig struct {
		Name    string `mapstructure:"name"`
		BaseURL string `mapstructure:"base_url"`
	}
	var genericProviders []GenericProviderConfig
	if err := viper.UnmarshalKey("generic_providers", &genericProviders); err == nil {
		// Pre-compile name validation: alphanumeric, dash, underscore only.
		// This prevents malformed Gin route patterns (G1).
		validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		for _, gp := range genericProviders {
			if gp.Name == "" || gp.BaseURL == "" {
				logging.Logger.Warn().Msgf("Skipping generic provider with empty name or base_url")
				continue
			}
			// G1: Reject names with special characters
			if !validName.MatchString(gp.Name) {
				logging.Logger.Error().Msgf("Invalid generic provider name %q: only alphanumeric, dash, and underscore are allowed", gp.Name)
				continue
			}
			// S1 & S3: Detect collision with built-in or previously registered provider
			if _, err := reg.Get(gp.Name); err == nil {
				logging.Logger.Error().Msgf("Generic provider %q conflicts with an already-registered provider; skipping to prevent shadowing", gp.Name)
				continue
			}
			reg.Register(generic.NewGenericProvider(gp.Name, gp.BaseURL))
			logging.Logger.Info().Msgf("Registered generic provider: %s (%s)", gp.Name, gp.BaseURL)
		}
	}

	var alertingCfg alerting.AlertingConfig
	var alerter *alerting.Alerter
	var loopDetector *loop.Detector
	if redisClient != nil {
		if err := viper.UnmarshalKey("alerting", &alertingCfg); err == nil {
			alerter = alerting.NewAlerter(alertingCfg, redisClient.GetUnderlyingClient())
		}

		var loopCfg loop.Config
		if err := viper.UnmarshalKey("loop_detection", &loopCfg); err == nil && loopCfg.Enabled {
			loopDetector = loop.NewDetector(loopCfg, redisClient.GetUnderlyingClient())
		}
	}

	shadowMode := viper.GetBool("server.shadow_mode")

	var otelCfg otel.Config
	var otelShutdown func(context.Context) error
	var otelEnabled bool
	if err := viper.UnmarshalKey("otel", &otelCfg); err == nil {
		otelEnabled = otelCfg.Enabled
		if shutdown, err := otel.Init(otelCfg, alerting.Version); err != nil {
			logging.Logger.Error().Err(err).Msg("failed to initialize OTel")
			otelEnabled = false
		} else {
			otelShutdown = shutdown
		}
	}

	s := &Server{
		router:       r,
		redis:        redisClient,
		pricing:      pricingStore,
		registry:     reg,
		alerter:      alerter,
		loopDetector: loopDetector,
		shadowMode:   shadowMode,
		otelEnabled:  otelEnabled,
		otelShutdown: otelShutdown,
	}

	// Setup custom ReverseProxy with modifyResponse callback
	s.proxy = proxy.NewProxy(s.modifyResponse)

	s.setupRoutes()
	return s
}

// GetOtelShutdown returns the shutdown function for OpenTelemetry.
func (s *Server) GetOtelShutdown() func(context.Context) error {
	return s.otelShutdown
}

func (s *Server) setupRoutes() {
	s.router.Use(RequestID())
	s.router.Use(Recovery())
	s.router.Use(RequestLogger())
	s.router.Use(MaxBytesReader(10 << 20)) // Wrap request body with 10MB limit
	s.router.Use(KeyExtractor())

	s.router.GET("/health", s.handleHealth)
	s.router.GET("/budget/status", s.handleBudgetStatus)

	s.proxyGroup = s.router.Group("/")

	maxInflight := viper.GetInt("server.max_inflight")
	if maxInflight <= 0 {
		maxInflight = 2000
	}
	s.proxyGroup.Use(ConcurrencyLimiter(maxInflight))
	s.proxyGroup.Use(otel.TraceMiddleware(s.otelEnabled))
	s.proxyGroup.Use(BodyBuffer())

	for _, p := range s.registry.All() {
		providerName := p.Name()
		s.proxyGroup.POST("/"+providerName+"/*path", func(c *gin.Context) {
			s.handleProxy(c, providerName)
		})
	}
}

// RegisterProviderRoute registers a provider and adds its route to the proxy group (with BodyBuffer applied).
// This is intended for use in tests only.
func (s *Server) RegisterProviderRoute(p provider.Provider) {
	s.registry.Register(p)
	providerName := p.Name()
	s.proxyGroup.POST("/"+providerName+"/*path", func(c *gin.Context) {
		s.handleProxy(c, providerName)
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	if err := s.redis.Ping(ctx); err != nil {
		logging.Logger.Error().Err(err).Msg("Health check failed: Redis unreachable")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Redis connection down",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (s *Server) handleBudgetStatus(c *gin.Context) {
	rawProxyKey, exists := c.Get("RawProxyKey")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	rawKeyStr := rawProxyKey.(string)
	if !keyring.ValidateLoopersKey(rawKeyStr) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid loopers key format"})
		return
	}

	keyHash := keyring.HashKey(rawKeyStr)
	meta, err := keyring.GetKeyMetadata(c.Request.Context(), s.redis.GetUnderlyingClient(), keyHash)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key not registered"})
		return
	}
	if meta.Active != "true" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key has been revoked"})
		return
	}

	status, err := s.redis.GetBudgetStatus(c.Request.Context(), keyHash)
	if err != nil {
		logging.Logger.Error().Err(err).Msg("Failed to query budget status")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to get budget status from cache"})
		return
	}

	c.JSON(http.StatusOK, status)
}

func (s *Server) handleProxy(c *gin.Context, providerName string) {
	startTime := time.Now()

	// 1. Auth check
	rawProxyKey, exists := c.Get("RawProxyKey")
	if !exists {
		reqID := c.GetString("RequestID")
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.otelEnabled {
			otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
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
		if s.otelEnabled {
			otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
		}
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
			if s.otelEnabled {
				otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
			}
			if s.alerter != nil {
				go s.alerter.TriggerAuthFail(detachedTraceContext(c.Request.Context()), reqID, "Key not registered")
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key not registered"})
		} else {
			logging.Logger.Error().Err(err).Msg("failed to get key metadata")
			requestsTotal.WithLabelValues(providerName, "unknown", "503").Inc()
			if s.otelEnabled {
				otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
			}
			if s.alerter != nil {
				go s.alerter.TriggerFailClosed(detachedTraceContext(c.Request.Context()), reqID, "Failed to get key metadata")
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable: internal error"})
		}
		return
	}
	if meta.Active != "true" {
		requestsTotal.WithLabelValues(providerName, "unknown", "401").Inc()
		if s.otelEnabled {
			otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
		}
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

	// 6. Check and reserve budget via Lease (fast path)
	err = s.redis.LeaseManager.Acquire(c.Request.Context(), keyHash, estimatedCost, 1.0)
	if err != nil {
		if err == budget.ErrBudgetExceeded {
			originalPath := c.Request.URL.Path
			fallbackModel := s.pricing.GetFallback(providerName, model)
			if fallbackModel != "" && fallbackModel != model {
				fallbackInputPrice, fallbackOutputPrice, _ := s.pricing.GetModelPrice(providerName, fallbackModel)
				fallbackMutatedBody, rewriteErr := prov.RewriteModel(c.Request, mutatedBody, fallbackModel)
				if rewriteErr == nil {
					fallbackInputTokens, countErr := prov.CountInputTokens(c.Request.Context(), fallbackModel, fallbackMutatedBody, providerKeyStr)
					if countErr == nil {
						fallbackCost := pricing.EstimateCost(fallbackInputTokens, maxTokensVal, fallbackInputPrice, fallbackOutputPrice)
						fallbackErr := s.redis.LeaseManager.Acquire(c.Request.Context(), keyHash, fallbackCost, 1.0)
						if fallbackErr == nil {
							logging.Logger.Info().Str("key_hash", keyHash).Str("original_model", model).Str("fallback_model", fallbackModel).Msg("fallback_routing_successful")
							estimatedCost = fallbackCost
							model = fallbackModel
							inputPrice = fallbackInputPrice
							outputPrice = fallbackOutputPrice
							inputTokens = fallbackInputTokens
							mutatedBody = fallbackMutatedBody
							c.Request.Body = io.NopCloser(bytes.NewReader(mutatedBody))
							c.Request.ContentLength = int64(len(mutatedBody))
							c.Writer.Header().Set("X-Loopers-Fallback", fallbackModel)
							goto sessionCheck
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

				// In shadow mode, we didn't reserve the budget (it rolled back), so we set estimatedCost to 0
				// so that Reconcile() later adds the full actual cost to the current spend.
				estimatedCost = 0
				goto sessionCheck
			} else {
				if s.otelEnabled {
					otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
				}
				if s.alerter != nil {
					go s.alerter.TriggerBlockAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, model, "budget", 0, 0, estimatedCost)
				}
				requestsTotal.WithLabelValues(providerName, model, "429").Inc()

				// We don't have the exact window in the new simple ErrBudgetExceeded, so just log generically
				logging.Logger.Warn().Str("key_hash", keyHash).Float64("cost", estimatedCost).Msg("Budget exceeded")

				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "budget exceeded",
					"type":  "budget_exceeded",
				})
				return
			}
		}
		logging.Logger.Error().Err(err).Msg("Budget check failed closed due to backend connection error")
		requestsTotal.WithLabelValues(providerName, model, "503").Inc()
		if s.otelEnabled {
			otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
		}
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
		return
	}

sessionCheck:
	// 6.5 Session budget and step limits check (if X-Loopers-Session-ID is present)
	sessionID := c.GetHeader("X-Loopers-Session-ID")
	if sessionID != "" {
		if !validSessionID.MatchString(sessionID) {
			requestsTotal.WithLabelValues(providerName, model, "400").Inc()
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID format"})
			return
		}
	}
	var sessionBudget float64
	var sessionMaxSteps int
	if sessionID != "" {
		if budgetHeader := c.GetHeader("X-Loopers-Session-Budget"); budgetHeader != "" {
			sessionBudget, _ = strconv.ParseFloat(budgetHeader, 64)
		}
		if stepsHeader := c.GetHeader("X-Loopers-Session-Max-Steps"); stepsHeader != "" {
			sessionMaxSteps, _ = strconv.Atoi(stepsHeader)
		}

		// Check session limits (1 hour TTL)
		// For sessions, we fallback to strict checking for now
		allowed, val1, val2, status, err := s.redis.CheckAndReserveSession(c.Request.Context(), sessionID, estimatedCost, sessionBudget, sessionMaxSteps, 3600)
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
			return
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
				sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
				sessionStepsKey := fmt.Sprintf("loopers:session:%s:steps", sessionID)
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
				return
			}
		}
	}

	// 6.6 Loop detection (deterministic, rule-based)
	if s.loopDetector != nil && sessionID != "" {
		result, err := s.loopDetector.Check(c.Request.Context(), sessionID, providerName+c.Request.URL.Path, body)
		if err != nil {
			// Log but never block on internal error — loop detection failure is not a fail-closed event
			logging.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("loop_detection_check_failed")
		} else if result != nil && result.Detected {
			if result.ShouldBlock {
				loopBlocksTotal.WithLabelValues(providerName, result.Rule).Inc()

				// Refund key budget reservation
				s.redis.LeaseManager.ReconcileSpend(c.Request.Context(), keyHash, estimatedCost, 0)
				sessionSpendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
				_, _ = s.redis.GetUnderlyingClient().IncrByFloat(c.Request.Context(), sessionSpendKey, -estimatedCost).Result()

				logging.Logger.Warn().
					Str("session_id", sessionID).
					Str("rule", result.Rule).
					Str("detail", result.Detail).
					Msg("loop_detected_blocked")
				requestsTotal.WithLabelValues(providerName, model, "429").Inc()

				if s.otelEnabled {
					otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
				}
				if s.alerter != nil {
					go s.alerter.TriggerLoopAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, sessionID, result.Rule, result.Detail, true)
				}

				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":      "loop detected",
					"type":       "loop_detected",
					"rule":       result.Rule,
					"session_id": sessionID,
				})
				return
			}

			// ShouldBlock=false: warn-only mode (stall detection default)
			loopWarnsTotal.WithLabelValues(providerName, result.Rule).Inc()
			if s.otelEnabled {
				otel.PromoteToSampled(trace.SpanFromContext(c.Request.Context()))
			}
			logging.Logger.Warn().
				Str("session_id", sessionID).
				Str("rule", result.Rule).
				Str("detail", result.Detail).
				Msg("loop_detected_warn_only")

			if s.alerter != nil {
				go s.alerter.TriggerLoopAlert(detachedTraceContext(c.Request.Context()), reqID, keyHash, meta.Name, providerName, sessionID, result.Rule, result.Detail, false)
			}
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

// GetRouter retrieves the Gin engine.
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// GetAdminRouter retrieves the Gin engine for the admin port.
func (s *Server) GetAdminRouter() *gin.Engine {
	adminRouter := gin.New()
	adminRouter.Use(gin.Recovery())
	adminRouter.GET("/health", s.handleHealth)
	adminRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
	return adminRouter
}

// GetRegistry retrieves the provider registry. Primarily used for testing.
func (s *Server) GetRegistry() *provider.Registry {
	return s.registry
}

// detachedTraceContext creates a new background context but carries over the trace span context.
// This is used for firing alerts in goroutines without risking cancellation of the original context.
func detachedTraceContext(ctx context.Context) context.Context {
	return trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))
}
