package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type ShutdownTracing func(context.Context) error

func SetupTracing(ctx context.Context, serviceName string) (ShutdownTracing, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	sampleRatio, err := traceSampleRatio()
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	serviceResource, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(serviceResource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

func traceSampleRatio() (float64, error) {
	value := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if value == "" {
		value = "0.01"
	}

	ratio, err := strconv.ParseFloat(value, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be a number between 0 and 1")
	}
	return ratio, nil
}

func TraceHTTPHandler(serviceName string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(
		next,
		serviceName+".http",
		otelhttp.WithFilter(func(request *http.Request) bool {
			switch request.URL.Path {
			case "/healthz", "/readyz", "/metrics":
				return false
			default:
				return true
			}
		}),
		otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
			return "HTTP " + request.Method
		}),
	)
}

func TraceHTTPTransport(transport http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(transport)
}

func InjectTraceContext(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier
}

func ExtractTraceContext(ctx context.Context, values map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(values))
}

func RecordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func NewLogger() *slog.Logger {
	return slog.New(traceHandler{slog.NewJSONHandler(os.Stdout, nil)})
}

type traceHandler struct {
	slog.Handler
}

func (handler traceHandler) Handle(ctx context.Context, record slog.Record) error {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
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
