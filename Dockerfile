# Build stage - Frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build stage - Backend
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
# seed_apps.json.gz is pre-generated and committed to the repo.
# Run "make seed-data" to regenerate it before building a release.
RUN CGO_ENABLED=1 go build -o serverdash .

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl bash

# Install Caddy
ARG CADDY_VERSION=2.11.2
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') \
    && curl -sSL "https://github.com/caddyserver/caddy/releases/download/v${CADDY_VERSION}/caddy_${CADDY_VERSION}_linux_${ARCH}.tar.gz" -o /tmp/caddy.tar.gz \
    && curl -sSL "https://github.com/caddyserver/caddy/releases/download/v${CADDY_VERSION}/caddy_${CADDY_VERSION}_checksums.txt" -o /tmp/checksums.txt \
    && EXPECTED=$(grep " caddy_${CADDY_VERSION}_linux_${ARCH}.tar.gz$" /tmp/checksums.txt | awk '{print $1}') \
    && ACTUAL=$(sha256sum /tmp/caddy.tar.gz | awk '{print $1}') \
    && test -n "$EXPECTED" \
    && test "$ACTUAL" = "$EXPECTED" \
    && tar -xzf /tmp/caddy.tar.gz -C /tmp caddy \
    && install -m 0755 /tmp/caddy /usr/local/bin/caddy \
    && rm -f /tmp/caddy /tmp/caddy.tar.gz /tmp/checksums.txt

WORKDIR /app
COPY --from=backend /app/serverdash .
COPY --from=backend /app/web/dist ./web/dist

# Create data directory
RUN mkdir -p /app/data/logs /app/data/backups

# Environment defaults
ENV SERVERDASH_PORT=8080
ENV SERVERDASH_DATA_DIR=/app/data
ENV SERVERDASH_CADDY_BIN=/usr/local/bin/caddy

EXPOSE 8080 80 443

VOLUME ["/app/data"]

CMD ["./serverdash"]
