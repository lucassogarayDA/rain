BINARY  := rain
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: all build test lint lint-fix fmt vet tidy tools clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

# Equivalente a "ruff" para Go: se instala en $(go env GOPATH)/bin.
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0

clean:
	rm -f $(BINARY)
	rm -rf dist
