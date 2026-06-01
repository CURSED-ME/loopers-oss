package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"
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
func NewSSEStreamReader(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, bool)) io.ReadCloser {
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

func (sr *SSEStreamReader) processStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, bool)) {
	defer original.Close()
	defer sr.pipeWriter.Close()

	if prov.Name() == "bedrock" {
		sr.processBedrockStream(ctx, original, prov, inputPrice, outputPrice, checkBudget, onStreamEnd)
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
			onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
			return
		case chunk, ok := <-chunks:
			if !ok {
				// SSE stream ended normally or closed
				if err := scanner.Err(); err != nil {
					logging.Logger.Error().Err(err).Msg("SSE scanner error during read loop")
				}
				actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
				onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
				return
			}

			// Re-append the double newlines to reconstruct the standard SSE protocol frame
			outChunk := append(chunk, []byte("\n\n")...)

			inTokens, outTokens, isDone, err := prov.ParseStreamChunk(chunk)
			if err == nil {
				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
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
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onStreamEnd(cost, totalInputTokens, totalOutputTokens, true)
						return
					}
				}
			}

			_, _ = sr.pipeWriter.Write(outChunk)
		}
	}
}

func (sr *SSEStreamReader) processBedrockStream(ctx context.Context, original io.ReadCloser, prov provider.Provider, inputPrice, outputPrice float64, checkBudget func(float64) bool, onStreamEnd func(float64, int, int, bool)) {
	var totalInputTokens int
	var totalOutputTokens int

	for {
		select {
		case <-ctx.Done():
			actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
			onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
			return
		default:
			// Read one frame of binary event stream
			frameBytes, err := readEventStreamFrame(original)
			if err != nil {
				if err == io.EOF {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
					return
				}
				logging.Logger.Error().Err(err).Msg("Error reading AWS Bedrock EventStream frame")
				actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
				onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, true)
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
						// Write exception frame and stop
						_, _ = sr.pipeWriter.Write(prov.FormatBudgetExceededSSE())
						onStreamEnd(cost, totalInputTokens, totalOutputTokens, true)
						return
					}
				}

				if isDone {
					actualUSD := (float64(totalInputTokens)*inputPrice + float64(totalOutputTokens)*outputPrice) / 1000000.0
					onStreamEnd(actualUSD, totalInputTokens, totalOutputTokens, false)
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
