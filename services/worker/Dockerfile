# syntax=docker/dockerfile:1.7

FROM maven:3.9.12-eclipse-temurin-21-alpine AS build

WORKDIR /src

COPY services/worker/pom.xml ./pom.xml
COPY services/worker/src ./src
RUN --mount=type=cache,target=/root/.m2 mvn -B verify

FROM gcr.io/distroless/java21-debian13:nonroot

WORKDIR /app

COPY --from=build --chown=65532:65532 /src/target/nexus-worker.jar /app/worker.jar

ENV JAVA_TOOL_OPTIONS="-XX:MaxRAMPercentage=60.0 -XX:+ExitOnOutOfMemoryError"

USER 65532:65532
EXPOSE 8080

CMD ["/app/worker.jar"]
