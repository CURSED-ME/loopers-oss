package syntactic

import (
	"encoding/base64"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// base64TokenRegex identifies candidate base64 encoded strings (length >= 16)
	base64TokenRegex = regexp.MustCompile(`\b[A-Za-z0-9+/]{16,}={0,2}\b|\b[A-Za-z0-9-_]{16,}={0,2}\b`)

	// hexEscapeRegex matches \xHH or 0xHH patterns
	hexEscapeRegex = regexp.MustCompile(`(?i)(?:\\x|0x)([0-9a-f]{2})`)

	// unicodeEscapeRegex matches \uHHHH patterns
	unicodeEscapeRegex = regexp.MustCompile(`(?i)\\u([0-9a-f]{4})`)
)

// DecodeURLEncoding iteratively decodes URL percent-encoded strings up to maxDepth times.
// Handles single (%69) and double (%2569) percent-encoded sequences.
func DecodeURLEncoding(s string, maxDepth int) (string, bool) {
	if !strings.Contains(s, "%") || maxDepth <= 0 {
		return s, false
	}

	current := s
	modified := false

	for i := 0; i < maxDepth; i++ {
		// Only attempt if '%' is followed by hex digits or percent
		if !strings.Contains(current, "%") {
			break
		}
		decoded, err := url.QueryUnescape(current)
		if err != nil || decoded == current {
			break
		}
		current = decoded
		modified = true
	}

	return current, modified
}

// DecodeHTMLEntities decodes HTML named, decimal, and hexadecimal character entities.
func DecodeHTMLEntities(s string) (string, bool) {
	if !strings.Contains(s, "&") {
		return s, false
	}
	decoded := html.UnescapeString(s)
	return decoded, decoded != s
}

// DecodeEscapeSequences converts \xHH, 0xHH, and \uHHHH escape sequences to UTF-8 text.
func DecodeEscapeSequences(s string) (string, bool) {
	if !strings.Contains(s, `\x`) && !strings.Contains(s, `0x`) && !strings.Contains(s, `\u`) {
		return s, false
	}

	modified := false

	// Replace \uHHHH
	res := unicodeEscapeRegex.ReplaceAllStringFunc(s, func(match string) string {
		hexStr := match[2:]
		val, err := strconv.ParseInt(hexStr, 16, 32)
		if err == nil && utf8.ValidRune(rune(val)) {
			modified = true
			return string(rune(val))
		}
		return match
	})

	// Replace \xHH and 0xHH
	res = hexEscapeRegex.ReplaceAllStringFunc(res, func(match string) string {
		hexStr := match[2:]
		val, err := strconv.ParseUint(hexStr, 16, 8)
		if err == nil {
			modified = true
			return string([]byte{byte(val)})
		}
		return match
	})

	return res, modified
}

// ExtractBase64Layers scans a string for embedded base64 blocks, decodes them,
// and returns all valid, readable UTF-8 substrings discovered.
func ExtractBase64Layers(s string) []string {
	if len(s) < 16 {
		return nil
	}

	matches := base64TokenRegex.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}

	var results []string
	seen := make(map[string]bool)

	for _, token := range matches {
		// Attempt standard Base64 decoding
		var decoded []byte
		var err error

		if strings.ContainsAny(token, "-_") {
			decoded, err = base64.URLEncoding.DecodeString(padBase64(token))
		} else {
			decoded, err = base64.StdEncoding.DecodeString(padBase64(token))
		}

		if err == nil && len(decoded) > 0 && utf8.Valid(decoded) {
			decodedStr := string(decoded)
			// Filter out pure binary / high-entropy noise: verify printable ratio
			if isPrintableText(decodedStr) {
				if !seen[decodedStr] && decodedStr != token {
					seen[decodedStr] = true
					results = append(results, decodedStr)
				}
			}
		}
	}

	return results
}

func padBase64(s string) string {
	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}
	return s
}

// isPrintableText checks if at least 80% of runes in a string are printable ASCII or common whitespace.
func isPrintableText(s string) bool {
	if len(s) == 0 {
		return false
	}
	printableCount := 0
	totalRunes := 0

	for _, r := range s {
		totalRunes++
		if (r >= 0x20 && r <= 0x7E) || r == '\n' || r == '\r' || r == '\t' {
			printableCount++
		}
	}
	if totalRunes == 0 {
		return false
	}
	return float64(printableCount)/float64(totalRunes) >= 0.80
}

// RecursivelyDecodeEncodings runs up to maxDepth passes of URL, HTML, and Escape decoding.
func RecursivelyDecodeEncodings(s string, maxDepth int) (string, bool) {
	if maxDepth <= 0 {
		return s, false
	}

	current := s
	anyModified := false

	for depth := 0; depth < maxDepth; depth++ {
		iterModified := false

		// 1. URL Unescape
		if decoded, mod := DecodeURLEncoding(current, 1); mod {
			current = decoded
			iterModified = true
		}

		// 2. HTML Unescape
		if decoded, mod := DecodeHTMLEntities(current); mod {
			current = decoded
			iterModified = true
		}

		// 3. Escape Sequence Unescape
		if decoded, mod := DecodeEscapeSequences(current); mod {
			current = decoded
			iterModified = true
		}

		if !iterModified {
			break
		}
		anyModified = true
	}

	return current, anyModified
}
