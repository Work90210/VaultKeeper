.PHONY: dev build test test-int lint migrate-up migrate-down migrate-new coverage docker clean

# Development
dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

# Build
build:
	go build -o ./server ./cmd/server
	cd web && pnpm build

# Test
test:
	go test ./... -race -count=1 -coverprofile=coverage.out

test-int:
	go test ./... -tags=integration -race -count=1

# Lint
lint:
	golangci-lint run ./...
	cd web && pnpm lint

# Migrations
migrate-up:
	go run ./cmd/server --migrate-only

migrate-down:
	@echo "Use golang-migrate CLI: migrate -path migrations -database $$DATABASE_URL down 1"

migrate-new:
	@read -p "Migration name: " name; \
	num=$$(ls -1 migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	num=$$(printf "%03d" $$((num + 1))); \
	touch "migrations/$${num}_$${name}.up.sql" "migrations/$${num}_$${name}.down.sql"; \
	echo "Created migrations/$${num}_$${name}.up.sql and .down.sql"

# Coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Docker
docker:
	docker build -t vaultkeeper:latest .

# Clean
clean:
	rm -f server coverage.out coverage.html
	cd web && rm -rf .next node_modules
