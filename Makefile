SHELL := /bin/bash
CGO_ENABLED ?= 0
CORE_TEST_PACKAGES := $(shell go list ./... | grep -v '/cmd/' | grep -v '/internal/app$$')

.PHONY: fmt lint test run migrate compose-up compose-down

fmt:
	@if command -v gofumpt >/dev/null 2>&1; then gofumpt -w .; else gofmt -w $$(find . -name '*.go'); fi

lint:
	CGO_ENABLED=$(CGO_ENABLED) go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; fi

test:
	CGO_ENABLED=$(CGO_ENABLED) go test ./...
	CGO_ENABLED=$(CGO_ENABLED) go test $(CORE_TEST_PACKAGES) -coverprofile=coverage.out -covermode=atomic
	@coverage=$$(CGO_ENABLED=$(CGO_ENABLED) go tool cover -func=coverage.out | awk '/total:/ {print $$3}' | tr -d '%'); \
	python3 -c "import sys; sys.exit(0 if float('$$coverage') >= 80.0 else 1)" || (echo \"coverage gate failed: $$coverage% < 80%\" && exit 1)

run:
	CGO_ENABLED=$(CGO_ENABLED) go run ./cmd/orders-service

migrate:
	CGO_ENABLED=$(CGO_ENABLED) go run ./cmd/orders-service -mode migrate

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
