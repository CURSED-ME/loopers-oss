package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/loop"
	"github.com/CURSED-ME/loopers-oss/internal/policy"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// anchorKey returns the Redis key storing the session's anchoring prompt bi-grams.
func anchorKey(keyHash, sessionID string) string {
	return fmt.Sprintf("loopers:session:{%s}:%s:anchor", keyHash, sessionID)
}

var stopWords = map[string]struct{}{
	"the": {}, "and": {}, "a": {}, "an": {}, "of": {}, "in": {}, "to": {}, "for": {},
	"is": {}, "it": {}, "are": {}, "how": {}, "do": {}, "what": {}, "can": {}, "you": {},
	"with": {}, "this": {}, "that": {}, "from": {}, "as": {}, "by": {}, "on": {},
}

// CleanText removes common high-frequency English stopwords to focus on domain content.
func CleanText(text string) string {
	words := strings.Fields(strings.ToLower(text))
	var filtered []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?:;\"'()[]{}<>/\\`~@#$%^&*-_=+|")
		if _, ok := stopWords[w]; !ok && len(w) > 1 {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return strings.ToLower(text)
	}
	return strings.Join(filtered, " ")
}

// ExtractTriGrams extracts sorted deduplicated 3-character trigram hashes from text.
func ExtractTriGrams(text string) []uint16 {
	cleaned := CleanText(text)
	data := []byte(cleaned)
	if len(data) < 3 {
		return loop.ExtractBiGrams(data)
	}
	set := make(map[uint16]struct{})
	for i := 0; i < len(data)-2; i++ {
		// FNV-1a 32-bit hash folded to uint16
		h := uint32(2166136261)
		h = (h ^ uint32(data[i])) * 16777619
		h = (h ^ uint32(data[i+1])) * 16777619
		h = (h ^ uint32(data[i+2])) * 16777619
		val := uint16(h ^ (h >> 16))
		set[val] = struct{}{}
	}
	grams := make([]uint16, 0, len(set))
	for k := range set {
		grams = append(grams, k)
	}
	sort.Slice(grams, func(i, j int) bool { return grams[i] < grams[j] })
	return grams
}

// GetOrCreateAnchor registers the initial prompt as the session anchor if none exists,
// or retrieves the existing encoded anchor bi-grams.
func (m *Manager) GetOrCreateAnchor(ctx context.Context, keyHash, sessionID, prompt string) (string, error) {
	if prompt == "" {
		return "", nil
	}
	key := anchorKey(keyHash, sessionID)
	val, err := m.rdb.Get(ctx, key).Result()
	if err == nil && val != "" {
		return val, nil
	}
	if err != nil && err != redis.Nil {
		return "", fmt.Errorf("redis error reading session anchor: %w", err)
	}

	// First turn: compute and save encoded trigrams with lowercase normalization
	grams := ExtractTriGrams(prompt)
	encoded := loop.EncodeBiGrams(grams)

	// Use SetNX to ensure concurrency safety
	set, setErr := m.rdb.SetNX(ctx, key, encoded, sessionDataTTL).Result()
	if setErr != nil {
		return "", fmt.Errorf("redis error saving session anchor: %w", setErr)
	}
	if !set {
		// Another concurrent request saved the anchor first; fetch it
		return m.rdb.Get(ctx, key).Result()
	}
	return encoded, nil
}

// ContainmentSimilarity computes |a ∩ b| / |a|, measuring what fraction of grams in a exist in context b.
func ContainmentSimilarity(a, b []uint16) float64 {
	if len(a) == 0 {
		return 1.0
	}
	if len(b) == 0 {
		return 0.0
	}
	intersection := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			intersection++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return float64(intersection) / float64(len(a))
}

