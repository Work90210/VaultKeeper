# Stage 1: Build Go binary
FROM golang:1.25-bookworm AS go-builder

RUN apt-get update && apt-get install -yqq --no-install-recommends \
      git ca-certificates gcc libc6-dev pkg-config \
      libmupdf-dev libffi-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /build/server ./cmd/server

# Stage 2: Build Next.js
FROM node:20-alpine AS web-builder

WORKDIR /build

# Placeholder env vars so NextAuth's requireEnv() passes during
# Next.js's build-time page-data collection. Real values are injected
# at runtime from /app/.env on the server.
ENV KEYCLOAK_URL=http://keycloak:8080 \
    KEYCLOAK_REALM=vaultkeeper \
    KEYCLOAK_CLIENT_ID=build-placeholder \
    KEYCLOAK_CLIENT_SECRET=build-placeholder \
    NEXTAUTH_SECRET=build-placeholder-at-least-32-characters-long-xx \
    NEXTAUTH_URL=https://vaultkeeper.eu

COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY web/ .
RUN pnpm build

# Stage 3: Runtime
FROM debian:12-slim

RUN apt-get update && apt-get install -yqq --no-install-recommends \
      ca-certificates wget tzdata libmupdf-dev libffi8 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system vaultkeeper \
    && useradd --system --gid vaultkeeper vaultkeeper

WORKDIR /app

COPY --from=go-builder /build/server ./server
COPY --from=web-builder /build/.next/standalone ./web/
COPY --from=web-builder /build/.next/static ./web/.next/static
COPY --from=web-builder /build/public ./web/public
COPY migrations/ ./migrations/

RUN chown -R vaultkeeper:vaultkeeper /app

USER vaultkeeper

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
