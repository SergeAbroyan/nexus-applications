package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

func NewLogger() *slog.Logger {
	return slog.New(traceHandler{slog.NewJSONHandler(os.Stdout, nil)})
}

type traceHandler struct {
	slog.Handler
}

func (handler traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	return handler.Handler.Handle(ctx, record)
}

func (handler traceHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return traceHandler{handler.Handler.WithAttrs(attributes)}
}

func (handler traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{handler.Handler.WithGroup(name)}
}
