package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathAuthWrapper(t *testing.T) {
	// A simple mock handler to verify the request modification
	var capturedPath string
	var capturedAuth string
	var capturedProviderKey string

	mockNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedProviderKey = r.Header.Get("X-Loopers-Provider-Key")
		w.WriteHeader(http.StatusOK)
	})

	handler := PathAuthWrapper(mockNext)

	tests := []struct {
		name                string
		reqPath             string
		reqAuth             string
		expectedPath        string
		expectedAuth        string
		expectedProviderKey string
	}{
		{
			name:                "Valid lp- key in path",
			reqPath:             "/lp-12345/openai/v1/chat/completions",
			reqAuth:             "Bearer sk-proj-upstream-key",
			expectedPath:        "/openai/v1/chat/completions",
			expectedAuth:        "Bearer lp-12345",
			expectedProviderKey: "sk-proj-upstream-key",
		},
		{
			name:                "Valid eyJ (JWT) key in path",
			reqPath:             "/eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz/anthropic/v1/messages",
			reqAuth:             "Bearer sk-ant-real-key",
			expectedPath:        "/anthropic/v1/messages",
			expectedAuth:        "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz",
			expectedProviderKey: "sk-ant-real-key",
		},
		{
			name:                "No proxy key in path (standard)",
			reqPath:             "/openai/v1/chat/completions",
			reqAuth:             "Bearer lp-12345",
			expectedPath:        "/openai/v1/chat/completions",
			expectedAuth:        "Bearer lp-12345",
			expectedProviderKey: "", // unchanged by wrapper
		},
		{
			name:                "Proxy key in path without authorization header",
			reqPath:             "/lp-test/generic/v1",
			reqAuth:             "",
			expectedPath:        "/generic/v1",
			expectedAuth:        "Bearer lp-test",
			expectedProviderKey: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.reqPath, nil)
			if tc.reqAuth != "" {
				req.Header.Set("Authorization", tc.reqAuth)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedPath, capturedPath, "Path mismatch")
			assert.Equal(t, tc.expectedAuth, capturedAuth, "Authorization header mismatch")
			assert.Equal(t, tc.expectedProviderKey, capturedProviderKey, "X-Loopers-Provider-Key mismatch")
		})
	}
}
