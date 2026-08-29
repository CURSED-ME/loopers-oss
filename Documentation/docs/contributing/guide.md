---
id: guide
title: Contributing Guide
sidebar_label: Contributing Guide
description: How to contribute to Loopers, setup, guidelines, and PR process.
---

# Contributing to Loopers

We welcome contributions from the community. Loopers is an open source project and we appreciate your help in making it better.

## Code of Conduct

By participating in this project, you agree to follow our Code of Conduct. We expect all participants to be respectful, inclusive, and professional.

## Development Setup

### Prerequisites

To build and test Loopers, you will need:
* Go (version 1.20 or higher)
* Redis (version 7.0 or higher)
* Docker (for running integration tests)
* golangci-lint (for code style checks)

### Building the project

1. Clone the repository and navigate inside:
   ```bash
   git clone https://github.com/try-loopers/loopers-oss.git
   cd loopers-oss
   ```

2. Download Go modules:
   ```bash
   go mod download
   ```

3. Build the program:
   ```bash
   go build -o loopers ./cmd/loopers
   ```

4. Run all unit tests:
   ```bash
   go test -v -race ./...
   ```
   Note: Some tests require Redis running on localhost:6379.

### Running locally

1. Start Redis in the background:
   ```bash
   docker run -d -p 6379:6379 redis:7-alpine
   ```

2. Start the proxy server in debug mode (bypassing TLS):
   ```bash
   SERVER_INSECURE_DEV=true ./loopers serve -v
   ```

## Pull Request Guidelines

### 1. Correctness and Security

Loopers values correctness and security above anything else. Any changes you propose must preserve our budget enforcement guarantees:
* No race conditions allowing double spending.
* The proxy must fail closed if Redis goes down.
* Real provider API keys must never be saved on disk or written to logs.

### 2. Add Tests

Every bug fix or new feature must include unit tests. You can run tests and check code coverage using:
```bash
go test -v -race ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 3. Code Quality

Ensure your code is formatted and linted before submitting:
```bash
gofmt -w .
golangci-lint run
```

### 4. Commits

Please format your commit messages using Conventional Commits:
```
feat(budget): add per session step counter enforcement
fix(proxy): handle SSE disconnect during mid stream cutoff
docs(readme): update quickstart with Docker command
test(budget): add concurrent flood test for Lua script
```

## Repository Structure

Here is how the project files are organized:

```
loopers-oss/
 cmd/loopers/        # The command line application
 internal/
    alerting/       # Webhook notifications for budget limits
    budget/         # Redis Lua scripts and budget engine
    cache/          # Simple in-memory cache helpers
    keyring/        # Key creation, storage, and validation
    logging/        # Structured JSON logs
    loop/           # Deterministic agent loop, velocity, and stall detection
    otel/           # OpenTelemetry tracing configuration and span helpers
    pricing/        # Model price lookup and cost estimation
    provider/       # Adapters for each AI provider (OpenAI, Gemini, etc.)
    proxy/          # Reverse proxy core and streaming interceptor
    server/         # Gin HTTP server and middleware chain
 pkg/                # Public packages and shared API types
 sdk/                # Python and TypeScript client libraries
 helm/               # Kubernetes deployment chart
 grafana/            # Observability dashboard configuration
```
