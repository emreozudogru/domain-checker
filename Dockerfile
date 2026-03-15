# Stage 1: Build
FROM golang:1.22-alpine AS builder

# Ensure fundamental system requirements specifically useful for Go on scratch
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app
COPY . .

# Initialize Go modules inside the builder just in case they are missing locally
RUN go mod init github.com/emreozudogru/domain-checker || true \
    && go mod tidy || true

# Build specifically optimized for Linux ARM64 (Raspberry Pi 5)
# Using CGO_ENABLED=0 guarantees statically linked binary strictly compatible with scratch container.
# -ldflags="-w -s" removes debugging info for absolute smallest binary size.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o domain-monitor -ldflags="-w -s" .

# Stage 2: Create execution context
FROM scratch

# Need CA certificates for standard network whois queries and proper parsing of timezones
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app
# We only copy the compiled binary, dropping all golang SDK sizes ensuring an ultra-small image footprint!
COPY --from=builder /app/domain-monitor .

# Expose server port
EXPOSE 8080

# Starts the binary directly
ENTRYPOINT ["/app/domain-monitor"]
