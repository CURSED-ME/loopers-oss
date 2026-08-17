package inspector

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsValidLuhn(t *testing.T) {
	// Standard valid test card numbers
	validCards := []string{
		"49927398716",
		"49927398717", // invalid
		"1234567812345670",
		"1234567812345678", // invalid
	}

	if !IsValidLuhn(validCards[0]) {
		t.Errorf("expected %s to be valid Luhn", validCards[0])
	}
	if IsValidLuhn(validCards[1]) {
		t.Errorf("expected %s to be invalid Luhn", validCards[1])
	}
	if !IsValidLuhn(validCards[2]) {
		t.Errorf("expected %s to be valid Luhn", validCards[2])
	}
	if IsValidLuhn(validCards[3]) {
		t.Errorf("expected %s to be invalid Luhn", validCards[3])
	}
}

func TestInspectDLPContent_Secrets(t *testing.T) {
	cfg := DLPConfig{
		Enabled:     true,
		Action:      "quarantine",
		ScanSecrets: true,
	}

	secretText := "Here is the key: AKIAIOSFODNN7EXAMPLE and sk-1234567890abcdef1234567890abcdef"
	res, mutated := InspectDLPContent(secretText, cfg)

	if res.Action != "quarantine" {
		t.Errorf("expected quarantine, got %s", res.Action)
	}
	if strings.Contains(mutated, "AKIAIOSFODNN7EXAMPLE") || strings.Contains(mutated, "sk-1234567890abcdef") {
		t.Errorf("expected secrets to be masked in output, got: %s", mutated)
	}
}

func TestInspectDLPContent_PII(t *testing.T) {
	cfg := DLPConfig{
		Enabled:      true,
		Action:       "mask",
		ScanPII:      true,
		AllowedHosts: []string{"example.com"},
	}

	text := "Contact user@secretcorp.com or allowed@example.com, SSN: 123-45-6789, Phone: +1-555-123-4567"
	res, mutated := InspectDLPContent(text, cfg)

	if res.Action != "mask" {
		t.Errorf("expected mask, got %s", res.Action)
	}
	if strings.Contains(mutated, "user@secretcorp.com") {
		t.Errorf("expected user@secretcorp.com to be masked")
	}
	if !strings.Contains(mutated, "allowed@example.com") {
		t.Errorf("expected allowed@example.com to be retained")
	}
	if strings.Contains(mutated, "123-45-6789") {
		t.Errorf("expected SSN to be masked")
	}
	if strings.Contains(mutated, "+1-555-123-4567") {
		t.Errorf("expected phone to be masked")
	}

	// Test allowlisted email alongside SSN: reason should NOT mention Email
	textAllowedEmailOnly := "Contact allowed@example.com, SSN: 123-45-6789"
	resAllowed, _ := InspectDLPContent(textAllowedEmailOnly, cfg)
	if strings.Contains(resAllowed.Reason, "Email address detected") {
		t.Errorf("expected reason not to contain Email address detected when email is allowlisted, got: %s", resAllowed.Reason)
	}

	// Test Phone formatting variations
	phoneTests := []struct {
		input  string
		masked bool
	}{
		{"Call (555) 123-4567", true},
		{"Call 555-123-4567", true},
		{"Call +1 555 123 4567", true},
		{"Call 1-555-123-4567", true},
		{"Serial 12345678901234", false},
	}
	for _, pt := range phoneTests {
		_, pMutated := InspectDLPContent(pt.input, cfg)
		hasMask := strings.Contains(pMutated, "***")
		if hasMask != pt.masked {
			t.Errorf("for input %q expected masked=%v, got %v (output: %q)", pt.input, pt.masked, hasMask, pMutated)
		}
	}
}

func TestInspectDLPContent_Network(t *testing.T) {
	cfg := DLPConfig{
		Enabled:      true,
		Action:       "mask",
		ScanNetwork:  true,
		AllowedHosts: []string{"localhost"},
	}

	text := "Backend running at 192.168.1.100 and localhost:8080"
	res, mutated := InspectDLPContent(text, cfg)

	if res.Action != "mask" {
		t.Errorf("expected mask, got %s", res.Action)
	}
	if strings.Contains(mutated, "192.168.") {
		t.Errorf("expected 192.168. to be masked")
	}
	if !strings.Contains(mutated, "localhost") {
		t.Errorf("expected localhost to be retained as allowed host")
	}
}

func TestInspectJSONCompletion_OpenAI(t *testing.T) {
	cfg := DLPConfig{
		Enabled:     true,
		Action:      "mask",
		ScanSecrets: true,
		ScanPII:     true,
	}

	openAIPayload := `{
		"id": "chatcmpl-123",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Your API key is sk-1234567890abcdef1234567890abcdef and your email is admin@corp.net"
				}
			}
		]
	}`

	res, newBody, err := InspectJSONCompletion([]byte(openAIPayload), "openai", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Action != "mask" {
		t.Errorf("expected mask action, got %s", res.Action)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(newBody, &parsed); err != nil {
		t.Fatalf("failed to parse transformed json: %v", err)
	}

	choices := parsed["choices"].([]interface{})
	msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content := msg["content"].(string)

	if strings.Contains(content, "sk-1234567890abcdef") || strings.Contains(content, "admin@corp.net") {
		t.Errorf("sensitive tokens not masked in OpenAI response: %s", content)
	}
}

func TestInspectJSONCompletion_Anthropic(t *testing.T) {
	cfg := DLPConfig{
		Enabled:     true,
		Action:      "quarantine",
		ScanSecrets: true,
	}

	anthropicPayload := `{
		"id": "msg-123",
		"type": "message",
		"content": [
			{
				"type": "text",
				"text": "The AWS key is AKIAIOSFODNN7EXAMPLE"
			}
		]
	}`

	res, newBody, err := InspectJSONCompletion([]byte(anthropicPayload), "anthropic", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Action != "quarantine" {
		t.Errorf("expected quarantine action, got %s", res.Action)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(newBody, &parsed); err != nil {
		t.Fatalf("failed to parse transformed json: %v", err)
	}

	content := parsed["content"].([]interface{})
	part := content[0].(map[string]interface{})
	text := part["text"].(string)

	if strings.Contains(text, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key not masked: %s", text)
	}
}

func TestInspectJSONCompletion_Gemini(t *testing.T) {
	cfg := DLPConfig{
		Enabled: true,
		Action:  "mask",
		ScanPII: true,
	}

	geminiPayload := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "User SSN is 987-65-4321"
						}
					]
				}
			}
		]
	}`

	res, newBody, err := InspectJSONCompletion([]byte(geminiPayload), "gemini", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Action != "mask" {
		t.Errorf("expected mask action, got %s", res.Action)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(newBody, &parsed); err != nil {
		t.Fatalf("failed to parse transformed json: %v", err)
	}

	candidates := parsed["candidates"].([]interface{})
	cand := candidates[0].(map[string]interface{})
	content := cand["content"].(map[string]interface{})
	parts := content["parts"].([]interface{})
	text := parts[0].(map[string]interface{})["text"].(string)

	if strings.Contains(text, "987-65-4321") {
		t.Errorf("SSN not masked: %s", text)
	}
}
