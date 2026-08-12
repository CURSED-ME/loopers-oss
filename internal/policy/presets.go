package policy

import (
	"embed"
	"fmt"
)

//go:embed presets/*.yaml
var presetFS embed.FS

// GetPreset returns the raw YAML data for a given preset name.
func GetPreset(name string) ([]byte, error) {
	data, err := presetFS.ReadFile(fmt.Sprintf("presets/%s.yaml", name))
	if err != nil {
		return nil, fmt.Errorf("preset %q not found: %w", name, err)
	}
	return data, nil
}
