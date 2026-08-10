package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/CURSED-ME/loopers-oss/cmd/loopers/ui"
	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"
)

func screenLoop() {
	ui.PrintLogo()
	ui.PrintHeader("Loop & Stall Detection Configuration")
	fmt.Println()

	var (
		enabled               = true
		similarityThreshold   = "0.95"
		fingerprintThreshold  = "3"
		fingerprintWindowSecs = "60"
		defeatPadding         = true

		maxRPS              = "5"
		maxEndpointRepeats  = "10"
		repeatWindowSeconds = "30"

		minHammingDistance    = "5"
		lowDiversityThreshold = "5"
		stallAction           = "warn"
	)

	// Attempt to read current loopers.yaml to pre-fill
	// Ignore errors, we'll just use defaults if it doesn't exist
	data, err := os.ReadFile("loopers.yaml")
	if err == nil {
		var cfg map[string]interface{}
		if yaml.Unmarshal(data, &cfg) == nil {
			if loopCfg, ok := cfg["loop_detection"].(map[string]interface{}); ok {
				if v, ok := loopCfg["enabled"].(bool); ok {
					enabled = v
				}
				if fpCfg, ok := loopCfg["fingerprint"].(map[string]interface{}); ok {
					if v, ok := fpCfg["similarity_threshold"].(float64); ok {
						similarityThreshold = fmt.Sprintf("%g", v)
					}
					if v, ok := fpCfg["threshold"].(int); ok {
						fingerprintThreshold = fmt.Sprintf("%d", v)
					}
					if v, ok := fpCfg["window_seconds"].(int); ok {
						fingerprintWindowSecs = fmt.Sprintf("%d", v)
					}
					if v, ok := fpCfg["defeat_padding"].(bool); ok {
						defeatPadding = v
					}
				}
				if velCfg, ok := loopCfg["velocity"].(map[string]interface{}); ok {
					if v, ok := velCfg["max_rps"].(float64); ok {
						maxRPS = fmt.Sprintf("%g", v)
					}
					if v, ok := velCfg["max_endpoint_repeats"].(int); ok {
						maxEndpointRepeats = fmt.Sprintf("%d", v)
					}
					if v, ok := velCfg["repeat_window_seconds"].(int); ok {
						repeatWindowSeconds = fmt.Sprintf("%d", v)
					}
				}
				if stallCfg, ok := loopCfg["stall"].(map[string]interface{}); ok {
					if v, ok := stallCfg["min_hamming_distance"].(int); ok {
						minHammingDistance = fmt.Sprintf("%d", v)
					}
					if v, ok := stallCfg["low_diversity_threshold"].(int); ok {
						lowDiversityThreshold = fmt.Sprintf("%d", v)
					}
					if v, ok := stallCfg["action"].(string); ok {
						stallAction = v
					}
				}
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Enable Loop Detection (Recommended: Yes)").Value(&enabled),
			huh.NewInput().Title("Similarity Threshold (0.0 to 1.0) (Recommended: 0.95)").Value(&similarityThreshold),
			huh.NewInput().Title("Fingerprint Threshold (repeats) (Recommended: 3)").Value(&fingerprintThreshold),
			huh.NewInput().Title("Window Seconds (Recommended: 60s)").Value(&fingerprintWindowSecs),
			huh.NewConfirm().Title("Defeat Padding? (Recommended: Yes)").Description("Detect identical content padded with junk").Value(&defeatPadding),
		).Title("Fingerprinting"),

		huh.NewGroup(
			huh.NewInput().Title("Max RPS (Recommended: 5)").Value(&maxRPS),
			huh.NewInput().Title("Max Endpoint Repeats (Recommended: 10)").Value(&maxEndpointRepeats),
			huh.NewInput().Title("Repeat Window Seconds (Recommended: 30s)").Value(&repeatWindowSeconds),
		).Title("Velocity Limits"),

		huh.NewGroup(
			huh.NewInput().Title("Min Hamming Distance (Recommended: 5)").Value(&minHammingDistance),
			huh.NewInput().Title("Low Diversity Threshold (Recommended: 5)").Value(&lowDiversityThreshold),
			huh.NewSelect[string]().Title("Stall Action (Recommended: Warn)").Options(
				huh.NewOption("Warn", "warn"),
				huh.NewOption("Block", "block"),
			).Value(&stallAction),
		).Title("Stall Detection"),
	).WithTheme(ui.GetHuhTheme()).WithKeyMap(ui.GetHuhKeyMap()).Run()

	if err != nil {
		return
	}

	// Read existing yaml, map to map[string]interface{}, update loop_detection block, and write back
	data, err = os.ReadFile("loopers.yaml")
	var root map[string]interface{}
	if err == nil {
		yaml.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	// Update block
	loopBlock := map[string]interface{}{
		"enabled": enabled,
		"fingerprint": map[string]interface{}{
			"similarity_threshold": parseFloat(similarityThreshold),
			"threshold":            parseInt(fingerprintThreshold),
			"window_seconds":       parseInt(fingerprintWindowSecs),
			"defeat_padding":       defeatPadding,
		},
		"velocity": map[string]interface{}{
			"max_rps":               parseFloat(maxRPS),
			"max_endpoint_repeats":  parseInt(maxEndpointRepeats),
			"repeat_window_seconds": parseInt(repeatWindowSeconds),
		},
		"stall": map[string]interface{}{
			"min_hamming_distance":    parseInt(minHammingDistance),
			"low_diversity_threshold": parseInt(lowDiversityThreshold),
			"action":                  stallAction,
		},
	}
	root["loop_detection"] = loopBlock

	out, err := yaml.Marshal(&root)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to encode YAML: %v", err))
	} else {
		err = os.WriteFile("loopers.yaml", out, 0600)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to write loopers.yaml: %v", err))
		} else {
			ui.Success("Updated loop_detection configuration in loopers.yaml")
			ui.Warn("You must restart the proxy server for changes to take effect.")
		}
	}

	pressEnterToContinue()
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
