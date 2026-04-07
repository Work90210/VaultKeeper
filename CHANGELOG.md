# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-04-07

Phase 1 complete: Fully functional sovereign evidence management platform.

### Added

- Go project structure with `internal/` package convention and SOLID interfaces
- Environment configuration module with comprehensive validation and secret redaction
- PostgreSQL schema: 9 core tables (cases, case_roles, evidence_items, custody_log, witnesses, disclosures, notifications, api_keys, backup_log)
- Database indexes: 11 indexes including partial indexes for performance
- Row-Level Security on custody_log enforcing append-only audit trail
- Migration runner with advisory locking for safe concurrent deployments
- Structured logging with slog (JSON in production, text in development)
- Sensitive field redaction in logs (tokens, passwords, witness identities)
- Docker Compose stack: PostgreSQL 16, MinIO, Keycloak 24, Meilisearch 1.7, Caddy
- Multi-stage Dockerfile (Go builder + Next.js builder + Alpine runtime)
- Caddy reverse proxy with security headers (HSTS, CSP, X-Frame-Options)
- GitHub Actions CI pipeline (lint, test, integration test, build, security scan)
- GitHub Actions release pipeline (Docker build + push to GHCR, changelog)
- Next.js 14 frontend with App Router, TypeScript strict mode, Tailwind CSS
- i18n support (English + French) via next-intl
- Typed API client matching Go response envelope
- Keycloak realm export with OIDC clients, roles, brute force protection
- Makefile with dev, test, lint, build, coverage, migrate targets
- Cases: CRUD operations, role assignments, legal hold, jurisdiction filtering, cursor-based pagination
- Evidence: Multipart upload with SHA-256 hashing, classification levels, versioning, soft destruction, EXIF extraction, thumbnail generation
- Chain of Custody: Immutable, hash-chained audit trail with advisory locks
- RFC 3161 Timestamps: Trusted timestamping with background retry for failed attempts
- Authentication: Keycloak OIDC with JWT validation, system and case-level RBAC
- Full-text Search: Meilisearch integration with faceted filtering, typo tolerance, highlighting, role-based result filtering
- Notifications: In-app notification system with event routing, unread counts, SMTP email dispatch
- Health Monitoring: Public and detailed endpoints with parallel service checks and caching
- Integrity Verification: Async case-level SHA-256 re-computation with CRITICAL alerts on mismatch
- Backups: Automated encrypted backups (AES-256-GCM) with consecutive failure alerting
- Case Export: ZIP archive with evidence files, metadata CSV, custody log CSV, hash manifest
- Custody Reports: PDF generation for court-submittable chain of custody
- Infrastructure: Terraform (Hetzner Cloud) + Ansible deployment playbooks
- Monitoring: Uptime Kuma integration for instance health monitoring
- Load Testing: k6 performance benchmarks for concurrent user scenarios

### Security

- Role-based search filtering (users only see evidence from assigned cases)
- Defence users restricted to disclosed evidence only
- Email notifications never contain evidence content or witness identities
- SMTP credentials never logged
- Backup encryption key from environment variable, never in code
- Parameterized SQL queries throughout
- Input validation at all system boundaries
- Security headers via Caddy (HSTS, CSP, X-Frame-Options, X-Content-Type-Options)
