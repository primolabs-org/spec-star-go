package platform

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

type loggerKey struct{}

// LoggerFactory creates per-request enriched loggers with correlation fields.
type LoggerFactory struct {
	base      *slog.Logger
	coldStart atomic.Bool
}

// NewLoggerFactory creates a LoggerFactory with a JSON handler writing to os.Stdout
// and sets slog.SetDefault so all log output uses the same structured format.
func NewLoggerFactory(service string, level slog.Leveler) *LoggerFactory {
	return newLoggerFactory(service, level, os.Stdout)
}

func newLoggerFactory(service string, level slog.Leveler, w io.Writer) *LoggerFactory {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	base := slog.New(handler).With("service", service)
	slog.SetDefault(base)

	f := &LoggerFactory{base: base}
	f.coldStart.Store(true)
	return f
}

// FromContext returns a logger enriched with trigger, operation, request_id (from
// Lambda context when available), and cold_start fields.
func (f *LoggerFactory) FromContext(ctx context.Context, trigger string, operation string) *slog.Logger {
	coldStart := f.coldStart.Swap(false)

	attrs := []any{
		"trigger", trigger,
		"operation", operation,
		"cold_start", coldStart,
	}

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		attrs = append(attrs, "request_id", lc.AwsRequestID)
	}

	return f.base.With(attrs...)
}

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext retrieves the logger from context. Returns slog.Default() when
// no logger is stored.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
