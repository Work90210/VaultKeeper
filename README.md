# VaultKeeper

Sovereign evidence management platform for international courts, tribunals, and human rights organizations.

**"The evidence locker that no foreign government can shut off."**

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go (chi router, pgx) |
| Frontend | Next.js 14 (App Router, TypeScript) |
| Database | PostgreSQL 16 |
| File Storage | MinIO (S3-compatible) |
| Authentication | Keycloak (OIDC/SAML) |
| Search | Meilisearch |
| Reverse Proxy | Caddy (auto TLS) |
| Deployment | Docker Compose |

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20 LTS + pnpm
- Docker Compose v2

### Setup

```bash
# Clone the repository
git clone https://github.com/vaultkeeper/vaultkeeper.git
cd vaultkeeper

# Copy and configure environment
cp .env.example .env
# Edit .env with your values (change all CHANGE_ME passwords)

# Start all services
docker compose up -d

# Or for development (with hot reload and exposed ports)
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```

### Development

```bash
# Run tests
make test

# Run linter
make lint

# Run with coverage
make coverage

# Create a new migration
make migrate-new

# Build Docker image
make docker
```

### Project Structure

```
vaultkeeper/
├── cmd/server/          # Application entry point
├── internal/
│   ├── auth/            # JWT validation, permissions
│   ├── cases/           # Case management domain
│   ├── config/          # Environment configuration
│   ├── custody/         # Chain of custody (append-only)
│   ├── database/        # Migration runner
│   ├── evidence/        # Evidence management domain
│   ├── integrity/       # Hash verification, RFC 3161 TSA
│   ├── logging/         # Structured logging, redaction
│   ├── notifications/   # In-app + email notifications
│   ├── search/          # Meilisearch integration
│   └── server/          # HTTP server, routes, middleware
├── migrations/          # PostgreSQL migrations
├── web/                 # Next.js frontend
├── keycloak/            # Keycloak realm configuration
└── .github/workflows/   # CI/CD pipelines
```

## Security

This platform stores war crimes evidence and witness identities. Security is non-negotiable:

- All evidence files hashed with SHA-256 and timestamped via RFC 3161
- Immutable, hash-chained custody log enforced at the database level (RLS)
- Witness identity fields encrypted at rest
- Keycloak handles all authentication (brute force protection, MFA-ready)
- Caddy provides auto-TLS with security headers (HSTS, CSP, X-Frame-Options)
- Application connects to PostgreSQL as non-superuser role

## License

[AGPL-3.0](LICENSE) - Copyleft ensures sovereignty. Institutions can self-host freely.
