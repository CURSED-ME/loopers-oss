package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/try-loopers/loopers-oss/internal/budget"
	"github.com/try-loopers/loopers-oss/internal/keyring"
	"github.com/try-loopers/loopers-oss/internal/pricing"
	"github.com/alicebob/miniredis/v2"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/spf13/viper"
)

func TestZSPAuthRouting(t *testing.T) {
	// Setup Redis and Pricing
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("Failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	pricingStore, _ := pricing.LoadStore("../../pricing.yaml")

	// Setup JWKS Endpoint
	rawRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}
	serverKey, _ := jwk.Import(rawRSA)
	serverKey.Set(jwk.KeyIDKey, "server-key-id")
	serverKey.Set(jwk.AlgorithmKey, "RS256")
	keySet := jwk.NewSet()
	keySet.AddKey(serverKey)

	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keySet)
	}))
	defer jwksSrv.Close()

	// Setup ZSP config in viper and re-initialize validator
	viper.Set("zsp.enabled", true)
	viper.Set("zsp.jwks_url", jwksSrv.URL)
	defer viper.Reset()

	// Setup Server
	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer upstream.Close()
	s.RegisterProviderRoute(&mockProvider{baseURL: upstream.URL})

	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	// Create ephemeral client key for DPoP
	rawClientKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	clientKey, _ := jwk.Import(rawClientKey)
	clientKey.Set(jwk.AlgorithmKey, jwa.RS256())
	thumb, _ := clientKey.Thumbprint(crypto.SHA256)
	b64Thumb := base64.RawURLEncoding.EncodeToString(thumb)

	createJWT := func(tampered bool, bound bool, expired bool) string {
		tok := jwt.New()
		tok.Set(jwt.SubjectKey, "agent-123")
		tok.Set("loopers/provider", "mock")
		if expired {
			tok.Set(jwt.IssuedAtKey, time.Now().Add(-2*time.Hour))
			tok.Set(jwt.ExpirationKey, time.Now().Add(-1*time.Hour))
		} else {
			tok.Set(jwt.IssuedAtKey, time.Now())
			tok.Set(jwt.ExpirationKey, time.Now().Add(1*time.Hour))
		}
		if bound {
			cnf := map[string]string{"jkt": b64Thumb}
			tok.Set("cnf", cnf)
		}
		alg, _ := serverKey.Algorithm()
		signed, _ := jwt.Sign(tok, jwt.WithKey(alg, serverKey))

		if tampered {
			return string(signed) + "tamper"
		}
		return string(signed)
	}

	createDPoP := func(method, url string, wrongJti bool) string {
		tok := jwt.New()
		jti := "dpop-jti-" + time.Now().String()
		if wrongJti {
			jti = "dpop-jti-reused"
		}
		tok.Set(jwt.JwtIDKey, jti)
		tok.Set("htm", method)
		tok.Set("htu", url)
		tok.Set(jwt.IssuedAtKey, time.Now())

		hdrs := jws.NewHeaders()
		hdrs.Set(jws.TypeKey, "dpop+jwt")
		pubKey, _ := clientKey.PublicKey()
		hdrs.Set(jws.JWKKey, pubKey)

		signed, _ := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), clientKey, jws.WithProtectedHeaders(hdrs)))
		return string(signed)
	}

	makeReq := func(token, dpop string, isStatus bool) *closeNotifierRecorder {
		var req *http.Request
		if isStatus {
			req, _ = http.NewRequest("GET", "/budget/status", nil)
		} else {
			req, _ = http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer([]byte(`{"model": "mock-model"}`)))
		}
		req.Host = "api.example.com"
		req.Header.Set("Authorization", "Bearer "+token)
		if dpop != "" {
			req.Header.Set("DPoP", dpop)
		}
		w := newCloseNotifierRecorder()
		s.GetRouter().ServeHTTP(w, req)
		return w
	}

	// Wait for JWKS cache to fetch
	time.Sleep(100 * time.Millisecond)

	t.Run("NEW-01 Valid JWT Proxy Pass", func(t *testing.T) {
		tok := createJWT(false, false, false)
		w := makeReq(tok, "", false)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("NEW-02 Tampered JWT Fails", func(t *testing.T) {
		tok := createJWT(true, false, false)
		w := makeReq(tok, "", false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("NEW-03 Valid Bound DPoP", func(t *testing.T) {
		tok := createJWT(false, true, false)
		dpop := createDPoP("POST", "https://api.example.com/mock/v1/chat", false)
		w := makeReq(tok, dpop, false)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("NEW-04 Bound JWT Missing DPoP Header", func(t *testing.T) {
		tok := createJWT(false, true, false)
		w := makeReq(tok, "", false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("NEW-05 Bound JWT Wrong DPoP", func(t *testing.T) {
		tok := createJWT(false, true, false)
		dpop := createDPoP("GET", "https://api.example.com/mock/v1/chat", false) // wrong method
		w := makeReq(tok, dpop, false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("NEW-06 Budget Status Valid JWT", func(t *testing.T) {
		tok := createJWT(false, false, false)
		w := makeReq(tok, "", true)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("NEW-07 DPoP Replay", func(t *testing.T) {
		tok := createJWT(false, true, false)
		dpop := createDPoP("POST", "https://api.example.com/mock/v1/chat", true)
		w1 := makeReq(tok, dpop, false)
		if w1.Code != http.StatusOK {
			t.Errorf("first request expected 200, got %d", w1.Code)
		}
		w2 := makeReq(tok, dpop, false)
		if w2.Code != http.StatusUnauthorized {
			t.Errorf("replay request expected 401, got %d", w2.Code)
		}
	})

	t.Run("NEW-08 Revoked Static Key Fails", func(t *testing.T) {
		rawKey, _ := keyring.GenerateRawKey()
		keyHash := keyring.HashKey(rawKey)
		rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
			"name":     "revoked-key",
			"provider": "mock",
			"active":   "false", // revoked
		})
		defer rdb.Del(ctx, "loopers:key:"+keyHash)

		w := makeReq(rawKey, "", false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for revoked key, got %d", w.Code)
		}
	})
}
