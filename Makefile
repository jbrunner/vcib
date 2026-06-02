.PHONY: fmt fmt-check lint build test check

fmt:
	go fmt ./...

lint:
	golangci-lint run --new-from-rev=""

build:
	go build ./...

test:
	go test ./...

check: fmt lint test
