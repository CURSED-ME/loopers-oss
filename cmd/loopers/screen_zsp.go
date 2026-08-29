package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/try-loopers/loopers-oss/cmd/loopers/ui"
	"gopkg.in/yaml.v3"
)

func screenZSP() {
	ui.PrintLogo()
	ui.PrintHeader("Zero-Trust Security (ZSP & A2A) Configuration")
	fmt.Println()

	var (
		enabled           = true
		jwtOnly           = true
		jwksUrl           = ""
		escalationEnabled = true
		escalationSecret  = ""
	)

	// Attempt to read current loopers.yaml to pre-fill
	data, err := os.ReadFile("loopers.yaml")
	if err == nil {
		var cfg map[string]interface{}
		if yaml.Unmarshal(data, &cfg) == nil {
			if zCfg, ok := cfg["zsp"].(map[string]interface{}); ok {
				if v, ok := zCfg["enabled"].(bool); ok {
					enabled = v
				}
				if v, ok := zCfg["jwt_only"].(bool); ok {
					jwtOnly = v
				}
				if v, ok := zCfg["jwks_url"].(string); ok {
					jwksUrl = v
				}
				if v, ok := zCfg["escalation_enabled"].(bool); ok {
					escalationEnabled = v
				}
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Enable Agent-to-Agent Mesh? (Recommended: Yes)").Value(&enabled),
			huh.NewConfirm().Title("Require JWT Verification? (Recommended: Yes)").Value(&jwtOnly),
			huh.NewInput().Title("JWKS URL (Optional)").Value(&jwksUrl).Description("URL to fetch public keys for JWT validation"),
		).Title("Zero-Trust Security Protocol"),

		huh.NewGroup(
			huh.NewConfirm().Title("Enable Escalation Broker? (Recommended: Yes)").Description("Allow manual budget overrides via UI").Value(&escalationEnabled),
			huh.NewInput().Title("Escalation HMAC Secret (Optional)").Value(&escalationSecret).EchoMode(huh.EchoModePassword),
		).Title("Escalations"),
	).WithTheme(ui.GetHuhTheme()).WithKeyMap(ui.GetHuhKeyMap()).Run()

	if err != nil {
		return
	}

	// Read existing yaml, map to map[string]interface{}, update zsp block, and write back
	data, err = os.ReadFile("loopers.yaml")
	var root map[string]interface{}
	if err == nil {
		yaml.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	zspBlock := map[string]interface{}{
		"enabled":            enabled,
		"jwt_only":           jwtOnly,
		"jwks_url":           jwksUrl,
		"escalation_enabled": escalationEnabled,
	}

	// Keep existing secret if empty input
	if escalationSecret != "" {
		zspBlock["escalation_secret"] = escalationSecret
	} else {
		if existingZsp, ok := root["zsp"].(map[string]interface{}); ok {
			if secret, ok := existingZsp["escalation_secret"]; ok {
				zspBlock["escalation_secret"] = secret
			}
		}
	}

	root["zsp"] = zspBlock

	out, err := yaml.Marshal(&root)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to encode YAML: %v", err))
	} else {
		err = os.WriteFile("loopers.yaml", out, 0600)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to write loopers.yaml: %v", err))
		} else {
			ui.Success("Updated zsp configuration in loopers.yaml")
			ui.Warn("You must restart the firewall server for changes to take effect.")
		}
	}

	pressEnterToContinue()
}
