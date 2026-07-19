package keyring

import (
	"crypto"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
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
	if err := token.Get("htu", &htu); err != nil || !strings.EqualFold(htu, requestURL) {
		return "", errors.New("invalid htu claim")
	}

	// Check time
	iat, ok := token.IssuedAt()
	if !ok {
		return "", errors.New("missing iat claim")
	}
	if time.Since(iat) > 1*time.Minute || time.Since(iat) < -1*time.Minute {
		return "", errors.New("DPoP token expired or issued in the future")
	}

	// Check jti
	jti, ok := token.JwtID()
	if !ok || jti == "" {
		return "", errors.New("missing jti claim")
	}

	// Check thumbprint against expectedJkt
	if expectedJkt != "" {
		thumb, err := ephemeralJWK.Thumbprint(crypto.SHA256)
		if err != nil {
			return "", fmt.Errorf("failed to compute thumbprint: %w", err)
		}
		b64Thumb := base64.RawURLEncoding.EncodeToString(thumb)
		if b64Thumb != expectedJkt {
			return "", errors.New("jkt mismatch")
		}
	}

	return jti, nil
}
