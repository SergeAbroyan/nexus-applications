package com.nexus.worker;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.sqs.SqsClient;

@Configuration
class SqsConfiguration {

    @Bean(destroyMethod = "close")
    SqsClient sqsClient(WorkerProperties properties, Environment environment) {
        var builder = SqsClient.builder()
                .region(Region.of(environment.getProperty("AWS_REGION", "us-east-1")));

        if (properties.endpoint() != null) {
            builder.endpointOverride(properties.endpoint());
        }
        return builder.build();
    }
}
