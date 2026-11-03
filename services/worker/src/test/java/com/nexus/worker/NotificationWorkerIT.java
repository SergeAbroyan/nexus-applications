package com.nexus.worker;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.opentelemetry.api.OpenTelemetry;
import java.net.URI;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.QueueAttributeName;
import tools.jackson.databind.ObjectMapper;

@Testcontainers
class NotificationWorkerIT {

    @Container
    private static final GenericContainer<?> ELASTICMQ =
            new GenericContainer<>(DockerImageName.parse("softwaremill/elasticmq-native:1.7.1")).withExposedPorts(9324);

    @Test
    void processesAndDeletesAQueuedNotification() {
        try (var sqs = SqsClient.builder()
                .endpointOverride(URI.create("http://" + ELASTICMQ.getHost() + ":" + ELASTICMQ.getMappedPort(9324)))
                .region(Region.US_EAST_1)
                .credentialsProvider(StaticCredentialsProvider.create(AwsBasicCredentials.create("local", "local")))
                .build()) {
            var queueUrl = sqs.createQueue(request -> request.queueName("nexus-jobs")).queueUrl();
            sqs.sendMessage(request -> request.queueUrl(queueUrl).messageBody("""
                    {"type":"notification.requested","message":"hello","createdAt":"2026-10-27T09:30:00Z"}
                    """));
            var meters = new SimpleMeterRegistry();
            var worker = new NotificationWorker(
                    sqs, new WorkerProperties(queueUrl, 1, 30, 0), new ObjectMapper(), meters, OpenTelemetry.noop());

            worker.poll();

            assertThat(meters.counter("nexus.worker.messages", "outcome", "processed").count()).isEqualTo(1);
            var attributes = sqs.getQueueAttributes(request -> request
                            .queueUrl(queueUrl)
                            .attributeNames(
                                    QueueAttributeName.APPROXIMATE_NUMBER_OF_MESSAGES,
                                    QueueAttributeName.APPROXIMATE_NUMBER_OF_MESSAGES_NOT_VISIBLE))
                    .attributes();
            assertThat(attributes)
                    .containsEntry(QueueAttributeName.APPROXIMATE_NUMBER_OF_MESSAGES, "0")
                    .containsEntry(QueueAttributeName.APPROXIMATE_NUMBER_OF_MESSAGES_NOT_VISIBLE, "0");
        }
    }
}
