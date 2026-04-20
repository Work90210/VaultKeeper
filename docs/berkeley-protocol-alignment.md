# Berkeley Protocol Alignment

## Introduction

The [Berkeley Protocol on Digital Open Source Investigations](https://www.ohchr.org/en/publications/policy-and-methodological-publications/berkeley-protocol-digital-open-source) (OHCHR/UC Berkeley, 2020) establishes international standards for the collection, preservation, and use of online open source information in investigations of violations of international criminal, human rights, and humanitarian law.

VaultKeeper is designed to support and enforce these standards through its evidence management pipeline. This document maps each Berkeley Protocol requirement to the corresponding VaultKeeper feature, identifies gaps, and provides a roadmap for full alignment.

**Disclaimer**: This is a self-assessment. VaultKeeper has not been independently certified against the Berkeley Protocol. Organizations should conduct their own compliance evaluation for their specific use cases.

---

## Alignment Table

| # | Berkeley Protocol Requirement | VaultKeeper Feature | Status |
|---|------|------|--------|
| **Principles** | | | |
| P1 | Accuracy/impartiality in evidence handling | Role-based access (6 roles), classification system, immutable audit trail | **Covered** |
| P2 | Reproducible, documented processes | Chain of custody log with hash chaining, upload attempt tracking | **Covered** |
| P3 | Privacy protection | Witness encryption, GDPR erasure workflow, redaction with purpose codes | **Covered** |
| P4 | Do-no-harm / protect investigators | Investigator safety profiles (opsec levels, pseudonyms, VPN/Tor requirements), collector identity encryption | **Covered** |
| **Phase 1: Online Inquiry** | | | |
| 1.1 | Document search strategies & parameters | `investigation_inquiry_logs` table: search_strategy, search_keywords, search_operators | **Covered** |
| 1.2 | Record search tools/engines used | `investigation_inquiry_logs`: search_tool, search_tool_version, search_url | **Covered** |
| 1.3 | Document discovery timeline | `investigation_inquiry_logs`: search_started_at, search_ended_at, results_count/relevant/collected | **Covered** |
| **Phase 2: Preliminary Assessment** | | | |
| 2.1 | Evaluate relevance before collection | `evidence_assessments`: relevance_score (1-5), relevance_rationale | **Covered** |
| 2.2 | Evaluate reliability/credibility | `evidence_assessments`: reliability_score (1-5), reliability_rationale, source_credibility | **Covered** |
| 2.3 | Filter misleading/unreliable sources | `evidence_assessments`: misleading_indicators, recommendation (collect/monitor/deprioritize/discard) | **Covered** |
| **Phase 3: Collection** | | | |
| 3.1 | Forensically sound capture | SHA-256 hash at upload, client hash verification (constant-time), RFC 3161 TSA timestamps | **Covered** |
| 3.2 | Preserve original metadata | EXIF extraction, original filename preservation, capture metadata table | **Covered** |
| 3.3 | Record source URL | `source_url` field in `evidence_capture_metadata` | **Covered** |
| 3.4 | Record source platform | `platform` enum field with 11 platforms | **Covered** |
| 3.5 | Record capture method | `capture_method` enum field (screenshot, web_archive, forensic_tool, etc.) | **Covered** |
| 3.6 | Record capture timestamp | `capture_timestamp` (distinct from upload time) | **Covered** |
| 3.7 | Record original publication timestamp | `publication_timestamp` (distinct from capture time) | **Covered** |
| 3.8 | Hash at capture | SHA-256 computed server-side + client X-Content-SHA256 verification | **Covered** |
| 3.9 | Record collector identity | `collector_user_id` + encrypted `collector_display_name` | **Covered** |
| 3.10 | Record content creator account/profile | Structured fields: handle, display name, URL, account ID | **Covered** |
| 3.11 | Record content description | `content_description` field | **Covered** |
| 3.12 | Record geolocation | Lat/lon, place name, geo source (EXIF/manual/platform/derived) | **Covered** |
| 3.13 | Record content language | `content_language` (BCP 47 validated) | **Covered** |
| 3.14 | Record content availability status | `availability_status` enum (accessible, deleted, geo_blocked, etc.) | **Covered** |
| 3.15 | Record browser/tool used for capture | `capture_tool_name`, `browser_name`, `browser_version` fields | **Covered** |
| 3.16 | Record network context | JSONB field: VPN, Tor, proxy, region. Role-gated to investigator/prosecutor | **Covered** |
| **Phase 4: Preservation** | | | |
| 4.1 | Secure long-term archiving | MinIO object storage, TSA timestamps, retention controls | **Covered** |
| 4.2 | Prevent deletion/degradation | Retention dates, legal hold, destruction authority requirements | **Covered** |
| 4.3 | Maintain content + contextual metadata | Evidence versioning, EXIF preservation, capture metadata table | **Covered** |
| 4.4 | Evidentiary vs working copies | Redaction creates derivative with parent link, original preserved | **Covered** |
| 4.5 | SHA-256 hash preservation | `sha256_hash` column, indexed | **Covered** |
| 4.6 | Immutable audit trail | Chain of custody with hash chaining (previous_hash -> current) | **Covered** |
| **Phase 5: Verification** | | | |
| 5.1 | Source authentication | `evidence_verification_records`: structured verification with type, methodology, tools, finding, confidence, reviewer sign-off | **Covered** |
| 5.2 | Content verification | `evidence_verification_records`: finding_rationale, limitations, caveats; auto-upgrades capture_metadata.verification_status | **Covered** |
| 5.3 | Multi-source corroboration | `corroboration_claims` + `corroboration_evidence`: claims with typed roles (primary/supporting/contextual/contradicting), minimum 2 items | **Covered** |
| **Phase 6: Investigative Analysis** | | | |
| 6.1 | Documented analytical reasoning | `investigative_analysis_notes`: typed analysis (10 types), methodology, content | **Covered** |
| 6.2 | Iterative refinement through earlier phases | `investigative_analysis_notes`: related_evidence/inquiry/assessment/verification IDs, supersession chain | **Covered** |
| **Security** | | | |
| S1 | Encryption and access controls | Classification system, encrypted witness PII, role-based access | **Covered** |
| S2 | Investigator anonymity | `investigator_safety_profiles`: pseudonyms, use_pseudonym flag, role-gated display, encrypted collector identity | **Covered** |
| S3 | Separate professional/personal activities | — | **Out of scope** |
| S4 | Isolate concurrent investigations | Case-based data isolation, per-case roles | **Covered** |
| S5 | Regular security assessments | — | **Operational** |
| **Reporting** | | | |
| R1 | Document investigation purpose/methods | `investigation_reports`: structured sections (purpose, methodology, findings, analysis, conclusions) | **Covered** |
| R2 | Present findings with supporting evidence | Search + export, investigation reports with referenced_evidence_ids, capture_metadata.csv | **Covered** |
| R3 | Maintain transparency about limitations | `investigation_reports`: limitations[], caveats[], assumptions[] fields | **Covered** |

---

## Alignment Status

**Full alignment achieved.** All six Berkeley Protocol investigative phases, principles, security requirements, reporting standards, and Annex templates are implemented. No remaining gaps.

### Implementation Summary

| Area | Tables | Status |
|------|--------|--------|
| Phase 1: Online Inquiry | `investigation_inquiry_logs` | Covered |
| Phase 2: Preliminary Assessment | `evidence_assessments` | Covered |
| Phase 3: Collection | `evidence_capture_metadata` | Covered |
| Phase 4: Preservation | `evidence_items` + MinIO + TSA | Covered |
| Phase 5: Verification | `evidence_verification_records`, `corroboration_claims` | Covered |
| Phase 6: Investigative Analysis | `investigative_analysis_notes` | Covered |
| Annexes 1-3 | `investigation_templates`, `investigation_template_instances` | Covered |
| Reporting (R1, R3) | `investigation_reports` | Covered |
| Investigator Safety (P4, S2) | `investigator_safety_profiles` | Covered |

---

## Methodology

This alignment assessment was conducted by mapping each section of the Berkeley Protocol (Chapters 4-9 and Annexes) to VaultKeeper's implemented features. Each requirement was categorized as:

- **Covered**: Feature fully implements the requirement
- **Partial**: Feature partially addresses the requirement with known limitations
- **Gap**: No current implementation; tracked in roadmap
- **Out of scope**: Operational or organizational requirement, not a software feature
- **Operational**: Process-level requirement that depends on organizational implementation

The assessment is based on VaultKeeper's codebase as of the current release. It has not been independently audited or certified by OHCHR, UC Berkeley, or any third party.
