package api

// Message represents an OpenAI chat completion message.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []interface{}
	Name    string      `json:"name,omitempty"`
}

// ChatCompletionRequest represents an OpenAI chat completion request.
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// OpenAIUsage represents the usage object in OpenAI stream chunks.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk is the structural representation of an OpenAI stream payload.
type OpenAIStreamChunk struct {
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

// AnthropicUsage represents the usage object in Anthropic stream events.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// AnthropicMessageStart represents the message_start event payload.
type AnthropicMessageStart struct {
	Message struct {
		Usage AnthropicUsage `json:"usage"`
	} `json:"message"`
}

// AnthropicMessageDelta represents the message_delta event payload.
type AnthropicMessageDelta struct {
	Usage AnthropicUsage `json:"usage"`
}

// AnthropicCountResponse holds the token count from Anthropic's count_tokens API.
type AnthropicCountResponse struct {
	InputTokens int `json:"input_tokens"`
}
