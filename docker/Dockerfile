# Build stage for frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Build stage for backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./cmd/dbnest/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o dbnest ./cmd/dbnest

# Final stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata docker-cli

WORKDIR /app

# Create non-root user
RUN adduser -D -g '' dbnest && \
    addgroup dbnest docker 2>/dev/null || true

# Copy binary
COPY --from=backend-builder /app/dbnest /app/dbnest

# Create data directory
RUN mkdir -p /data && chown -R dbnest:dbnest /data

# Run as root to access Docker socket
# For better security, use a Docker socket proxy in production
USER root

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["/app/dbnest"]
CMD ["--port", "8080", "--data", "/data", "--socket", "/var/run/docker.sock"]
