# Builder Stage
FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o loopers ./cmd/loopers

# Final Stage
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d

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
