package syntactic

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// homoglyphMap maps non-ASCII confusable runes from Unicode TR39 (Cyrillic, Greek,
// Fullwidth, Enclosed, Mathematical Alphanumerics, Latin Extended) to standard lowercase ASCII runes.
var homoglyphMap = map[rune]rune{
	// --- Cyrillic Lowercase ---
	'а': 'a', // U+0430 Cyrillic Small Letter A
	'б': 'b', // U+0431 Cyrillic Small Letter Be (often used as b)
	'в': 'v', // U+0432 Cyrillic Small Letter Ve
	'г': 'r', // U+0433 Cyrillic Small Letter Ghe (looks like r)
	'д': 'd', // U+0434 Cyrillic Small Letter De
	'е': 'e', // U+0435 Cyrillic Small Letter Ie
	'ж': 'z', // U+0436 Cyrillic Small Letter Zhe
	'з': 'z', // U+0437 Cyrillic Small Letter Ze
	'і': 'i', // U+0456 Cyrillic Small Letter Byelorussian-Ukrainian I
	'ї': 'i', // U+0457 Cyrillic Small Letter Yi
	'ј': 'j', // U+0458 Cyrillic Small Letter Je
	'к': 'k', // U+043A Cyrillic Small Letter Ka
	'л': 'l', // U+043B Cyrillic Small Letter El
	'м': 'm', // U+043C Cyrillic Small Letter Em
	'н': 'h', // U+043D Cyrillic Small Letter En (looks like h)
	'о': 'o', // U+043E Cyrillic Small Letter O
	'п': 'n', // U+043F Cyrillic Small Letter Pe (looks like n)
	'р': 'p', // U+0440 Cyrillic Small Letter Er
	'с': 'c', // U+0441 Cyrillic Small Letter Es
	'т': 't', // U+0442 Cyrillic Small Letter Te
	'у': 'y', // U+0443 Cyrillic Small Letter U
	'ф': 'f', // U+0444 Cyrillic Small Letter Ef
	'х': 'x', // U+0445 Cyrillic Small Letter Ha
	'ц': 'u', // U+0446 Cyrillic Small Letter Tse
	'ч': 'y', // U+0447 Cyrillic Small Letter Che (looks like y or 4)
	'ш': 'w', // U+0448 Cyrillic Small Letter Sha (looks like w)
	'щ': 'w', // U+0449 Cyrillic Small Letter Shcha
	'ы': 'y', // U+044B Cyrillic Small Letter Yeru
	'ь': 'b', // U+044C Cyrillic Small Letter Soft Sign (looks like b)
	'э': 'e', // U+044D Cyrillic Small Letter E
	'ю': 'u', // U+044E Cyrillic Small Letter Yu
	'я': 'r', // U+044F Cyrillic Small Letter Ya (reversed R)
	'ѕ': 's', // U+0455 Cyrillic Small Letter Dze

	// --- Cyrillic Uppercase (mapped to lowercase ASCII) ---
	'А': 'a', 'В': 'b', 'С': 'c', 'Е': 'e', 'Н': 'h', 'І': 'i', 'Ї': 'i',
	'Ј': 'j', 'К': 'k', 'М': 'm', 'О': 'o', 'Р': 'p', 'Т': 't', 'Х': 'x',
	'У': 'y', 'Ѕ': 's',

	// --- Greek Lowercase ---
	'α': 'a', // U+03B1 Greek Small Letter Alpha
	'β': 'b', // U+03B2 Greek Small Letter Beta
	'γ': 'y', // U+03B3 Greek Small Letter Gamma
	'δ': 'd', // U+03B4 Greek Small Letter Delta
	'ε': 'e', // U+03B5 Greek Small Letter Epsilon
	'ζ': 'z', // U+03B6 Greek Small Letter Zeta
	'η': 'n', // U+03B7 Greek Small Letter Eta (looks like n)
	'θ': 'o', // U+03B8 Greek Small Letter Theta
	'ι': 'i', // U+03B9 Greek Small Letter Iota
	'κ': 'k', // U+03BA Greek Small Letter Kappa
	'λ': 'l', // U+03BB Greek Small Letter Lambda
	'μ': 'u', // U+03BC Greek Small Letter Mu (looks like u)
	'ν': 'v', // U+03BD Greek Small Letter Nu (looks like v)
	'ξ': 'e', // U+03BE Greek Small Letter Xi
	'ο': 'o', // U+03BF Greek Small Letter Omicron
	'π': 'n', // U+03C0 Greek Small Letter Pi
	'ρ': 'p', // U+03C1 Greek Small Letter Rho
	'σ': 'o', // U+03C3 Greek Small Letter Sigma
	'ς': 's', // U+03C2 Greek Small Letter Final Sigma
	'τ': 't', // U+03C4 Greek Small Letter Tau
	'υ': 'u', // U+03C5 Greek Small Letter Upsilon (looks like u/v)
	'φ': 'f', // U+03C6 Greek Small Letter Phi
	'χ': 'x', // U+03C7 Greek Small Letter Chi
	'ψ': 'y', // U+03C8 Greek Small Letter Psi
	'ω': 'w', // U+03C9 Greek Small Letter Omega

	// --- Greek Uppercase ---
	'Α': 'a', 'Β': 'b', 'Ε': 'e', 'Ζ': 'z', 'Η': 'h', 'Ι': 'i', 'Κ': 'k',
	'Μ': 'm', 'Ν': 'n', 'Ο': 'o', 'Ρ': 'p', 'Τ': 't', 'Υ': 'y', 'Χ': 'x',

	// --- Latin Extended & IPA Substitutions ---
	'ı': 'i', // U+0131 Latin Small Letter Dotless I
	'ȷ': 'j', // U+0237 Latin Small Letter Dotless J
	'ł': 'l', // U+0142 Latin Small Letter L with Stroke
	'ø': 'o', // U+00F8 Latin Small Letter O with Stroke
	'ß': 's', // U+00DF Latin Small Letter Sharp S
	'æ': 'a', // U+00E6 Latin Small Letter AE
	'œ': 'o', // U+0153 Latin Small Ligature OE
	'ð': 'd', // U+00F0 Latin Small Letter Eth
	'þ': 't', // U+00FE Latin Small Letter Thorn
	'ɡ': 'g', // U+0261 Latin Small Letter Script G
	'ɢ': 'g', // U+0262 Latin Letter Small Capital G
	'ɩ': 'i', // U+0269 Latin Small Letter Iota
	'ɪ': 'i', // U+026A Latin Letter Small Capital I
	'ʏ': 'y', // U+028F Latin Letter Small Capital Y
	'ʋ': 'v', // U+028B Latin Small Letter V with Hook
	'ʌ': 'v', // U+028C Latin Small Letter Turned V

	// --- Enclosed Alphanumerics (Circled/Parenthesized: U+2460–U+24FF) ---
	'①': '1', '②': '2', '③': '3', '④': '4', '⑤': '5',
	'⑥': '6', '⑦': '7', '⑧': '8', '⑨': '9', '⓪': '0',
	'ⓐ': 'a', 'ⓑ': 'b', 'ⓒ': 'c', 'ⓓ': 'd', 'ⓔ': 'e', 'ⓕ': 'f', 'ⓖ': 'g',
	'ⓗ': 'h', 'ⓘ': 'i', 'ⓙ': 'j', 'ⓚ': 'k', 'ⓛ': 'l', 'ⓜ': 'm', 'ⓝ': 'n',
	'ⓞ': 'o', 'ⓟ': 'p', 'ⓠ': 'q', 'ⓡ': 'r', 'ⓢ': 's', 'ⓣ': 't', 'ⓤ': 'u',
	'ⓥ': 'v', 'ⓦ': 'w', 'ⓧ': 'x', 'ⓨ': 'y', 'ⓩ': 'z',
	'Ⓐ': 'a', 'Ⓑ': 'b', 'Ⓒ': 'c', 'Ⓓ': 'd', 'Ⓔ': 'e', 'Ⓕ': 'f', 'Ⓖ': 'g',
	'Ⓗ': 'h', 'Ⓘ': 'i', 'Ⓙ': 'j', 'Ⓚ': 'k', 'Ⓛ': 'l', 'Ⓜ': 'm', 'Ⓝ': 'n',
	'Ⓞ': 'o', 'Ⓟ': 'p', 'Ⓠ': 'q', 'Ⓡ': 'r', 'Ⓢ': 's', 'Ⓣ': 't', 'Ⓤ': 'u',
	'Ⓥ': 'v', 'Ⓦ': 'w', 'Ⓧ': 'x', 'Ⓨ': 'y', 'Ⓩ': 'z',
}