// ComputeDrift calculates the semantic drift metrics between currentPrompt, the initial session anchor,
// and recent conversation traces.
func (m *Manager) ComputeDrift(ctx context.Context, keyHash, sessionID, currentPrompt string, traces []policy.SessionTrace) (policy.SessionDriftContext, error) {
	// Count user prompt turns accurately from traces
	llmPromptCount := 0
	for _, tr := range traces {
		if tr.Type == "llm_call" || tr.Type == "" {
			llmPromptCount++
		}
	}

	result := policy.SessionDriftContext{
		AnchorSimilarity:    1.0,
		PriorTurnSimilarity: 1.0,
		DriftScore:          0.0,
		DriftDetected:       false,
		TurnCount:           llmPromptCount + 1,
	}

	if currentPrompt == "" || sessionID == "" {
		return result, nil
	}

	currGrams := ExtractTriGrams(currentPrompt)
	if len(currGrams) == 0 {
		return result, nil
	}

	// 1. Get or create session anchor
	anchorB64, err := m.GetOrCreateAnchor(ctx, keyHash, sessionID, currentPrompt)
	if err != nil {
		return result, err
	}

	var anchorGrams []uint16
	if anchorB64 != "" {
		if ag, dErr := loop.DecodeBiGrams(anchorB64); dErr == nil {
			anchorGrams = ag
		}
	}
	if len(anchorGrams) > 0 {
		result.AnchorSimilarity = ContainmentSimilarity(currGrams, anchorGrams)
	}

	// 2. Compute similarity with historical user prompt traces (conversation continuity)
	var maxPriorSim float64 = 0.0
	for _, tr := range traces {
		if (tr.Type == "llm_call" || tr.Type == "") && tr.Content != "" {
			priorGrams := ExtractTriGrams(tr.Content)
			if len(priorGrams) > 0 {
				sim := ContainmentSimilarity(currGrams, priorGrams)
				if sim > maxPriorSim {
					maxPriorSim = sim
				}
			}
		}
	}
	result.PriorTurnSimilarity = maxPriorSim

	// 3. Configuration thresholds & weights
	anchorWeight := viper.GetFloat64("session.drift_detection.anchor_weight")
	if anchorWeight <= 0.0 || anchorWeight > 1.0 {
		anchorWeight = 0.75
	}
	priorWeight := viper.GetFloat64("session.drift_detection.prior_weight")
	if priorWeight <= 0.0 || priorWeight > 1.0 {
		priorWeight = 0.25
	}
	totalWeight := anchorWeight + priorWeight
	anchorWeight /= totalWeight
	priorWeight /= totalWeight

	minTurns := viper.GetInt("session.drift_detection.min_turns")
	if minTurns <= 0 {
		minTurns = 3
	}

	anchorSimFloor := viper.GetFloat64("session.drift_detection.anchor_similarity_threshold")
	if anchorSimFloor <= 0.0 {
		anchorSimFloor = 0.08
	}

	driftScoreCeiling := viper.GetFloat64("session.drift_detection.drift_score_threshold")
	if driftScoreCeiling <= 0.0 {
		driftScoreCeiling = 0.45
	}

	// 4. Calculate normalized drift score (0.0 = coherent, 1.0 = total divergence)
	combinedSim := (anchorWeight*result.AnchorSimilarity + priorWeight*result.PriorTurnSimilarity) / totalWeight

	// Full continuity baseline requires 0.35 (35%) domain containment.
	continuity := combinedSim / 0.35
	if continuity > 1.0 {
		continuity = 1.0
	}
	result.DriftScore = 1.0 - continuity
	if result.DriftScore < 0.0 {
		result.DriftScore = 0.0
	}

	// 5. Evaluate if drift detection criteria are met
	if result.TurnCount >= minTurns {
		if result.DriftScore >= driftScoreCeiling || (result.AnchorSimilarity < anchorSimFloor && result.PriorTurnSimilarity < 0.05) {
			result.DriftDetected = true
		}
	}

	return result, nil
}

func deduplicateAndSortGrams(grams []uint16) []uint16 {
	if len(grams) == 0 {
		return nil
	}
	set := make(map[uint16]struct{}, len(grams))
	for _, g := range grams {
		set[g] = struct{}{}
	}
	res := make([]uint16, 0, len(set))
	for g := range set {
		res = append(res, g)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	return res
}
