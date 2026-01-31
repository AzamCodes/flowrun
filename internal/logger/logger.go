package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
)

func init() {
	// Default to text handler on stderr, info level
	defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Config holds configuration for the logger.
type Config struct {
	Level  string // DEBUG, INFO, WARN, ERROR
	Format string // text, json
	Output io.Writer
}

// Setup configures the global logger.
func Setup(cfg Config) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Level),
	}

	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	defaultLogger = slog.New(handler)
}

func parseLevel(lvl string) slog.Level {
	switch lvl {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Debug logs at DEBUG level.
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info logs at INFO level.
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn logs at WARN level.
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error logs at ERROR level.
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// Log logs with a context.
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	defaultLogger.Log(ctx, level, msg, args...)
}
