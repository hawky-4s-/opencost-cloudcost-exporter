# Variables
GO       ?= go
BINARY   ?= opencost-cloudcost-exporter
PKG      ?= ./...
GIT_TAG  := $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD)
DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  ?= -s -w -X main.version=$(GIT_TAG) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Default target
.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o dist/$(BINARY) .

.PHONY: run
run:
	$(GO) run -ldflags "$(LDFLAGS)" .

.PHONY: test
test:
	$(GO) test -race -cover -coverprofile=coverage.out $(PKG)

.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not installed"; fi

.PHONY: install
install:
	$(GO) install -ldflags "$(LDFLAGS)" .

.PHONY: clean
clean:
	rm -rf dist coverage.out

.PHONY: docker-build
docker-build:
	docker build -t $(BINARY):latest .

.PHONY: helm-lint
helm-lint:
	helm lint charts/opencost-cloudcost-exporter

# Requires goreleaser installed or use Docker
.PHONY: release
release:
	docker run --rm --privileged \
	-v $${PWD}:/go/src/github.com/hawky-4s-/opencost-cloudcost-exporter \
	-v $${HOME}/.colima/docker.sock:/var/run/docker.sock \
	-w /go/src/github.com/hawky-4s-/opencost-cloudcost-exporter \
	-e GITHUB_TOKEN \
	-e DOCKER_USERNAME \
	-e DOCKER_PASSWORD \
	-e DOCKER_REGISTRY \
	goreleaser/goreleaser release --clean

.PHONY: release-dryrun
release-dryrun:
	docker run --rm --privileged \
	-v $${PWD}:/go/src/github.com/hawky-4s-/opencost-cloudcost-exporter \
	-v $${HOME}/.colima/docker.sock:/var/run/docker.sock \
	-w /go/src/github.com/hawky-4s-/opencost-cloudcost-exporter \
	-e GITHUB_TOKEN \
	-e DOCKER_USERNAME \
	-e DOCKER_PASSWORD \
	-e DOCKER_REGISTRY \
	goreleaser/goreleaser release --snapshot --skip publish --clean

# Help
.PHONY: help
help:
	@grep -E '^\.PHONY: [a-zA-Z_-]+$$' $(MAKEFILE_LIST) | sed 's/\.PHONY: /make /'
