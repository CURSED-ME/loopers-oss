package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/CURSED-ME/loopers-oss/internal/alerting"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	proxyPkg "github.com/CURSED-ME/loopers-oss/internal/proxy"
	"github.com/CURSED-ME/loopers-oss/internal/session"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg            Config
	budgetClient   *budget.Client
	pricingStore   *pricing.Store
	alerter        *alerting.Alerter
	circuitBreaker *CircuitBreaker
	sessionManager *session.Manager
	proxy          *Proxy
	servers        map[string]string
	allowedMethods map[string]bool
}

func NewHandler(cfg Config, budgetClient *budget.Client, pricingStore *pricing.Store, alerter *alerting.Alerter, sessionManager *session.Manager) *Handler {
	cb := NewCircuitBreaker(cfg.CircuitBreaker, budgetClient.GetUnderlyingClient())

	servers := make(map[string]string)
	for _, srv := range cfg.Servers {
		servers[srv.Name] = srv.URL
	}

	allowedMethods := make(map[string]bool)
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"tools/list", "tools/call", "initialize", "ping", "notifications/initialized"}
	}
	for _, m := range cfg.AllowedMethods {
		allowedMethods[m] = true
	}

	h := &Handler{
		cfg:            cfg,
		budgetClient:   budgetClient,
		pricingStore:   pricingStore,
		alerter:        alerter,
		circuitBreaker: cb,
		sessionManager: sessionManager,
		servers:        servers,
		allowedMethods: allowedMethods,
	}

	h.proxy = NewProxy(h.modifyResponse)
	return h
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	req := resp.Request
	ctx := req.Context()
	type contextKey string
	const mcpMethodCtx contextKey = "MCPMethod"
	const mcpServerCtx contextKey = "MCPServer"

	mcpMethod, _ := ctx.Value(mcpMethodCtx).(string)
	serverName, _ := ctx.Value(mcpServerCtx).(string)
	if mcpMethod == "tools/list" && resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			sanitized, _ := SanitizeToolList(body, h.cfg.Sanitizer, serverName)
			resp.Body = io.NopCloser(bytes.NewReader(sanitized))
			resp.ContentLength = int64(len(sanitized))
			resp.Header.Set("Content-Length", strconv.Itoa(len(sanitized)))
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	reservedCost, hasCost := ctx.Value(proxyPkg.RequestCostCtx).(float64)
	keyHash, hasKey := ctx.Value(proxyPkg.ProxyKeyHashCtx).(string)

	if hasCost && hasKey {
		if resp.StatusCode != http.StatusOK {
			// Refund cost on failure
			h.budgetClient.LeaseManager.ReconcileSpend(ctx, keyHash, reservedCost, 0)
			return nil
		}

		// Tool call successful, commit the spend
		h.budgetClient.LeaseManager.ReconcileSpend(ctx, keyHash, reservedCost, reservedCost)
		resp.Header.Set("X-Loopers-Tool-Cost", fmt.Sprintf("%.6f", reservedCost))
	}

	return nil
}

