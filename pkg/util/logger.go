package util

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NewLogger returns a configured zerolog.Logger with the specified log level.
func NewLogger(level zerolog.Level) zerolog.Logger {
	// Initialize base logger with console output for development or JSON for production
	var logger zerolog.Logger
	stage := os.Getenv("STAGE")
	if strings.EqualFold(stage, "local") {
		// Pretty printing for development
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			With().
			Str("app", "ike-"+stage).
			Timestamp().
			Logger()
	} else {
		// JSON output for production
		logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Str("app", "ike-"+stage).
			Logger()
	}

	// Set UNIX timestamp format for production
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Set log level
	zerolog.SetGlobalLevel(level)

	return logger
}
