package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapLLMCall_OpenAI(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello!"}
		]
	}`)
	prompt, err := MapLLMCall("openai", body)
	assert.NoError(t, err)
	assert.Equal(t, "You are helpful.\nHello!", prompt)
}

func TestMapLLMCall_Anthropic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"system": "System instructions",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Run query."}]}
		]
	}`)
	prompt, err := MapLLMCall("anthropic", body)
	assert.NoError(t, err)
	assert.Equal(t, "System instructions\nRun query.", prompt)
}

func TestMapLLMCall_Gemini(t *testing.T) {
	body := []byte(`{
		"contents": [
			{
				"role": "user",
				"parts": [{"text": "Hello Gemini!"}]
			}
		]
	}`)
	prompt, err := MapLLMCall("gemini", body)
	assert.NoError(t, err)
	assert.Equal(t, "Hello Gemini!", prompt)
}

func TestMapLLMRequestToContext_OpenAI(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello!"},
			{
				"role": "assistant",
				"content": null,
				"tool_calls": [
					{
						"id": "call_123",
						"type": "function",
						"function": {
							"name": "search_db",
							"arguments": "{\"query\":\"test\"}"
						}
					}
				]
			}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "search_db",
					"description": "Search database",
					"parameters": {
						"type": "object",
						"properties": {"query": {"type": "string"}}
					}
				}
			}
		]
	}`)
	ctx, err := MapLLMRequestToContext("openai", body)
	assert.NoError(t, err)
	assert.Equal(t, "llm_call", ctx.Type)
	assert.Equal(t, "openai", ctx.Provider)
	assert.Equal(t, "gpt-4o", ctx.Model)
	assert.Equal(t, "You are helpful.\nHello!", ctx.PromptText)
	assert.Len(t, ctx.Messages, 3)
	assert.Equal(t, "system", ctx.Messages[0].Role)
	assert.Equal(t, "You are helpful.", ctx.Messages[0].Content)
	assert.Equal(t, "user", ctx.Messages[1].Role)
	assert.Equal(t, "Hello!", ctx.Messages[1].Content)

	assert.Len(t, ctx.Tools, 1)
	assert.Equal(t, "search_db", ctx.Tools[0].Name)
	assert.Equal(t, "Search database", ctx.Tools[0].Description)

	assert.Len(t, ctx.ToolCalls, 1)
	assert.Equal(t, "call_123", ctx.ToolCalls[0].ID)
	assert.Equal(t, "search_db", ctx.ToolCalls[0].Name)
	assert.Equal(t, "test", ctx.ToolCalls[0].Arguments["query"])
}

func TestMapLLMRequestToContext_Anthropic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"system": "You are Claude.",
		"messages": [
			{
				"role": "user",
				"content": "Can you check logs?"
			},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Sure, checking."},
					{
						"type": "tool_use",
						"id": "toolu_01",
						"name": "get_logs",
						"input": {"service": "api"}
					}
				]
			}
		],
		"tools": [
			{
				"name": "get_logs",
				"description": "Fetch system logs",
				"input_schema": {
					"type": "object",
					"properties": {"service": {"type": "string"}}
				}
			}
		]
	}`)
	ctx, err := MapLLMRequestToContext("anthropic", body)
	assert.NoError(t, err)
	assert.Equal(t, "llm_call", ctx.Type)
	assert.Equal(t, "anthropic", ctx.Provider)
	assert.Equal(t, "claude-3-5-sonnet", ctx.Model)
	assert.Equal(t, "You are Claude.\nCan you check logs?\nSure, checking.", ctx.PromptText)
	assert.Len(t, ctx.Messages, 3)
	assert.Equal(t, "system", ctx.Messages[0].Role)
	assert.Equal(t, "You are Claude.", ctx.Messages[0].Content)
	assert.Equal(t, "user", ctx.Messages[1].Role)
	assert.Equal(t, "assistant", ctx.Messages[2].Role)
	assert.Equal(t, "Sure, checking.", ctx.Messages[2].Content)

	assert.Len(t, ctx.Tools, 1)
	assert.Equal(t, "get_logs", ctx.Tools[0].Name)
	assert.Equal(t, "Fetch system logs", ctx.Tools[0].Description)

	assert.Len(t, ctx.ToolCalls, 1)
	assert.Equal(t, "toolu_01", ctx.ToolCalls[0].ID)
	assert.Equal(t, "get_logs", ctx.ToolCalls[0].Name)
	assert.Equal(t, "api", ctx.ToolCalls[0].Arguments["service"])
}

func TestMapLLMRequestToContext_Gemini(t *testing.T) {
	body := []byte(`{
		"contents": [
			{
				"role": "user",
				"parts": [{"text": "Execute command"}]
			},
			{
				"role": "model",
				"parts": [
					{
						"function_call": {
							"name": "bash",
							"args": {"cmd": "ls"}
						}
					}
				]
			}
		],
		"tools": [
			{
				"function_declarations": [
					{
						"name": "bash",
						"description": "Execute bash command",
						"parameters": {
							"type": "object"
						}
					}
				]
			}
		]
	}`)
	ctx, err := MapLLMRequestToContext("gemini", body)
	assert.NoError(t, err)
	assert.Equal(t, "llm_call", ctx.Type)
	assert.Equal(t, "gemini", ctx.Provider)
	assert.Equal(t, "Execute command", ctx.PromptText)
	assert.Len(t, ctx.Messages, 2)
	assert.Equal(t, "user", ctx.Messages[0].Role)
	assert.Equal(t, "Execute command", ctx.Messages[0].Content)
	assert.Equal(t, "assistant", ctx.Messages[1].Role)
	assert.Equal(t, "", ctx.Messages[1].Content)

	assert.Len(t, ctx.Tools, 1)
	assert.Equal(t, "bash", ctx.Tools[0].Name)
	assert.Equal(t, "Execute bash command", ctx.Tools[0].Description)

	assert.Len(t, ctx.ToolCalls, 1)
	assert.Equal(t, "bash", ctx.ToolCalls[0].Name)
	assert.Equal(t, "ls", ctx.ToolCalls[0].Arguments["cmd"])
}

func TestMapLLMRequestToContext_Fallback(t *testing.T) {
	ctx, err := MapLLMRequestToContext("openai", []byte("plain text prompt"))
	assert.NoError(t, err)
	assert.Equal(t, "plain text prompt", ctx.PromptText)

	ctxEmpty, err := MapLLMRequestToContext("openai", nil)
	assert.NoError(t, err)
	assert.Empty(t, ctxEmpty.PromptText)
}
