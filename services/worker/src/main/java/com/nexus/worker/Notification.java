package com.nexus.worker;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
record Notification(String type, String message, Instant createdAt) {

    static final String REQUESTED = "notification.requested";

    Notification {
        if (!REQUESTED.equals(type)) {
            throw new IllegalArgumentException("unsupported notification type: " + type);
        }
        if (message == null || message.isBlank()) {
            throw new IllegalArgumentException("notification message is empty");
        }
        if (createdAt == null) {
            throw new IllegalArgumentException("notification timestamp is missing");
        }
    }
}
