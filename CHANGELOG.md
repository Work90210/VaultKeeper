# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- i18n support (English + French) ready for next-intl integration
- Typed API client matching Go response envelope
- Keycloak realm export with OIDC clients, roles, brute force protection
- Makefile with dev, test, lint, build, coverage, migrate targets
