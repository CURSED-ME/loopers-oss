package keyring

import (
	"regexp"
	"strings"
)

var (
	loopersKeyRegex   = regexp.MustCompile(`^lp-[a-zA-Z0-9]{43}$`)
	openaiKeyRegex    = regexp.MustCompile(`^sk-[A-Za-z0-9]{32,}$`)
	anthropicKeyRegex = regexp.MustCompile(`^sk-ant-api03-[A-Za-z0-9]{93,}$`)
)

// ValidateLoopersKey checks if a raw loopers key format is correct.
func ValidateLoopersKey(key string) bool {
	return loopersKeyRegex.MatchString(key)
}

// ValidateProviderKey checks if a provider key format is syntactically valid.
func ValidateProviderKey(provider, key string) bool {
	key = strings.TrimSpace(key)
	if provider == "openai" {
		return openaiKeyRegex.MatchString(key)
	} else if provider == "anthropic" {
		return anthropicKeyRegex.MatchString(key)
	}
	return false
}
