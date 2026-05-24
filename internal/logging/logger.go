package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger is the global logger for the loopers proxy.
var Logger zerolog.Logger

// InitLogger initializes the global logger.
func InitLogger(levelStr string) {
	var level zerolog.Level
	switch levelStr {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	default:
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339

	// Wrap standard output with redaction filter
	redactedStdout := NewRedactWriter(os.Stdout)

	Logger = zerolog.New(redactedStdout).With().Timestamp().Logger()
}
