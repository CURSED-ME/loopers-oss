package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionManager_EnforceAbsoluteTTL(t *testing.T) {
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
	sessionID := "test-session"

	// 1. Initial check (new session) -> should be valid
	valid, err := manager.EnforceAbsoluteTTL(ctx, "hash1", sessionID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected new session to be valid")
	}

	// 2. Immediate second check -> should still be valid
	valid, err = manager.EnforceAbsoluteTTL(ctx, "hash1", sessionID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected session to still be valid")
	}

	// 3. Sleep 2.1 seconds so Go clock advances past the 1s TTL (Unix time diff > 1)
	time.Sleep(2100 * time.Millisecond)

	// 4. Check again after TTL expiry -> should be invalid
	valid, err = manager.EnforceAbsoluteTTL(ctx, "hash1", sessionID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Errorf("expected session to be invalid after absolute TTL expiry")
	}
}

func TestSessionManager_CheckBlastRadius(t *testing.T) {
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
	sessionID := "test-session-blast"

	// 1. Add server 1 (limit = 2) -> allowed
	allowed, err := manager.CheckBlastRadius(ctx, "hash1", sessionID, "server1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected server1 to be allowed")
	}

	// 2. Add server 1 again -> allowed (already member)
	allowed, err = manager.CheckBlastRadius(ctx, "hash1", sessionID, "server1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected server1 to be allowed again")
	}

	// 3. Add server 2 -> allowed
	allowed, err = manager.CheckBlastRadius(ctx, "hash1", sessionID, "server2", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected server2 to be allowed")
	}

	// 4. Add server 3 -> blocked (exceeds limit 2)
	allowed, err = manager.CheckBlastRadius(ctx, "hash1", sessionID, "server3", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("expected server3 to be blocked (limit is 2)")
	}
}

func TestIsValidID(t *testing.T) {
	valid := []string{"550e8400-e29b-41d4-a716-446655440000", "00000000-0000-0000-0000-000000000000"}
	invalid := []string{"sess-123", "user.name_01", "sess:123", "user@domain", "path/to/session", "space in id"}

	for _, id := range valid {
		if !IsValidID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
	for _, id := range invalid {
		if IsValidID(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}

func TestSessionManager_AppendAndGetTaintFlags(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	manager := NewManager(rdb)
	ctx := context.Background()
	sessionID := "taint-test-session"
	keyHash := "taint-hash1"

	// 1. Initially no flags
	flags, err := manager.GetTaintFlags(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error getting empty taint flags: %v", err)
	}
	if len(flags) != 0 {
		t.Errorf("expected empty flags map, got %v", flags)
	}

	// 2. Append a flag
	if err := manager.AppendTaintFlag(ctx, keyHash, sessionID, "secret_accessed"); err != nil {
		t.Fatalf("failed to append taint flag: %v", err)
	}

	// 3. Verify flag is present
	flags, err = manager.GetTaintFlags(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error getting taint flags: %v", err)
	}
	if !flags["secret_accessed"] {
		t.Errorf("expected 'secret_accessed' flag to be set, got %v", flags)
	}

	// 4. Append a second flag
	if err := manager.AppendTaintFlag(ctx, keyHash, sessionID, "pii_accessed"); err != nil {
		t.Fatalf("failed to append second taint flag: %v", err)
	}

	// 5. Both flags present
	flags, err = manager.GetTaintFlags(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flags["secret_accessed"] || !flags["pii_accessed"] {
		t.Errorf("expected both flags set, got %v", flags)
	}

	// 6. Idempotent — adding same flag again should not duplicate or error
	if err := manager.AppendTaintFlag(ctx, keyHash, sessionID, "secret_accessed"); err != nil {
		t.Fatalf("idempotent append failed: %v", err)
	}
	flags, _ = manager.GetTaintFlags(ctx, keyHash, sessionID)
	if len(flags) != 2 {
		t.Errorf("expected 2 unique flags after idempotent append, got %d: %v", len(flags), flags)
	}
}

func TestSessionManager_AppendAndGetToolHistory(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	manager := NewManager(rdb)
	ctx := context.Background()
	sessionID := "tools-test-session"
	keyHash := "tools-hash1"

	// 1. Initially empty history
	history, err := manager.GetToolCallHistory(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error getting empty tool history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %v", history)
	}

	// 2. Append tools in order
	tools := []string{"read_file", "write_file", "search_web"}
	for _, tool := range tools {
		if err := manager.AppendToolCall(ctx, keyHash, sessionID, tool); err != nil {
			t.Fatalf("failed to append tool call %q: %v", tool, err)
		}
	}

	// 3. Verify history is newest-first (LPUSH reverses order)
	history, err = manager.GetToolCallHistory(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	// LPush makes "search_web" appear first (most recent)
	if history[0] != "search_web" {
		t.Errorf("expected newest tool first, got %q", history[0])
	}
	if history[2] != "read_file" {
		t.Errorf("expected oldest tool last, got %q", history[2])
	}

	// 4. Cap test: fill up past 100 entries; history should stay at 100
	for i := 0; i < 105; i++ {
		_ = manager.AppendToolCall(ctx, keyHash, sessionID, "bulk_tool")
	}
	history, _ = manager.GetToolCallHistory(ctx, keyHash, sessionID)
	if len(history) > 50 {
		// GetToolCallHistory caps at 50 (LRANGE 0 49)
		t.Errorf("expected at most 50 entries returned, got %d", len(history))
	}
}

