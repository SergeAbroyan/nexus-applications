package com.nexus.worker;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import software.amazon.awssdk.core.exception.SdkClientException;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.DeleteMessageRequest;
import software.amazon.awssdk.services.sqs.model.DeleteMessageResponse;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageRequest;
import software.amazon.awssdk.services.sqs.model.ReceiveMessageResponse;
import tools.jackson.databind.ObjectMapper;

class NotificationWorkerTest {

    private static final String QUEUE_URL = "https://sqs.us-east-1.amazonaws.com/111111111111/jobs";

    private final FakeSqs sqs = new FakeSqs();
    private final SimpleMeterRegistry meters = new SimpleMeterRegistry();
    private final NotificationWorker worker = new NotificationWorker(
            sqs, new WorkerProperties(QUEUE_URL, 20, 60, 1000), new ObjectMapper(), meters);

    @Test
    void deletesAMessageAfterProcessingIt() {
        sqs.queue("""
                {"type":"notification.requested","message":"hello","createdAt":"2026-10-27T09:30:00Z"}
                """);

        worker.poll();

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

        worker.poll();

        assertThat(sqs.deleted).isEmpty();
        assertThat(messages("failed")).isEqualTo(1);
    }

    @Test
    void countsReceiveFailures() {
        sqs.receiveFailure = SdkClientException.create("unreachable");

        worker.poll();

        assertThat(meters.counter("nexus.worker.receive.failures").count()).isEqualTo(1);
    }

    private double messages(String outcome) {
        return meters.counter("nexus.worker.messages", "outcome", outcome).count();
    }

    private static final class FakeSqs implements SqsClient {

        private final List<Message> received = new ArrayList<>();
        private final List<DeleteMessageRequest> deleted = new ArrayList<>();
        private RuntimeException receiveFailure;

        void queue(String body) {
            received.add(Message.builder().messageId("message-1").receiptHandle("receipt-1").body(body).build());
        }

        @Override
        public ReceiveMessageResponse receiveMessage(ReceiveMessageRequest request) {
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
