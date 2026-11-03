package com.nexus.worker;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.ContextPropagators;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.testing.exporter.InMemorySpanExporter;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.export.SimpleSpanProcessor;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import software.amazon.awssdk.core.exception.SdkClientException;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.DeleteMessageRequest;
import software.amazon.awssdk.services.sqs.model.DeleteMessageResponse;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.MessageAttributeValue;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageRequest;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageResponse;
import tools.jackson.databind.ObjectMapper;

class NotificationWorkerTest {

    private static final String QUEUE_URL = "https://sqs.us-east-1.amazonaws.com/111111111111/jobs";
    private static final String NOTIFICATION = """
            {"type":"notification.requested","message":"hello","createdAt":"2026-10-27T09:30:00Z"}
            """;
    private static final String TRACE_ID = "4bf92f3577b34da6a3ce929d0e0e4736";
    private static final String PARENT_SPAN_ID = "00f067aa0ba902b7";

    private final FakeSqs sqs = new FakeSqs();
    private final SimpleMeterRegistry meters = new SimpleMeterRegistry();

    @Test
    void deletesAMessageAfterProcessingIt() {
        sqs.queue(NOTIFICATION);

        worker(OpenTelemetry.noop()).poll();

        assertThat(sqs.deleted).containsExactly(DeleteMessageRequest.builder()
                .queueUrl(QUEUE_URL)
                .receiptHandle("receipt-1")
                .build());
        assertThat(messages("processed")).isEqualTo(1);
    }

    @Test
    void keepsAnInvalidMessageForRedelivery() {
        sqs.queue("""
                {"type":"unknown"}
                """);

        worker(OpenTelemetry.noop()).poll();

        assertThat(sqs.deleted).isEmpty();
        assertThat(messages("failed")).isEqualTo(1);
    }

    @Test
    void countsReceiveFailures() {
        sqs.receiveFailure = SdkClientException.create("unreachable");

        worker(OpenTelemetry.noop()).poll();

        assertThat(meters.counter("nexus.worker.receive.failures").count()).isEqualTo(1);
    }

    @Test
    void continuesTheTraceOfTheRequestThatQueuedTheMessage() {
        var spans = InMemorySpanExporter.create();
        var openTelemetry = OpenTelemetrySdk.builder()
                .setTracerProvider(SdkTracerProvider.builder()
                        .addSpanProcessor(SimpleSpanProcessor.create(spans))
                        .build())
                .setPropagators(ContextPropagators.create(W3CTraceContextPropagator.getInstance()))
                .build();
        sqs.queue(NOTIFICATION, Map.of("traceparent", MessageAttributeValue.builder()
                .dataType("String")
                .stringValue("00-" + TRACE_ID + "-" + PARENT_SPAN_ID + "-01")
                .build()));

        worker(openTelemetry).poll();

        assertThat(sqs.lastReceive.messageAttributeNames()).contains("traceparent", "tracestate");
        assertThat(spans.getFinishedSpanItems()).singleElement().satisfies(span -> {
            assertThat(span.getName()).isEqualTo("process jobs");
            assertThat(span.getKind()).isEqualTo(SpanKind.CONSUMER);
            assertThat(span.getTraceId()).isEqualTo(TRACE_ID);
            assertThat(span.getParentSpanId()).isEqualTo(PARENT_SPAN_ID);
        });
    }

    private NotificationWorker worker(OpenTelemetry openTelemetry) {
        return new NotificationWorker(
                sqs, new WorkerProperties(QUEUE_URL, 20, 60, 1000), new ObjectMapper(), meters, openTelemetry);
    }

    private double messages(String outcome) {
        return meters.counter("nexus.worker.messages", "outcome", outcome).count();
    }

    private static final class FakeSqs implements SqsClient {

        private final List<Message> received = new ArrayList<>();
        private final List<DeleteMessageRequest> deleted = new ArrayList<>();
        private RuntimeException receiveFailure;
        private ReceiveMessageRequest lastReceive;

        void queue(String body) {
            queue(body, Map.of());
        }

        void queue(String body, Map<String, MessageAttributeValue> attributes) {
            received.add(Message.builder()
                    .messageId("message-1")
                    .receiptHandle("receipt-1")
                    .body(body)
                    .messageAttributes(attributes)
                    .build());
        }

        @Override
        public ReceiveMessageResponse receiveMessage(ReceiveMessageRequest request) {
            lastReceive = request;
            if (receiveFailure != null) {
                throw receiveFailure;
            }
            return ReceiveMessageResponse.builder().messages(received).build();
        }

        @Override
        public DeleteMessageResponse deleteMessage(DeleteMessageRequest request) {
            deleted.add(request);
            return DeleteMessageResponse.builder().build();
        }

        @Override
        public String serviceName() {
            return SERVICE_NAME;
        }

        @Override
        public void close() {
        }
    }
}
