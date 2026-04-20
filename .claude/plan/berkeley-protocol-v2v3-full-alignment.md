# Berkeley Protocol Full Alignment — v2+v3 Combined Sprint

## Overview

Single sprint to close ALL remaining Berkeley Protocol gaps, achieving full alignment across all six investigative phases plus security and reporting requirements.

**Deliverables:**
1. **Phase 1**: Investigation inquiry logs (requirements 1.1-1.3)
2. **Phase 2**: Preliminary assessment workflow (requirements 2.1-2.3)
3. **Phase 5 upgrade**: Structured verification records (5.1-5.2 enhanced)
4. **Phase 5**: Multi-source corroboration (requirement 5.3)
5. **Phase 6**: Investigative analysis notes (requirements 6.1-6.2)
6. **Collection enhancement**: Bulk upload Berkeley fields in _metadata.csv
7. **Safety**: Investigator safety tooling (P4, S2)
8. **Templates**: Investigation plan (Annex 1), threat assessment (Annex 2), digital landscape (Annex 3)
9. **Reporting**: Structured investigation reports with limitations/caveats (R1, R3)

## Task Type
- [x] Frontend (forms, detail displays, template editors, report builder)
- [x] Backend (7 new tables, Go structs, API endpoints, validation)
- [x] Fullstack (parallel)

---

## Part 1: Database Migrations

### Migration 022 — Investigation Inquiry Logs (Phase 1)

File: `migrations/022_investigation_inquiry_logs.up.sql`

```sql
BEGIN;

CREATE TABLE investigation_inquiry_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    evidence_id         UUID REFERENCES evidence_items(id) ON DELETE SET NULL,

    -- Search strategy documentation (Berkeley 1.1)
    search_strategy     TEXT NOT NULL,
    search_keywords     TEXT[],
    search_operators    TEXT,

    -- Tools and engines used (Berkeley 1.2)
    search_tool         TEXT NOT NULL,
    search_tool_version TEXT,
    search_url          TEXT,

    -- Discovery timeline (Berkeley 1.3)
    search_started_at   TIMESTAMPTZ NOT NULL,
    search_ended_at     TIMESTAMPTZ,
    results_count       INTEGER,
    results_relevant    INTEGER,
    results_collected   INTEGER,

    -- Context
    objective           TEXT NOT NULL,
    notes               TEXT,
    performed_by        UUID NOT NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inquiry_logs_case ON investigation_inquiry_logs(case_id);
CREATE INDEX idx_inquiry_logs_performer ON investigation_inquiry_logs(performed_by);
CREATE INDEX idx_inquiry_logs_started ON investigation_inquiry_logs(search_started_at);

COMMIT;
```

Down: `DROP TABLE IF EXISTS investigation_inquiry_logs;`

### Migration 023 — Preliminary Assessments (Phase 2)

File: `migrations/023_preliminary_assessments.up.sql`

```sql
BEGIN;

CREATE TABLE evidence_assessments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id         UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    -- Relevance evaluation (Berkeley 2.1)
    relevance_score     INTEGER NOT NULL CHECK (relevance_score BETWEEN 1 AND 5),
    relevance_rationale TEXT NOT NULL,

    -- Reliability/credibility evaluation (Berkeley 2.2)
    reliability_score   INTEGER NOT NULL CHECK (reliability_score BETWEEN 1 AND 5),
    reliability_rationale TEXT NOT NULL,

    -- Source filtering (Berkeley 2.3)
    source_credibility  TEXT NOT NULL CHECK (source_credibility IN (
                            'established', 'credible', 'uncertain',
                            'unreliable', 'unassessed'
                        )),
    misleading_indicators TEXT[],
    recommendation      TEXT NOT NULL CHECK (recommendation IN (
                            'collect', 'monitor', 'deprioritize', 'discard'
                        )),

    -- Assessment metadata
    methodology         TEXT,
    assessed_by         UUID NOT NULL,
    reviewed_by         UUID,
    reviewed_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assessments_evidence ON evidence_assessments(evidence_id);
CREATE INDEX idx_assessments_case ON evidence_assessments(case_id);
CREATE INDEX idx_assessments_recommendation ON evidence_assessments(recommendation);

COMMIT;
```

Down: `DROP TABLE IF EXISTS evidence_assessments;`

### Migration 024 — Structured Verification Records (Phase 5 upgrade)

File: `migrations/024_verification_records.up.sql`

```sql
BEGIN;

CREATE TABLE evidence_verification_records (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id         UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    -- Verification methodology (Berkeley 5.1-5.2)
    verification_type   TEXT NOT NULL CHECK (verification_type IN (
                            'source_authentication', 'content_verification',
                            'reverse_image_search', 'geolocation_verification',
                            'chronolocation', 'metadata_analysis',
                            'witness_corroboration', 'expert_analysis',
                            'open_source_cross_reference', 'other'
                        )),
    methodology         TEXT NOT NULL,
    tools_used          TEXT[],
    sources_consulted   TEXT[],

    -- Findings
    finding             TEXT NOT NULL CHECK (finding IN (
                            'authentic', 'likely_authentic', 'inconclusive',
                            'likely_manipulated', 'manipulated', 'unable_to_verify'
                        )),
    finding_rationale   TEXT NOT NULL,
    confidence_level    TEXT NOT NULL CHECK (confidence_level IN (
                            'high', 'medium', 'low'
                        )),

    -- Limitations and caveats
    limitations         TEXT,
    caveats             TEXT[],

    -- Sign-off
    verified_by         UUID NOT NULL,
    reviewer            UUID,
    reviewer_approved   BOOLEAN,
    reviewer_notes      TEXT,
    reviewed_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_verification_evidence ON evidence_verification_records(evidence_id);
CREATE INDEX idx_verification_case ON evidence_verification_records(case_id);
CREATE INDEX idx_verification_type ON evidence_verification_records(verification_type);
CREATE INDEX idx_verification_finding ON evidence_verification_records(finding);

COMMIT;
```

Down: `DROP TABLE IF EXISTS evidence_verification_records;`

### Migration 025 — Corroboration Links (Phase 5)

File: `migrations/025_corroboration_links.up.sql`

