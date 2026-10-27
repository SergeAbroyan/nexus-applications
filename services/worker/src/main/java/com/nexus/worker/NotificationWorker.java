package com.nexus.worker;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import java.util.List;
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

    private final SqsClient sqs;
    private final WorkerProperties properties;
    private final ObjectMapper objectMapper;
    private final Counter processed;
    private final Counter failed;
    private final Counter receiveFailures;

    NotificationWorker(
            SqsClient sqs,
            WorkerProperties properties,
            ObjectMapper objectMapper,
            MeterRegistry meterRegistry) {
        this.sqs = sqs;
        this.properties = properties;
        this.objectMapper = objectMapper;
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
                .messageAttributeNames(TRACE_PARENT)
                .build();
    }

    private void process(Message message) {
        var traceId = traceId(message);
        if (traceId != null) {
            MDC.put(TRACE_ID, traceId);
        }
        try {
            var notification = objectMapper.readValue(message.body(), Notification.class);
            logger.info("Processed notification created at {}: {}", notification.createdAt(), notification.message());
            sqs.deleteMessage(DeleteMessageRequest.builder()
                    .queueUrl(properties.queueUrl())
                    .receiptHandle(message.receiptHandle())
                    .build());
            processed.increment();
        } catch (RuntimeException error) {
            failed.increment();
            logger.warn("Could not process message {}; SQS will deliver it again", message.messageId(), error);
        } finally {
            MDC.remove(TRACE_ID);
        }
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
