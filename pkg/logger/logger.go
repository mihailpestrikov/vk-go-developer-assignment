package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"vk-go-developer-assignment/pkg/config"
)

func SetupLogger(cfg config.LogConfig) zerolog.Logger {
	var output io.Writer = os.Stdout
	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	level := zerolog.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	zerolog.SetGlobalLevel(level)

	return zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()
}
