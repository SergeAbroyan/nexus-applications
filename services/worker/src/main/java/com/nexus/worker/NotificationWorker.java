package com.nexus.worker;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.Scope;
import io.opentelemetry.context.propagation.TextMapGetter;
import io.opentelemetry.context.propagation.TextMapPropagator;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.DeleteMessageRequest;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.MessageAttributeValue;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageRequest;
import tools.jackson.databind.ObjectMapper;

@Component
class NotificationWorker {

    private static final Logger logger = LoggerFactory.getLogger(NotificationWorker.class);
    private static final String TRACE_PARENT = "traceparent";
    private static final String TRACE_ID = "trace_id";
    private static final TextMapGetter<Map<String, MessageAttributeValue>> MESSAGE_ATTRIBUTES = new TextMapGetter<>() {
        @Override
        public Iterable<String> keys(Map<String, MessageAttributeValue> attributes) {
            return attributes.keySet();
        }

        @Override
        public String get(Map<String, MessageAttributeValue> attributes, String key) {
            if (attributes == null) {
                return null;
            }
            var value = attributes.get(key);
            return value == null ? null : value.stringValue();
        }
    };

    private final SqsClient sqs;
    private final WorkerProperties properties;
    private final ObjectMapper objectMapper;
    private final Tracer tracer;
    private final TextMapPropagator propagator;
    private final List<String> attributeNames;
    private final String queueName;
    private final Counter processed;
    private final Counter failed;
    private final Counter receiveFailures;

    NotificationWorker(
            SqsClient sqs,
            WorkerProperties properties,
            ObjectMapper objectMapper,
            MeterRegistry meterRegistry,
            OpenTelemetry openTelemetry) {
        this.sqs = sqs;
        this.properties = properties;
        this.objectMapper = objectMapper;
        this.tracer = openTelemetry.getTracer("com.nexus.worker");
        this.propagator = openTelemetry.getPropagators().getTextMapPropagator();
        var names = new LinkedHashSet<>(propagator.fields());
        names.add(TRACE_PARENT);
        this.attributeNames = List.copyOf(names);
        this.queueName = properties.queueUrl().substring(properties.queueUrl().lastIndexOf('/') + 1);
        this.processed = meterRegistry.counter("nexus.worker.messages", "outcome", "processed");
        this.failed = meterRegistry.counter("nexus.worker.messages", "outcome", "failed");
        this.receiveFailures = meterRegistry.counter("nexus.worker.receive.failures");
    }

    @Scheduled(fixedDelayString = "${nexus.worker.poll-delay-ms}")
    void poll() {
        List<Message> messages;
        try {
            messages = sqs.receiveMessage(receiveRequest()).messages();
        } catch (RuntimeException error) {
            receiveFailures.increment();
            logger.warn("Could not receive messages from SQS", error);
            return;
        }
        messages.forEach(this::process);
    }

    private ReceiveMessageRequest receiveRequest() {
        return ReceiveMessageRequest.builder()
                .queueUrl(properties.queueUrl())
                .maxNumberOfMessages(10)
                .waitTimeSeconds(properties.waitTimeSeconds())
                .visibilityTimeout(properties.visibilityTimeoutSeconds())
                .messageAttributeNames(attributeNames)
                .build();
    }

    private void process(Message message) {
        var span = startProcessSpan(message);
        var traceId = span.getSpanContext().isValid() ? span.getSpanContext().getTraceId() : traceId(message);
        if (traceId != null) {
            MDC.put(TRACE_ID, traceId);
        }
        try (Scope ignored = span.makeCurrent()) {
            var notification = objectMapper.readValue(message.body(), Notification.class);
            logger.info("Processed notification created at {}: {}", notification.createdAt(), notification.message());
            sqs.deleteMessage(DeleteMessageRequest.builder()
                    .queueUrl(properties.queueUrl())
                    .receiptHandle(message.receiptHandle())
                    .build());
            processed.increment();
        } catch (RuntimeException error) {
            span.recordException(error);
            span.setStatus(StatusCode.ERROR, "message processing failed");
            failed.increment();
            logger.warn("Could not process message {}; SQS will deliver it again", message.messageId(), error);
        } finally {
            span.end();
            MDC.remove(TRACE_ID);
        }
    }

    private Span startProcessSpan(Message message) {
        var producer = propagator.extract(Context.root(), message.messageAttributes(), MESSAGE_ATTRIBUTES);
        return tracer.spanBuilder("process " + queueName)
                .setParent(producer)
                .setSpanKind(SpanKind.CONSUMER)
                .setAttribute("messaging.system", "aws_sqs")
                .setAttribute("messaging.operation.type", "process")
                .setAttribute("messaging.destination.name", queueName)
                .setAttribute("messaging.message.id", message.messageId())
                .startSpan();
    }

    private static String traceId(Message message) {
        MessageAttributeValue attribute = message.messageAttributes().get(TRACE_PARENT);
        if (attribute == null || attribute.stringValue() == null) {
            return null;
        }
        var fields = attribute.stringValue().split("-");
        return fields.length == 4 && fields[1].length() == 32 ? fields[1] : null;
    }
}
