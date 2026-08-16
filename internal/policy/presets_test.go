package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPreset(t *testing.T) {
	presets := []string{"safety", "pci", "mcp_sandbox", "zero_trust"}
	for _, p := range presets {
		data, err := GetPreset(p)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		card, err := ParseYAML(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, card.Metadata.Name)

		regoCode, err := TranspileToRego(card)
		assert.NoError(t, err)
		assert.Contains(t, regoCode, "package loopers.policy")
	}

	_, err := GetPreset("non_existent")
	assert.Error(t, err)
}

func TestDefaultPresetLoading(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		PolicyDir:  t.TempDir(),
		PolicyFile: "",
	}
	engine, err := NewEngine(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	assert.Equal(t, []string{"safety"}, engine.cfg.Presets)
	assert.Contains(t, engine.modules, "preset:safety")
}
