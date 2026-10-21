GO_IMAGE ?= golang:1.27.1-alpine
MAVEN_IMAGE ?= maven:3.9.12-eclipse-temurin-21-alpine
API_IMAGE ?= nexus/api:dev
WORKER_IMAGE ?= nexus/worker:dev

.PHONY: test test-go test-java build build-api build-worker run-api local-up local-down local-logs local-status

test: test-go test-java

test-go:
	docker run --rm -v $(CURDIR):/src -w /src $(GO_IMAGE) go test ./...

test-java:
	docker run --rm -v $(CURDIR):/src -v nexus-maven-cache:/root/.m2 -w /src $(MAVEN_IMAGE) mvn -B -f services/worker/pom.xml verify

build: build-api build-worker

build-api:
	docker build -f services/api/Dockerfile -t $(API_IMAGE) .

build-worker:
	docker build -f services/worker/Dockerfile -t $(WORKER_IMAGE) .

run-api: build-api
	docker run --rm -p 8081:8080 $(API_IMAGE)

local-up:
	docker compose up --build --detach --wait

local-down:
	docker compose down

local-logs:
	docker compose logs --follow

local-status:
	docker compose ps
