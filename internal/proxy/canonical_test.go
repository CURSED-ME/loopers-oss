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
