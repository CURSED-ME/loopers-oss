package a2a

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/session"
	"github.com/redis/go-redis/v9"
)

type EscalationBroker struct {
	rdb    *redis.Client
	secret string
}

// NewEscalationBroker initializes a new escalation broker.
// It is highly recommended to pass a dedicated redis.Client to prevent pub/sub
// subscriptions from exhausting the primary connection pool used for hot-path budget checks.
func NewEscalationBroker(rdb *redis.Client, secret string) *EscalationBroker {
	return &EscalationBroker{rdb: rdb, secret: secret}
}

type EscalationRequest struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
	AgentName string `json:"agent_name"`
	Nonce     string `json:"nonce"`
}

type EscalationResponse struct {
	Approved  bool   `json:"approved"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

// RequestEscalation suspends the current request and waits for an approval via Redis Pub/Sub.
func (b *EscalationBroker) RequestEscalation(ctx context.Context, req EscalationRequest, timeout time.Duration) (*EscalationResponse, error) {
	if !session.IsValidID(req.SessionID) {
		return nil, fmt.Errorf("invalid session ID format")
	}

	req.Nonce = fmt.Sprintf("%d", time.Now().UnixNano())
	reqData, _ := json.Marshal(req)

	// Hash session ID to prevent channel collision/injection
	hash := sha256.Sum256([]byte(req.SessionID))
	channel := fmt.Sprintf("loopers:escalation:%x", hash)

	// Start subscribing to the specific session channel
	pubsub := b.rdb.Subscribe(ctx, channel)
	defer pubsub.Close()

	// Wait for subscription to be confirmed before publishing
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Push the request to a Redis list so the SaaS control plane can consume it
	err = b.rdb.RPush(ctx, "loopers:escalation_requests", reqData).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Wait for the SaaS control plane to publish a response on the session channel
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("escalation request timed out after %v", timeout)
	case msg := <-pubsub.Channel():
		var resp EscalationResponse
		if err := json.Unmarshal([]byte(msg.Payload), &resp); err != nil {
			return nil, fmt.Errorf("invalid response payload: %w", err)
		}

		// Verify HMAC signature
		mac := hmac.New(sha256.New, []byte(b.secret))
		mac.Write([]byte(req.Nonce))
		mac.Write([]byte(fmt.Sprintf("%t", resp.Approved)))
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		if b.secret != "" && resp.Signature != expectedSig {
			return nil, fmt.Errorf("invalid escalation signature")
		}

		return &resp, nil
	}
}
