package sqs

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestMessageAttributesCarryTheTraceContext(t *testing.T) {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	attributes := messageAttributes{}
	propagation.TraceContext{}.Inject(ctx, attributes)

	extracted := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), attributes))
	if extracted.TraceID() != spanContext.TraceID() {
		t.Fatalf("trace ID = %s, want %s", extracted.TraceID(), spanContext.TraceID())
	}
	if dataType := attributes["traceparent"].DataType; dataType == nil || *dataType != "String" {
		t.Fatal("traceparent must be a String message attribute")
	}
}
