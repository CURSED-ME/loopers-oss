package syntactic

import (
	"strings"
	"unicode"
)

// isDelimiterChar checks if a byte or rune is a common token splitting delimiter
func isDelimiterChar(r rune) bool {
	switch r {
	case '.', '_', '-', '/', '\\', '|', '*', '~':
		return true
	}
	return false
}

// isAlphanumeric checks ASCII alphanumeric
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// CollapseDelimiters collapses intra-word delimiter padding (e.g., "i.g.n.o.r.e" -> "ignore", "i_g_n_o_r_e" -> "ignore", "i/g/n/o/r/e" -> "ignore", "i g n o r e" -> "ignore").
// Preserves natural multi-word boundaries.
func CollapseDelimiters(s string) (string, bool) {
	if len(s) < 3 {
		return s, false
	}

	words := strings.Fields(s)
	if len(words) == 0 {
		return s, false
	}

	modified := false
	var resultWords []string

	// 1. First pass: collapse tokens with internal punctuation delimiters (e.g. i.g.n.o.r.e)
	for _, word := range words {
		runes := []rune(word)
		if len(runes) >= 3 {
			if isAlphanumeric(runes[0]) && isDelimiterChar(runes[1]) {
				delim := runes[1]
				isDelimited := true
				var collapsed strings.Builder

				for i := 0; i < len(runes); i++ {
					if i%2 == 0 {
						if !isAlphanumeric(runes[i]) {
							isDelimited = false
							break
						}
						collapsed.WriteRune(runes[i])
					} else {
						if runes[i] != delim {
							isDelimited = false
							break
						}
					}
				}

				if isDelimited && collapsed.Len() >= 2 {
					resultWords = append(resultWords, collapsed.String())
					modified = true
					continue
				}
			}
		}
		resultWords = append(resultWords, word)
	}

	// 2. Second pass: collapse sequences of 3 or more consecutive single-letter words (e.g. "i g n o r e" -> "ignore")
	var finalWords []string
	i := 0
	for i < len(resultWords) {
		if len([]rune(resultWords[i])) == 1 && isAlphanumeric([]rune(resultWords[i])[0]) {
			// Find extent of single letter run
			j := i
			for j < len(resultWords) && len([]rune(resultWords[j])) == 1 && isAlphanumeric([]rune(resultWords[j])[0]) {
				j++
			}
			runLen := j - i
			if runLen >= 3 {
				var sb strings.Builder
				for k := i; k < j; k++ {
					sb.WriteString(resultWords[k])
				}
				finalWords = append(finalWords, sb.String())
				modified = true
				i = j
				continue
			}
		}
		finalWords = append(finalWords, resultWords[i])
		i++
	}

	res := strings.Join(finalWords, " ")
	return res, modified || res != s
}

// leetspeakMap maps common numeric and symbolic substitutions to English ASCII letters
var leetspeakMap = map[rune]rune{
	'0': 'o',
	'1': 'i',
	'3': 'e',
	'4': 'a',
	'5': 's',
	'7': 't',
	'@': 'a',
	'$': 's',
	'+': 't',
	'!': 'i',
}

// FoldLeetspeak converts numeric and symbolic leetspeak substitutions to ASCII characters.
func FoldLeetspeak(s string) (string, bool) {
	if len(s) == 0 {
		return "", false
	}

	var sb strings.Builder
	sb.Grow(len(s))
	hasLeetspeak := false

	for _, r := range s {
		if mapped, ok := leetspeakMap[r]; ok {
			sb.WriteRune(mapped)
			hasLeetspeak = true
		} else {
			sb.WriteRune(unicode.ToLower(r))
		}
	}

	return sb.String(), hasLeetspeak
}