func (h *Handler) HandleMCP(c *gin.Context) {
	serverName := c.Param("server")
	targetURL, ok := h.servers[serverName]
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("MCP server '%s' not configured", serverName)})
		return
	}

	// Auth check using the same lp-xxx key system
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
	meta, err := keyring.GetKeyMetadata(c.Request.Context(), h.budgetClient.GetUnderlyingClient(), keyHash)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key not registered"})
		return
	}
	if meta.Active != "true" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: key has been revoked"})
		return
	}

	// Parse request body for JSON-RPC
	// The body is available via the BodyBuffer middleware context key
	// Wait, internal/server/server.go uses RequestBodyCtx which is a constant in server package.
	// We might need to export it or fetch from context by string. It's usually "RequestBody".
	bodyBytes, exists := c.Get("RequestBody")
	if !exists {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "RequestBody missing"})
		return
	}
	body, _ := bodyBytes.([]byte)

	if h.cfg.MaxRequestSize > 0 && int64(len(body)) > h.cfg.MaxRequestSize {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "MCP request body exceeds maximum allowed size"})
		return
	}

	req, err := ParseJSONRPC(body)
	if err != nil || req == nil {
		// Pass-through transparently if not JSON-RPC
		h.forward(c, targetURL, nil, 0)
		return
	}

	if !h.allowedMethods[req.Method] {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("MCP method '%s' is not allowed", req.Method),
			"type":  "mcp_method_not_allowed",
		})
		return
	}

	type contextKey string
	const mcpMethodCtx contextKey = "MCPMethod"
	const mcpServerCtx contextKey = "MCPServer"

	if req.Method != "tools/call" {
		if req.Method == "tools/list" {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), mcpMethodCtx, "tools/list"))
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), mcpServerCtx, serverName))
		// Pass-through transparently for allowed non-tools/call methods
		h.forward(c, targetURL, nil, 0)
		return
	}

	toolParams, err := ParseToolCallParams(req.Params)
	if err != nil || toolParams == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid tools/call params"})
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("gen_ai.system.mcp.tool", toolParams.Name))

	// 1. Circuit Breaker Check
	sessionID := c.GetHeader("X-Loopers-Session-ID")
	if sessionID != "" && h.cfg.CircuitBreaker.Enabled {
		if !session.IsValidID(sessionID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID format"})
			return
		}
		span.SetAttributes(attribute.String("gen_ai.system.session.id", sessionID))
		maxTotalTools := 0
		if maxToolsHeader := c.GetHeader("X-Loopers-Session-Max-Tools"); maxToolsHeader != "" {
			if parsed, err := strconv.Atoi(maxToolsHeader); err == nil && parsed > 0 {
				maxTotalTools = parsed
			}
		}

		cbRes, err := h.circuitBreaker.Check(c.Request.Context(), sessionID, toolParams.Name, toolParams.Arguments, maxTotalTools)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("MCP circuit breaker check failed")
		} else if cbRes.TotalTripped {
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "circuit_breaker_trip",
				SessionID: sessionID,
				Provider:  serverName,
				ToolName:  toolParams.Name,
				Reason:    "mcp_total_tools_exceeded",
				Detail:    "Total tool invocation limit exceeded",
			})
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "total tool invocation limit exceeded",
				"type":  "mcp_total_tools_exceeded",
			})
			return
		} else if cbRes.Tripped {
			event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
				EventType: "circuit_breaker_trip",
				SessionID: sessionID,
				Provider:  serverName,
				ToolName:  toolParams.Name,
				Reason:    "mcp_circuit_breaker",
				Detail:    "Tool-call circuit breaker tripped",
			})
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "tool-call circuit breaker tripped",
				"type":  "mcp_circuit_breaker",
				"tool":  toolParams.Name,
			})
			return
		}
	}

	// 2. Blast Radius Check
	if sessionID != "" && h.sessionManager != nil {
		maxServers := 0
		if maxServersHeader := c.GetHeader("X-Loopers-Session-Max-Servers"); maxServersHeader != "" {
			if parsed, err := strconv.Atoi(maxServersHeader); err == nil && parsed > 0 {
				maxServers = parsed
			}
		}
		if maxServers > 0 {
			allowed, err := h.sessionManager.CheckBlastRadius(c.Request.Context(), sessionID, serverName, maxServers)
			if err != nil {
				logging.Logger.Error().Err(err).Msg("MCP blast radius check failed")
			} else if !allowed {
				event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
					EventType: "blast_radius_block",
					SessionID: sessionID,
					Provider:  serverName, // in MCP context we use provider for serverName
					Reason:    "blast_radius_exceeded",
					Detail:    "Maximum number of distinct servers exceeded",
				})
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "maximum number of distinct servers exceeded",
					"type":  "mcp_blast_radius_exceeded",
				})
				return
			}
		}
	}

	// 3. Budget Check
	toolCost := h.pricingStore.GetToolCost(toolParams.Name)
	if toolCost > 0 {
		err = h.budgetClient.LeaseManager.Acquire(c.Request.Context(), keyHash, toolCost, 1.0)
		if err != nil {
			if errors.Is(err, budget.ErrBudgetExceeded) {
				event.EmitBlockEvent(c.Request.Context(), event.BlockEvent{
					EventType: "budget_block",
					KeyHash:   keyHash,
					Provider:  serverName,
					ToolName:  toolParams.Name,
					Reason:    "budget_exceeded",
					Detail:    "MCP tool invocation budget exceeded",
				})
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "budget exceeded",
					"type":  "mcp_budget_exceeded",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable"})
			return
		}
	}

	// Forward the request and restore body
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	ctx := c.Request.Context()
	tracer := otel.Tracer("loopers-proxy")
	ctx, toolSpan := tracer.Start(ctx, "loopers.mcp.tool_call", trace.WithSpanKind(trace.SpanKindClient))
	toolSpan.SetAttributes(
		attribute.String("mcp.tool.name", toolParams.Name),
		attribute.String("mcp.server.name", serverName),
		attribute.Float64("mcp.tool.cost", toolCost),
		attribute.String("loopers.enforcement.action", "allowed"),
	)
	defer toolSpan.End()

	c.Request = c.Request.WithContext(ctx)

	h.forward(c, targetURL, &keyHash, toolCost)
}

func (h *Handler) forward(c *gin.Context, targetURL string, keyHash *string, cost float64) {
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, MCPTargetCtx, targetURL)
	if keyHash != nil {
		ctx = context.WithValue(ctx, proxyPkg.ProxyKeyHashCtx, *keyHash)
		ctx = context.WithValue(ctx, proxyPkg.RequestCostCtx, cost)
	}
	c.Request = c.Request.WithContext(ctx)

	// Strip the /mcp/:server prefix before forwarding if needed
	// Example: /mcp/filesystem/tools/call -> /tools/call
	path := c.Request.URL.Path
	prefix := fmt.Sprintf("/mcp/%s", c.Param("server"))
	c.Request.URL.Path = strings.TrimPrefix(path, prefix)
	if c.Request.URL.RawPath != "" {
		c.Request.URL.RawPath = strings.TrimPrefix(c.Request.URL.RawPath, prefix)
	}

	h.proxy.ServeHTTP(c.Writer, c.Request)
}
