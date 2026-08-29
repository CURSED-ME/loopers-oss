package proxy

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/try-loopers/loopers-oss/internal/inspector"
	"github.com/try-loopers/loopers-oss/internal/provider/anthropic"
	"github.com/try-loopers/loopers-oss/internal/provider/openai"
)

type mockReadCloser struct {
	io.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestSSEFrameParsing(t *testing.T) {
	part1 := "data: hello\n"
	part2 := "\ndata: world\n\n"

	reader := &mockReadCloser{Reader: strings.NewReader(part1 + part2)}

	ctx := context.Background()
	r := NewSSEStreamReader(ctx, reader, openai.NewOpenAIProvider(), 0.0, 0.0, func(cost float64) bool { return true }, func(float64, int, int, string, bool) {}, inspector.DLPConfig{}, nil)
	defer r.Close()

	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data: hello\n\ndata: world\n\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestDONEHandling(t *testing.T) {
	stream := "data: {\"usage\":null}\n\ndata: [DONE]\n\n"
	reader := &mockReadCloser{Reader: strings.NewReader(stream)}

	ctx := context.Background()
	reconciled := false
	r := NewSSEStreamReader(ctx, reader, openai.NewOpenAIProvider(), 0.0, 0.0, func(cost float64) bool { return true }, func(actualCost float64, inTokens, outTokens int, completion string, forcedCut bool) {
		reconciled = true
	}, inspector.DLPConfig{}, nil)
	defer r.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)

	if !reconciled {
		t.Error("expected reconciliation to be triggered on [DONE]")
	}
}

func TestAnthropicEventParsing(t *testing.T) {
	stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\n\n"

	reader := &mockReadCloser{Reader: strings.NewReader(stream)}

	ctx := context.Background()
	var actualRecorded float64
	r := NewSSEStreamReader(ctx, reader, anthropic.NewAnthropicProvider(), 1.0, 2.0, func(cost float64) bool { return true }, func(actualCost float64, inTokens, outTokens int, completion string, forcedCut bool) {
		actualRecorded = actualCost
	}, inspector.DLPConfig{}, nil)
	defer r.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)

	expectedCost := (10*1.0 + 5*2.0) / 1000000.0
	if actualRecorded != expectedCost {
		t.Errorf("expected cost %f, got %f", expectedCost, actualRecorded)
	}
}

func TestStreamBudgetCutoff(t *testing.T) {
	stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"forbidden_text\"}}\n\n"

	reader := &mockReadCloser{Reader: strings.NewReader(stream)}

	ctx := context.Background()
	r := NewSSEStreamReader(ctx, reader, anthropic.NewAnthropicProvider(), 1.0, 2.0, func(cost float64) bool { return cost <= 0.000005 }, func(actualCost float64, inTokens, outTokens int, completion string, forcedCut bool) {}, inspector.DLPConfig{}, nil)
	defer r.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)

	output := buf.String()
	if !strings.Contains(output, "Budget exceeded mid-stream") {
		t.Error("expected output to contain budget exceeded error event")
	}
	if !strings.Contains(output, "message_delta") {
		t.Error("expected stream to flush final message_delta chunk before severing")
	}
	if strings.Contains(output, "forbidden_text") {
		t.Error("expected stream to be cut off before subsequent chunks are written")
	}
}

func TestStreamDLP_MaskPII(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"Contact admin@secretcorp.com for key\"}}]}\n\n"
	reader := &mockReadCloser{Reader: strings.NewReader(stream)}

	dlpCfg := inspector.DLPConfig{
		Enabled: true,
		Action:  "mask",
		ScanPII: true,
	}

	ctx := context.Background()
	r := NewSSEStreamReader(ctx, reader, openai.NewOpenAIProvider(), 0.0, 0.0, func(cost float64) bool { return true }, func(float64, int, int, string, bool) {}, dlpCfg, nil)
	defer r.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)

	output := buf.String()
	if strings.Contains(output, "admin@secretcorp.com") {
		t.Errorf("expected email to be masked in streaming response, got: %s", output)
	}
	if !strings.Contains(output, "***") {
		t.Errorf("expected mask *** in output, got: %s", output)
	}
}

func TestStreamDLP_QuarantineSecret(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"Secret: AKIAIOSFODNN7EXAMPLE\"}}]}\n\n"
	reader := &mockReadCloser{Reader: strings.NewReader(stream)}

	dlpCfg := inspector.DLPConfig{
		Enabled:     true,
		Action:      "quarantine",
		ScanSecrets: true,
	}

	quarantined := false
	ctx := context.Background()
	r := NewSSEStreamReader(ctx, reader, openai.NewOpenAIProvider(), 0.0, 0.0, func(cost float64) bool { return true }, func(float64, int, int, string, bool) {}, dlpCfg, func() {
		quarantined = true
	})
	defer r.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)

	output := buf.String()
	if !quarantined {
		t.Error("expected onDLPQuarantine callback to be executed")
	}
	if !strings.Contains(output, "dlp_quarantine") {
		t.Errorf("expected dlp_quarantine error frame, got: %s", output)
	}
}
