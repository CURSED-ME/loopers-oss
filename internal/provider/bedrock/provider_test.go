package bedrock

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
)

func TestBedrockParseRequest(t *testing.T) {
	prov := NewBedrockProvider()

	// 1. Non-streaming request
	req1, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/bedrock/model/anthropic.claude-3-5-sonnet-20240620-v1:0/invoke", nil)
	body1 := []byte(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":1000}`)
	
	model, isStream, maxTokens, _, err := prov.ParseRequest(req1, body1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "anthropic.claude-3-5-sonnet-20240620-v1:0" {
		t.Errorf("expected model 'anthropic.claude-3-5-sonnet-20240620-v1:0', got '%s'", model)
	}
	if isStream {
		t.Error("expected non-streaming request")
	}
	if maxTokens != 1000 {
		t.Errorf("expected maxTokens 1000, got %d", maxTokens)
	}

	// 2. Streaming request
	req2, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/bedrock/model/meta.llama3-1-400b-instruct-v1:0/invoke-with-response-stream", nil)
	body2 := []byte(`{"prompt":"hello llama"}`)

	model, isStream, maxTokens, _, err = prov.ParseRequest(req2, body2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "meta.llama3-1-400b-instruct-v1:0" {
		t.Errorf("expected model 'meta.llama3-1-400b-instruct-v1:0', got '%s'", model)
	}
	if !isStream {
		t.Error("expected streaming request")
	}
	if maxTokens != 0 {
		t.Errorf("expected maxTokens 0, got %d", maxTokens)
	}
}

func TestBedrockRewritePath(t *testing.T) {
	prov := NewBedrockProvider()
	rewritten := prov.RewritePath("/bedrock/model/anthropic.claude-3-5-sonnet-20240620-v1:0/invoke")
	expected := "/model/anthropic.claude-3-5-sonnet-20240620-v1:0/invoke"
	if rewritten != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewritten)
	}
}

func TestBedrockParseNonStreamResponse(t *testing.T) {
	prov := NewBedrockProvider()
	
	// 1. Claude format
	body1 := []byte(`{"usage":{"input_tokens":10,"output_tokens":20}}`)
	inTokens, outTokens, err := prov.ParseNonStreamResponse(body1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inTokens != 10 || outTokens != 20 {
		t.Errorf("expected 10 & 20, got %d & %d", inTokens, outTokens)
	}

	// 2. Titan/Llama format
	body2 := []byte(`{"prompt_token_count":15,"generation_token_count":25}`)
	inTokens, outTokens, err = prov.ParseNonStreamResponse(body2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inTokens != 15 || outTokens != 25 {
		t.Errorf("expected 15 & 25, got %d & %d", inTokens, outTokens)
	}
}

func TestBedrockCountInputTokensFallback(t *testing.T) {
	prov := NewBedrockProvider()

	// 1. Messages format (Claude)
	body1 := []byte(`{"messages":[{"role":"user","content":"hello world"}]}`)
	tokens, err := prov.CountInputTokens(context.Background(), "anthropic.claude-3", body1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "hello world" in cl100k_base has 2 tokens
	// msg.Role "user" has 1 token
	// Turn overhead is 4. Total = 7 tokens.
	// 7 * 1.15 = 8
	if tokens != 8 {
		t.Errorf("expected 8 tokens, got %d", tokens)
	}

	// 2. Prompt format
	body2 := []byte(`{"prompt":"hello world"}`)
	tokens, err = prov.CountInputTokens(context.Background(), "meta.llama3", body2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "hello world" has 2 tokens.
	// 2 * 1.15 = 2.3 -> 2
	if tokens != 2 {
		t.Errorf("expected 2 tokens, got %d", tokens)
	}
}

func TestBedrockStreamChunkParsing(t *testing.T) {
	prov := NewBedrockProvider()

	// 1. Claude message_start
	payload1 := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":35}}}`)
	inT, outT, isDone, err := prov.ParseStreamChunk(payload1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inT != 35 || outT != 0 || isDone {
		t.Errorf("expected 35, 0, false, got %d, %d, %t", inT, outT, isDone)
	}

	// 2. Claude message_delta
	payload2 := []byte(`{"type":"message_delta","usage":{"output_tokens":45}}`)
	inT, outT, isDone, err = prov.ParseStreamChunk(payload2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inT != 0 || outT != 45 || isDone {
		t.Errorf("expected 0, 45, false, got %d, %d, %t", inT, outT, isDone)
	}

	// 3. Claude message_stop
	payload3 := []byte(`{"type":"message_stop"}`)
	inT, outT, isDone, err = prov.ParseStreamChunk(payload3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDone {
		t.Error("expected isDone to be true")
	}

	// 4. Amazon Titan / General metrics chunk
	payload4 := []byte(`{"amazon-bedrock-invocationMetrics":{"inputTokenCount":8,"outputTokenCount":12}}`)
	inT, outT, isDone, err = prov.ParseStreamChunk(payload4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inT != 8 || outT != 12 || isDone {
		t.Errorf("expected 8, 12, false, got %d, %d, %t", inT, outT, isDone)
	}
}

func TestBedrockFormatBudgetExceededFrame(t *testing.T) {
	prov := NewBedrockProvider()
	frameBytes := prov.FormatBudgetExceededSSE()
	
	decoder := eventstream.NewDecoder()
	msg, err := decoder.Decode(bytes.NewReader(frameBytes), nil)
	if err != nil {
		t.Fatalf("failed to decode generated budget exceeded frame: %v", err)
	}

	// Verify header
	hasExceptionHeader := false
	for _, h := range msg.Headers {
		if h.Name == ":message-type" && h.Value.String() == "exception" {
			hasExceptionHeader = true
		}
	}
	if !hasExceptionHeader {
		t.Error("expected exception message-type header")
	}

	expectedPayload := `{"message":"Budget exceeded mid-stream"}`
	if string(msg.Payload) != expectedPayload {
		t.Errorf("expected payload '%s', got '%s'", expectedPayload, string(msg.Payload))
	}
}
