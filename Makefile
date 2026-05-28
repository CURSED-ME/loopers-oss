.PHONY: lint test build install docker

lint:
	golangci-lint run ./...

test:
	go test -v -race -count=1 ./...

build:
	go build -ldflags="-s -w" -o loopers ./cmd/loopers

install:
	go install -ldflags="-s -w" ./cmd/loopers
docker:
	docker build -t loopers:latest .
