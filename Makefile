VERSION ?= dev

.PHONY: build run test test-race lint clean mcp

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/crates ./cmd/crates

run:
	go run ./cmd/crates

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/

mcp:
	go run ./cmd/crates --mcp