// resolveMathAlphanumeric handles Mathematical Alphanumeric Symbols (U+1D400–U+1D7FF).
// These Unicode planes contain bold, italic, script, fraktur, double-struck, and monospace characters.
func resolveMathAlphanumeric(r rune) (rune, bool) {
	if r < 0x1D400 || r > 0x1D7FF {
		return r, false
	}

	blocks := []struct {
		startUpper rune
		startLower rune
	}{
		{0x1D400, 0x1D41A}, // Bold A-Z, a-z
		{0x1D434, 0x1D44E}, // Italic A-Z, a-z
		{0x1D468, 0x1D482}, // Bold Italic A-Z, a-z
		{0x1D49C, 0x1D4B6}, // Script A-Z, a-z
		{0x1D4D0, 0x1D4EA}, // Bold Script A-Z, a-z
		{0x1D504, 0x1D51E}, // Fraktur A-Z, a-z
		{0x1D538, 0x1D552}, // Double-struck A-Z, a-z
		{0x1D56C, 0x1D586}, // Bold Fraktur A-Z, a-z
		{0x1D5A0, 0x1D5BA}, // Sans-serif A-Z, a-z
		{0x1D5D4, 0x1D5EE}, // Sans-serif Bold A-Z, a-z
		{0x1D608, 0x1D622}, // Sans-serif Italic A-Z, a-z
		{0x1D63C, 0x1D656}, // Sans-serif Bold Italic A-Z, a-z
		{0x1D670, 0x1D68A}, // Monospace A-Z, a-z
	}

	for _, b := range blocks {
		if r >= b.startUpper && r <= b.startUpper+25 {
			return 'a' + (r - b.startUpper), true
		}
		if r >= b.startLower && r <= b.startLower+25 {
			return 'a' + (r - b.startLower), true
		}
	}

	// Mathematical digits (Bold, Double-struck, Sans-serif, etc.)
	digitBlocks := []rune{0x1D7CE, 0x1D7D8, 0x1D7E2, 0x1D7EC, 0x1D7F6}
	for _, db := range digitBlocks {
		if r >= db && r <= db+9 {
			return '0' + (r - db), true
		}
	}

	return r, false
}

