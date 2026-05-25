# Builder Stage
FROM golang:1.26-alpine@sha256:f44b851aa23dfa219d18db6eab743203245429d355cb619cf96a2ffe2a84ba7a AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o loopers ./cmd/loopers

# Final Stage
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

RUN apk --no-cache add ca-certificates && \
    addgroup -S loopers && adduser -S loopers -G loopers

WORKDIR /app

COPY --from=builder /app/loopers .
COPY --from=builder /app/pricing.yaml .

# Set permissions
RUN chown -R loopers:loopers /app

USER loopers

EXPOSE 8080

ENTRYPOINT ["/app/loopers"]
CMD ["serve"]
