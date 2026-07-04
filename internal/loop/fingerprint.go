package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync/atomic"
	"time"
)

var memberCounter atomic.Uint64

// computeSimHash generates a 64-bit Locality Sensitive Hash using 3-byte trigrams.
func computeSimHash(data []byte) uint64 {
	// If the data is too short for trigrams, return a direct hash to prevent 0 collision
	if len(data) < 3 {
		h := fnv.New64a()
		h.Write(data)
		return h.Sum64()
	}

	var v [64]int32
	h := fnv.New64a()
	for i := 0; i < len(data)-2; i++ {
		h.Reset()
		h.Write(data[i : i+3])
		hashVal := h.Sum64()
		for b := 0; b < 64; b++ {
			if ((hashVal >> b) & 1) == 1 {
				v[b]++
			} else {
				v[b]--
			}
		}
	}
	var simhash uint64
	for b := 0; b < 64; b++ {
		if v[b] > 0 {
			simhash |= (1 << b)
		}
	}
	return simhash
}

// NormalizeAndHash strips volatile fields from the body and returns
// a stable hex-encoded SimHash of the normalized content.
// It also returns the raw uint64 hash value.
func NormalizeAndHash(body []byte) (string, uint64, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		// If we can't parse, hash the raw bytes directly (safe fallback)
		v := computeSimHash(body)
		return fmt.Sprintf("%016x", v), v, nil
	}

	// Volatile fields to strip before hashing
	volatileFields := []string{"stream", "max_tokens", "temperature", "seed", "n", "user"}
	for _, f := range volatileFields {
		delete(raw, f)
	}

	normalized, err := json.Marshal(raw)
	if err != nil {
		return "", 0, err
	}

	v := computeSimHash(normalized)
	return fmt.Sprintf("%016x", v), v, nil
}

// CheckFingerprint checks whether the request hash has appeared >= threshold
// times in the sliding window. Returns (isLoop, count, error).
func (d *Detector) CheckFingerprint(ctx context.Context, sessionID, hash string) (bool, int64, error) {
	if d.cfg.Fingerprint.Threshold <= 0 {
		return false, 0, nil
	}

	ringKey := fmt.Sprintf("loopers:loop:fp:%s", sessionID)
	now := time.Now().Unix()

	memberID := strconv.FormatUint(memberCounter.Add(1), 10)

	window := int64(d.cfg.Fingerprint.WindowSeconds)
	if window <= 0 {
		window = 60
	}

	maxDist := d.cfg.Fingerprint.MaxDistance
	if maxDist <= 0 {
		maxDist = 3
	}

	res, err := d.fpScript.Run(ctx, d.rdb, []string{ringKey}, hash, strconv.FormatInt(now, 10), strconv.FormatInt(window, 10), strconv.Itoa(d.cfg.Fingerprint.Threshold), memberID, strconv.Itoa(maxDist)).Result()
	if err != nil {
		return false, 0, fmt.Errorf("fingerprint redis script error: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return false, 0, fmt.Errorf("unexpected script response type: %v", res)
	}

	isLoop := resSlice[0].(int64) == 1
	count := resSlice[1].(int64)

	return isLoop, count, nil
}