// resolveFullwidth handles Fullwidth ASCII variants (U+FF01–U+FF5E).
func resolveFullwidth(r rune) (rune, bool) {
	if r >= 0xFF01 && r <= 0xFF5E {
		// Maps U+FF01 ('！') to 0x21 ('!'), U+FF41 ('ａ') to 'a'
		ascii := rune(r - 0xFEE0)
		return unicode.ToLower(ascii), true
	}
	return r, false
}

// CanonicalizeHomoglyphs maps confusable Unicode runes in a string to their ASCII equivalents.
// Returns the canonicalized string and a boolean indicating if any homoglyphs were substituted.
func CanonicalizeHomoglyphs(s string) (string, bool) {
	if len(s) == 0 {
		return "", false
	}

	// Fast path: Check if string is pure 7-bit ASCII
	isPureASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			isPureASCII = false
			break
		}
	}
	if isPureASCII {
		return strings.ToLower(s), false
	}

	// Apply NFKC normalization first to decompose compatibility characters
	nfkc := norm.NFKC.String(s)

	var sb strings.Builder
	sb.Grow(len(nfkc))
	hasHomoglyph := nfkc != s

	for _, r := range nfkc {
		// 1. Direct homoglyph map
		if mapped, ok := homoglyphMap[r]; ok {
			sb.WriteRune(mapped)
			hasHomoglyph = true
			continue
		}

		// 2. Math Alphanumeric Symbols
		if mapped, ok := resolveMathAlphanumeric(r); ok {
			sb.WriteRune(mapped)
			hasHomoglyph = true
			continue
		}

		// 3. Fullwidth forms
		if mapped, ok := resolveFullwidth(r); ok {
			sb.WriteRune(mapped)
			hasHomoglyph = true
			continue
		}

		// 4. Standard Unicode lowercase fallback
		lower := unicode.ToLower(r)
		if mapped, ok := homoglyphMap[lower]; ok {
			sb.WriteRune(mapped)
			hasHomoglyph = true
		} else {
			sb.WriteRune(lower)
		}
	}

	return sb.String(), hasHomoglyph
}
