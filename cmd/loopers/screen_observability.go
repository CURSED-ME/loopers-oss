package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/try-loopers/loopers-oss/cmd/loopers/ui"
	"gopkg.in/yaml.v3"
)

func screenObservability() {
	var action string
	err := huh.NewSelect[string]().
		Title("Observability & Alerting").
		Options(
			huh.NewOption("OpenTelemetry (Tracing)", "otel"),
			huh.NewOption("Alerting (Webhooks)", "alerting"),
			huh.NewOption("Back", "back"),
		).
		Value(&action).
		WithTheme(ui.GetHuhTheme()).
		Run()
	if err != nil || action == "back" {
		return
	}

	switch action {
	case "otel":
		runOTelScreen()
	case "alerting":
		runAlertingScreen()
	}
}

func runOTelScreen() {
	ui.PrintLogo()
	ui.PrintHeader("OpenTelemetry Configuration")
	fmt.Println()

	var (
		enabled      = true
		endpoint     = "localhost:4317"
		protocol     = "grpc"
		insecure     = true
		samplingRate = "1.0"
	)

	data, err := os.ReadFile("loopers.yaml")
	if err == nil {
		var cfg map[string]interface{}
		if yaml.Unmarshal(data, &cfg) == nil {
			if otelCfg, ok := cfg["otel"].(map[string]interface{}); ok {
				if v, ok := otelCfg["enabled"].(bool); ok {
					enabled = v
				}
				if v, ok := otelCfg["endpoint"].(string); ok {
					endpoint = v
				}
				if v, ok := otelCfg["protocol"].(string); ok {
					protocol = v
				}
				if v, ok := otelCfg["insecure"].(bool); ok {
					insecure = v
				}
				if v, ok := otelCfg["sampling_rate"].(float64); ok {
					samplingRate = fmt.Sprintf("%g", v)
				}
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Enable OpenTelemetry? (Recommended: Yes)").Value(&enabled),
			huh.NewInput().Title("OTLP Endpoint (Recommended: localhost:4317)").Value(&endpoint),
			huh.NewSelect[string]().Title("Protocol (Recommended: gRPC)").Options(
				huh.NewOption("gRPC", "grpc"),
				huh.NewOption("HTTP/Protobuf", "http"),
				huh.NewOption("Stdout (Console Print)", "stdout"),
			).Value(&protocol),
			huh.NewConfirm().Title("Insecure Connection? (Recommended: Yes)").Value(&insecure),
			huh.NewInput().Title("Sampling Rate (0.0 to 1.0) (Recommended: 1.0)").Value(&samplingRate),
		).Title("OpenTelemetry Configuration"),
	).WithTheme(ui.GetHuhTheme()).WithKeyMap(ui.GetHuhKeyMap()).Run()

	if err != nil {
		return
	}

	data, err = os.ReadFile("loopers.yaml")
	var root map[string]interface{}
	if err == nil {
		yaml.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	root["otel"] = map[string]interface{}{
		"enabled":       enabled,
		"endpoint":      endpoint,
		"protocol":      protocol,
		"insecure":      insecure,
		"sampling_rate": parseFloat(samplingRate),
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to encode YAML: %v", err))
	} else {
		err = os.WriteFile("loopers.yaml", out, 0600)
		if err == nil {
			ui.Success("Updated otel configuration in loopers.yaml")
		}
	}
	pressEnterToContinue()
}

func runAlertingScreen() {
	ui.PrintLogo()
	ui.PrintHeader("Alerting & Webhook Configuration")
	fmt.Println()

	var (
		webhookURL    = ""
		webhookSecret = ""
		bufferSize    = "100"
		t1Percent     = "50"
		t1Message     = "Budget 50% consumed"
		t2Percent     = "80"
		t2Message     = ""
		t3Percent     = "95"
		t3Message     = ""
	)

	data, err := os.ReadFile("loopers.yaml")
	if err == nil {
		var cfg map[string]interface{}
		if yaml.Unmarshal(data, &cfg) == nil {
			if aCfg, ok := cfg["alerting"].(map[string]interface{}); ok {
				if v, ok := aCfg["webhook_url"].(string); ok {
					webhookURL = v
				}
				if v, ok := aCfg["buffer_size"].(int); ok {
					bufferSize = fmt.Sprintf("%d", v)
				}

				if tCfg, ok := aCfg["thresholds"].([]interface{}); ok {
					if len(tCfg) > 0 {
						if m, ok := tCfg[0].(map[string]interface{}); ok {
							if p, ok := m["percent"].(int); ok {
								t1Percent = fmt.Sprintf("%d", p)
							}
							if msg, ok := m["message"].(string); ok {
								t1Message = msg
							}
						}
					}
					if len(tCfg) > 1 {
						if m, ok := tCfg[1].(map[string]interface{}); ok {
							if p, ok := m["percent"].(int); ok {
								t2Percent = fmt.Sprintf("%d", p)
							}
							if msg, ok := m["message"].(string); ok {
								t2Message = msg
							}
						}
					}
					if len(tCfg) > 2 {
						if m, ok := tCfg[2].(map[string]interface{}); ok {
							if p, ok := m["percent"].(int); ok {
								t3Percent = fmt.Sprintf("%d", p)
							}
							if msg, ok := m["message"].(string); ok {
								t3Message = msg
							}
						}
					}
				}
			}
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Webhook URL (Optional)").Value(&webhookURL).Description("Leave blank to disable alerting"),
			huh.NewInput().Title("HMAC Secret (Optional)").Value(&webhookSecret).Description("For payload signature verification"),
			huh.NewInput().Title("Buffer Size (Recommended: 100)").Value(&bufferSize),
		).Title("Webhook Configuration"),
		huh.NewGroup(
			huh.NewInput().Title("Threshold 1 (%) (Recommended: 50%)").Value(&t1Percent),
			huh.NewInput().Title("Threshold 1 Message").Value(&t1Message),
			huh.NewInput().Title("Threshold 2 (%) (Recommended: 80%)").Value(&t2Percent),
			huh.NewInput().Title("Threshold 2 Message").Value(&t2Message),
			huh.NewInput().Title("Threshold 3 (%) (Recommended: 95%)").Value(&t3Percent),
			huh.NewInput().Title("Threshold 3 Message").Value(&t3Message),
		).Title("Alert Thresholds"),
	).WithTheme(ui.GetHuhTheme()).WithKeyMap(ui.GetHuhKeyMap()).Run()

	if err != nil {
		return
	}

	data, err = os.ReadFile("loopers.yaml")
	var root map[string]interface{}
	if err == nil {
		yaml.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	thresholds := []map[string]interface{}{}
	if t1Percent != "" && t1Percent != "0" {
		thresholds = append(thresholds, map[string]interface{}{"percent": parseInt(t1Percent), "message": t1Message})
	}
	if t2Percent != "" && t2Percent != "0" {
		thresholds = append(thresholds, map[string]interface{}{"percent": parseInt(t2Percent), "message": t2Message})
	}
	if t3Percent != "" && t3Percent != "0" {
		thresholds = append(thresholds, map[string]interface{}{"percent": parseInt(t3Percent), "message": t3Message})
	}

	aBlock := map[string]interface{}{
		"webhook_url": webhookURL,
		"buffer_size": parseInt(bufferSize),
		"thresholds":  thresholds,
	}

	if webhookSecret != "" {
		aBlock["webhook_secret"] = webhookSecret
	} else {
		if existingA, ok := root["alerting"].(map[string]interface{}); ok {
			if secret, ok := existingA["webhook_secret"]; ok {
				aBlock["webhook_secret"] = secret
			}
		}
	}

	root["alerting"] = aBlock

	out, err := yaml.Marshal(&root)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to encode YAML: %v", err))
	} else {
		err = os.WriteFile("loopers.yaml", out, 0600)
		if err == nil {
			ui.Success("Updated alerting configuration in loopers.yaml")
		}
	}
	pressEnterToContinue()
}
