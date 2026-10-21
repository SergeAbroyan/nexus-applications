package com.nexus.worker;

import java.time.Instant;

record Notification(String type, String message, Instant createdAt) {}
