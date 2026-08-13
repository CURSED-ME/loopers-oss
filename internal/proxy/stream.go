package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"io"
	"time"
)

// splitSSEFrames is a bufio.SplitFunc that splits on double newlines (\n\n).
func splitSSEFrames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// SSEStreamReader intercepts response content on-the-fly via an io.Pipe.
type SSEStreamReader struct {
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

// NewSSEStreamReader creates a new SSEStreamReader implementing io.ReadCloser.
func NewSSEStreamReader(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, string, bool)) io.ReadCloser {
	pr, pw := io.Pipe()

	sr := &SSEStreamReader{
		pipeReader: pr,
		pipeWriter: pw,
	}

	go sr.processStream(ctx, original, prov, inputPrice, outputPrice, checkBudget, onStreamEnd)

	return sr
}

// Read reads intercepted data from the pipe.
func (sr *SSEStreamReader) Read(p []byte) (int, error) {
	return sr.pipeReader.Read(p)
}

// Close closes the reader.
func (sr *SSEStreamReader) Close() error {
	return sr.pipeReader.Close()
}

func (sr *SSEStreamReader) processStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, string, bool)) {
	defer original.Close()
	defer sr.pipeWriter.Close()

	if prov.Name() == "bedrock" {
		sr.processBedrockStream(ctx, original, prov, inputPrice, outputPrice, checkBudget, onStreamEnd)
		return
	}

	scanner := bufio.NewScanner(original)
	const maxSSEFrameSize = 10 * 1024 * 1024 // 10MB
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEFrameSize)
	scanner.Split(splitSSEFrames)

	chunks := make(chan []byte, 32)
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Upstream reader goroutine with slow-client detection
	go func() {
		defer close(chunks)
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		for scanner.Scan() {
			b := scanner.Bytes()
			cp := make([]byte, len(b))
			copy(cp, b)

			select {
			case chunks <- cp:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(5 * time.Second)
			case <-timer.C:
				logging.Logger.Warn().Msg("Slow client detected; aborting upstream streaming reader")
				cancel()
				return
			case <-streamCtx.Done():
				return
			}
		}
	}()

	var totalInputTokens int
	var totalOutputTokens int
	var accumulatedText string

	for {
		select {
		case <-ctx.Done():
			actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
			onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
			return
		case chunk, ok := <-chunks:
			if !ok {
				// SSE stream ended normally or closed
				if err := scanner.Err(); err != nil {
					logging.Logger.Error().Err(err).Msg("SSE scanner error during read loop")
				}
				actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
				onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
				return
			}

			// Re-append the double newlines to reconstruct the standard SSE protocol frame
			outChunk := append(chunk, []byte("\n\n")...)

			// Accumulate response text for transient trace logging
			if len(accumulatedText) < 512 {
				txt := extractTextFromChunk(chunk)
				if txt != "" {
					accumulatedText += txt
					if len(accumulatedText) > 512 {
						accumulatedText = accumulatedText[:512]
					}
				}
			}

			inTokens, outTokens, isDone, err := prov.ParseStreamChunk(chunk)
			if err == nil {
				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
					_, _ = sr.pipeWriter.Write(outChunk)
					return
				}

				if inTokens > 0 {
					totalInputTokens = inTokens
				}
				if outTokens > 0 {
					totalOutputTokens = outTokens
				}

				if inTokens > 0 || outTokens > 0 {
					cost := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					// Check if we have enough budget
					if !checkBudget(cost) {
						// Flush the final chunk containing usage stats that triggered the cutoff.
						// This ensures accurate client-side tracking before we sever the connection.
						_, _ = sr.pipeWriter.Write(outChunk)
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onStreamEnd(cost, totalInputTokens, totalOutputTokens, accumulatedText, true)
						return
					}
				}
			}

			_, _ = sr.pipeWriter.Write(outChunk)
		}
	}
}

func (sr *SSEStreamReader) processBedrockStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, string, bool)) {
	var totalInputTokens int
	var totalOutputTokens int
	var accumulatedText string

	for {
		select {
		case <-ctx.Done():
			actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
			onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
			return
		default:
			// Read one frame of binary event stream
			frameBytes, err := readEventStreamFrame(original)
			if err != nil {
				if err == io.EOF {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
					return
				}
				logging.Logger.Error().Err(err).Msg("Error reading AWS Bedrock EventStream frame")
				actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
				onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, true)
				return
			}

			// We need to parse the tokens from the frame
			inTokens, outTokens, isDone, err := prov.ParseStreamChunk(frameBytes)
			if err == nil {
				if inTokens > 0 {
					totalInputTokens = inTokens
				}
				if outTokens > 0 {
					totalOutputTokens = outTokens
				}

				if inTokens > 0 || outTokens > 0 {
					cost := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					if !checkBudget(cost) {
						// Flush the final frame containing usage stats that triggered the cutoff
						// before writing the exception frame and stopping.
						_, _ = sr.pipeWriter.Write(frameBytes)
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onStreamEnd(cost, totalInputTokens, totalOutputTokens, accumulatedText, true)
						return
					}
				}

				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, accumulatedText, false)
					_, _ = sr.pipeWriter.Write(frameBytes)
					return
				}
			}

			// Forward the original raw binary frame to the client
			_, _ = sr.pipeWriter.Write(frameBytes)
		}
	}
}

func readEventStreamFrame(r io.Reader) ([]byte, error) {
	prelude := make([]byte, 4)
	_, err := io.ReadFull(r, prelude)
	if err != nil {
		return nil, err
	}
	totalLen := binary.BigEndian.Uint32(prelude)
	if totalLen < 12 {
		return nil, fmt.Errorf("invalid eventstream message length: %d", totalLen)
	}
	if totalLen > 10*1024*1024 { // 10MB max frame size
		return nil, fmt.Errorf("eventstream frame too large: %d bytes", totalLen)
	}
	frameBytes := make([]byte, totalLen)
	copy(frameBytes[0:4], prelude)
	_, err = io.ReadFull(r, frameBytes[4:])
	if err != nil {
		return nil, err
	}
	return frameBytes, nil
}

func extractTextFromChunk(chunk []byte) string {
	raw := bytes.TrimSpace(chunk)
	if bytes.HasPrefix(raw, []byte("data: ")) {
		raw = bytes.TrimPrefix(raw, []byte("data: "))
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("[DONE]")) {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}

	// OpenAI/Compatible: choices[0].delta.content
	if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
		if firstChoice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := firstChoice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					return content
				}
			}
		}
	}

	// Anthropic: delta.text
	if delta, ok := data["delta"].(map[string]interface{}); ok {
		if text, ok := delta["text"].(string); ok {
			return text
		}
	}

	// Gemini: candidates[0].content.parts[0].text
	if candidates, ok := data["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if firstCand, ok := candidates[0].(map[string]interface{}); ok {
			if content, ok := firstCand["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
					if firstPart, ok := parts[0].(map[string]interface{}); ok {
						if text, ok := firstPart["text"].(string); ok {
							return text
						}
					}
				}
			}
		}
	}

	return ""
}
