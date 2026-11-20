BIN := bin/route
GO  ?= go

.PHONY: all build test cover vet hooks fmt clean

all: build

build:
	$(GO) build -trimpath -o $(BIN) ./cmd/route

test:
	$(GO) test ./...

cover:
	$(GO) test -covermode=atomic -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

hooks: build
	$(BIN) install-hooks -H hooks

clean:
	rm -rf bin coverage.out
