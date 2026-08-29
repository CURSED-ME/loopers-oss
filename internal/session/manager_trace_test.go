package session

import (
	"context"
	"testing"
	"time"

	"github.com/try-loopers/loopers-oss/internal/policy"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionManager_SessionTraces(t *testing.T) {
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

	// 1. Initial get should return empty traces list without error
	traces, err := manager.GetSessionTraces(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("unexpected error getting initial traces: %v", err)
	}
	if len(traces) != 0 {
		t.Errorf("expected 0 traces initially, got %d", len(traces))
	}

	// 2. Append trace 1
	t1 := policy.SessionTrace{
		Timestamp: time.Now().Unix(),
		Type:      "llm_call",
		Provider:  "openai",
		Model:     "gpt-4o",
		Content:   "hello world",
	}
	if err := manager.AppendSessionTrace(ctx, keyHash, sessionID, t1); err != nil {
		t.Fatalf("failed to append trace 1: %v", err)
	}

	// 3. Append trace 2
	t2 := policy.SessionTrace{
		Timestamp: time.Now().Unix(),
		Type:      "mcp_tool_call",
		Provider:  "database",
		ToolName:  "query_db",
		Arguments: map[string]interface{}{"query": "select * from users"},
	}
	if err := manager.AppendSessionTrace(ctx, keyHash, sessionID, t2); err != nil {
		t.Fatalf("failed to append trace 2: %v", err)
	}

	// 4. Retrieve traces and verify order (newest first: t2 should be at index 0)
	traces, err = manager.GetSessionTraces(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("failed to get traces: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("expected 2 traces, got %d", len(traces))
	}
	if traces[0].Type != "mcp_tool_call" || traces[0].ToolName != "query_db" {
		t.Errorf("expected first trace to be database tool call, got %s", traces[0].Type)
	}
	if traces[1].Type != "llm_call" || traces[1].Content != "hello world" {
		t.Errorf("expected second trace to be llm call, got %s", traces[1].Type)
	}

	// 5. Verify capping (LTrim to 15 items)
	for i := 0; i < 20; i++ {
		tTemp := policy.SessionTrace{
			Timestamp: time.Now().Unix(),
			Type:      "llm_call",
			Content:   "fill-trace",
		}
		_ = manager.AppendSessionTrace(ctx, keyHash, sessionID, tTemp)
	}

	traces, err = manager.GetSessionTraces(ctx, keyHash, sessionID)
	if err != nil {
		t.Fatalf("failed to get traces after flood: %v", err)
	}
	if len(traces) != 15 {
		t.Errorf("expected traces list capped at 15, got %d", len(traces))
	}
}