```sql
BEGIN;

-- A "claim" that multiple evidence items corroborate
CREATE TABLE corroboration_claims (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    claim_summary       TEXT NOT NULL,
    claim_type          TEXT NOT NULL CHECK (claim_type IN (
                            'event_occurrence', 'identity_confirmation',
                            'location_confirmation', 'timeline_confirmation',
                            'pattern_of_conduct', 'contextual_corroboration',
                            'other'
                        )),
    strength            TEXT NOT NULL CHECK (strength IN (
                            'strong', 'moderate', 'weak', 'contested'
                        )),
    analysis_notes      TEXT,

    created_by          UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Junction table: which evidence items support which claims
CREATE TABLE corroboration_evidence (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    claim_id            UUID NOT NULL REFERENCES corroboration_claims(id) ON DELETE CASCADE,
    evidence_id         UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    role_in_claim       TEXT NOT NULL CHECK (role_in_claim IN (
                            'primary', 'supporting', 'contextual', 'contradicting'
                        )),
    contribution_notes  TEXT,
    added_by            UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (claim_id, evidence_id)
);

CREATE INDEX idx_corroboration_claims_case ON corroboration_claims(case_id);
CREATE INDEX idx_corroboration_evidence_claim ON corroboration_evidence(claim_id);
CREATE INDEX idx_corroboration_evidence_item ON corroboration_evidence(evidence_id);

COMMIT;
```

Down: `DROP TABLE IF EXISTS corroboration_evidence; DROP TABLE IF EXISTS corroboration_claims;`

### Migration 026 — Investigative Analysis Notes (Phase 6)

File: `migrations/026_analysis_notes.up.sql`

```sql
BEGIN;

CREATE TABLE investigative_analysis_notes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    -- Analysis documentation (Berkeley 6.1)
    title               TEXT NOT NULL,
    analysis_type       TEXT NOT NULL CHECK (analysis_type IN (
                            'factual_finding', 'pattern_analysis',
                            'timeline_reconstruction', 'geographic_analysis',
                            'network_analysis', 'legal_assessment',
                            'credibility_assessment', 'gap_identification',
                            'hypothesis_testing', 'other'
                        )),
    content             TEXT NOT NULL,
    methodology         TEXT,

    -- Iterative refinement (Berkeley 6.2)
    -- Links back to earlier phases: which inquiry logs, assessments,
    -- verification records, or evidence items informed this analysis
    related_evidence_ids    UUID[],
    related_inquiry_ids     UUID[],
    related_assessment_ids  UUID[],
    related_verification_ids UUID[],

    -- Status tracking
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                            'draft', 'in_review', 'approved', 'superseded'
                        )),
    superseded_by       UUID REFERENCES investigative_analysis_notes(id),

    -- Authorship
    author_id           UUID NOT NULL,
    reviewer_id         UUID,
    reviewed_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_analysis_notes_case ON investigative_analysis_notes(case_id);
CREATE INDEX idx_analysis_notes_type ON investigative_analysis_notes(analysis_type);
CREATE INDEX idx_analysis_notes_status ON investigative_analysis_notes(status);

COMMIT;
```

Down: `DROP TABLE IF EXISTS investigative_analysis_notes;`

### Migration 027 — Investigation Templates (Annexes 1-3)

File: `migrations/027_investigation_templates.up.sql`

```sql
BEGIN;

CREATE TABLE investigation_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Template metadata
    template_type       TEXT NOT NULL CHECK (template_type IN (
                            'investigation_plan',
                            'threat_assessment',
                            'digital_landscape'
                        )),
    name                TEXT NOT NULL,
    description         TEXT,
    version             INTEGER NOT NULL DEFAULT 1,
    is_default          BOOLEAN NOT NULL DEFAULT false,

    -- Template structure (JSONB schema)
    -- Each template type has a defined section structure.
    -- investigation_plan: {objective, scope, methodology, resources, timeline, risks, ethical_considerations}
    -- threat_assessment: {threat_actors, digital_risks, mitigation_measures, monitoring_plan}
    -- digital_landscape: {platforms, data_types, access_methods, legal_frameworks, technical_constraints}
    schema_definition   JSONB NOT NULL,

    -- Ownership
    created_by          UUID,
    is_system_template  BOOLEAN NOT NULL DEFAULT false,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Instances: a filled-in template for a specific case
CREATE TABLE investigation_template_instances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id         UUID NOT NULL REFERENCES investigation_templates(id) ON DELETE RESTRICT,
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    -- Filled content
    content             JSONB NOT NULL,
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                            'draft', 'active', 'completed', 'archived'
                        )),

    -- Sign-off
    prepared_by         UUID NOT NULL,
    approved_by         UUID,
    approved_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_type ON investigation_templates(template_type);
CREATE INDEX idx_template_instances_case ON investigation_template_instances(case_id);
CREATE INDEX idx_template_instances_template ON investigation_template_instances(template_id);

COMMIT;
```

Down: `DROP TABLE IF EXISTS investigation_template_instances; DROP TABLE IF EXISTS investigation_templates;`

### Migration 028 — Investigation Reports (Reporting)

File: `migrations/028_investigation_reports.up.sql`

```sql
BEGIN;

CREATE TABLE investigation_reports (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    -- Report metadata (Berkeley R1)
    title               TEXT NOT NULL,
    report_type         TEXT NOT NULL CHECK (report_type IN (
                            'interim', 'final', 'supplementary', 'expert_opinion'
                        )),

    -- Structured content sections
    -- Stored as JSONB to allow flexible section ordering and content
    -- Expected shape: [{section_type, title, content, order}]
    -- section_type: 'purpose', 'methodology', 'findings', 'evidence_summary',
    --               'analysis', 'conclusions', 'recommendations', 'limitations',
    --               'appendix', 'custom'
    sections            JSONB NOT NULL DEFAULT '[]',

    -- Transparency about limitations (Berkeley R3)
    limitations         TEXT[],
    caveats             TEXT[],
    assumptions         TEXT[],

    -- Evidence references
    referenced_evidence_ids UUID[],
    referenced_analysis_ids UUID[],

    -- Workflow
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                            'draft', 'in_review', 'approved', 'published', 'withdrawn'
                        )),

    -- Authorship
    author_id           UUID NOT NULL,
    reviewer_id         UUID,
    reviewed_at         TIMESTAMPTZ,
    approved_by         UUID,
    approved_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reports_case ON investigation_reports(case_id);
CREATE INDEX idx_reports_type ON investigation_reports(report_type);
CREATE INDEX idx_reports_status ON investigation_reports(status);

COMMIT;
```

Down: `DROP TABLE IF EXISTS investigation_reports;`

### Migration 029 — Investigator Safety Profiles

File: `migrations/029_investigator_safety.up.sql`

