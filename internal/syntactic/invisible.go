package syntactic

import "strings"

// InvisibleRunes contains invisible Unicode characters, zero-width joiners/spaces,
// directional overrides, soft hyphens, combining marks, variation selectors, and invisible operators.
var InvisibleRunes = map[rune]bool{
	// Zero-Width
	'\u200B': true, // Zero-Width Space (ZWSP)
	'\u200C': true, // Zero-Width Non-Joiner (ZWNJ)
	'\u200D': true, // Zero-Width Joiner (ZWJ)
	'\uFEFF': true, // Zero-Width No-Break Space / Byte Order Mark (BOM)
	'\u2060': true, // Word Joiner (WJ)

	// Bidi & Directional Overrides / Isolates
	'\u200E': true, // Left-to-Right Mark (LRM)
	'\u200F': true, // Right-to-Left Mark (RLM)
	'\u202A': true, // Left-to-Right Embedding (LRE)
	'\u202B': true, // Right-to-Left Embedding (RLE)
	'\u202C': true, // Pop Directional Formatting (PDF)
	'\u202D': true, // Left-to-Right Override (LRO)
	'\u202E': true, // Right-to-Left Override (RLO)
	'\u2066': true, // Left-to-Right Isolate (LRI)
	'\u2067': true, // Right-to-Left Isolate (RLI)
	'\u2068': true, // First Strong Isolate (FSI)
	'\u2069': true, // Pop Directional Isolate (PDI)

	// Invisible Spaces, Hyphens & Combining Graphemes
	'\u00AD': true, // Soft Hyphen (SHY)
	'\u034F': true, // Combining Grapheme Joiner (CGJ)
	'\u180E': true, // Mongolian Vowel Separator (MVS)
	'\u2000': true, // En Quad (often used for whitespace splitting)
	'\u2001': true, // Em Quad
	'\u2002': true, // En Space
	'\u2003': true, // Em Space
	'\u2004': true, // Three-Per-Em Space
	'\u2005': true, // Four-Per-Em Space
	'\u2006': true, // Six-Per-Em Space
	'\u2007': true, // Figure Space
	'\u2008': true, // Punctuation Space
	'\u2009': true, // Thin Space
	'\u200A': true, // Hair Space
	'\u202F': true, // Narrow No-Break Space
	'\u205F': true, // Medium Mathematical Space

	// Invisible Mathematical Operators
	'\u2061': true, // Function Application
	'\u2062': true, // Invisible Times
	'\u2063': true, // Invisible Separator
	'\u2064': true, // Invisible Plus

	// Variation Selectors (U+FE00–U+FE0F)
	'\uFE00': true, '\uFE01': true, '\uFE02': true, '\uFE03': true,
	'\uFE04': true, '\uFE05': true, '\uFE06': true, '\uFE07': true,
	'\uFE08': true, '\uFE09': true, '\uFE0A': true, '\uFE0B': true,
	'\uFE0C': true, '\uFE0D': true, '\uFE0E': true, '\uFE0F': true,
}

// StripInvisibleCharacters purges invisible, zero-width, bidi formatting, and non-printable control characters.
// Returns the cleaned string and a boolean indicating whether any invisible runes were removed.
func StripInvisibleCharacters(s string) (string, bool) {
	if len(s) == 0 {
		return "", false
	}

	var sb strings.Builder
	sb.Grow(len(s))
	hasInvisible := false

	for _, r := range s {
		// 1. Check known invisible table
		if InvisibleRunes[r] {
			hasInvisible = true
			continue
		}

		// 2. Filter ASCII control characters (except standard newline, carriage return, and tab)
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			hasInvisible = true
			continue
		}
		if r == 0x7F { // DEL
			hasInvisible = true
			continue
		}

		// 3. Filter Supplementary Variation Selectors (U+E0100–U+E01EF)
		if r >= 0xE0100 && r <= 0xE01EF {
			hasInvisible = true
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String(), hasInvisible
}
