package keyring

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func TestValidateDPoP(t *testing.T) {
	// 1. Generate client's ephemeral RSA key
	rawKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	key, err := jwk.Import(rawKey)
	if err != nil {
		t.Fatalf("failed to import raw key: %v", err)
	}
	key.Set(jwk.AlgorithmKey, jwa.RS256())

	// Calculate thumbprint
	thumb, err := key.Thumbprint(crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to compute thumbprint: %v", err)
	}
	b64Thumb := base64.RawURLEncoding.EncodeToString(thumb)

	// 2. Create DPoP JWT payload
	tok := jwt.New()
	tok.Set(jwt.JwtIDKey, "unique-jti")
	tok.Set("htm", "POST")
	tok.Set("htu", "https://api.example.com/v1/proxy")
	tok.Set(jwt.IssuedAtKey, time.Now())

	// 3. Create JWS headers
	hdrs := jws.NewHeaders()
	hdrs.Set(jws.TypeKey, "dpop+jwt")
	
	// Create public key for the header
	pubKey, err := key.PublicKey()
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	hdrs.Set(jws.JWKKey, pubKey)

	// 4. Sign to create DPoP token
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(hdrs)))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	tokenString := string(signed)

	// 5. Test validation (Success)
	_, err = ValidateDPoP(tokenString, "POST", "https://api.example.com/v1/proxy", b64Thumb)
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	// 6. Test validation (Failure - Wrong Method)
	_, err = ValidateDPoP(tokenString, "GET", "https://api.example.com/v1/proxy", b64Thumb)
	if err == nil {
		t.Errorf("expected failure on wrong method")
	}

	// 7. Test validation (Failure - Wrong URI)
	_, err = ValidateDPoP(tokenString, "POST", "https://api.example.com/v2/proxy", b64Thumb)
	if err == nil {
		t.Errorf("expected failure on wrong URI")
	}

	// 8. Test validation (Failure - Wrong jkt)
	_, err = ValidateDPoP(tokenString, "POST", "https://api.example.com/v1/proxy", "wrong-thumbprint")
	if err == nil {
		t.Errorf("expected failure on wrong jkt")
	}
}
