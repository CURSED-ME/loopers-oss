package a2a

import (
	"context"
	"crypto/sha256"
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

	broker := NewEscalationBroker(rdb, "")

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
		resp := EscalationResponse{
			Approved: true,
			Message:  "Approved by admin",
		}
		respData, _ := json.Marshal(resp)

		hash := sha256.Sum256([]byte(req.SessionID))
		channel := fmt.Sprintf("loopers:escalation:%x", hash)
		rdb.Publish(ctx, channel, respData)
	}()

	req := EscalationRequest{
		SessionID: "session-123",
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

	broker := NewEscalationBroker(rdb, "")
	ctx := context.Background()

	req := EscalationRequest{
		SessionID: "session-456",
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

	broker := NewEscalationBroker(rdb, "")
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
