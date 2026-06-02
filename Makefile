.PHONY: fmt fmt-check lint build test helm-lint check

fmt:
	go fmt ./...

lint:
	golangci-lint run --new-from-rev=""

build:
	go build ./...

test:
	go test ./...

helm-lint:
	helm lint chart/

check: fmt lint test helm-lint
