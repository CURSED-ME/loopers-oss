# Builder Stage
FROM golang:1.26.4-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o loopers ./cmd/loopers

# Final Stage
FROM alpine:3.21

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
