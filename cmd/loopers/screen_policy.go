package main

import (
	"fmt"
	"os"

	"github.com/try-loopers/loopers-oss/cmd/loopers/ui"
	"github.com/try-loopers/loopers-oss/internal/policy"
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
		policyFile    = "./policies.yaml"
		presets       []string
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
				if v, ok := polCfg["policy_file"].(string); ok {
					policyFile = v
				}
				if v, ok := polCfg["presets"].([]interface{}); ok {
					for _, item := range v {
						if pStr, ok := item.(string); ok {
							presets = append(presets, pStr)
						}
					}
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
			huh.NewMultiSelect[string]().Title("Enable Safety & Compliance Presets (Optional)").Options(
				huh.NewOption("Safety Guardrails (SSN, Creds, Bash injection)", "safety"),
				huh.NewOption("Safety Drift (Multi-turn goal hijacking)", "safety_drift"),
				huh.NewOption("PCI-DSS Compliance (Credit Cards, CVV, SQLi)", "pci"),
				huh.NewOption("MCP Sandbox (Path traversal, FSM bash dry-run)", "mcp_sandbox"),
				huh.NewOption("Zero Trust (Persistent behavioral risk gating)", "zero_trust"),
				huh.NewOption("OWASP LLM Top 10 (Injection, agency, insecure output, secrets)", "owasp_llm_top10"),
				huh.NewOption("NIST AI RMF (Identity, containment, HITL escalation, drift)", "nist_ai_rmf"),
				huh.NewOption("EU AI Act (Prohibited practices, social scoring, biometric, oversight)", "eu_ai_act"),
			).Value(&presets),
			huh.NewInput().Title("Policy File (YAML Guardrails Card)").Value(&policyFile),
			huh.NewInput().Title("Policy Directory (Rego Files)").Value(&policyDir),
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
		"policy_file":    policyFile,
		"presets":        presets,
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
			ui.Warn("You must restart the firewall server for changes to take effect.")
		}
	}

	// Check the policy file to display status
	if policyFile != "" {
		fileData, err := os.ReadFile(policyFile)
		if err == nil {
			card, err := policy.ParseYAML(fileData)
			if err == nil {
				fmt.Printf("\nFound YAML Policy Card '%s' with %d rules:\n", card.Metadata.Name, len(card.Rules))
				for _, r := range card.Rules {
					fmt.Printf("  - %s (%s -> %s)\n", r.Name, r.Match.Type, r.Action)
				}
			} else {
				fmt.Printf("\nFound YAML Policy File '%s', but failed to parse: %v\n", policyFile, err)
			}
		} else if !os.IsNotExist(err) {
			fmt.Printf("\nError reading YAML Policy File '%s': %v\n", policyFile, err)
		} else {
			fmt.Printf("\nYAML Policy File '%s' does not exist.\n", policyFile)
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

		fmt.Printf("\nFound %d Rego policy files in %s:\n", len(regoFiles), policyDir)
		for _, file := range regoFiles {
			fmt.Printf("  - %s\n", file)
		}
	} else {
		fmt.Printf("\nNote: Rego policy directory '%s' does not exist or cannot be read.\n", policyDir)
	}

	if len(presets) > 0 {
		fmt.Printf("\nEnabled Safety Presets:\n")
		for _, p := range presets {
			fmt.Printf("  - %s\n", p)
		}
	}

	pressEnterToContinue()
}
