package session

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionManager_SessionFSMState(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	manager := NewManager(rdb)
	ctx := context.Background()
	keyHash := "test-hash"
	sessionID := "test-session-id"

	// 1. Initial state should return the defaultState parameter
	state, err := manager.GetSessionState(ctx, keyHash, sessionID, "UNAUTHENTICATED")
	if err != nil {
		t.Fatalf("unexpected error getting initial state: %v", err)
	}
	if state != "UNAUTHENTICATED" {
		t.Errorf("expected default state 'UNAUTHENTICATED', got %q", state)
	}

	// 2. Set new FSM state
	if err := manager.SetSessionState(ctx, keyHash, sessionID, "AUTHENTICATED"); err != nil {
		t.Fatalf("failed to set session state: %v", err)
	}

	// 3. Get state and verify it changed
	state, err = manager.GetSessionState(ctx, keyHash, sessionID, "UNAUTHENTICATED")
	if err != nil {
		t.Fatalf("failed to get session state: %v", err)
	}
	if state != "AUTHENTICATED" {
		t.Errorf("expected state 'AUTHENTICATED', got %q", state)
	}

	// 4. Overwrite state
	if err := manager.SetSessionState(ctx, keyHash, sessionID, "TRANSACTION_ACTIVE"); err != nil {
		t.Fatalf("failed to overwrite session state: %v", err)
	}

	state, err = manager.GetSessionState(ctx, keyHash, sessionID, "UNAUTHENTICATED")
	if err != nil {
		t.Fatalf("failed to get session state: %v", err)
	}
	if state != "TRANSACTION_ACTIVE" {
		t.Errorf("expected state 'TRANSACTION_ACTIVE', got %q", state)
	}
}
