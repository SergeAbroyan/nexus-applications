package com.nexus.worker;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Path;
import java.time.Instant;
import org.junit.jupiter.api.Test;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;

class NotificationTest {

    private final ObjectMapper objectMapper = new ObjectMapper();

    @Test
    void readsTheMessageThatTheApiSends() {
        var notification = objectMapper.readValue("""
                {"type":"notification.requested","message":"hello","createdAt":"2026-10-27T09:30:00Z"}
                """, Notification.class);

        assertThat(notification.message()).isEqualTo("hello");
        assertThat(notification.createdAt()).isEqualTo(Instant.parse("2026-10-27T09:30:00Z"));
    }

    @Test
    void readsTheSharedContractExample() {
        var notification = objectMapper.readValue(
                Path.of("../../contracts/notification.example.json").toFile(), Notification.class);

        assertThat(notification.type()).isEqualTo("notification.requested");
        assertThat(notification.message()).isEqualTo("hello");
    }

    @Test
    void ignoresFieldsItDoesNotKnow() {
        var notification = objectMapper.readValue("""
                {"type":"notification.requested","message":"hello","createdAt":"2026-10-27T09:30:00Z","priority":"high"}
                """, Notification.class);

        assertThat(notification.message()).isEqualTo("hello");
    }

    @Test
    void rejectsAnUnsupportedType() {
        assertThatThrownBy(() -> objectMapper.readValue("""
                {"type":"unknown","message":"hello","createdAt":"2026-10-27T09:30:00Z"}
                """, Notification.class))
                .isInstanceOf(JacksonException.class)
                .hasMessageContaining("unsupported notification type");
    }

    @Test
    void rejectsAnEmptyMessage() {
        assertThatThrownBy(() -> objectMapper.readValue("""
                {"type":"notification.requested","message":" ","createdAt":"2026-10-27T09:30:00Z"}
                """, Notification.class))
                .isInstanceOf(JacksonException.class)
                .hasMessageContaining("message is empty");
    }
}
