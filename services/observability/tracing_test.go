package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceSampleRatio(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

	ratio, err := traceSampleRatio()
	if err != nil {
		t.Fatal(err)
	}
	if ratio != 0.25 {
		t.Fatalf("expected sample ratio 0.25, got %f", ratio)
	}
}

func TestRejectsInvalidTraceSampleRatio(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "2")

	if _, err := traceSampleRatio(); err == nil {
		t.Fatal("expected an invalid sample ratio error")
	}
}

func TestTraceContextRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	original := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), original)

	values := InjectTraceContext(ctx)
	extracted := trace.SpanContextFromContext(ExtractTraceContext(context.Background(), values))

	if extracted.TraceID() != original.TraceID() {
		t.Fatalf("expected trace ID %s, got %s", original.TraceID(), extracted.TraceID())
	}
	if !extracted.IsRemote() {
		t.Fatal("expected the extracted span context to be remote")
	}
}
