package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Logger defines minimal logging interface used across layers.
type Logger interface {
	Debug(ctx context.Context, msg string, kv ...any)
	Debugf(ctx context.Context, format string, args ...any)
	Info(ctx context.Context, msg string, kv ...any)
	Infof(ctx context.Context, format string, args ...any)
	Warn(ctx context.Context, msg string, kv ...any)
	Warnf(ctx context.Context, format string, args ...any)
	Error(ctx context.Context, msg string, kv ...any)
	Errorf(ctx context.Context, format string, args ...any)
	With(kv ...any) Logger
}

type contextKey struct{}

var loggerKey contextKey

// WithLogger stores a logger in context.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves a logger from context, returns default logger if absent.
func FromContext(ctx context.Context) Logger {
	if v, ok := ctx.Value(loggerKey).(Logger); ok && v != nil {
		return v
	}
	return humanLogger(slog.LevelInfo)
}

// New constructs a new Logger of given format (text|json|human) and level.
func New(format string, level slog.Leveler) (Logger, error) {
	return NewWithWriter(format, level, os.Stderr)
}

// NewWithWriter constructs a new Logger of given format, level, and output writer.
func NewWithWriter(format string, level slog.Leveler, w io.Writer) (Logger, error) {
	switch format {
	case "", "human":
		return humanLoggerWithWriter(level, w), nil
	case "text":
		return &slogWrapper{logger: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level, AddSource: false}))}, nil
	case "json":
		return &slogWrapper{logger: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, AddSource: false}))}, nil
	default:
		return nil, errors.New("unsupported log format: " + format)
	}
}

// slogWrapper adapts slog.Logger to Logger.
type slogWrapper struct{ logger *slog.Logger }

func (l *slogWrapper) Debug(ctx context.Context, msg string, kv ...any) {
	l.logger.DebugContext(ctx, msg, kv...)
}
func (l *slogWrapper) Debugf(ctx context.Context, format string, args ...any) {
	l.logger.DebugContext(ctx, fmt.Sprintf(format, args...))
}
func (l *slogWrapper) Info(ctx context.Context, msg string, kv ...any) {
	l.logger.InfoContext(ctx, msg, kv...)
}
func (l *slogWrapper) Infof(ctx context.Context, format string, args ...any) {
	l.logger.InfoContext(ctx, fmt.Sprintf(format, args...))
}
func (l *slogWrapper) Warn(ctx context.Context, msg string, kv ...any) {
	l.logger.WarnContext(ctx, msg, kv...)
}
func (l *slogWrapper) Warnf(ctx context.Context, format string, args ...any) {
	l.logger.WarnContext(ctx, fmt.Sprintf(format, args...))
}
func (l *slogWrapper) Error(ctx context.Context, msg string, kv ...any) {
	l.logger.ErrorContext(ctx, msg, kv...)
}
func (l *slogWrapper) Errorf(ctx context.Context, format string, args ...any) {
	l.logger.ErrorContext(ctx, fmt.Sprintf(format, args...))
}

func (l *slogWrapper) With(kv ...any) Logger { return &slogWrapper{logger: l.logger.With(kv...)} }

var (
	humanLoggerOnce  sync.Once
	humanLoggerValue *slogWrapper
)

func humanLogger(level slog.Leveler) *slogWrapper {
	return humanLoggerWithWriter(level, os.Stderr)
}

func humanLoggerWithWriter(level slog.Leveler, w io.Writer) *slogWrapper {
	// Set level for the std log logger used by slog's default logger output.
	slog.SetLogLoggerLevel(level.Level())
	if w == os.Stderr {
		humanLoggerOnce.Do(func() {
			humanLoggerValue = &slogWrapper{logger: slog.Default()}
		})
		return humanLoggerValue
	}
	// For non-stderr writers, create a new handler
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level, AddSource: false})
	return &slogWrapper{logger: slog.New(handler)}
}
