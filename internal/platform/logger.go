package platform

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"go.opentelemetry.io/otel/trace"
)

type loggerKey struct{}

type traceContextHandler struct {
	inner slog.Handler
}

func (h *traceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		sc := span.SpanContext()
		if sc.IsValid() {
			record.AddAttrs(
				slog.String("trace_id", sc.TraceID().String()),
				slog.String("span_id", sc.SpanID().String()),
			)
		}
	}
	return h.inner.Handle(ctx, record)
}

func (h *traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceContextHandler) WithGroup(name string) slog.Handler {
	return &traceContextHandler{inner: h.inner.WithGroup(name)}
}

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
	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	handler := &traceContextHandler{inner: jsonHandler}
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