```sql
BEGIN;

CREATE TABLE investigator_safety_profiles (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    user_id             UUID NOT NULL,

    -- Anonymization settings (Berkeley S2)
    pseudonym           TEXT,
    use_pseudonym       BOOLEAN NOT NULL DEFAULT false,

    -- Operational security (Berkeley P4)
    opsec_level         TEXT NOT NULL DEFAULT 'standard' CHECK (opsec_level IN (
                            'standard', 'elevated', 'high_risk'
                        )),
    required_vpn        BOOLEAN NOT NULL DEFAULT false,
    required_tor        BOOLEAN NOT NULL DEFAULT false,
    approved_devices    TEXT[],
    prohibited_platforms TEXT[],

    -- Threat context
    threat_level        TEXT NOT NULL DEFAULT 'low' CHECK (threat_level IN (
                            'low', 'medium', 'high', 'critical'
                        )),
    threat_notes        TEXT,

    -- Safety guidance
    safety_briefing_completed BOOLEAN NOT NULL DEFAULT false,
    safety_briefing_date TIMESTAMPTZ,
    safety_officer_id   UUID,

    -- SECURITY: entire row encrypted at rest via BYTEA if opsec_level = 'high_risk'
    -- For standard/elevated, individual fields are sufficient

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (case_id, user_id)
);

CREATE INDEX idx_safety_profiles_case ON investigator_safety_profiles(case_id);
CREATE INDEX idx_safety_profiles_user ON investigator_safety_profiles(user_id);

COMMIT;
```

Down: `DROP TABLE IF EXISTS investigator_safety_profiles;`

---

## Part 2: Go Domain Models

### Step 1: Inquiry Logs

File: `internal/investigation/inquiry_log.go`

```go
// InquiryLog represents a documented search session (Berkeley Phase 1).
//
// Struct fields: ID, CaseID, EvidenceID (*uuid.UUID), SearchStrategy, SearchKeywords ([]string),
//   SearchOperators, SearchTool, SearchToolVersion, SearchURL, SearchStartedAt,
//   SearchEndedAt (*time.Time), ResultsCount (*int), ResultsRelevant (*int),
//   ResultsCollected (*int), Objective, Notes, PerformedBy, CreatedAt, UpdatedAt
//
// InquiryLogInput: validated input struct with string timestamps, URL scheme validation
//   on SearchURL, required fields: SearchStrategy, SearchTool, SearchStartedAt, Objective
//
// Validate(): enforces required fields, http/https on SearchURL if present,
//   ResultsRelevant <= ResultsCount, SearchEndedAt >= SearchStartedAt
```

### Step 2: Preliminary Assessments

File: `internal/investigation/assessment.go`

```go
// SourceCredibility enum: established, credible, uncertain, unreliable, unassessed
// Recommendation enum: collect, monitor, deprioritize, discard
//
// EvidenceAssessment struct: ID, EvidenceID, CaseID, RelevanceScore (1-5),
//   RelevanceRationale, ReliabilityScore (1-5), ReliabilityRationale,
//   SourceCredibility, MisleadingIndicators ([]string), Recommendation,
//   Methodology, AssessedBy, ReviewedBy (*uuid.UUID), ReviewedAt, CreatedAt, UpdatedAt
//
// AssessmentInput: validated input with scores clamped 1-5, required rationales
//
// Validate(): score range, non-empty rationales, valid enum values
```

### Step 3: Verification Records

File: `internal/investigation/verification_record.go`

```go
// VerificationType enum: source_authentication, content_verification,
//   reverse_image_search, geolocation_verification, chronolocation,
//   metadata_analysis, witness_corroboration, expert_analysis,
//   open_source_cross_reference, other
//
// Finding enum: authentic, likely_authentic, inconclusive,
//   likely_manipulated, manipulated, unable_to_verify
//
// ConfidenceLevel enum: high, medium, low
//
// VerificationRecord struct: all fields from migration 024
//
// VerificationRecordInput: validated input, required: verification_type,
//   methodology, finding, finding_rationale, confidence_level
//
// When a verification record is created with finding=authentic + confidence=high,
// the service should auto-update evidence_capture_metadata.verification_status
// to 'verified'. Requires prosecutor/judge role.
```

### Step 4: Corroboration Claims

File: `internal/investigation/corroboration.go`

```go
// ClaimType enum: event_occurrence, identity_confirmation, location_confirmation,
//   timeline_confirmation, pattern_of_conduct, contextual_corroboration, other
//
// Strength enum: strong, moderate, weak, contested
//
// RoleInClaim enum: primary, supporting, contextual, contradicting
//
// CorroborationClaim struct + CorroborationEvidence struct
//
// CorroborationClaimInput: claim_summary required, at least 2 evidence_ids required
//   (corroboration requires multiple sources by definition)
```

### Step 5: Analysis Notes

File: `internal/investigation/analysis_note.go`

```go
// AnalysisType enum: factual_finding, pattern_analysis, timeline_reconstruction,
//   geographic_analysis, network_analysis, legal_assessment,
//   credibility_assessment, gap_identification, hypothesis_testing, other
//
// AnalysisStatus enum: draft, in_review, approved, superseded
//
// AnalysisNote struct: all fields from migration 026
//
// AnalysisNoteInput: title, analysis_type, content required
//   Related IDs are optional arrays linking back to earlier phases
//
// Supersession: when a note is superseded, the old note's status is set to
//   'superseded' and superseded_by is set to the new note's ID
```

### Step 6: Investigation Templates

File: `internal/investigation/template.go`

```go
// TemplateType enum: investigation_plan, threat_assessment, digital_landscape
//
// InvestigationTemplate struct: ID, TemplateType, Name, Description, Version,
//   IsDefault, SchemaDefinition (map[string]any), CreatedBy, IsSystemTemplate, ...
//
// TemplateInstance struct: ID, TemplateID, CaseID, Content (map[string]any),
//   Status, PreparedBy, ApprovedBy, ApprovedAt, ...
//
// Default schemas (seeded in migration or application startup):
//
// Investigation Plan (Annex 1):
//   sections: objective, scope, methodology, resources_required,
//   timeline, ethical_considerations, risk_assessment, data_management,
//   communication_protocols, review_schedule
//
// Threat Assessment (Annex 2):
//   sections: threat_actors, digital_footprint_risks, device_security,
//   account_security, network_security, physical_security,
//   mitigation_measures, incident_response, monitoring_plan
//
// Digital Landscape (Annex 3):
//   sections: platforms_in_scope, data_types_available, access_methods,
//   legal_frameworks, technical_constraints, preservation_strategies,
//   tool_requirements, language_considerations
```

### Step 7: Investigation Reports

File: `internal/investigation/report.go`

