package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/loopers-oss/loopers/internal/provider"
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
func NewSSEStreamReader(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice, reservedUSD float64, onReconcile func(float64, int, int)) io.ReadCloser {
	pr, pw := io.Pipe()

	sr := &SSEStreamReader{
		pipeReader: pr,
		pipeWriter: pw,
	}

	go sr.processStream(ctx, original, prov, inputPrice, outputPrice, reservedUSD, onReconcile)

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

func (sr *SSEStreamReader) processStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice, reservedUSD float64, onReconcile func(float64, int, int)) {
	defer original.Close()
	defer sr.pipeWriter.Close()

	if prov.Name() == "bedrock" {
		sr.processBedrockStream(ctx, original, prov, inputPrice, outputPrice, reservedUSD, onReconcile)
		return
	}

	scanner := bufio.NewScanner(original)
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

	for {
		select {
		case <-ctx.Done():
			actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
			onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
			return
		case chunk, ok := <-chunks:
			if !ok {
				// SSE stream ended normally or closed
				if err := scanner.Err(); err != nil {
					logging.Logger.Error().Err(err).Msg("SSE scanner error during read loop")
				}
				actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
				onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
				return
			}

			// Re-append the double newlines to reconstruct the standard SSE protocol frame
			outChunk := append(chunk, []byte("\n\n")...)

			inTokens, outTokens, isDone, err := prov.ParseStreamChunk(chunk)
			if err == nil {
				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
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
					if cost > reservedUSD {
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onReconcile(cost, totalInputTokens, totalOutputTokens)
						return
					}
				}
			}

			_, _ = sr.pipeWriter.Write(outChunk)
		}
	}
}

func (sr *SSEStreamReader) processBedrockStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice, reservedUSD float64, onReconcile func(float64, int, int)) {
	var totalInputTokens int
	var totalOutputTokens int

	for {
		select {
		case <-ctx.Done():
			actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
			onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
			return
		default:
			// Read one frame of binary event stream
			frameBytes, err := readEventStreamFrame(original)
			if err != nil {
				if err == io.EOF {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
					return
				}
				logging.Logger.Error().Err(err).Msg("Error reading AWS Bedrock EventStream frame")
				return
			}

			// Decode the message using eventstream decoder to inspect payload
			decoder := eventstream.NewDecoder()
			msg, err := decoder.Decode(bytes.NewReader(frameBytes), nil)
			if err != nil {
				logging.Logger.Error().Err(err).Msg("Error decoding AWS Bedrock EventStream message")
				// Even if decoding fails, forward the raw frame to avoid blocking client
				_, _ = sr.pipeWriter.Write(frameBytes)
				continue
			}

			// Inspect message payload for token count
			inTokens, outTokens, isDone, err := prov.ParseStreamChunk(msg.Payload)
			if err == nil {
				if inTokens > 0 {
					totalInputTokens = inTokens
				}
				if outTokens > 0 {
					totalOutputTokens = outTokens
				}

				if inTokens > 0 || outTokens > 0 {
					cost := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					if cost > reservedUSD {
						// Write exception frame and stop
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onReconcile(cost, totalInputTokens, totalOutputTokens)
						return
					}
				}

				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onReconcile(actualUSD, totalInputTokens, totalOutputTokens)
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
	frameBytes := make([]byte, totalLen)
	copy(frameBytes[0:4], prelude)
	_, err = io.ReadFull(r, frameBytes[4:])
	if err != nil {
		return nil, err
	}
	return frameBytes, nil
}
