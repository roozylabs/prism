# Build stage. The Go version matches go.mod exactly, so the build never
# downloads a newer toolchain at image build time.
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application: no cgo, no build paths in the binary, no symbol table.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/reencrypt ./cmd/reencrypt

# Runtime stage
FROM alpine:3.22

# ca-certificates for upstream TLS, curl for the compose healthcheck.
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -H -u 10001 -g prism prism

WORKDIR /app

# Copy binary from builder. Nothing is written at run time: migrations are
# read-only and the binaries only talk to PostgreSQL and Redis.
COPY --from=builder /app/server .
COPY --from=builder /app/reencrypt .
COPY --from=builder /app/migrations ./migrations

USER prism

EXPOSE 8080

CMD ["./server"]
