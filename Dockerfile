# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o opencost-cloudcost-exporter .

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/opencost-cloudcost-exporter /opencost-cloudcost-exporter

# Expose metrics port
EXPOSE 9100

# Run as non-root user (distroless nonroot user)
USER nonroot:nonroot

ENTRYPOINT ["/opencost-cloudcost-exporter"]
