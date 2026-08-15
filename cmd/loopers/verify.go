package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/verifier"
	"github.com/spf13/cobra"
)

var (
	verifyTracePath       string
	verifyPolicyFile      string
	verifyPolicyDir       string
	verifyPresets         []string
	verifyDefaultAction   string
	verifyFormat          string
	verifyFailOnViolation bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Audit session JSON traces against OPA policies and Policy Cards",
	Long: `Verify replays and audits recorded execution trace logs against your declarative
YAML Policy Cards and OPA Rego policies to validate behavioral compliance and FSM path sequences.`,
	Example: `  # Audit a trace using built-in security presets
  loopers verify --trace ./traces/agent_run.json --presets safety,mcp_sandbox

  # Audit a trace using a custom YAML Policy Card
  loopers verify -t ./traces/session.json -f ./policies.yaml

  # Emit JSON output for automated CI/CD compliance gates
  loopers verify -t ./traces/session.json -f ./policies.yaml --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if verifyTracePath == "" {
			return fmt.Errorf("required flag --trace (-t) not provided")
		}

		vCfg := verifier.Config{
			PolicyFile:    verifyPolicyFile,
			PolicyDir:     verifyPolicyDir,
			Presets:       verifyPresets,
			DefaultAction: verifyDefaultAction,
		}

		v, err := verifier.NewVerifier(vCfg)
		if err != nil {
			return fmt.Errorf("failed to initialize verification engine: %w", err)
		}

		ctx := context.Background()
		report, err := v.VerifyTraceFile(ctx, verifyTracePath)
		if err != nil {
			return fmt.Errorf("verification error: %w", err)
		}

		if strings.ToLower(verifyFormat) == "json" {
			outBytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format report as JSON: %w", err)
			}
			fmt.Println(string(outBytes))
		} else {
			printPrettyReport(report)
		}

		if verifyFailOnViolation && report.Status == "FAILED" {
			os.Exit(1)
		}

		return nil
	},
}

func printPrettyReport(report *verifier.VerificationReport) {
	fmt.Println("================================================================================")
	fmt.Println("                        LOOPERS FORMAL TRACE VERIFICATION                      ")
	fmt.Println("================================================================================")
	if report.SessionID != "" {
		fmt.Printf(" Session ID:       %s\n", report.SessionID)
	}
	fmt.Printf(" Total Steps:      %d\n", report.TotalSteps)
	fmt.Printf(" Actions Audited:  %d\n", report.ActionsAudited)
	fmt.Printf(" Violations Found: %d\n", report.ViolationsCount)
	fmt.Printf(" Duration:         %dms\n", report.DurationMs)

	if report.Status == "PASSED" {
		fmt.Println(" Result:           [PASSED] Trace is fully compliant with loaded policies.")
	} else {
		fmt.Println(" Result:           [FAILED] Policy violations detected during execution.")
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println(" VIOLATION DETAILS:")
		for i, viol := range report.Violations {
			target := viol.Action.Type
			if viol.Action.ToolName != "" {
				target = fmt.Sprintf("%s (Tool: %s)", target, viol.Action.ToolName)
			} else if viol.Action.Model != "" {
				target = fmt.Sprintf("%s (Model: %s)", target, viol.Action.Model)
			}
			fmt.Printf("  #%d. Step [%d] | Target: %s\n", i+1, viol.StepIndex, target)
			fmt.Printf("      Reason: %s\n", viol.Reason)
			if viol.Action.PromptText != "" {
				snippet := viol.Action.PromptText
				if len(snippet) > 80 {
					snippet = snippet[:77] + "..."
				}
				fmt.Printf("      Prompt: %q\n", snippet)
			}
			if len(viol.Action.ToolArguments) > 0 {
				argsBytes, _ := json.Marshal(viol.Action.ToolArguments)
				fmt.Printf("      Arguments: %s\n", string(argsBytes))
			}
			fmt.Println()
		}
	}
	fmt.Println("================================================================================")
}

func init() {
	verifyCmd.Flags().StringVarP(&verifyTracePath, "trace", "t", "", "Path to the JSON execution trace file (required)")
	verifyCmd.Flags().StringVarP(&verifyPolicyFile, "policy-file", "f", "", "Path to declarative YAML Policy Card (e.g. policies.yaml)")
	verifyCmd.Flags().StringVarP(&verifyPolicyDir, "policy-dir", "d", "", "Directory containing custom Rego policy files (.rego)")
	verifyCmd.Flags().StringSliceVarP(&verifyPresets, "presets", "p", []string{}, "Comma-separated list of built-in security presets (safety, pci, mcp_sandbox)")
	verifyCmd.Flags().StringVar(&verifyDefaultAction, "default-action", "allow", "Default action when no rule matches (allow or deny)")
	verifyCmd.Flags().StringVar(&verifyFormat, "format", "pretty", "Output format (pretty or json)")
	verifyCmd.Flags().BoolVar(&verifyFailOnViolation, "fail-on-violation", true, "Exit with non-zero status code if policy violations are detected")

	rootCmd.AddCommand(verifyCmd)
}
