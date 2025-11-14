package todo

import (
	"log/slog"
	"os"
	"strings"
)

func NewLogger() *slog.Logger {
	level := slog.LevelInfo
	switch s := os.Getenv("LOG_LEVEL"); {
	case strings.EqualFold(s, "DEBUG"):
		level = slog.LevelDebug
	case strings.EqualFold(s, "INFO"):
		level = slog.LevelInfo
	case strings.EqualFold(s, "WARN"):
		level = slog.LevelWarn
	case strings.EqualFold(s, "ERROR"):
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