```go
// ReportType enum: interim, final, supplementary, expert_opinion
// ReportStatus enum: draft, in_review, approved, published, withdrawn
//
// ReportSection struct:
//   SectionType: purpose, methodology, findings, evidence_summary,
//                analysis, conclusions, recommendations, limitations,
//                appendix, custom
//   Title, Content, Order (int)
//
// InvestigationReport struct: all fields from migration 028
//   Sections stored as []ReportSection, marshalled to/from JSONB
//
// ReportInput: title, report_type required, at least one section required
//   Limitations/caveats/assumptions are optional arrays (Berkeley R3)
```

### Step 8: Investigator Safety

File: `internal/investigation/safety_profile.go`

```go
// OpsecLevel enum: standard, elevated, high_risk
// ThreatLevel enum: low, medium, high, critical
//
// SafetyProfile struct: all fields from migration 029
//
// SafetyProfileInput: opsec_level required
//   When opsec_level = elevated or high_risk:
//     - Pseudonym recommended (warn if empty)
//     - VPN/Tor requirements enforced at capture metadata level
//       (service checks safety profile before allowing capture metadata submission
//        and warns if network_context doesn't match requirements)
//
// SECURITY: safety profiles are read-only for the investigator themselves
//   and writable only by prosecutor/judge/safety_officer
//
// RedactForRole: all fields visible only to investigator (own profile),
//   prosecutor, judge. Defence/observer see nothing.
```

---

## Part 3: Repository Layer

File: `internal/investigation/repository.go`

```go
// InvestigationRepository interface consolidating all sub-domain repositories:
//
// Inquiry Logs:
//   CreateInquiryLog(ctx, log) (InquiryLog, error)
//   ListInquiryLogs(ctx, caseID, page) ([]InquiryLog, int, error)
//   GetInquiryLog(ctx, id) (InquiryLog, error)
//   UpdateInquiryLog(ctx, id, input) (InquiryLog, error)
//   DeleteInquiryLog(ctx, id) error
//
// Assessments:
//   CreateAssessment(ctx, assessment) (EvidenceAssessment, error)
//   GetAssessmentsByEvidence(ctx, evidenceID) ([]EvidenceAssessment, error)
//   GetAssessment(ctx, id) (EvidenceAssessment, error)
//   UpdateAssessment(ctx, id, input) (EvidenceAssessment, error)
//
// Verification Records:
//   CreateVerificationRecord(ctx, record) (VerificationRecord, error)
//   ListVerificationRecords(ctx, evidenceID) ([]VerificationRecord, error)
//   GetVerificationRecord(ctx, id) (VerificationRecord, error)
//
// Corroboration:
//   CreateCorroborationClaim(ctx, claim) (CorroborationClaim, error)
//   ListCorroborationClaims(ctx, caseID) ([]CorroborationClaim, error)
//   GetCorroborationClaim(ctx, id) (CorroborationClaim, error)
//   AddEvidenceToClaim(ctx, claimID, evidenceID, role, notes, addedBy) error
//   RemoveEvidenceFromClaim(ctx, claimID, evidenceID) error
//   GetClaimsByEvidence(ctx, evidenceID) ([]CorroborationClaim, error)
//
// Analysis Notes:
//   CreateAnalysisNote(ctx, note) (AnalysisNote, error)
//   ListAnalysisNotes(ctx, caseID, filter) ([]AnalysisNote, int, error)
//   GetAnalysisNote(ctx, id) (AnalysisNote, error)
//   UpdateAnalysisNote(ctx, id, input) (AnalysisNote, error)
//   SupersedeAnalysisNote(ctx, oldID, newNote) (AnalysisNote, error)
//
// Templates:
//   ListTemplates(ctx, templateType) ([]InvestigationTemplate, error)
//   GetTemplate(ctx, id) (InvestigationTemplate, error)
//   CreateTemplateInstance(ctx, instance) (TemplateInstance, error)
//   ListTemplateInstances(ctx, caseID) ([]TemplateInstance, error)
//   GetTemplateInstance(ctx, id) (TemplateInstance, error)
//   UpdateTemplateInstance(ctx, id, content, status) (TemplateInstance, error)
//
// Reports:
//   CreateReport(ctx, report) (InvestigationReport, error)
//   ListReports(ctx, caseID) ([]InvestigationReport, error)
//   GetReport(ctx, id) (InvestigationReport, error)
//   UpdateReport(ctx, id, input) (InvestigationReport, error)
//
// Safety Profiles:
//   UpsertSafetyProfile(ctx, caseID, userID, input) (SafetyProfile, error)
//   GetSafetyProfile(ctx, caseID, userID) (SafetyProfile, error)
//   ListSafetyProfiles(ctx, caseID) ([]SafetyProfile, error)
//
// PG implementation: internal/investigation/pg_repository.go
// INSERT/UPDATE patterns follow existing capture_metadata_repository.go style
```

---

## Part 4: Service Layer

File: `internal/investigation/service.go`

```go
// InvestigationService struct:
//   repo      InvestigationRepository
//   custody   CustodyRecorder
//   indexer   search.SearchIndexer
//   cases     CaseLookup
//   logger    *slog.Logger
//   captureMetadata CaptureMetadataUpdater  // to auto-update verification_status
//
// Constructor: NewInvestigationService(repo, custody, indexer, cases, logger)
//   With functional options: WithCaptureMetadataUpdater(...)
//
// Methods follow same pattern as evidence service:
//   1. Validate input
//   2. Check authorization (role from handler)
//   3. Perform operation
//   4. Record custody event
//   5. Return result
//
// Custody events (new actions to register):
//   inquiry_log_created, inquiry_log_updated, inquiry_log_deleted
//   assessment_created, assessment_updated, assessment_reviewed
//   verification_record_created, verification_record_reviewed
//   corroboration_claim_created, corroboration_evidence_added,
//     corroboration_evidence_removed
//   analysis_note_created, analysis_note_updated,
//     analysis_note_superseded, analysis_note_approved
//   template_instance_created, template_instance_updated,
//     template_instance_approved
//   report_created, report_updated, report_approved, report_published
//   safety_profile_updated
//
// Auto-verification upgrade:
//   When CreateVerificationRecord is called with finding='authentic' AND
//   confidence='high', the service calls captureMetadata.UpsertByEvidenceID
//   to set verification_status='verified'. Requires prosecutor/judge role.
//   Records both verification_record_created AND
//   capture_metadata_verification_changed custody events.
//
// Safety profile enforcement:
//   When UpsertCaptureMetadata is called, service checks if the actor has a
//   safety profile with required_vpn=true. If so and network_context is missing
//   or vpn_used=false, returns a warning (not error) — advisory only.
```

---

## Part 5: HTTP Handlers

File: `internal/investigation/handler.go`

