package riskprofile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = rdb.Close() })

	return mr, rdb
}

func TestGetProfile_NewKey(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()

	rp, err := GetProfile(ctx, rdb, "non_existent_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp == nil {
		t.Fatal("expected non-nil profile")
	}

	if rp.KeyHash != "non_existent_key" {
		t.Errorf("expected KeyHash 'non_existent_key', got %s", rp.KeyHash)
	}

	if rp.RiskScore != 0 {
		t.Errorf("expected RiskScore 0, got %d", rp.RiskScore)
	}

	if rp.LastDecayTime.IsZero() {
		t.Error("expected non-zero LastDecayTime")
	}

	if len(rp.PersistentTaintFlags) != 0 {
		t.Errorf("expected empty PersistentTaintFlags, got %v", rp.PersistentTaintFlags)
	}
}

func TestUpdateRiskScore_ClampingAndCounters(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()
	keyHash := "agent_123"

	// 1. Add +10 for policy block
	rp, err := UpdateRiskScore(ctx, rdb, keyHash, 10, false, "policy_block")
	if err != nil {
		t.Fatalf("UpdateRiskScore failed: %v", err)
	}
	if rp.RiskScore != 10 {
		t.Errorf("expected score 10, got %d", rp.RiskScore)
	}
	if rp.TotalPolicyBlocks != 1 {
		t.Errorf("expected 1 policy block, got %d", rp.TotalPolicyBlocks)
	}

	// 2. Add +25 for quarantine (isHighRisk = true)
	rp, err = UpdateRiskScore(ctx, rdb, keyHash, 25, true, "quarantine")
	if err != nil {
		t.Fatalf("UpdateRiskScore failed: %v", err)
	}
	if rp.RiskScore != 35 {
		t.Errorf("expected score 35, got %d", rp.RiskScore)
	}
	if rp.LastHighRiskAction.IsZero() {
		t.Error("expected non-zero LastHighRiskAction")
	}

	// 3. Add +15 for escalate
	rp, err = UpdateRiskScore(ctx, rdb, keyHash, 15, false, "escalate")
	if err != nil {
		t.Fatalf("UpdateRiskScore failed: %v", err)
	}
	if rp.RiskScore != 50 {
		t.Errorf("expected score 50, got %d", rp.RiskScore)
	}
	if rp.TotalEscalations != 1 {
		t.Errorf("expected 1 escalation, got %d", rp.TotalEscalations)
	}

	// 4. Test max clamp to 100
	rp, err = UpdateRiskScore(ctx, rdb, keyHash, 100, false, "overflow_test")
	if err != nil {
		t.Fatalf("UpdateRiskScore failed: %v", err)
	}
	if rp.RiskScore != 100 {
		t.Errorf("expected score clamped to 100, got %d", rp.RiskScore)
	}

	// 5. Test min clamp to 0
	rp, err = UpdateRiskScore(ctx, rdb, keyHash, -200, false, "underflow_test")
	if err != nil {
		t.Fatalf("UpdateRiskScore failed: %v", err)
	}
	if rp.RiskScore != 0 {
		t.Errorf("expected score clamped to 0, got %d", rp.RiskScore)
	}
}

func TestUpdateRiskScore_Decay(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()
	keyHash := "agent_decay_test"

	// Create profile with score 50 and LastDecayTime 49 hours ago (2 days = -10 decay)
	initial := &AgentRiskProfile{
		KeyHash:       keyHash,
		RiskScore:     50,
		LastDecayTime: time.Now().Add(-49 * time.Hour),
	}
	if err := SaveProfile(ctx, rdb, initial, 0); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Calling GetProfile triggers lazy decay evaluation
	rp, err := GetProfile(ctx, rdb, keyHash)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	// 50 - (2 * 5) = 40
	if rp.RiskScore != 40 {
		t.Errorf("expected decayed score 40, got %d", rp.RiskScore)
	}

	// Verify persistence of decayed score
	rpReloaded, err := GetProfile(ctx, rdb, keyHash)
	if err != nil {
		t.Fatalf("GetProfile reloaded failed: %v", err)
	}
	if rpReloaded.RiskScore != 40 {
		t.Errorf("expected persisted decayed score 40, got %d", rpReloaded.RiskScore)
	}
}

