package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/SladkyCitron/slogcolor"
	"github.com/danielmichaels/shunt/config"
)

func NewLogger(cfg *config.LogConfig) (*slog.Logger, error) {
	if cfg == nil {
		return nil, fmt.Errorf("logger config is nil")
	}

	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	switch cfg.Encoding {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	default:
		handler = slogcolor.NewHandler(os.Stderr, &slogcolor.Options{Level: level})
	}

	return slog.New(handler), nil
}

func NewNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