```go
// InvestigationHandler struct:
//   service *InvestigationService
//   audit   auth.AuditLogger
//
// RegisterRoutes(r chi.Router):
//
//   // Phase 1: Inquiry Logs
//   r.Route("/api/cases/{caseID}/inquiry-logs", func(r chi.Router) {
//       r.Post("/", h.CreateInquiryLog)
//       r.Get("/", h.ListInquiryLogs)
//   })
//   r.Route("/api/inquiry-logs/{id}", func(r chi.Router) {
//       r.Get("/", h.GetInquiryLog)
//       r.Put("/", h.UpdateInquiryLog)
//       r.Delete("/", h.DeleteInquiryLog)
//   })
//
//   // Phase 2: Assessments
//   r.Route("/api/evidence/{evidenceID}/assessments", func(r chi.Router) {
//       r.Post("/", h.CreateAssessment)
//       r.Get("/", h.ListAssessments)
//   })
//   r.Route("/api/assessments/{id}", func(r chi.Router) {
//       r.Get("/", h.GetAssessment)
//       r.Put("/", h.UpdateAssessment)
//   })
//
//   // Phase 5: Verification Records
//   r.Route("/api/evidence/{evidenceID}/verifications", func(r chi.Router) {
//       r.Post("/", h.CreateVerificationRecord)
//       r.Get("/", h.ListVerificationRecords)
//   })
//   r.Get("/api/verifications/{id}", h.GetVerificationRecord)
//
//   // Phase 5: Corroboration
//   r.Route("/api/cases/{caseID}/corroborations", func(r chi.Router) {
//       r.Post("/", h.CreateCorroborationClaim)
//       r.Get("/", h.ListCorroborationClaims)
//   })
//   r.Route("/api/corroborations/{id}", func(r chi.Router) {
//       r.Get("/", h.GetCorroborationClaim)
//       r.Post("/evidence", h.AddEvidenceToClaim)
//       r.Delete("/evidence/{evidenceID}", h.RemoveEvidenceFromClaim)
//   })
//
//   // Phase 6: Analysis Notes
//   r.Route("/api/cases/{caseID}/analysis-notes", func(r chi.Router) {
//       r.Post("/", h.CreateAnalysisNote)
//       r.Get("/", h.ListAnalysisNotes)
//   })
//   r.Route("/api/analysis-notes/{id}", func(r chi.Router) {
//       r.Get("/", h.GetAnalysisNote)
//       r.Put("/", h.UpdateAnalysisNote)
//       r.Post("/supersede", h.SupersedeAnalysisNote)
//       r.Post("/approve", h.ApproveAnalysisNote)
//   })
//
//   // Templates (Annexes 1-3)
//   r.Get("/api/templates", h.ListTemplates)
//   r.Get("/api/templates/{id}", h.GetTemplate)
//   r.Route("/api/cases/{caseID}/template-instances", func(r chi.Router) {
//       r.Post("/", h.CreateTemplateInstance)
//       r.Get("/", h.ListTemplateInstances)
//   })
//   r.Route("/api/template-instances/{id}", func(r chi.Router) {
//       r.Get("/", h.GetTemplateInstance)
//       r.Put("/", h.UpdateTemplateInstance)
//       r.Post("/approve", h.ApproveTemplateInstance)
//   })
//
//   // Reports (R1, R3)
//   r.Route("/api/cases/{caseID}/reports", func(r chi.Router) {
//       r.Post("/", h.CreateReport)
//       r.Get("/", h.ListReports)
//   })
//   r.Route("/api/reports/{id}", func(r chi.Router) {
//       r.Get("/", h.GetReport)
//       r.Put("/", h.UpdateReport)
//       r.Post("/approve", h.ApproveReport)
//       r.Post("/publish", h.PublishReport)
//       r.Get("/export", h.ExportReport)  // PDF/DOCX generation
//   })
//
//   // Safety Profiles (P4, S2)
//   r.Route("/api/cases/{caseID}/safety-profiles", func(r chi.Router) {
//       r.Get("/", h.ListSafetyProfiles)        // prosecutor/judge only
//       r.Get("/mine", h.GetMySafetyProfile)     // own profile
//       r.Put("/{userID}", h.UpsertSafetyProfile) // prosecutor/judge only
//   })
//
// Access matrix:
//   Inquiry logs: investigator/prosecutor/judge can create/edit; observer read-only
//   Assessments: investigator/prosecutor can create; judge/observer read-only
//   Verification records: prosecutor/judge can create; investigator can view
//   Corroboration: investigator/prosecutor can create; all case roles read
//   Analysis notes: investigator/prosecutor can create; prosecutor/judge approve
//   Templates: all case roles read; investigator/prosecutor create instances
//   Reports: prosecutor/judge create/approve/publish; others read published only
//   Safety profiles: prosecutor/judge/safety_officer write; investigator reads own
```

---

## Part 6: Frontend — Types

File: update `web/src/types/index.ts`

```typescript
// Add all new interfaces:
//
// InquiryLog: id, case_id, evidence_id?, search_strategy, search_keywords[],
//   search_tool, search_url?, search_started_at, search_ended_at?,
//   results_count?, results_relevant?, results_collected?,
//   objective, notes?, performed_by, created_at, updated_at
//
// EvidenceAssessment: id, evidence_id, case_id, relevance_score (1-5),
//   relevance_rationale, reliability_score (1-5), reliability_rationale,
//   source_credibility, misleading_indicators[], recommendation,
//   methodology?, assessed_by, reviewed_by?, reviewed_at?, created_at, updated_at
//
// VerificationRecord: id, evidence_id, case_id, verification_type,
//   methodology, tools_used[], sources_consulted[], finding,
//   finding_rationale, confidence_level, limitations?, caveats[],
//   verified_by, reviewer?, reviewer_approved?, reviewer_notes?,
//   reviewed_at?, created_at, updated_at
//
// CorroborationClaim: id, case_id, claim_summary, claim_type,
//   strength, analysis_notes?, evidence (CorroborationEvidence[]),
//   created_by, created_at, updated_at
//
// CorroborationEvidence: id, claim_id, evidence_id, role_in_claim,
//   contribution_notes?, added_by, created_at
//
// AnalysisNote: id, case_id, title, analysis_type, content,
//   methodology?, related_evidence_ids[], related_inquiry_ids[],
//   related_assessment_ids[], related_verification_ids[],
//   status, superseded_by?, author_id, reviewer_id?,
//   reviewed_at?, created_at, updated_at
//
// InvestigationTemplate: id, template_type, name, description?,
//   version, is_default, schema_definition, is_system_template, created_at
//
// TemplateInstance: id, template_id, case_id, content,
//   status, prepared_by, approved_by?, approved_at?, created_at, updated_at
//
// InvestigationReport: id, case_id, title, report_type,
//   sections (ReportSection[]), limitations[], caveats[], assumptions[],
//   referenced_evidence_ids[], referenced_analysis_ids[],
//   status, author_id, reviewer_id?, reviewed_at?,
//   approved_by?, approved_at?, created_at, updated_at
//
// ReportSection: section_type, title, content, order
//
// SafetyProfile: id, case_id, user_id, pseudonym?, use_pseudonym,
//   opsec_level, required_vpn, required_tor, approved_devices[],
//   prohibited_platforms[], threat_level, threat_notes?,
//   safety_briefing_completed, safety_briefing_date?, created_at, updated_at
//
// Enum constants: SOURCE_CREDIBILITIES, RECOMMENDATIONS, VERIFICATION_TYPES,
//   FINDINGS, CONFIDENCE_LEVELS, CLAIM_TYPES, CLAIM_STRENGTHS,
//   ROLES_IN_CLAIM, ANALYSIS_TYPES, ANALYSIS_STATUSES, TEMPLATE_TYPES,
//   REPORT_TYPES, REPORT_STATUSES, REPORT_SECTION_TYPES,
//   OPSEC_LEVELS, THREAT_LEVELS
```

