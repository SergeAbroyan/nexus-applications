package com.nexus.worker;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.MessageSystemAttributeName;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageRequest;
import tools.jackson.databind.ObjectMapper;

@Component
class NotificationWorker {

    private static final Logger logger = LoggerFactory.getLogger(NotificationWorker.class);

    private final SqsClient sqsClient;
    private final WorkerProperties properties;
    private final NotificationParser parser;
    private final Counter processed;
    private final Counter failed;

    NotificationWorker(
            SqsClient sqsClient,
            WorkerProperties properties,
            ObjectMapper objectMapper,
            MeterRegistry meterRegistry) {
        this.sqsClient = sqsClient;
        this.properties = properties;
        this.parser = new NotificationParser(objectMapper);
        this.processed = meterRegistry.counter("nexus.worker.messages.processed");
        this.failed = meterRegistry.counter("nexus.worker.messages.failed");
    }

    @Scheduled(fixedDelayString = "${nexus.worker.poll-delay-ms}")
    void poll() {
        try {
            receiveMessages().forEach(this::process);
        } catch (RuntimeException error) {
            failed.increment();
            logger.warn("Could not receive SQS messages", error);
        }
    }

    private List<Message> receiveMessages() {
        var request = ReceiveMessageRequest.builder()
                .queueUrl(properties.queueUrl())
                .maxNumberOfMessages(10)
                .waitTimeSeconds(properties.waitTimeSeconds())
                .visibilityTimeout(properties.visibilityTimeoutSeconds())
                .messageSystemAttributeNames(MessageSystemAttributeName.SENT_TIMESTAMP)
                .messageAttributeNames("traceparent", "tracestate", "baggage")
                .build();
        return sqsClient.receiveMessage(request).messages();
    }

    private void process(Message message) {
        try {
            var notification = parser.parse(message.body());
            logger.info(
                    "Notification processed type={} createdAt={} message={}",
                    notification.type(),
                    notification.createdAt(),
                    notification.message());
            sqsClient.deleteMessage(request -> request
                    .queueUrl(properties.queueUrl())
                    .receiptHandle(message.receiptHandle()));
            processed.increment();
        } catch (RuntimeException error) {
            failed.increment();
            logger.warn("Notification processing failed; SQS will retry it", error);
        }
    }
}
