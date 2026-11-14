.DEFAULT_GOAL := build

GO ?= go
LINT := $(GO) tool -modfile=tools/go.mod golangci-lint

.PHONY: build demo test vet fmt fmt-check lint check mod-check

build:
	$(GO) build -o bin/siftpane ./cmd/siftpane

demo:
	$(GO) run ./cmd/siftpane --demo

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(LINT) fmt ./...

fmt-check:
	$(LINT) fmt --diff ./...

lint:
	$(LINT) config verify
	$(LINT) run ./...

mod-check:
	$(GO) mod tidy -diff
	$(GO) -C tools mod tidy -diff
	$(GO) mod verify
	$(GO) -C tools mod verify

check: fmt-check lint vet test build mod-check