---

## Part 7: Frontend — Pages & Components

### Phase 1: Inquiry Log Manager

File: `web/src/components/investigation/inquiry-log-form.tsx`

```
┌─ Log Search Session ─────────────────────────────────────┐
│ Objective *          [What are you looking for?]          │
│ Search Strategy *    [textarea: describe approach]        │
│ Keywords             [tag input: keyword1, keyword2]      │
│ Search Tool *        [text: e.g. Google, Shodan, Wayback] │
│ Search URL           [url: search engine URL used]        │
│                                                           │
│ Started At *         [datetime picker]                    │
│ Ended At             [datetime picker]                    │
│ Results Found        [number]  Relevant [number]          │
│ Collected            [number]                             │
│ Notes                [textarea]                           │
│                                                           │
│ [Save Log]  [Cancel]                                      │
└───────────────────────────────────────────────────────────┘
```

File: `web/src/components/investigation/inquiry-log-list.tsx`
- Table view of all inquiry logs for a case
- Columns: Date, Objective, Tool, Keywords, Results, Performed By
- Click to expand/edit

### Phase 2: Assessment Form

File: `web/src/components/investigation/assessment-form.tsx`

```
┌─ Preliminary Assessment ─────────────────────────────────┐
│ Evidence: [ICC-01/04-01/07-00046] osint-capture-twitter   │
│                                                           │
│ RELEVANCE (1-5)          ● ● ● ● ○  [4/5]               │
│ Rationale *              [textarea]                       │
│                                                           │
│ RELIABILITY (1-5)        ● ● ● ○ ○  [3/5]               │
│ Rationale *              [textarea]                       │
│                                                           │
│ Source Credibility        [▼ Credible]                    │
│ Misleading Indicators     [tag input]                     │
│ Recommendation           [▼ Collect]                      │
│ Methodology              [textarea]                       │
│                                                           │
│ [Save Assessment]  [Cancel]                               │
└───────────────────────────────────────────────────────────┘
```

### Phase 5: Verification Record Form

File: `web/src/components/investigation/verification-form.tsx`

```
┌─ Add Verification Record ────────────────────────────────┐
│ Evidence: [ICC-01/04-01/07-00046]                         │
│                                                           │
│ Verification Type *  [▼ Source Authentication]            │
│ Methodology *        [textarea: describe process]         │
│ Tools Used           [tag input: InVID, Google Lens]      │
│ Sources Consulted    [tag input]                          │
│                                                           │
│ Finding *            [▼ Authentic]                        │
│ Rationale *          [textarea]                           │
│ Confidence *         [▼ High]                            │
│                                                           │
│ Limitations          [textarea]                           │
│ Caveats              [tag input]                         │
│                                                           │
│ [Submit for Review]  [Save Draft]                         │
└───────────────────────────────────────────────────────────┘
```

When finding=authentic + confidence=high → auto-updates capture_metadata verification_status to 'verified'

### Phase 5: Corroboration Builder

File: `web/src/components/investigation/corroboration-builder.tsx`

```
┌─ Corroboration Claim ────────────────────────────────────┐
│ Claim *              [What do these items collectively    │
│                       prove?]                             │
│ Type *               [▼ Event Occurrence]                │
│ Strength             [▼ Moderate]                        │
│                                                           │
│ LINKED EVIDENCE                                           │
│ ┌────────────────────────────────────────────────────┐   │
│ │ + Add evidence item                                │   │
│ │                                                    │   │
│ │ ICC-...-00046  osint-capture-twitter  [Primary ▼]  │   │
│ │ ICC-...-00041  alpha.txt              [Supporting▼] │   │
│ │ ICC-...-00043  bravo.txt              [Contextual▼] │   │
│ └────────────────────────────────────────────────────┘   │
│                                                           │
│ Analysis Notes       [textarea]                           │
│ [Save Claim]  [Cancel]                                    │
└───────────────────────────────────────────────────────────┘
```

### Phase 6: Analysis Note Editor

File: `web/src/components/investigation/analysis-note-editor.tsx`

```
┌─ Analysis Note ──────────────────────────────────────────┐
│ Title *              [Factual finding: timeline of ...]   │
│ Type *               [▼ Timeline Reconstruction]         │
│ Status               Draft                                │
│                                                           │
│ Content *            [rich textarea / markdown]           │
│                                                           │
│ ▸ Related Items (optional)                                │
│   Evidence:     [multi-select from case evidence]         │
│   Inquiry Logs: [multi-select from case logs]             │
│   Assessments:  [multi-select]                            │
│   Verifications:[multi-select]                            │
│                                                           │
│ Methodology          [textarea]                           │
│                                                           │
│ [Save Draft]  [Submit for Review]                         │
└───────────────────────────────────────────────────────────┘
```

### Templates (Annexes 1-3)

File: `web/src/components/investigation/template-editor.tsx`

Dynamic form renderer that reads schema_definition from the template and renders appropriate form fields per section. Each section has a title, description hint, and content area (textarea/rich text).

Three default templates seeded:
1. **Investigation Plan** (Annex 1): Objective, Scope, Methodology, Resources, Timeline, Ethical Considerations, Risk Assessment, Data Management
2. **Threat Assessment** (Annex 2): Threat Actors, Digital Risks, Device Security, Account Security, Network Security, Mitigation Measures, Incident Response
3. **Digital Landscape** (Annex 3): Platforms, Data Types, Access Methods, Legal Frameworks, Technical Constraints, Preservation Strategies

