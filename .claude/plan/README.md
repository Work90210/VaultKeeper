# VaultKeeper Sprint Plans

Sovereign Evidence Management Platform — 21 sprints across 4 phases.

## Phase 1: Foundation (Sprints 1-6, Weeks 1-12)

Goal: A working, deployable product an NGO can use. Release **v1.0.0**.

| Sprint | Weeks | Focus | Deliverable |
|--------|-------|-------|-------------|
| [Sprint 1](sprint-01-project-scaffolding.md) | 1-2 | Project scaffolding, Docker Compose, DB schema, CI | Go project structure, 9-table schema, all 6 services running |
| [Sprint 2](sprint-02-auth-middleware.md) | 3-4 | JWT auth, two-level role system, permissions | Keycloak integration, system + case role enforcement |
| [Sprint 3](sprint-03-cases-custody.md) | 5-6 | Cases CRUD, custody logging middleware | Case management, hash-chained custody log |
| [Sprint 4](sprint-04-evidence-upload.md) | 7-8 | Evidence upload, hashing, TSA timestamping | tus upload, SHA-256, RFC 3161, MinIO SSE storage |
| [Sprint 5](sprint-05-search-notifications-health.md) | 9-10 | Search, notifications, health endpoints | Meilisearch, SMTP email, monitoring endpoints |
| [Sprint 6](sprint-06-backups-export-infra.md) | 11-12 | Backups, export, i18n, Terraform/Ansible | Encrypted backups, ZIP export, infrastructure-as-code, **v1.0.0** |

## Phase 2: Institutional Features (Sprints 7-12, Weeks 13-24)

Goal: Features that win paid support contracts. Release **v2.0.0**.

| Sprint | Weeks | Focus | Deliverable |
|--------|-------|-------|-------------|
| [Sprint 7](sprint-07-witnesses-versioning.md) | 13-14 | Witness management, evidence versioning | Encrypted identity, version chains |
| [Sprint 8](sprint-08-redaction-disclosure.md) | 15-16 | Document redaction, disclosure workflow | Server-side redaction, prosecutor→defence disclosure |
| [Sprint 9](sprint-09-classifications-legal-hold.md) | 17-18 | Classifications, legal hold, retention | 4-tier classification, audited destruction |
| [Sprint 10](sprint-10-migration-bulk-upload.md) | 19-20 | Data migration tool, bulk upload | 5-step hash-bridging protocol, attestation certificate |
| [Sprint 11](sprint-11-timeline-audit-integrity.md) | 21-22 | Timeline, audit dashboard, integrity | Visual timeline, admin audit, scheduled verification |
| [Sprint 11.5](sprint-11-5-pilot-readiness.md) | 23-25 | **Pilot readiness hardening** | Client-side hashing, storage-layer legal hold, enforced lifecycle, CLI verifier, Berkeley Protocol report, air-gap mode, operator handbook. **Tag: v1.9.0-pilot-ready.** Shifts downstream sprints by 3 weeks. |
| [Sprint 12](sprint-12-french-cicd-certificates.md) | 26-27 | French i18n, CI/CD, chain certificates | Full French, rolling updates, **v2.0.0** |

## Phase 3: AI & Advanced Features (Sprints 13-18, Weeks 25-36)

Goal: Features that justify six-figure contracts. Release **v3.0.0**.

| Sprint | Weeks | Focus | Deliverable |
|--------|-------|-------|-------------|
| [Sprint 13](sprint-13-ai-transcription.md) | 25-26 | AI transcription (Whisper) | Self-hosted audio/video transcription |
| [Sprint 14](sprint-14-ai-translation-ocr.md) | 27-28 | AI translation, OCR | Self-hosted translation (Ollama), Tesseract OCR |
| [Sprint 15](sprint-15-entity-extraction.md) | 29-30 | Entity extraction, knowledge graph | NER, entity dedup, interactive graph visualization |
| [Sprint 16](sprint-16-semantic-search.md) | 31-32 | Semantic search | pgvector embeddings, hybrid keyword+semantic search |
| [Sprint 17](sprint-17-external-api.md) | 33-34 | External API, API keys | Scoped API keys, OpenAPI docs, webhooks |
| [Sprint 18](sprint-18-per-case-encryption-federation.md) | 35-36 | Per-case encryption, federation | Key isolation, cross-institution evidence sharing, **v3.0.0** |

## Phase 4: Enterprise & Scale (Sprints 19-21, Weeks 37-42)

Goal: ICC-scale deployments, mobile field capture. Release **v4.0.0**.

| Sprint | Weeks | Focus | Deliverable |
|--------|-------|-------|-------------|
| [Sprint 19](sprint-19-analytics-workflows.md) | 37-38 | Cross-case analytics, workflow engine | Multi-case pattern detection, configurable approval stages |
| [Sprint 20](sprint-20-mobile-capture.md) | 39-40 | Mobile evidence capture (Flutter) | Offline-first field app for iOS + Android |
| [Sprint 21](sprint-21-kubernetes-compliance.md) | 41-42 | Kubernetes Helm chart, compliance | HA deployment, ISO 27001/SOC 2 readiness, **v4.0.0** |

---

## Cross-Cutting Concerns (Every Sprint)

- **Security:** Every sprint has a security checklist. All input validated, all queries parameterized, all secrets from env vars, all custody events logged.
- **Testing:** 100% coverage target on business logic. Unit + integration + E2E. TDD workflow (red → green → refactor).
- **DRY/SOLID:** Single responsibility per package. Interfaces for all dependencies. Immutable data patterns.
- **i18n:** All UI strings via `next-intl` from Sprint 1.
- **Custody logging:** Every mutation logged with hash chain from Sprint 3 onward.

## Tech Stack

| Layer | Choice |
|-------|--------|
| Backend | Go 1.22+ |
| Frontend | Next.js 14 (React, TypeScript, Tailwind, shadcn/ui) |
| Database | PostgreSQL 16 + pgvector |
| Storage | MinIO (S3-compatible, SSE) |
| Auth | Keycloak (OIDC/SAML) |
| Search | Meilisearch |
| AI | Whisper + Ollama + Tesseract |
| Proxy | Caddy (auto TLS) |
| Mobile | Flutter |
| Deploy | Docker Compose → Helm/Kubernetes |
| Infra | Terraform (Hetzner) + Ansible |
| CI/CD | GitHub Actions |
