package keyring

import (
	"reflect"
	"testing"
)

func TestParseAllowedTools(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{"empty string", "", map[string]bool{}},
		{"single value", "tool1", map[string]bool{"tool1": true}},
		{"multiple values", "tool1,tool2,tool3", map[string]bool{"tool1": true, "tool2": true, "tool3": true}},
		{"with spaces", " tool1 , tool2 ", map[string]bool{"tool1": true, "tool2": true}},
		{"trailing comma", "tool1,", map[string]bool{"tool1": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &KeyMetadata{AllowedTools: tt.input}
			result := m.ParseAllowedTools()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseAllowedProviders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{"empty string", "", map[string]bool{}},
		{"single value", "openai", map[string]bool{"openai": true}},
		{"multiple values", "openai,anthropic", map[string]bool{"openai": true, "anthropic": true}},
		{"with spaces", " openai , anthropic ", map[string]bool{"openai": true, "anthropic": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &KeyMetadata{AllowedProviders: tt.input}
			result := m.ParseAllowedProviders()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{"empty string", "", map[string]string{}},
		{"single pair", "env=prod", map[string]string{"env": "prod"}},
		{"multiple pairs", "env=prod,team=alpha", map[string]string{"env": "prod", "team": "alpha"}},
		{"with spaces", " env = prod , team = alpha ", map[string]string{"env": "prod", "team": "alpha"}},
		{"missing value", "env", map[string]string{"env": ""}},
		{"extra equals", "env=prod=extra", map[string]string{"env": "prod=extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &KeyMetadata{Tags: tt.input}
			result := m.ParseTags()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
