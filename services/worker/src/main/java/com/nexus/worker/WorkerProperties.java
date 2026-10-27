package com.nexus.worker;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("nexus.worker")
public record WorkerProperties(
        String queueUrl,
        int waitTimeSeconds,
        int visibilityTimeoutSeconds,
        long pollDelayMs) {

    public WorkerProperties {
        if (queueUrl == null || queueUrl.isBlank()) {
            throw new IllegalArgumentException("SQS_QUEUE_URL is required");
        }
        if (waitTimeSeconds < 1 || waitTimeSeconds > 20) {
            throw new IllegalArgumentException("SQS_WAIT_TIME_SECONDS must be between 1 and 20");
        }
        if (visibilityTimeoutSeconds < 1 || visibilityTimeoutSeconds > 43_200) {
            throw new IllegalArgumentException("SQS_VISIBILITY_TIMEOUT_SECONDS must be between 1 and 43200");
        }
        if (pollDelayMs < 0) {
            throw new IllegalArgumentException("SQS_POLL_DELAY_MS cannot be negative");
        }
    }
}
