package mcp

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
)

// Proxy handles forwarding requests to upstream MCP servers.
type Proxy struct {
	reverseProxy *httputil.ReverseProxy
}

type mcpTargetCtxKey string

const MCPTargetCtx mcpTargetCtxKey = "MCPTarget"

// NewProxy creates a new ReverseProxy wrapper for MCP proxying.
func NewProxy(modifyResponse func(*http.Response) error) *Proxy {
	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   1000,
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	director := func(req *http.Request) {
		ctx := req.Context()
		targetStr, _ := ctx.Value(MCPTargetCtx).(string)

		if targetStr != "" {
			target, err := url.Parse(targetStr)
			if err == nil && target != nil {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host
				// Preserve the original Path or rewrite it if needed. For now, pass through.
			}
		}
	}

	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		if err == context.Canceled || r.Context().Err() == context.Canceled {
			logging.Logger.Debug().Msg("Client cancelled MCP connection; aborting backend request")
			return
		}
		logging.Logger.Error().Err(err).Msg("MCP reverse proxy forwarding error")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"Bad gateway forwarding MCP request","type":"bad_gateway_error"}}`))
	}

	rp := &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  0,
		ModifyResponse: modifyResponse,
		ErrorHandler:   errorHandler,
	}

	return &Proxy{reverseProxy: rp}
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.reverseProxy.ServeHTTP(w, r)
}
