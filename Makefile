GO_IMAGE ?= golang:1.27.1-alpine
FRONTEND_IMAGE ?= nexus/frontend:dev

.PHONY: test build-frontend run-frontend

test:
	docker run --rm -v $(CURDIR):/src -w /src $(GO_IMAGE) go test ./...

build-frontend:
	docker build -f services/frontend/Dockerfile -t $(FRONTEND_IMAGE) .

run-frontend: build-frontend
	docker run --rm -p 8081:8080 $(FRONTEND_IMAGE)
