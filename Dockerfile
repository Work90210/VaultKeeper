# Stage 1: Build Go binary
FROM golang:1.22-alpine AS go-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /build/server ./cmd/server

# Stage 2: Build Next.js
FROM node:20-alpine AS web-builder

WORKDIR /build

COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY web/ .
RUN pnpm build

# Stage 3: Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget tzdata \
    && addgroup -S vaultkeeper \
    && adduser -S -G vaultkeeper vaultkeeper

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
