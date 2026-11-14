.PHONY: build test demo check
build:
	go build -o bin/siftpane ./cmd/siftpane
test:
	go test -race ./...
demo:
	go run ./cmd/siftpane --demo
check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race ./...
	go build -o bin/siftpane ./cmd/siftpane
