package syntactic

import "strings"

// ObfuscationReport contains structured detection metadata from syntactic analysis.
type ObfuscationReport struct {
	ObfuscationDetected bool     `json:"obfuscation_detected"`
	HasHomoglyphs       bool     `json:"has_homoglyphs"`
	HasInvisibleChars   bool     `json:"has_invisible_chars"`
	HasBase64Payloads   bool     `json:"has_base64_payloads"`
	HasEncodingAttacks  bool     `json:"has_encoding_attacks"`
	HasDelimPadding     bool     `json:"has_delim_padding"`
	NormalizedText      string   `json:"normalized_text"`
	DecodedLayers       []string `json:"decoded_layers,omitempty"`
}

// Config controls syntactic normalization parameters.
type Config struct {
	Enabled             bool `mapstructure:"enabled"`
	StripInvisibleChars bool `mapstructure:"strip_invisible_chars"`
	NormalizeHomoglyphs bool `mapstructure:"normalize_homoglyphs"`
	DecodeEncodings     bool `mapstructure:"decode_encodings"`
	CollapseDelimiters  bool `mapstructure:"collapse_delimiters"`
	MaxDecodeDepth      int  `mapstructure:"max_decode_depth"`
}

// DefaultConfig returns recommended production defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		StripInvisibleChars: true,
		NormalizeHomoglyphs: true,
		DecodeEncodings:     true,
		CollapseDelimiters:  true,
		MaxDecodeDepth:      3,
	}
}

// Normalize applies the complete 4-stage syntactic normalization pipeline to a string:
// 1. Recursive encoding decoding (URL, Hex, HTML)
// 2. Invisible & control character stripping
// 3. Unicode TR39 homoglyph canonicalization
// 4. Intra-word delimiter collapsing & whitespace normalization
// Returns the lowercase, fully de-obfuscated canonical string.
func Normalize(input string) string {
	if len(input) == 0 {
		return ""
	}

	// Fast Path: If input is pure printable ASCII (0x20 - 0x7E) with no escape symbols ('%', '&', '\')
	isFastASCII := true
	for i := 0; i < len(input); i++ {
		b := input[i]
		if b < 0x20 || b > 0x7E || b == '%' || b == '&' || b == '\\' {
			isFastASCII = false
			break
		}
	}
	if isFastASCII {
		// Quick check if delimiters exist in fast ASCII
		if strings.ContainsAny(input, "._-/") {
			collapsed, _ := CollapseDelimiters(input)
			return strings.ToLower(collapsed)
		}
		return strings.ToLower(input)
	}

	// Stage 1: Recursive Encodings (URL, HTML, Escape Sequences)
	decoded, _ := RecursivelyDecodeEncodings(input, 3)

	// Stage 2: Strip Invisible & Control Characters
	clean, _ := StripInvisibleCharacters(decoded)

	// Stage 3: Unicode TR39 Homoglyph & Math Alphanumerics Canonicalization
	canonical, _ := CanonicalizeHomoglyphs(clean)

	// Stage 4: Delimiter Collapsing
	final, _ := CollapseDelimiters(canonical)

	return strings.ToLower(final)
}

// ExtractAllTextLayers extracts candidate decoded strings across all obfuscation layers.
// This includes:
// - Original text
// - Normalized text
// - Extracted and decoded Base64 blocks (with normalization applied)
// - Recursively unescaped URL/HTML/Hex representations
func ExtractAllTextLayers(input string) []string {
	if len(input) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var layers []string

	addLayer := func(s string) {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			layers = append(layers, trimmed)
		}
	}

	// 1. Raw input
	addLayer(input)

	// 2. Normalized representation
	normalized := Normalize(input)
	addLayer(normalized)

	// 3. Invisible character space-substitution layer (catches ZWSP/ZWNJ used as word boundaries)
	var spaceSub strings.Builder
	hasInv := false
	for _, r := range input {
		if InvisibleRunes[r] {
			spaceSub.WriteRune(' ')
			hasInv = true
		} else {
			spaceSub.WriteRune(r)
		}
	}
	if hasInv {
		addLayer(spaceSub.String())
		addLayer(Normalize(spaceSub.String()))
	}

	// 4. Base64 payload layers
	b64Blocks := ExtractBase64Layers(input)
	for _, block := range b64Blocks {
		addLayer(block)
		addLayer(Normalize(block))
	}

	// 5. Also scan for Base64 inside decoded encodings
	if decoded, mod := RecursivelyDecodeEncodings(input, 3); mod {
		addLayer(decoded)
		addLayer(Normalize(decoded))
		nestedB64 := ExtractBase64Layers(decoded)
		for _, block := range nestedB64 {
			addLayer(block)
			addLayer(Normalize(block))
		}
	}

	return layers
}

// AnalyzeObfuscation inspects a string and generates an ObfuscationReport with detailed telemetry.
func AnalyzeObfuscation(input string) ObfuscationReport {
	report := ObfuscationReport{
		NormalizedText: input,
		DecodedLayers:  ExtractAllTextLayers(input),
	}

	if len(input) == 0 {
		return report
	}

	// 1. Check Encodings
	_, hasEncodings := RecursivelyDecodeEncodings(input, 3)
	report.HasEncodingAttacks = hasEncodings

	// 2. Check Base64
	b64Layers := ExtractBase64Layers(input)
	report.HasBase64Payloads = len(b64Layers) > 0

	// 3. Check Invisible Chars
	_, hasInvisible := StripInvisibleCharacters(input)
	report.HasInvisibleChars = hasInvisible

	// 4. Check Homoglyphs
	_, hasHomoglyphs := CanonicalizeHomoglyphs(input)
	report.HasHomoglyphs = hasHomoglyphs

	// 5. Check Delimiters
	_, hasDelims := CollapseDelimiters(input)
	report.HasDelimPadding = hasDelims

	// Set overall detection status
	if report.HasHomoglyphs || report.HasInvisibleChars || report.HasBase64Payloads || report.HasEncodingAttacks || report.HasDelimPadding {
		report.ObfuscationDetected = true
	}

	report.NormalizedText = Normalize(input)
	return report
}
