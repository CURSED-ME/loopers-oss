package keyring

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func TestJWKSValidator(t *testing.T) {
	ctx := context.Background()

	// 1. Create a raw RSA key
	rawKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	// 2. Create JWK and JWKS
	key, err := jwk.Import(rawKey)
	if err != nil {
		t.Fatalf("failed to import raw key: %v", err)
	}
	err = key.Set(jwk.KeyIDKey, "test-key-id")
	if err != nil {
		t.Fatalf("failed to set kid: %v", err)
	}
	err = key.Set(jwk.AlgorithmKey, "RS256")
	if err != nil {
		t.Fatalf("failed to set alg: %v", err)
	}

	keySet := jwk.NewSet()
	keySet.AddKey(key)

	// 3. Create a test HTTP server that serves the JWKS
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keySet)
	}))
	defer srv.Close()

	// 4. Initialize JWKSValidator
	validator, err := NewJWKSValidator(ctx, srv.URL)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// 5. Create a valid JWT signed by this key
	tok := jwt.New()
	tok.Set(jwt.SubjectKey, "agent-123")
	tok.Set("loopers/provider", "openai")
	tok.Set("loopers/allowed_tools", "search,calculator")
	tok.Set(jwt.IssuedAtKey, time.Now())

	alg, _ := key.Algorithm()
	signed, err := jwt.Sign(tok, jwt.WithKey(alg, key))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 6. Validate the JWT
	meta, err := validator.ValidateJWT(ctx, string(signed))
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if meta.Name != "agent-123" {
		t.Errorf("expected name agent-123, got %s", meta.Name)
	}
	if meta.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", meta.Provider)
	}
	if meta.AllowedTools != "search,calculator" {
		t.Errorf("expected allowed_tools search,calculator, got %s", meta.AllowedTools)
	}
	if meta.Active != "true" {
		t.Errorf("expected active true, got %s", meta.Active)
	}
}
