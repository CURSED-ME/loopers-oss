package keyring

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// JWKSValidator handles statelessly verifying JWTs using a cached JWKS endpoint.
type JWKSValidator struct {
	cache   *jwk.Cache
	jwksURL string
}

// NewJWKSValidator creates a new JWKSValidator and starts the background cache refresh.
func NewJWKSValidator(ctx context.Context, jwksURL string) (*JWKSValidator, error) {
	c, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	err = c.Register(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to register jwks url: %w", err)
	}

	// Fetch immediately to ensure it works (optional, but good for fail-fast)
	_, err = c.Lookup(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial jwks: %w", err)
	}

	return &JWKSValidator{
		cache:   c,
		jwksURL: jwksURL,
	}, nil
}

// ValidateJWT parses and validates the JWT signature against the cached JWKS.
// If valid, it returns the KeyMetadata derived from the claims.
func (v *JWKSValidator) ValidateJWT(ctx context.Context, tokenString string) (*KeyMetadata, error) {
	keySet, err := v.cache.Lookup(ctx, v.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get jwks: %w", err)
	}

	// Parse and verify the token signature
	token, err := jwt.Parse([]byte(tokenString), jwt.WithKeySet(keySet), jwt.WithValidate(true))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Map claims to KeyMetadata based on the plan
	meta := &KeyMetadata{
		Active: "true",
	}

	if sub, ok := token.Subject(); ok && sub != "" {
		meta.Name = sub
	}

	var provider string
	if err := token.Get("loopers/provider", &provider); err == nil {
		meta.Provider = provider
	}

	var agentName string
	if err := token.Get("loopers/agent_name", &agentName); err == nil {
		meta.AgentName = agentName
	}

	var allowedTools string
	if err := token.Get("loopers/allowed_tools", &allowedTools); err == nil {
		meta.AllowedTools = allowedTools
	}

	var allowedProviders string
	if err := token.Get("loopers/allowed_providers", &allowedProviders); err == nil {
		meta.AllowedProviders = allowedProviders
	}

	var owner string
	if err := token.Get("loopers/owner", &owner); err == nil {
		meta.Owner = owner
	}

	if iat, ok := token.IssuedAt(); ok && !iat.IsZero() {
		meta.CreatedAt = iat.Format(time.RFC3339)
	}

	// cnf claim extraction for DPoP binding
	var cnf map[string]interface{}
	if err := token.Get("cnf", &cnf); err == nil && cnf != nil {
		if jkt, ok := cnf["jkt"].(string); ok {
			meta.Jkt = jkt
		}
	}

	return meta, nil
}
