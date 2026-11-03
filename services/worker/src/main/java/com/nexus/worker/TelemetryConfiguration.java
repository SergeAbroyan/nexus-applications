package com.nexus.worker;

import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.sdk.autoconfigure.AutoConfiguredOpenTelemetrySdk;
import java.util.Map;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.util.StringUtils;

@Configuration(proxyBeanMethods = false)
class TelemetryConfiguration {

    static final String SERVICE_NAME = "nexus-worker";

    @Bean
    OpenTelemetry openTelemetry(Environment environment) {
        if (!StringUtils.hasText(environment.getProperty("OTEL_EXPORTER_OTLP_ENDPOINT"))) {
            return OpenTelemetry.noop();
        }
        return AutoConfiguredOpenTelemetrySdk.builder()
                .addPropertiesSupplier(() -> Map.of(
                        "otel.service.name", SERVICE_NAME,
                        "otel.metrics.exporter", "none",
                        "otel.logs.exporter", "none"))
                .build()
                .getOpenTelemetrySdk();
    }

    @Bean
    Profiler profiler(Environment environment) {
        return new Profiler(SERVICE_NAME, environment.getProperty("PYROSCOPE_SERVER_ADDRESS", ""));
    }
}
