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
