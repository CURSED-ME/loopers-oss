package keyring

import (
	"context"
	"crypto"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/redis/go-redis/v9"
)

// ValidateDPoP verifies a DPoP proof token according to RFC 9449.
// tokenString: The DPoP header value
// method: The HTTP method of the current request
// requestURL: The full URL (without query params) of the current request
// expectedJkt: The jkt extracted from the access token's cnf claim (if DPoP is bound)
// Returns the jti claim for replay protection checks, or an error if invalid.
func ValidateDPoP(tokenString, method, requestURL, expectedJkt string) (string, error) {
	if tokenString == "" {
		return "", errors.New("missing DPoP header")
	}

	// Parse JWS to inspect headers
	msg, err := jws.Parse([]byte(tokenString))
	if err != nil {
		return "", fmt.Errorf("invalid DPoP token: %w", err)
	}

	if len(msg.Signatures()) != 1 {
		return "", errors.New("DPoP token must have exactly one signature")
	}

	hdrs := msg.Signatures()[0].ProtectedHeaders()

	typ, ok := hdrs.Type()
	if !ok || !strings.EqualFold(typ, "dpop+jwt") {
		return "", errors.New("invalid typ header, must be dpop+jwt")
	}

	ephemeralJWK, ok := hdrs.JWK()
	if !ok || ephemeralJWK == nil {
		return "", errors.New("missing jwk in DPoP header")
	}

	alg, ok := ephemeralJWK.Algorithm()
	if !ok {
		return "", errors.New("missing alg in DPoP jwk")
	}
	algStr := alg.String()
	if algStr != "ES256" && algStr != "ES384" && algStr != "RS256" && algStr != "PS256" {
		return "", errors.New("disallowed alg in DPoP jwk")
	}

	// Verify the signature using this key
	token, err := jwt.Parse([]byte(tokenString), jwt.WithKey(alg, ephemeralJWK), jwt.WithValidate(true))
	if err != nil {
		return "", fmt.Errorf("failed to verify DPoP signature: %w", err)
	}

	// Check HTTP Method
	var htm string
	if err := token.Get("htm", &htm); err != nil || !strings.EqualFold(htm, method) {
		return "", errors.New("invalid htm claim")
	}

	// Check HTTP URI
	var htu string
	if err := token.Get("htu", &htu); err != nil {
		return "", errors.New("invalid htu claim")
	}
	u1, err1 := url.Parse(htu)
	u2, err2 := url.Parse(requestURL)
	if err1 != nil || err2 != nil || !strings.EqualFold(u1.Scheme, u2.Scheme) || !strings.EqualFold(u1.Host, u2.Host) || u1.Path != u2.Path {
		return "", errors.New("invalid htu claim")
	}

	// Check time
	iat, ok := token.IssuedAt()
	if !ok {
		return "", errors.New("missing iat claim")
	}
	if time.Since(iat) > 30*time.Second || time.Since(iat) < -30*time.Second {
		return "", errors.New("DPoP token expired or issued in the future")
	}

	// Check jti
	jti, ok := token.JwtID()
	if !ok || jti == "" {
		return "", errors.New("missing jti claim")
	}

	// Check thumbprint against expectedJkt
	if expectedJkt == "" {
		return "", errors.New("access token must be bound to a jkt if DPoP is used")
	}
	thumb, err := ephemeralJWK.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("failed to compute thumbprint: %w", err)
	}
	b64Thumb := base64.RawURLEncoding.EncodeToString(thumb)
	if b64Thumb != expectedJkt {
		return "", errors.New("jkt mismatch")
	}

	return jti, nil
}

// ValidateDPoPAndReplay verifies a DPoP proof token and simultaneously sets the JTI in Redis to prevent replay attacks.
// This prevents race conditions where a JTI check is validated but not immediately claimed.
func ValidateDPoPAndReplay(ctx context.Context, rdb *redis.Client, tokenString, method, requestURL, expectedJkt string) (string, error) {
	jti, err := ValidateDPoP(tokenString, method, requestURL, expectedJkt)
	if err != nil {
		return "", err
	}

	jtiKey := "loopers:dpop_jti:" + jti
	set, err := rdb.SetNX(ctx, jtiKey, "1", 390*time.Second).Result()
	if err != nil || !set {
		return "", errors.New("DPoP token replay detected")
	}

	return jti, nil
}
