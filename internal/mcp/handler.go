package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/alerting"
	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	proxyPkg "github.com/CURSED-ME/loopers-oss/internal/proxy"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg            Config
	budgetClient   *budget.Client
	pricingStore   *pricing.Store
	alerter        *alerting.Alerter
	circuitBreaker *CircuitBreaker
	proxy          *Proxy
	servers        map[string]string
}

func NewHandler(cfg Config, budgetClient *budget.Client, pricingStore *pricing.Store, alerter *alerting.Alerter) *Handler {
	cb := NewCircuitBreaker(cfg.CircuitBreaker, budgetClient.GetUnderlyingClient())

	servers := make(map[string]string)
	for _, srv := range cfg.Servers {
		servers[srv.Name] = srv.URL
	}

	h := &Handler{
		cfg:            cfg,
		budgetClient:   budgetClient,
		pricingStore:   pricingStore,
		alerter:        alerter,
		circuitBreaker: cb,
		servers:        servers,
	}

	h.proxy = NewProxy(h.modifyResponse)
	return h
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	req := resp.Request
	ctx := req.Context()

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

	req, err := ParseJSONRPC(body)
	if err != nil || req == nil {
		// Pass-through transparently if not JSON-RPC
		h.forward(c, targetURL, nil, 0)
		return
	}

	if req.Method != "tools/call" {
		// Pass-through transparently for other MCP methods
		h.forward(c, targetURL, nil, 0)
		return
	}

	toolParams, err := ParseToolCallParams(req.Params)
	if err != nil || toolParams == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid tools/call params"})
		return
	}

	// 1. Circuit Breaker Check
	sessionID := c.GetHeader("X-Loopers-Session-ID")
	if sessionID != "" {
		cbRes, err := h.circuitBreaker.Check(c.Request.Context(), sessionID, toolParams.Name, toolParams.Arguments)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("MCP circuit breaker check failed")
		} else if cbRes.Tripped {
			logging.Logger.Warn().Str("session_id", sessionID).Str("tool", toolParams.Name).Msg("MCP circuit breaker tripped")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "tool-call circuit breaker tripped",
				"type":  "mcp_circuit_breaker",
				"tool":  toolParams.Name,
			})
			return
		}
	}

	// 2. Budget Check
	toolCost := h.pricingStore.GetToolCost(toolParams.Name)
	if toolCost > 0 {
		err = h.budgetClient.LeaseManager.Acquire(c.Request.Context(), keyHash, toolCost, 1.0)
		if err != nil {
			if err == budget.ErrBudgetExceeded {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "budget exceeded",
					"type":  "budget_exceeded",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable"})
			return
		}
	}

	// Forward the request and restore body
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
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
