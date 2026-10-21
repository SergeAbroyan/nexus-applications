package com.nexus.worker;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import tools.jackson.databind.ObjectMapper;

class NotificationParserTest {

    private final NotificationParser parser = new NotificationParser(new ObjectMapper());

    @Test
    void parsesValidNotification() throws Exception {
        var notification = parser.parse("""
                {"type":"notification.requested","message":"hello","createdAt":"2026-10-21T10:00:00Z"}
                """);

        assertThat(notification.message()).isEqualTo("hello");
    }

    @Test
    void rejectsUnsupportedNotification() {
        assertThatThrownBy(() -> parser.parse("""
                {"type":"unknown","message":"hello","createdAt":"2026-10-21T10:00:00Z"}
                """))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("unsupported notification type");
    }
}