### Reports (R1, R3)

File: `web/src/components/investigation/report-builder.tsx`

```
┌─ Investigation Report ───────────────────────────────────┐
│ Title *              [Final Report — Case ICC-01/04-01/07]│
│ Type *               [▼ Final]                           │
│                                                           │
│ SECTIONS  [+ Add Section]                                 │
│ ┌────────────────────────────────────────────────────┐   │
│ │ 1. Purpose          [textarea]          [▲▼ ✕]    │   │
│ │ 2. Methodology      [textarea]          [▲▼ ✕]    │   │
│ │ 3. Findings         [textarea]          [▲▼ ✕]    │   │
│ │ 4. Evidence Summary  [textarea + refs]  [▲▼ ✕]    │   │
│ │ 5. Conclusions      [textarea]          [▲▼ ✕]    │   │
│ │ 6. Limitations *    [textarea]          [▲▼ ✕]    │   │
│ └────────────────────────────────────────────────────┘   │
│                                                           │
│ TRANSPARENCY (Berkeley R3)                                │
│ Limitations          [tag input]                         │
│ Caveats              [tag input]                         │
│ Assumptions          [tag input]                         │
│                                                           │
│ Referenced Evidence  [multi-select]                       │
│ Referenced Analysis  [multi-select]                       │
│                                                           │
│ [Save Draft]  [Submit for Review]  [Export PDF]           │
└───────────────────────────────────────────────────────────┘
```

### Safety Profiles

File: `web/src/components/investigation/safety-profile-form.tsx`

```
┌─ Investigator Safety Profile ────────────────────────────┐
│ Investigator: [username]                                  │
│                                                           │
│ OPSEC Level *        [▼ Elevated]                        │
│ Threat Level         [▼ Medium]                          │
│                                                           │
│ ANONYMIZATION                                             │
│ Use Pseudonym        [✓]                                 │
│ Pseudonym            [Field Researcher Alpha]             │
│                                                           │
│ REQUIREMENTS                                              │
│ Require VPN          [✓]                                 │
│ Require Tor          [ ]                                 │
│ Approved Devices     [tag input]                         │
│ Prohibited Platforms [tag input: Facebook, TikTok]        │
│                                                           │
│ THREAT CONTEXT                                            │
│ Threat Notes         [textarea]                           │
│                                                           │
│ SAFETY BRIEFING                                           │
│ Completed            [✓]  Date: [2026-04-15]             │
│ Safety Officer       [select user]                        │
│                                                           │
│ [Save Profile]                                            │
└───────────────────────────────────────────────────────────┘
```

---

## Part 8: Case Page Integration

The case detail page gets a new tab layout:

```
Overview | Evidence 44 | Investigation | Witnesses | Disclosures | Settings
                          ↑ NEW
```

The **Investigation** tab contains sub-navigation:

```
Investigation
├── Inquiry Logs          (Phase 1)
├── Assessments           (Phase 2)
├── Verifications         (Phase 5)
├── Corroborations        (Phase 5)
├── Analysis Notes        (Phase 6)
├── Templates             (Annexes 1-3)
├── Reports               (R1, R3)
└── Safety Profiles       (P4, S2) — prosecutor/judge only
```

---

## Part 9: Evidence Detail Integration

The evidence detail page gains new sections below "Capture Provenance":

```
[Existing: Metadata Card]
[Existing: Capture Provenance Card]

┌─ Assessment ────────────────────────────┐
│ Relevance: ●●●●○ (4/5) — Credible      │
│ Recommendation: Collect                  │
│ [View Full Assessment]                   │
└──────────────────────────────────────────┘

┌─ Verification Records ──────────────────┐
│ ✓ Source Authentication — Authentic      │
│   Confidence: High · By: prosecutor.test │
│ ✓ Geolocation Verification — Authentic   │
│   Confidence: Medium                     │
│ [Add Verification] [View All]            │
└──────────────────────────────────────────┘

┌─ Corroboration ─────────────────────────┐
│ Part of 2 corroboration claims           │
│ • "Event occurred on Apr 10" (Strong)    │
│ • "Subject identified at location" (Mod) │
│ [View Claims]                            │
└──────────────────────────────────────────┘
```

---

## Part 10: Bulk Upload Enhancement

File: update `internal/evidence/bulk.go`

Extend `BulkMetadata` struct and `_metadata.csv` parsing to accept optional Berkeley capture fields:

```csv
Title,Description,Tags,Classification,Source,SourceDate,source_url,platform,capture_method,capture_timestamp,publication_timestamp,creator_account_handle,content_language,availability_status
"Screenshot of post","Key evidence","osint,social","restricted","","","https://x.com/user/123","x","screenshot","2026-04-10T14:30:00Z","2026-04-09T08:00:00Z","@username","en","accessible"
```

When Berkeley fields are present, create `evidence_capture_metadata` rows alongside evidence items in the same transaction.

---

## Part 11: Search Integration

File: update `internal/search/models.go`

Add to EvidenceSearchDoc:
```go
// Assessment fields (non-sensitive)
RelevanceScore     *int    `json:"relevance_score,omitempty"`
Recommendation     *string `json:"recommendation,omitempty"`

// Verification summary
VerificationCount  *int    `json:"verification_count,omitempty"`
LatestFinding      *string `json:"latest_finding,omitempty"`

// Corroboration
CorroborationCount *int    `json:"corroboration_count,omitempty"`
```

Update `ConfigureEvidenceIndex` filterable attributes to include `recommendation`, `latest_finding`.

---

## Part 12: Export Integration

File: update `internal/cases/export.go`

Add to case export ZIP:
- `inquiry_logs.csv` — all inquiry logs for the case
- `assessments.csv` — all assessments (scores, recommendations)
- `verification_records.csv` — all verification records (type, finding, confidence)
- `corroboration_claims.csv` — claims with linked evidence IDs
- `analysis_notes.csv` — analysis notes (title, type, status, content)
- `investigation_report.json` — published reports with full sections

Update `README.txt` to document all new files.

---

## Part 13: i18n Key Inventory

### English (`web/src/messages/en.json`)

**Investigation tab** (8 keys): `investigation.title`, `.inquiryLogs`, `.assessments`, `.verifications`, `.corroborations`, `.analysisNotes`, `.templates`, `.reports`, `.safetyProfiles`

**Inquiry logs** (15 keys): field labels, form headers, list column headers

**Assessments** (20 keys): field labels, score labels (1-5), source credibility enum (5), recommendation enum (4)

**Verification records** (25 keys): verification type enum (10), finding enum (6), confidence enum (3), field labels

