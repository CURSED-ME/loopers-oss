package a2a

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/try-loopers/loopers-oss/internal/policy"
	"github.com/try-loopers/loopers-oss/internal/session"
	"github.com/google/uuid"
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
	if b.secret == "" {
		return nil, fmt.Errorf("escalation secret is not configured")
	}
	if !session.IsValidID(req.SessionID) {
		return nil, fmt.Errorf("invalid session ID format")
	}

	req.Nonce = uuid.NewString()
	reqData, _ := json.Marshal(req)

	// Hash session ID to prevent channel collision/injection
	hash := sha256.Sum256([]byte(req.SessionID))
	channel := fmt.Sprintf("loopers:escalation:%x:%s", hash, req.Nonce[:8])

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

		if resp.Signature != expectedSig {
			return nil, fmt.Errorf("invalid escalation signature")
		}

		return &resp, nil
	}
}

// RequestEscalationFromDecision is a convenience wrapper that constructs an
// EscalationRequest directly from a policy.Decision with Action == "escalate".
// If sessionID is empty or not a valid UUID, a synthetic one-shot UUID is generated
// so that escalation can proceed even when no X-Loopers-Session-ID header was sent.
func (b *EscalationBroker) RequestEscalationFromDecision(
	ctx context.Context,
	sessionID, agentName string,
	d policy.Decision,
	timeout time.Duration,
) (*EscalationResponse, error) {
	if !session.IsValidID(sessionID) {
		sessionID = uuid.NewString()
	}
	return b.RequestEscalation(ctx, EscalationRequest{
		SessionID: sessionID,
		AgentName: agentName,
		Reason:    d.Reason,
	}, timeout)
}
