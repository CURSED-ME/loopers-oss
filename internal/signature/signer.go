package signature

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/try-loopers/loopers-oss/internal/logging"
)

type Config struct {
	Enabled bool   `mapstructure:"enabled"`
	Type    string `mapstructure:"type"`   // "ed25519" or "hmac"
	Secret  string `mapstructure:"secret"` // Key path, PEM content, or HMAC secret key
}

type Signer struct {
	Enabled      bool
	Type         string
	hmacKey      []byte
	edPrivateKey ed25519.PrivateKey
	edPublicKey  ed25519.PublicKey
}

func NewSigner(cfg Config) (*Signer, error) {
	if !cfg.Enabled {
		return &Signer{Enabled: false}, nil
	}

	sigType := strings.ToLower(cfg.Type)
	if sigType == "" {
		sigType = "ed25519"
	}

	if sigType != "ed25519" && sigType != "hmac" {
		return nil, fmt.Errorf("unsupported signature type: %s", cfg.Type)
	}

	s := &Signer{
		Enabled: true,
		Type:    sigType,
	}

	if sigType == "hmac" {
		if cfg.Secret == "" {
			// Auto-generate transient HMAC secret
			transientSecret := make([]byte, 32)
			s.hmacKey = transientSecret
			logging.Logger.Info().Msg("[Signature Engine] No HMAC secret configured. Using a transient, in-memory HMAC key.")
		} else {
			s.hmacKey = []byte(cfg.Secret)
		}
		return s, nil
	}

	// Ed25519 private key parsing/loading
	if cfg.Secret == "" {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to generate transient Ed25519 key: %w", err)
		}
		s.edPrivateKey = priv
		s.edPublicKey = pub
		logging.Logger.Info().
			Str("public_key", hex.EncodeToString(pub)).
			Msg("[Signature Engine] No private key configured. Generated transient Ed25519 keypair.")
		return s, nil
	}

	privKey, pubKey, err := parseEd25519PrivateKey(cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to load Ed25519 private key: %w", err)
	}
	s.edPrivateKey = privKey
	s.edPublicKey = pubKey

	logging.Logger.Info().
		Str("public_key", hex.EncodeToString(pubKey)).
		Msg("[Signature Engine] Loaded Ed25519 private key successfully.")

	return s, nil
}

func parseEd25519PrivateKey(secret string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	// 1. Try treating it as a file path
	if _, err := os.Stat(secret); err == nil {
		data, readErr := os.ReadFile(secret)
		if readErr == nil {
			secret = string(data)
		}
	}

	// 2. Parse PEM
	block, _ := pem.Decode([]byte(secret))
	var keyBytes []byte
	if block != nil {
		keyBytes = block.Bytes
	} else {
		// Fallback: try decoding hex string
		decoded, hexErr := hex.DecodeString(strings.TrimSpace(secret))
		if hexErr == nil {
			keyBytes = decoded
		} else {
			// Or just raw bytes
			keyBytes = []byte(secret)
		}
	}

	// Try PKCS8
	if parsedKey, err := x509.ParsePKCS8PrivateKey(keyBytes); err == nil {
		if edKey, ok := parsedKey.(ed25519.PrivateKey); ok {
			return edKey, edKey.Public().(ed25519.PublicKey), nil
		}
	}

	// Try standard ed25519 private key sizes
	if len(keyBytes) == ed25519.SeedSize {
		privKey := ed25519.NewKeyFromSeed(keyBytes)
		return privKey, privKey.Public().(ed25519.PublicKey), nil
	} else if len(keyBytes) == ed25519.PrivateKeySize {
		privKey := ed25519.PrivateKey(keyBytes)
		return privKey, privKey.Public().(ed25519.PublicKey), nil
	}

	return nil, nil, fmt.Errorf("invalid ed25519 private key size: %d bytes (expected PKCS8, 32-byte seed, or 64-byte private key)", len(keyBytes))
}

// Sign computes the signature for a given payload, returning signature, timestamp, and error.
func (s *Signer) Sign(payload []byte) (string, int64, error) {
	if !s.Enabled {
		return "", 0, errors.New("signer is disabled")
	}

	t := time.Now().Unix()
	signPayload := []byte(fmt.Sprintf("%d.%s", t, string(payload)))

	if s.Type == "hmac" {
		mac := hmac.New(sha256.New, s.hmacKey)
		mac.Write(signPayload)
		sig := hex.EncodeToString(mac.Sum(nil))
		return sig, t, nil
	}

	// Ed25519
	sigBytes := ed25519.Sign(s.edPrivateKey, signPayload)
	sig := hex.EncodeToString(sigBytes)
	return sig, t, nil
}

// FormatHeader formats the signature parameters into the X-Loopers-Signature header format.
func (s *Signer) FormatHeader(t int64, sig string) string {
	return fmt.Sprintf("t=%d; sig=%s; type=%s", t, sig, s.Type)
}

// Verify verifies a signature against the payload and timestamp.
// This helper is useful for verification tests and downstream verifiers.
func (s *Signer) Verify(payload []byte, t int64, sigStr string) bool {
	if !s.Enabled {
		return false
	}

	sigBytes, err := hex.DecodeString(sigStr)
	if err != nil {
		return false
	}

	signPayload := []byte(fmt.Sprintf("%d.%s", t, string(payload)))

	if s.Type == "hmac" {
		mac := hmac.New(sha256.New, s.hmacKey)
		mac.Write(signPayload)
		expected := mac.Sum(nil)
		return hmac.Equal(sigBytes, expected)
	}

	// Ed25519
	return ed25519.Verify(s.edPublicKey, signPayload, sigBytes)
}

// GetPublicKeyHex returns the public key in hex format if using Ed25519.
func (s *Signer) GetPublicKeyHex() string {
	if s.Type == "ed25519" && len(s.edPublicKey) > 0 {
		return hex.EncodeToString(s.edPublicKey)
	}
	return ""
}
