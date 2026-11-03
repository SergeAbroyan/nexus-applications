# syntax=docker/dockerfile:1.7

FROM maven:3.9.16-eclipse-temurin-25-alpine AS build

WORKDIR /src

COPY pom.xml ./
COPY src ./src
RUN --mount=type=cache,target=/root/.m2 mvn -B -ntp package -Dmaven.test.skip=true

FROM gcr.io/distroless/java25-debian13:nonroot

WORKDIR /app

COPY --from=build /src/target/nexus-worker.jar /app/worker.jar

ENV JAVA_TOOL_OPTIONS="-XX:MaxRAMPercentage=60.0 -XX:+ExitOnOutOfMemoryError --enable-native-access=ALL-UNNAMED"

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["java", "-jar", "/app/worker.jar"]
