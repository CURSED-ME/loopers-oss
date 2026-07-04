package loop

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var memberCounter atomic.Uint64

// computeSimHash generates a 64-bit Locality Sensitive Hash using 3-byte trigrams.
// Preserved for Stall detection.
func computeSimHash(data []byte) uint64 {
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

// ExtractBiGrams extracts a sorted, deduplicated slice of 16-bit bi-grams from the data.
func ExtractBiGrams(data []byte) []uint16 {
	if len(data) < 2 {
		return []uint16{}
	}
	set := make(map[uint16]struct{})
	for i := 0; i < len(data)-1; i++ {
		val := binary.LittleEndian.Uint16(data[i : i+2])
		set[val] = struct{}{}
	}
	grams := make([]uint16, 0, len(set))
	for k := range set {
		grams = append(grams, k)
	}
	sort.Slice(grams, func(i, j int) bool { return grams[i] < grams[j] })
	return grams
}

// EncodeBiGrams encodes a sorted slice of uint16 into a base64 string.
func EncodeBiGrams(grams []uint16) string {
	buf := make([]byte, len(grams)*2)
	for i, g := range grams {
		binary.LittleEndian.PutUint16(buf[i*2:], g)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// DecodeBiGrams decodes a base64 string into a sorted slice of uint16.
func DecodeBiGrams(b64 string) ([]uint16, error) {
	buf, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(buf)%2 != 0 {
		return nil, fmt.Errorf("invalid bigram byte length")
	}
	grams := make([]uint16, len(buf)/2)
	for i := 0; i < len(grams); i++ {
		grams[i] = binary.LittleEndian.Uint16(buf[i*2 : i*2+2])
	}
	return grams, nil
}

// JaccardSimilarity calculates the exact Jaccard similarity score between two sorted uint16 slices.
func JaccardSimilarity(a, b []uint16) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
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
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// NormalizeAndHash strips volatile fields from the body and returns:
// 1. base64 encoded bi-gram set representation (for Jaccard fingerprinting)
// 2. 64-bit SimHash (for Stall checking)
// 3. error if any
func NormalizeAndHash(body []byte) (string, uint64, error) {
	var normalized []byte
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(body, &raw); err != nil {
		normalized = body
	} else {
		volatileFields := []string{"stream", "max_tokens", "temperature", "seed", "n", "user"}
		for _, f := range volatileFields {
			delete(raw, f)
		}
		var err error
		normalized, err = json.Marshal(raw)
		if err != nil {
			return "", 0, err
		}
	}

	grams := ExtractBiGrams(normalized)
	b64Grams := EncodeBiGrams(grams)
	simHash := computeSimHash(normalized)

	return b64Grams, simHash, nil
}

// CheckFingerprint checks whether requests with Jaccard similarity >= SimilarityThreshold
// have appeared >= threshold times in the sliding window. Returns (isLoop, count, error).
func (d *Detector) CheckFingerprint(ctx context.Context, sessionID, currentB64 string) (bool, int64, error) {
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

	thresholdSim := d.cfg.Fingerprint.SimilarityThreshold
	if thresholdSim <= 0.0 {
		thresholdSim = 0.95
	}

	currGrams, err := DecodeBiGrams(currentB64)
	if err != nil {
		return false, 0, fmt.Errorf("failed to decode current bi-grams: %w", err)
	}

	res, err := d.fpScript.Run(ctx, d.rdb, []string{ringKey}, currentB64, strconv.FormatInt(now, 10), strconv.FormatInt(window, 10), memberID).Result()
	if err != nil {
		return false, 0, fmt.Errorf("fingerprint redis script error: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok {
		return false, 0, fmt.Errorf("unexpected script response type: %T", res)
	}

	matchCount := int64(1) // count current request
	for _, m := range resSlice {
		mStr, ok := m.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(mStr, ":", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		prevGrams, err := DecodeBiGrams(parts[0])
		if err != nil {
			continue
		}
		sim := JaccardSimilarity(currGrams, prevGrams)
		if sim >= thresholdSim {
			matchCount++
		}
	}

	targetThreshold := int64(d.cfg.Fingerprint.Threshold)
	if targetThreshold <= 0 {
		targetThreshold = 3
	}

	isLoop := matchCount >= targetThreshold
	return isLoop, matchCount, nil
}
