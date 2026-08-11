package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type piModelsFile struct {
	Providers map[string]any `json:"providers"`
}

// injectPiProvider writes a Loopers proxy provider into ~/.pi/agent/models.json
// and returns a cleanup function to remove it after execution.
func injectPiProvider(localProxyURL string) (func(), error) {
	path := piModelsJSONPath()

	// Read existing config or start fresh
	raw, err := os.ReadFile(path)
	var config piModelsFile
	if err == nil {
		_ = json.Unmarshal(raw, &config)
	}
	if config.Providers == nil {
		config.Providers = map[string]any{}
	}

	// Inject loopers provider
	config.Providers["loopers"] = map[string]any{
		"baseUrl": localProxyURL,
		"api":     "openai-completions",
		"apiKey":  "loopers-managed",
		"models": []map[string]string{
			{"id": "loopers-proxy", "name": "Loopers Proxy"},
		},
	}

	if err := writeJSONAtomic(path, config); err != nil {
		return nil, err
	}

	cleanup := func() {
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg piModelsFile
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return
		}
		delete(cfg.Providers, "loopers")
		_ = writeJSONAtomic(path, cfg)
	}
	return cleanup, nil
}

func piModelsJSONPath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".pi", "agent", "models.json")
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
