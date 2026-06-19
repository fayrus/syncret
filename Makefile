VERSION := $(shell cat VERSION)
BINARY  := syncret
IMAGE   := fayrus/$(BINARY)

.PHONY: build test cover lint tidy docker-build docs-deps docs-serve docs-build

# ── Go ────────────────────────────────────────────────────────────────────────

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test -race -cover ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

# ── Docs ──────────────────────────────────────────────────────────────────────

docs-deps:
	pip-compile --generate-hashes --output-file docs/requirements.txt docs/requirements.in

docs-serve:
	mkdocs serve -f docs/mkdocs.yml

docs-build:
	mkdocs build -f docs/mkdocs.yml