func TestAddPersistentTaintFlag_Dedup(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()
	keyHash := "agent_taint_test"

	// 1. Add first flag
	rp, err := AddPersistentTaintFlag(ctx, rdb, keyHash, "secret_accessed")
	if err != nil {
		t.Fatalf("AddPersistentTaintFlag failed: %v", err)
	}
	if rp.RiskScore != 5 {
		t.Errorf("expected score 5, got %d", rp.RiskScore)
	}
	if len(rp.PersistentTaintFlags) != 1 || rp.PersistentTaintFlags[0] != "secret_accessed" {
		t.Errorf("expected ['secret_accessed'], got %v", rp.PersistentTaintFlags)
	}

	// 2. Add duplicate flag — should be no-op for score and array
	rp, err = AddPersistentTaintFlag(ctx, rdb, keyHash, "secret_accessed")
	if err != nil {
		t.Fatalf("AddPersistentTaintFlag failed on duplicate: %v", err)
	}
	if rp.RiskScore != 5 {
		t.Errorf("expected score to remain 5 on duplicate, got %d", rp.RiskScore)
	}
	if len(rp.PersistentTaintFlags) != 1 {
		t.Errorf("expected deduplicated slice length 1, got %d", len(rp.PersistentTaintFlags))
	}

	// 3. Add second distinct flag
	rp, err = AddPersistentTaintFlag(ctx, rdb, keyHash, "pii_accessed")
	if err != nil {
		t.Fatalf("AddPersistentTaintFlag failed on second flag: %v", err)
	}
	if rp.RiskScore != 10 {
		t.Errorf("expected score 10, got %d", rp.RiskScore)
	}
	if len(rp.PersistentTaintFlags) != 2 {
		t.Errorf("expected slice length 2, got %d", len(rp.PersistentTaintFlags))
	}
}

func TestIncrementSessionCountAndSpend(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()
	keyHash := "agent_spend_session_test"

	if err := IncrementSessionCount(ctx, rdb, keyHash); err != nil {
		t.Fatalf("IncrementSessionCount failed: %v", err)
	}
	if err := IncrementSessionCount(ctx, rdb, keyHash); err != nil {
		t.Fatalf("IncrementSessionCount failed 2nd time: %v", err)
	}

	if err := AddLifetimeSpend(ctx, rdb, keyHash, 0.05); err != nil {
		t.Fatalf("AddLifetimeSpend failed: %v", err)
	}
	if err := AddLifetimeSpend(ctx, rdb, keyHash, 0.15); err != nil {
		t.Fatalf("AddLifetimeSpend failed 2nd time: %v", err)
	}

	rp, err := GetProfile(ctx, rdb, keyHash)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	if rp.SessionCount != 2 {
		t.Errorf("expected SessionCount 2, got %d", rp.SessionCount)
	}
	if rp.TotalSpend < 0.199 || rp.TotalSpend > 0.201 {
		t.Errorf("expected TotalSpend ~0.20, got %f", rp.TotalSpend)
	}
}

func TestAtomicity_ConcurrentUpdates(t *testing.T) {
	_, rdb := setupTestRedis(t)
	ctx := context.Background()
	keyHash := "agent_concurrent_test"

	concurrency := 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, _ = UpdateRiskScore(ctx, rdb, keyHash, 2, false, "policy_block")
		}()
	}

	wg.Wait()

	rp, err := GetProfile(ctx, rdb, keyHash)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	expectedScore := concurrency * 2
	if rp.RiskScore != expectedScore {
		t.Errorf("expected score %d from concurrent updates, got %d", expectedScore, rp.RiskScore)
	}
	if rp.TotalPolicyBlocks != int64(concurrency) {
		t.Errorf("expected %d total policy blocks, got %d", concurrency, rp.TotalPolicyBlocks)
	}
}
