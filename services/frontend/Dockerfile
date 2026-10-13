# syntax=docker/dockerfile:1.7

FROM golang:1.27.1-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY services/frontend ./services/frontend
COPY services/observability ./services/observability

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/frontend \
    ./services/frontend

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/frontend /frontend

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/frontend"]
