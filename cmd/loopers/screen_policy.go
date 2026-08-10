package main

import (
	"fmt"
	"os"

	"github.com/CURSED-ME/loopers-oss/cmd/loopers/ui"
	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"
)

func screenPolicy() {
	ui.PrintLogo()
	ui.PrintHeader("OPA Policy Engine Configuration")
	fmt.Println()

	var (
		enabled       = true
		policyDir     = "./policies"
		defaultAction = "allow"
	)

	// Attempt to read current loopers.yaml to pre-fill
	data, err := os.ReadFile("loopers.yaml")
	if err == nil {
		var cfg map[string]interface{}
		if yaml.Unmarshal(data, &cfg) == nil {
			if polCfg, ok := cfg["policy"].(map[string]interface{}); ok {
				if v, ok := polCfg["enabled"].(bool); ok {
					enabled = v
				}
				if v, ok := polCfg["policy_dir"].(string); ok {
					policyDir = v
				}
				if v, ok := polCfg["default_action"].(string); ok {
					defaultAction = v
				}
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Enable Policy Enforcement? (Recommended: Yes)").Value(&enabled),
			huh.NewInput().Title("Policy Directory (Recommended: ./policies)").Value(&policyDir),
			huh.NewSelect[string]().Title("Default Action (Recommended: Allow)").Options(
				huh.NewOption("Allow", "allow"),
				huh.NewOption("Deny", "deny"),
			).Value(&defaultAction),
		).Title("Policy Engine"),
	).WithTheme(ui.GetHuhTheme()).WithKeyMap(ui.GetHuhKeyMap()).Run()

	if err != nil {
		return
	}

	// Read existing yaml, map to map[string]interface{}, update policy block, and write back
	data, err = os.ReadFile("loopers.yaml")
	var root map[string]interface{}
	if err == nil {
		yaml.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	root["policy"] = map[string]interface{}{
		"enabled":        enabled,
		"policy_dir":     policyDir,
		"default_action": defaultAction,
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to encode YAML: %v", err))
	} else {
		err = os.WriteFile("loopers.yaml", out, 0600)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to write loopers.yaml: %v", err))
		} else {
			ui.Success("Updated policy configuration in loopers.yaml")
			ui.Warn("You must restart the proxy server for changes to take effect.")
		}
	}

	// Check the policy directory for .rego files to display status
	entries, err := os.ReadDir(policyDir)
	if err == nil {
		var regoFiles []string
		for _, e := range entries {
			if !e.IsDir() && len(e.Name()) > 5 && e.Name()[len(e.Name())-5:] == ".rego" {
				regoFiles = append(regoFiles, e.Name())
			}
		}
		
		fmt.Printf("\nFound %d policy files in %s:\n", len(regoFiles), policyDir)
		for _, file := range regoFiles {
			fmt.Printf("  - %s\n", file)
		}
	} else {
		fmt.Printf("\nNote: Policy directory '%s' does not exist or cannot be read.\n", policyDir)
	}

	pressEnterToContinue()
}