**Corroboration** (15 keys): claim type enum (7), strength enum (4), role enum (4), field labels

**Analysis notes** (15 keys): analysis type enum (10), status enum (4), field labels

**Templates** (12 keys): template type labels (3), instance status enum (4), section labels

**Reports** (20 keys): report type enum (4), report status enum (5), section type enum (10), field labels

**Safety profiles** (15 keys): opsec level enum (3), threat level enum (4), field labels

**French** (`web/src/messages/fr.json`): mirror all keys with French translations

**Total new keys**: ~155 per language

---

## Part 14: Wiring

File: update `cmd/server/main.go`

```go
// Investigation subsystem
investigationRepo := investigation.NewPGRepository(pool)
investigationSvc := investigation.NewInvestigationService(
    investigationRepo, custodyLogger, searchIndexer, caseLookup, logger,
).WithCaptureMetadataUpdater(captureMetadataRepo)

investigationHandler := investigation.NewHandler(investigationSvc, auditLogger)

// Add to server route registrars
httpServer := server.NewHTTPServer(cfg, logger, version, jwks, auditLogger, healthHandler,
    ..., investigationHandler,
)
```

---

## Part 15: Alignment Document Update

File: update `docs/berkeley-protocol-alignment.md`

Move ALL Phase 1, 2, 5 (upgrade), 6, P4, S2, R1, R3 from "Gap" to "Covered" in the alignment table. Remove the v2/v3 roadmap section. Add note: "Full alignment achieved."

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `migrations/022_investigation_inquiry_logs.up.sql` | Create | Inquiry logs table |
| `migrations/023_preliminary_assessments.up.sql` | Create | Assessments table |
| `migrations/024_verification_records.up.sql` | Create | Structured verification records |
| `migrations/025_corroboration_links.up.sql` | Create | Claims + junction table |
| `migrations/026_analysis_notes.up.sql` | Create | Analysis notes table |
| `migrations/027_investigation_templates.up.sql` | Create | Templates + instances tables |
| `migrations/028_investigation_reports.up.sql` | Create | Reports table |
| `migrations/029_investigator_safety.up.sql` | Create | Safety profiles table |
| `migrations/022-029_*.down.sql` | Create | Down migrations (8 files) |
| `internal/investigation/inquiry_log.go` | Create | Domain model + validation |
| `internal/investigation/assessment.go` | Create | Domain model + validation |
| `internal/investigation/verification_record.go` | Create | Domain model + validation |
| `internal/investigation/corroboration.go` | Create | Domain model + validation |
| `internal/investigation/analysis_note.go` | Create | Domain model + validation |
| `internal/investigation/template.go` | Create | Domain model + schema definitions |
| `internal/investigation/report.go` | Create | Domain model + section handling |
| `internal/investigation/safety_profile.go` | Create | Domain model + redaction |
| `internal/investigation/repository.go` | Create | Repository interface |
| `internal/investigation/pg_repository.go` | Create | PG implementation |
| `internal/investigation/service.go` | Create | Business logic + custody events |
| `internal/investigation/handler.go` | Create | HTTP endpoints (30+ routes) |
| `internal/evidence/bulk.go` | Modify | Add Berkeley fields to CSV parsing |
| `internal/search/models.go` | Modify | Add assessment/verification/corroboration fields |
| `internal/search/meilisearch.go` | Modify | Update filterable attributes |
| `internal/cases/export.go` | Modify | Add 6 new CSV files to export |
| `cmd/server/main.go` | Modify | Wire investigation subsystem |
| `web/src/types/index.ts` | Modify | Add 12 new interfaces + 16 enum arrays |
| `web/src/lib/investigation-api.ts` | Create | API client (30+ functions) |
| `web/src/components/investigation/*.tsx` | Create | 10 new components |
| `web/src/app/[locale]/(app)/cases/[id]/investigation/page.tsx` | Create | Investigation tab page |
| `web/src/components/evidence/evidence-detail.tsx` | Modify | Add assessment/verification/corroboration cards |
| `web/src/messages/en.json` | Modify | ~155 new keys |
| `web/src/messages/fr.json` | Modify | ~155 new keys |
| `docs/berkeley-protocol-alignment.md` | Modify | Update all gaps to Covered |

---

## Testing Strategy (TDD — 80%+ coverage)

**Unit tests** (~200 test cases):
- All validation functions for all 8 domain models
- Enum validation, score range clamping, required fields
- URL scheme validation on search_url
- Role-gating on safety profiles
- Report section ordering and validation
- Template schema validation
- Corroboration minimum 2 evidence items check
- Auto-verification upgrade logic (finding=authentic + confidence=high → verified)
- Supersession logic for analysis notes

**Integration tests** (~80 test cases):
- Repository CRUD for all 8 tables
- FK constraints (ON DELETE RESTRICT fires correctly)
- UNIQUE constraints (safety profile per case+user, corroboration evidence per claim+item)
- CHECK constraints for all enums
- Cascade deletes on corroboration_evidence when claim deleted
- Template instance content matches schema structure

**Handler tests** (~100 test cases):
- All 30+ endpoints with valid/invalid input
- Role-based access: investigator vs prosecutor vs observer for each endpoint
- 400/403/404 cases
- Custody events recorded for each mutation
- Auto-verification triggers capture_metadata update
- Safety profile enforcement warnings

**Service tests** (~60 test cases):
- Custody event logging for all 20+ action types
- Search reindexing on assessment/verification changes
- Safety profile VPN requirement advisory
- Report publish workflow (draft → review → approved → published)
- Analysis note supersession chain

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Scope creep — 11 features in one sprint | Clear table boundaries; each feature is independent. Implement in order: Phase 1→2→5→6→templates→reports→safety |
| Template schema flexibility vs validation | JSONB content with type-level schema definitions. Schema validated at application layer, not DB |
| Report PDF generation complexity | Defer PDF rendering to v4; v2 exports as JSON. Add /export endpoint stub returning 501 |
| Corroboration claim with deleted evidence | ON DELETE RESTRICT prevents evidence deletion while linked to claims |
| Analysis note supersession chains | Limit chain depth to 10; warn on deep chains |
| Safety profile privacy | Entire safety_profiles table gated to prosecutor/judge/own-profile only |
| Bulk upload with Berkeley fields — backward compat | New columns are optional; existing CSVs work unchanged |
| 8 new migrations in sequence | Test each migration independently; down migrations verified |
| Large i18n surface area | Generate fr.json keys with English placeholders; translate in follow-up |
| Handler explosion (30+ routes) | Single `internal/investigation/` package with one handler struct; routes grouped logically |

---

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: (new session required)
- GEMINI_SESSION: (new session required)
