package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/loopers-oss/loopers/internal/proxy"
)

const (
	// Context keys
	RequestIDKey   = "RequestID"
	RequestBodyCtx = "RequestBody"
)

// RequestID injects X-Request-ID into context and response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set(RequestIDKey, reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// Recovery handles panics and returns an OpenAI-compatible 500 error, never crashing the server.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var reqID string
				if val, exists := c.Get(RequestIDKey); exists {
					reqID, _ = val.(string)
				}
				logging.Logger.Error().
					Interface("panic", err).
					Str("stack", string(debug.Stack())).
					Str("request_id", reqID).
					Msg("panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": "An internal error occurred. Check server logs.",
						"type":    "internal_server_error",
						"code":    "internal_error",
					},
				})
			}
		}()
		c.Next()
	}
}

// RequestLogger logs HTTP request metadata.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		var reqID string
		if val, exists := c.Get(RequestIDKey); exists {
			reqID, _ = val.(string)
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logging.Logger.Info().
			Str("request_id", reqID).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", raw).
			Int("status", status).
			Dur("latency", latency).
			Msg("HTTP request processed")
	}
}

// MaxBytesReader limits the request body size to prevent OOM.
func MaxBytesReader(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// KeyExtractor extracts X-Loopers-Provider-Key and stores it in context, stripping it from headers.
// It also extracts the Authorization header containing the Loopers proxy key.
func KeyExtractor() gin.HandlerFunc {
	return func(c *gin.Context) {
		providerKey := c.GetHeader("X-Loopers-Provider-Key")
		c.Request.Header.Del("X-Loopers-Provider-Key")

		// Extract proxy key from Authorization header: "Bearer lp-..." or "lp-..."
		authHeader := c.GetHeader("Authorization")
		var proxyKey string
		if strings.HasPrefix(authHeader, "Bearer ") {
			proxyKey = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			proxyKey = authHeader
		}

		if providerKey != "" {
			c.Set(proxy.ProviderKeyCtx, providerKey)
		}
		if proxyKey != "" {
			// We store the raw proxy key in context temporarily; keyring validation will convert to hash
			c.Set("RawProxyKey", proxyKey)
		}
		c.Next()
	}
}

// BodyBuffer reads and buffers request body for token count and mutations, then restores it.
func BodyBuffer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("Failed to read body: %v", err),
					"type":    "invalid_request_error",
					"code":    "invalid_request",
				},
			})
			return
		}

		// Restore r.Body for downstream
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Set(RequestBodyCtx, bodyBytes)
		c.Next()
	}
}
