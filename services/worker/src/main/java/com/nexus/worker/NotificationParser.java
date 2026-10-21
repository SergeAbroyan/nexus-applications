package com.nexus.worker;

import tools.jackson.core.JacksonException;
import tools.jackson.databind.DeserializationFeature;
import tools.jackson.databind.ObjectMapper;

final class NotificationParser {

    private final ObjectMapper objectMapper;

    NotificationParser(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper.rebuild()
                .enable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
                .build();
    }

    Notification parse(String body) throws JacksonException {
        var notification = objectMapper.readValue(body, Notification.class);
        if (!"notification.requested".equals(notification.type())) {
            throw new IllegalArgumentException("unsupported notification type");
        }
        if (notification.message() == null || notification.message().isBlank()) {
            throw new IllegalArgumentException("notification message is empty");
        }
        if (notification.createdAt() == null) {
            throw new IllegalArgumentException("notification timestamp is missing");
        }
        return notification;
    }
}
