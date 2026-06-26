package server

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/alerting"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/loop"
	"github.com/CURSED-ME/loopers-oss/internal/mcp"
	"github.com/CURSED-ME/loopers-oss/internal/otel"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/ratelimit"
	"github.com/CURSED-ME/loopers-oss/internal/session"
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
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
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
	mcpHandler     *mcp.Handler
	rateLimiter    *ratelimit.Limiter
	sessionManager *session.Manager
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
	var rateLimiter *ratelimit.Limiter
	var sessionManager *session.Manager
	if redisClient != nil {
		if err := viper.UnmarshalKey("alerting", &alertingCfg); err == nil {
			alerter = alerting.NewAlerter(alertingCfg, redisClient.GetUnderlyingClient())
		}

		var loopCfg loop.Config
		if err := viper.UnmarshalKey("loop_detection", &loopCfg); err == nil && loopCfg.Enabled {
			loopDetector = loop.NewDetector(loopCfg, redisClient.GetUnderlyingClient())
		}
		
		var rlCfg ratelimit.Config
		if err := viper.UnmarshalKey("rate_limit", &rlCfg); err == nil && rlCfg.Enabled {
			rateLimiter = ratelimit.NewLimiter(rlCfg, redisClient.GetUnderlyingClient())
		}

		sessionManager = session.NewManager(redisClient.GetUnderlyingClient())
	}

	var mcpCfg mcp.Config
	var mcpHandler *mcp.Handler
	if err := viper.UnmarshalKey("mcp", &mcpCfg); err == nil && mcpCfg.Enabled {
		if redisClient != nil {
			mcpHandler = mcp.NewHandler(mcpCfg, redisClient, pricingStore, alerter, sessionManager)
		} else {
			logging.Logger.Error().Msg("MCP is enabled but Redis client is not initialized. MCP routing will be disabled.")
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
		otelEnabled:    otelEnabled,
		otelShutdown:   otelShutdown,
		mcpHandler:     mcpHandler,
		rateLimiter:    rateLimiter,
		sessionManager: sessionManager,
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

	if s.mcpHandler != nil {
		mcpGroup := s.router.Group("/mcp")
		mcpGroup.Use(ConcurrencyLimiter(maxInflight))
		mcpGroup.Use(otel.TraceMiddleware(s.otelEnabled))
		mcpGroup.Use(BodyBuffer())
		mcpGroup.Any("/:server/*path", func(c *gin.Context) {
			s.mcpHandler.HandleMCP(c)
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

