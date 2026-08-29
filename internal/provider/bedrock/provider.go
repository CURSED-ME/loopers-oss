package bedrock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/try-loopers/loopers-oss/internal/logging"
	"github.com/try-loopers/loopers-oss/internal/provider"
)

type BedrockProvider struct{}

// NewBedrockProvider creates a new instance of BedrockProvider.
func NewBedrockProvider() *BedrockProvider {
	return &BedrockProvider{}
}

// Ensure BedrockProvider implements provider.Provider.
var _ provider.Provider = (*BedrockProvider)(nil)

func (b *BedrockProvider) Name() string {
	return "bedrock"
}

func (b *BedrockProvider) BaseURL() string {
	// Return a default region endpoint; it will be overwritten dynamically based on headers in InjectAuth
	return "https://bedrock-runtime.us-east-1.amazonaws.com"
}

func (b *BedrockProvider) InjectAuth(req *http.Request, providerKey string) {
	// Extract AWS credentials from headers
	accessKey := req.Header.Get("X-Loopers-AWS-Access-Key")
	secretKey := req.Header.Get("X-Loopers-AWS-Secret-Key")
	region := req.Header.Get("X-Loopers-AWS-Region")
	sessionToken := req.Header.Get("X-Loopers-AWS-Session-Token")

	// Delete custom headers before forwarding
	req.Header.Del("X-Loopers-AWS-Access-Key")
	req.Header.Del("X-Loopers-AWS-Secret-Key")
	req.Header.Del("X-Loopers-AWS-Region")
	req.Header.Del("X-Loopers-AWS-Session-Token")

	if accessKey == "" || secretKey == "" {
		logging.Logger.Error().Msg("AWS Bedrock Access Key or Secret Key is missing")
		return
	}
	if region == "" {
		region = "us-east-1"
	}

	// Update host and scheme dynamically based on region
	req.URL.Scheme = "https"
	req.URL.Host = fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", region)
	req.Host = req.URL.Host

	// Read body for hashing
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			logging.Logger.Error().Err(err).Msg("Failed to read Bedrock request body for signing")
			return
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Calculate SHA-256 payload hash
	hash := sha256.New()
	hash.Write(bodyBytes)
	payloadHash := hex.EncodeToString(hash.Sum(nil))

	// Create Signer and sign the HTTP request
	signer := v4.NewSigner()
	creds := aws.Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		SessionToken:    sessionToken,
	}

	err := signer.SignHTTP(req.Context(), creds, req, payloadHash, "bedrock", region, time.Now())
	if err != nil {
		logging.Logger.Error().Err(err).Msg("Failed to sign request with AWS Signature Version 4")
	}
}

func (b *BedrockProvider) RewritePath(originalPath string) string {
	// Path in Loopers: /bedrock/model/{modelId}/invoke or /bedrock/model/{modelId}/invoke-with-response-stream
	// -> Path in Upstream: /model/{modelId}/invoke or /model/{modelId}/invoke-with-response-stream
	return strings.TrimPrefix(originalPath, "/bedrock")
}

func (b *BedrockProvider) ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error) {
	path := req.URL.Path

	// Detect if streaming
	if strings.Contains(path, "/invoke-with-response-stream") {
		isStream = true
	}

	// Extract model ID from path, e.g. "/bedrock/model/anthropic.claude-3-sonnet-20240229-v1:0/invoke"
	// Take substring between "/model/" and "/invoke"
	idx := strings.Index(path, "/model/")
	if idx != -1 {
		start := idx + len("/model/")
		end := strings.Index(path[start:], "/invoke")
		if end != -1 {
			model = path[start : start+end]
		} else {
			model = path[start:]
		}
	}

	// Extract max output tokens from body depending on model family (Claude uses max_tokens)
	var claudePayload struct {
		MaxTokens *int `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &claudePayload); err == nil && claudePayload.MaxTokens != nil {
		maxTokens = *claudePayload.MaxTokens
	}

	return model, isStream, maxTokens, body, nil
}

func (b *BedrockProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	path := req.URL.Path
	idx := strings.Index(path, "/model/")
	if idx != -1 {
		start := idx + len("/model/")
		end := strings.Index(path[start:], "/invoke")
		if end != -1 {
			req.URL.Path = path[:start] + fallbackModel + path[start+end:]
		}
	}
	return body, nil
}

func (b *BedrockProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return countBedrockTokensFallback(body)
}

func (b *BedrockProvider) ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error) {
	// Let's try parsing Claude format
	var claudeStart struct {
		Type    string `json:"type"`
		Message struct {
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	var claudeDelta struct {
		Type  string `json:"type"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(chunk, &claudeStart); err == nil && claudeStart.Type == "message_start" {
		return claudeStart.Message.Usage.InputTokens, 0, false, nil
	}
	if err := json.Unmarshal(chunk, &claudeDelta); err == nil && (claudeDelta.Type == "message_delta" || claudeDelta.Type == "content_block_delta") {
		return 0, claudeDelta.Usage.OutputTokens, false, nil
	}
	if err := json.Unmarshal(chunk, &claudeDelta); err == nil && claudeDelta.Type == "message_stop" {
		return 0, 0, true, nil
	}

	// Try parsing Llama / general format
	var generalChunk struct {
		PromptTokenCount     int `json:"prompt_token_count"`
		GenerationTokenCount int `json:"generation_token_count"`
		AmazonMetrics        struct {
			InputTokenCount  int `json:"inputTokenCount"`
			OutputTokenCount int `json:"outputTokenCount"`
		} `json:"amazon-bedrock-invocationMetrics"`
	}
	if err := json.Unmarshal(chunk, &generalChunk); err == nil {
		inT := generalChunk.PromptTokenCount
		outT := generalChunk.GenerationTokenCount
		if generalChunk.AmazonMetrics.InputTokenCount > 0 || generalChunk.AmazonMetrics.OutputTokenCount > 0 {
			inT = generalChunk.AmazonMetrics.InputTokenCount
			outT = generalChunk.AmazonMetrics.OutputTokenCount
		}
		if inT > 0 || outT > 0 {
			return inT, outT, false, nil
		}
	}

	return 0, 0, false, nil
}

func (b *BedrockProvider) ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error) {
	// Extract output tokens from non-stream response (Claude format)
	var respObj struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &respObj); err == nil && respObj.Usage != nil {
		return respObj.Usage.InputTokens, respObj.Usage.OutputTokens, nil
	}

	// Titan / Llama format
	var generalObj struct {
		PromptTokenCount     int `json:"prompt_token_count"`
		GenerationTokenCount int `json:"generation_token_count"`
	}
	if err := json.Unmarshal(body, &generalObj); err == nil && (generalObj.PromptTokenCount > 0 || generalObj.GenerationTokenCount > 0) {
		return generalObj.PromptTokenCount, generalObj.GenerationTokenCount, nil
	}

	return 0, 0, nil
}

func (b *BedrockProvider) FormatBudgetExceededSSE() []byte {
	msg := eventstream.Message{
		Headers: eventstream.Headers{
			{Name: ":message-type", Value: eventstream.StringValue("exception")},
			{Name: ":exception-type", Value: eventstream.StringValue("ValidationException")},
			{Name: ":content-type", Value: eventstream.StringValue("application/json")},
		},
		Payload: []byte(`{"message":"Budget exceeded mid-stream"}`),
	}
	var buf bytes.Buffer
	encoder := eventstream.NewEncoder()
	_ = encoder.Encode(&buf, msg)
	return buf.Bytes()
}
