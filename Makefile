.PHONY: lint test build docker

lint:
	golangci-lint run ./...

test:
	go test -v -race -count=1 ./...

build:
	go build -v -o bin/loopers ./cmd/loopers

docker:
	docker build -t loopers:latest .
