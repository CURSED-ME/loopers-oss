package a2a

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestEscalationBroker(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	broker := NewEscalationBroker(rdb, "test_secret")

	ctx := context.Background()

	// Simulate SaaS control plane picking up the request and approving it
	go func() {
		// wait for the request to be published
		time.Sleep(100 * time.Millisecond)

		// Pop request
		res, err := rdb.LPop(ctx, "loopers:escalation_requests").Result()
		if err != nil {
			t.Errorf("failed to pop request: %v", err)
			return
		}

		var req EscalationRequest
		json.Unmarshal([]byte(res), &req)

		// Approve and publish response
		mac := hmac.New(sha256.New, []byte("test_secret"))
		mac.Write([]byte(req.Nonce))
		mac.Write([]byte("true"))

		resp := EscalationResponse{
			Approved:  true,
			Message:   "Approved by admin",
			Signature: hex.EncodeToString(mac.Sum(nil)),
		}
		respData, _ := json.Marshal(resp)

		hash := sha256.Sum256([]byte(req.SessionID))
		channel := fmt.Sprintf("loopers:escalation:%x:%s", hash, req.Nonce[:8])
		rdb.Publish(ctx, channel, respData)
	}()

	req := EscalationRequest{
		SessionID: "123e4567-e89b-12d3-a456-426614174000",
		Reason:    "Need extra budget",
		AgentName: "TestAgent",
	}

	resp, err := broker.RequestEscalation(ctx, req, 2*time.Second)
	if err != nil {
		t.Fatalf("RequestEscalation failed: %v", err)
	}

	if !resp.Approved {
		t.Errorf("expected approved=true, got false")
	}
}

func TestEscalationBrokerTimeout(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	broker := NewEscalationBroker(rdb, "test_secret")
	ctx := context.Background()

	req := EscalationRequest{
		SessionID: "456e1234-e89b-12d3-a456-426614174000",
		Reason:    "Will timeout",
		AgentName: "TestAgent",
	}

	// Should timeout since no one is responding
	_, err = broker.RequestEscalation(ctx, req, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

func TestEscalationBrokerInvalidSessionID(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	broker := NewEscalationBroker(rdb, "test_secret")
	ctx := context.Background()

	req := EscalationRequest{
		SessionID: "invalid id!\n",
		Reason:    "Invalid ID test",
		AgentName: "TestAgent",
	}

	_, err = broker.RequestEscalation(ctx, req, 2*time.Second)
	if err == nil || err.Error() != "invalid session ID format" {
		t.Fatalf("expected invalid session ID format error, got: %v", err)
	}
}
