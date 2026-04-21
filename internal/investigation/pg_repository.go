package investigation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// PGRepository is the Postgres implementation of Repository.
type PGRepository struct {
	pool dbPool
}

// NewPGRepository creates a new Postgres investigation repository.
func NewPGRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// --- Inquiry Logs ---

func (r *PGRepository) CreateInquiryLog(ctx context.Context, log InquiryLog) (InquiryLog, error) {
	log.ID = uuid.New()
	now := time.Now().UTC()
	log.CreatedAt = now
	log.UpdatedAt = now
	if log.Priority == "" {
		log.Priority = "normal"
	}
	if log.SealedStatus == "" {
		log.SealedStatus = "active"
	}

	_, err := r.pool.Exec(ctx, `INSERT INTO investigation_inquiry_logs
		(id, case_id, evidence_id, search_strategy, search_keywords, search_operators,
		 search_tool, search_tool_version, search_url, search_started_at, search_ended_at,
		 results_count, results_relevant, results_collected, objective, notes,
		 assigned_to, priority, sealed_status, sealed_at, performed_by,
		 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		log.ID, log.CaseID, log.EvidenceID, log.SearchStrategy, log.SearchKeywords,
		log.SearchOperators, log.SearchTool, log.SearchToolVersion, log.SearchURL,
		log.SearchStartedAt, log.SearchEndedAt, log.ResultsCount, log.ResultsRelevant,
		log.ResultsCollected, log.Objective, log.Notes,
		log.AssignedTo, log.Priority, log.SealedStatus, log.SealedAt, log.PerformedBy,
		log.CreatedAt, log.UpdatedAt)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("create inquiry log: %w", err)
	}
	return log, nil
}

func (r *PGRepository) ListInquiryLogs(ctx context.Context, caseID uuid.UUID, limit, offset int) ([]InquiryLog, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM investigation_inquiry_logs WHERE case_id = $1`, caseID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count inquiry logs: %w", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT id, case_id, evidence_id, search_strategy, search_keywords,
		search_operators, search_tool, search_tool_version, search_url, search_started_at,
		search_ended_at, results_count, results_relevant, results_collected, objective, notes,
		assigned_to, priority, sealed_status, sealed_at, performed_by, created_at, updated_at
		FROM investigation_inquiry_logs WHERE case_id = $1
		ORDER BY search_started_at DESC LIMIT $2 OFFSET $3`, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list inquiry logs: %w", err)
	}
	defer rows.Close()

	var logs []InquiryLog
	for rows.Next() {
		var l InquiryLog
		if err := rows.Scan(&l.ID, &l.CaseID, &l.EvidenceID, &l.SearchStrategy, &l.SearchKeywords,
			&l.SearchOperators, &l.SearchTool, &l.SearchToolVersion, &l.SearchURL,
			&l.SearchStartedAt, &l.SearchEndedAt, &l.ResultsCount, &l.ResultsRelevant,
			&l.ResultsCollected, &l.Objective, &l.Notes,
			&l.AssignedTo, &l.Priority, &l.SealedStatus, &l.SealedAt, &l.PerformedBy,
			&l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan inquiry log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (r *PGRepository) GetInquiryLog(ctx context.Context, id uuid.UUID) (InquiryLog, error) {
	var l InquiryLog
	err := r.pool.QueryRow(ctx, `SELECT id, case_id, evidence_id, search_strategy, search_keywords,
		search_operators, search_tool, search_tool_version, search_url, search_started_at,
		search_ended_at, results_count, results_relevant, results_collected, objective, notes,
		assigned_to, priority, sealed_status, sealed_at, performed_by, created_at, updated_at
		FROM investigation_inquiry_logs WHERE id = $1`, id).
		Scan(&l.ID, &l.CaseID, &l.EvidenceID, &l.SearchStrategy, &l.SearchKeywords,
			&l.SearchOperators, &l.SearchTool, &l.SearchToolVersion, &l.SearchURL,
			&l.SearchStartedAt, &l.SearchEndedAt, &l.ResultsCount, &l.ResultsRelevant,
			&l.ResultsCollected, &l.Objective, &l.Notes,
			&l.AssignedTo, &l.Priority, &l.SealedStatus, &l.SealedAt, &l.PerformedBy,
			&l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return InquiryLog{}, ErrNotFound
	}
	if err != nil {
		return InquiryLog{}, fmt.Errorf("get inquiry log: %w", err)
	}
	return l, nil
}

func (r *PGRepository) UpdateInquiryLog(ctx context.Context, id, caseID uuid.UUID, log InquiryLog) (InquiryLog, error) {
	log.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `UPDATE investigation_inquiry_logs SET
		search_strategy=$2, search_keywords=$3, search_operators=$4,
		search_tool=$5, search_tool_version=$6, search_url=$7,
		search_started_at=$8, search_ended_at=$9, results_count=$10,
		results_relevant=$11, results_collected=$12, objective=$13, notes=$14,
		assigned_to=$15, priority=$16, updated_at=$17 WHERE id=$1 AND case_id=$18`,
		id, log.SearchStrategy, log.SearchKeywords, log.SearchOperators,
		log.SearchTool, log.SearchToolVersion, log.SearchURL,
		log.SearchStartedAt, log.SearchEndedAt, log.ResultsCount,
		log.ResultsRelevant, log.ResultsCollected, log.Objective, log.Notes,
		log.AssignedTo, log.Priority, log.UpdatedAt, caseID)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("update inquiry log: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return InquiryLog{}, ErrNotFound
	}
	return r.GetInquiryLog(ctx, id)
}

func (r *PGRepository) DeleteInquiryLog(ctx context.Context, id, caseID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM investigation_inquiry_logs WHERE id = $1 AND case_id = $2`, id, caseID)
	if err != nil {
		return fmt.Errorf("delete inquiry log: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetInquiryLogSealedStatus updates sealed_status, sealed_at, and appends to notes atomically.
// This is the only method that writes sealed_status and sealed_at; UpdateInquiryLog deliberately
// does not touch these columns to prevent accidental overwrites.
func (r *PGRepository) SetInquiryLogSealedStatus(ctx context.Context, id uuid.UUID, status string, sealedAt *time.Time, notes *string) (InquiryLog, error) {
	updatedAt := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `UPDATE investigation_inquiry_logs SET
		sealed_status = $2,
		sealed_at = $3,
		notes = CASE WHEN $4::text IS NOT NULL
		             THEN COALESCE(notes || E'\n', '') || $4::text
		             ELSE notes END,
		updated_at = $5
		WHERE id = $1`,
		id, status, sealedAt, notes, updatedAt)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("set inquiry log sealed status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return InquiryLog{}, ErrNotFound
	}
	return r.GetInquiryLog(ctx, id)
}

// --- Assessments ---

func (r *PGRepository) CreateAssessment(ctx context.Context, a EvidenceAssessment) (EvidenceAssessment, error) {
	a.ID = uuid.New()
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `INSERT INTO evidence_assessments
		(id, evidence_id, case_id, relevance_score, relevance_rationale,
		 reliability_score, reliability_rationale, source_credibility,
		 misleading_indicators, recommendation, methodology, assessed_by,
		 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		a.ID, a.EvidenceID, a.CaseID, a.RelevanceScore, a.RelevanceRationale,
		a.ReliabilityScore, a.ReliabilityRationale, a.SourceCredibility,
		a.MisleadingIndicators, a.Recommendation, a.Methodology, a.AssessedBy,
		a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return EvidenceAssessment{}, fmt.Errorf("create assessment: %w", err)
	}
	return a, nil
}

func (r *PGRepository) GetAssessmentsByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceAssessment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, evidence_id, case_id, relevance_score, relevance_rationale,
		reliability_score, reliability_rationale, source_credibility, misleading_indicators,
		recommendation, methodology, assessed_by, reviewed_by, reviewed_at, created_at, updated_at
		FROM evidence_assessments WHERE evidence_id = $1 ORDER BY created_at DESC`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	defer rows.Close()

	var assessments []EvidenceAssessment
	for rows.Next() {
		var a EvidenceAssessment
		if err := rows.Scan(&a.ID, &a.EvidenceID, &a.CaseID, &a.RelevanceScore, &a.RelevanceRationale,
			&a.ReliabilityScore, &a.ReliabilityRationale, &a.SourceCredibility,
			&a.MisleadingIndicators, &a.Recommendation, &a.Methodology,
			&a.AssessedBy, &a.ReviewedBy, &a.ReviewedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan assessment: %w", err)
		}
		if a.MisleadingIndicators == nil {
			a.MisleadingIndicators = []string{}
		}
		assessments = append(assessments, a)
	}
	return assessments, rows.Err()
}

func (r *PGRepository) GetAssessment(ctx context.Context, id uuid.UUID) (EvidenceAssessment, error) {
	var a EvidenceAssessment
	err := r.pool.QueryRow(ctx, `SELECT id, evidence_id, case_id, relevance_score, relevance_rationale,
		reliability_score, reliability_rationale, source_credibility, misleading_indicators,
		recommendation, methodology, assessed_by, reviewed_by, reviewed_at, created_at, updated_at
		FROM evidence_assessments WHERE id = $1`, id).
		Scan(&a.ID, &a.EvidenceID, &a.CaseID, &a.RelevanceScore, &a.RelevanceRationale,
			&a.ReliabilityScore, &a.ReliabilityRationale, &a.SourceCredibility,
			&a.MisleadingIndicators, &a.Recommendation, &a.Methodology,
			&a.AssessedBy, &a.ReviewedBy, &a.ReviewedAt, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return EvidenceAssessment{}, ErrNotFound
	}
	if a.MisleadingIndicators == nil {
		a.MisleadingIndicators = []string{}
	}
	return a, err
}

func (r *PGRepository) UpdateAssessment(ctx context.Context, id, evidenceID uuid.UUID, a EvidenceAssessment) (EvidenceAssessment, error) {
	a.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `UPDATE evidence_assessments SET
		relevance_score=$2, relevance_rationale=$3, reliability_score=$4,
		reliability_rationale=$5, source_credibility=$6, misleading_indicators=$7,
		recommendation=$8, methodology=$9, updated_at=$10 WHERE id=$1 AND evidence_id=$11`,
		id, a.RelevanceScore, a.RelevanceRationale, a.ReliabilityScore,
		a.ReliabilityRationale, a.SourceCredibility, a.MisleadingIndicators,
		a.Recommendation, a.Methodology, a.UpdatedAt, evidenceID)
	if err != nil {
		return EvidenceAssessment{}, fmt.Errorf("update assessment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return EvidenceAssessment{}, ErrNotFound
	}
	return r.GetAssessment(ctx, id)
}

// --- Verification Records ---

func (r *PGRepository) CreateVerificationRecord(ctx context.Context, rec VerificationRecord) (VerificationRecord, error) {
	rec.ID = uuid.New()
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `INSERT INTO evidence_verification_records
		(id, evidence_id, case_id, verification_type, methodology, tools_used,
		 sources_consulted, finding, finding_rationale, confidence_level,
		 limitations, caveats, verified_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		rec.ID, rec.EvidenceID, rec.CaseID, rec.VerificationType, rec.Methodology,
		rec.ToolsUsed, rec.SourcesConsulted, rec.Finding, rec.FindingRationale,
		rec.ConfidenceLevel, rec.Limitations, rec.Caveats, rec.VerifiedBy,
		rec.CreatedAt, rec.UpdatedAt)
	if err != nil {
		return VerificationRecord{}, fmt.Errorf("create verification record: %w", err)
	}
	return rec, nil
}

func (r *PGRepository) ListVerificationRecords(ctx context.Context, evidenceID uuid.UUID) ([]VerificationRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, evidence_id, case_id, verification_type, methodology,
		tools_used, sources_consulted, finding, finding_rationale, confidence_level,
		limitations, caveats, verified_by, reviewer, reviewer_approved, reviewer_notes,
		reviewed_at, created_at, updated_at
		FROM evidence_verification_records WHERE evidence_id = $1 ORDER BY created_at DESC`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list verification records: %w", err)
	}
	defer rows.Close()

	var records []VerificationRecord
	for rows.Next() {
		var rec VerificationRecord
		if err := rows.Scan(&rec.ID, &rec.EvidenceID, &rec.CaseID, &rec.VerificationType,
			&rec.Methodology, &rec.ToolsUsed, &rec.SourcesConsulted, &rec.Finding,
			&rec.FindingRationale, &rec.ConfidenceLevel, &rec.Limitations, &rec.Caveats,
			&rec.VerifiedBy, &rec.Reviewer, &rec.ReviewerApproved, &rec.ReviewerNotes,
			&rec.ReviewedAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan verification record: %w", err)
		}
		if rec.ToolsUsed == nil {
			rec.ToolsUsed = []string{}
		}
		if rec.SourcesConsulted == nil {
			rec.SourcesConsulted = []string{}
		}
		if rec.Caveats == nil {
			rec.Caveats = []string{}
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *PGRepository) GetVerificationRecord(ctx context.Context, id uuid.UUID) (VerificationRecord, error) {
	var rec VerificationRecord
	err := r.pool.QueryRow(ctx, `SELECT id, evidence_id, case_id, verification_type, methodology,
		tools_used, sources_consulted, finding, finding_rationale, confidence_level,
		limitations, caveats, verified_by, reviewer, reviewer_approved, reviewer_notes,
		reviewed_at, created_at, updated_at
		FROM evidence_verification_records WHERE id = $1`, id).
		Scan(&rec.ID, &rec.EvidenceID, &rec.CaseID, &rec.VerificationType,
			&rec.Methodology, &rec.ToolsUsed, &rec.SourcesConsulted, &rec.Finding,
			&rec.FindingRationale, &rec.ConfidenceLevel, &rec.Limitations, &rec.Caveats,
			&rec.VerifiedBy, &rec.Reviewer, &rec.ReviewerApproved, &rec.ReviewerNotes,
			&rec.ReviewedAt, &rec.CreatedAt, &rec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return VerificationRecord{}, ErrNotFound
	}
	if rec.ToolsUsed == nil {
		rec.ToolsUsed = []string{}
	}
	if rec.SourcesConsulted == nil {
		rec.SourcesConsulted = []string{}
	}
	if rec.Caveats == nil {
		rec.Caveats = []string{}
	}
	return rec, err
}

// --- Corroboration ---

func (r *PGRepository) CreateCorroborationClaim(ctx context.Context, claim CorroborationClaim) (CorroborationClaim, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CorroborationClaim{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	claim.ID = uuid.New()
	now := time.Now().UTC()
	claim.CreatedAt = now
	claim.UpdatedAt = now

	_, err = tx.Exec(ctx, `INSERT INTO corroboration_claims
		(id, case_id, claim_summary, claim_type, strength, analysis_notes, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		claim.ID, claim.CaseID, claim.ClaimSummary, claim.ClaimType,
		claim.Strength, claim.AnalysisNotes, claim.CreatedBy, claim.CreatedAt, claim.UpdatedAt)
	if err != nil {
		return CorroborationClaim{}, fmt.Errorf("create corroboration claim: %w", err)
	}

	for i := range claim.Evidence {
		claim.Evidence[i].ID = uuid.New()
		claim.Evidence[i].ClaimID = claim.ID
		claim.Evidence[i].CreatedAt = now
		_, err = tx.Exec(ctx, `INSERT INTO corroboration_evidence
			(id, claim_id, evidence_id, role_in_claim, contribution_notes, added_by, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			claim.Evidence[i].ID, claim.ID, claim.Evidence[i].EvidenceID,
			claim.Evidence[i].RoleInClaim, claim.Evidence[i].ContributionNotes,
			claim.Evidence[i].AddedBy, claim.Evidence[i].CreatedAt)
		if err != nil {
			return CorroborationClaim{}, fmt.Errorf("add evidence to claim: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CorroborationClaim{}, fmt.Errorf("commit tx: %w", err)
	}
	return claim, nil
}

func (r *PGRepository) ListCorroborationClaims(ctx context.Context, caseID uuid.UUID) ([]CorroborationClaim, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, case_id, claim_summary, claim_type, strength,
		analysis_notes, created_by, created_at, updated_at
		FROM corroboration_claims WHERE case_id = $1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list corroboration claims: %w", err)
	}
	defer rows.Close()

	var claims []CorroborationClaim
	for rows.Next() {
		var c CorroborationClaim
		if err := rows.Scan(&c.ID, &c.CaseID, &c.ClaimSummary, &c.ClaimType, &c.Strength,
			&c.AnalysisNotes, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan claim: %w", err)
		}
		c.Evidence = []CorroborationEvidence{}
		claims = append(claims, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range claims {
		evidence, err := r.getClaimEvidence(ctx, claims[i].ID)
		if err != nil {
			return nil, err
		}
		claims[i].Evidence = evidence
	}
	return claims, nil
}

func (r *PGRepository) GetCorroborationClaim(ctx context.Context, id uuid.UUID) (CorroborationClaim, error) {
	var c CorroborationClaim
	err := r.pool.QueryRow(ctx, `SELECT id, case_id, claim_summary, claim_type, strength,
		analysis_notes, created_by, created_at, updated_at
		FROM corroboration_claims WHERE id = $1`, id).
		Scan(&c.ID, &c.CaseID, &c.ClaimSummary, &c.ClaimType, &c.Strength,
			&c.AnalysisNotes, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CorroborationClaim{}, ErrNotFound
	}
	if err != nil {
		return CorroborationClaim{}, fmt.Errorf("get claim: %w", err)
	}

	evidence, err := r.getClaimEvidence(ctx, c.ID)
	if err != nil {
		return CorroborationClaim{}, err
	}
	c.Evidence = evidence
	return c, nil
}

func (r *PGRepository) getClaimEvidence(ctx context.Context, claimID uuid.UUID) ([]CorroborationEvidence, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, claim_id, evidence_id, role_in_claim,
		contribution_notes, added_by, created_at
		FROM corroboration_evidence WHERE claim_id = $1 ORDER BY created_at`, claimID)
	if err != nil {
		return nil, fmt.Errorf("get claim evidence: %w", err)
	}
	defer rows.Close()

	var evidence []CorroborationEvidence
	for rows.Next() {
		var e CorroborationEvidence
		if err := rows.Scan(&e.ID, &e.ClaimID, &e.EvidenceID, &e.RoleInClaim,
			&e.ContributionNotes, &e.AddedBy, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan claim evidence: %w", err)
		}
		evidence = append(evidence, e)
	}
	if evidence == nil {
		evidence = []CorroborationEvidence{}
	}
	return evidence, rows.Err()
}

func (r *PGRepository) AddEvidenceToClaim(ctx context.Context, e CorroborationEvidence) error {
	e.ID = uuid.New()
	e.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `INSERT INTO corroboration_evidence
		(id, claim_id, evidence_id, role_in_claim, contribution_notes, added_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.ID, e.ClaimID, e.EvidenceID, e.RoleInClaim, e.ContributionNotes, e.AddedBy, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("add evidence to claim: %w", err)
	}
	return nil
}

func (r *PGRepository) RemoveEvidenceFromClaim(ctx context.Context, claimID, evidenceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM corroboration_evidence WHERE claim_id = $1 AND evidence_id = $2`, claimID, evidenceID)
	if err != nil {
		return fmt.Errorf("remove evidence from claim: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PGRepository) GetClaimsByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]CorroborationClaim, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT c.id, c.case_id, c.claim_summary, c.claim_type,
		c.strength, c.analysis_notes, c.created_by, c.created_at, c.updated_at
		FROM corroboration_claims c
		JOIN corroboration_evidence ce ON ce.claim_id = c.id
		WHERE ce.evidence_id = $1 ORDER BY c.created_at DESC`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("get claims by evidence: %w", err)
	}
	defer rows.Close()

	var claims []CorroborationClaim
	for rows.Next() {
		var c CorroborationClaim
		if err := rows.Scan(&c.ID, &c.CaseID, &c.ClaimSummary, &c.ClaimType, &c.Strength,
			&c.AnalysisNotes, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan claim: %w", err)
		}
		c.Evidence = []CorroborationEvidence{}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

// --- Analysis Notes ---

func (r *PGRepository) CreateAnalysisNote(ctx context.Context, note AnalysisNote) (AnalysisNote, error) {
	note.ID = uuid.New()
	now := time.Now().UTC()
	note.CreatedAt = now
	note.UpdatedAt = now
	note.Status = AnalysisStatusDraft

	_, err := r.pool.Exec(ctx, `INSERT INTO investigative_analysis_notes
		(id, case_id, title, analysis_type, content, methodology,
		 related_evidence_ids, related_inquiry_ids, related_assessment_ids,
		 related_verification_ids, status, author_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		note.ID, note.CaseID, note.Title, note.AnalysisType, note.Content, note.Methodology,
		note.RelatedEvidenceIDs, note.RelatedInquiryIDs, note.RelatedAssessmentIDs,
		note.RelatedVerificationIDs, note.Status, note.AuthorID, note.CreatedAt, note.UpdatedAt)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("create analysis note: %w", err)
	}
	return note, nil
}

func (r *PGRepository) ListAnalysisNotes(ctx context.Context, caseID uuid.UUID, limit, offset int) ([]AnalysisNote, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM investigative_analysis_notes WHERE case_id = $1`, caseID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count analysis notes: %w", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT id, case_id, title, analysis_type, content, methodology,
		related_evidence_ids, related_inquiry_ids, related_assessment_ids, related_verification_ids,
		status, superseded_by, author_id, reviewer_id, reviewed_at, created_at, updated_at
		FROM investigative_analysis_notes WHERE case_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list analysis notes: %w", err)
	}
	defer rows.Close()

	var notes []AnalysisNote
	for rows.Next() {
		var n AnalysisNote
		if err := rows.Scan(&n.ID, &n.CaseID, &n.Title, &n.AnalysisType, &n.Content, &n.Methodology,
			&n.RelatedEvidenceIDs, &n.RelatedInquiryIDs, &n.RelatedAssessmentIDs,
			&n.RelatedVerificationIDs, &n.Status, &n.SupersededBy, &n.AuthorID,
			&n.ReviewerID, &n.ReviewedAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan analysis note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, total, rows.Err()
}

func (r *PGRepository) GetAnalysisNote(ctx context.Context, id uuid.UUID) (AnalysisNote, error) {
	var n AnalysisNote
	err := r.pool.QueryRow(ctx, `SELECT id, case_id, title, analysis_type, content, methodology,
		related_evidence_ids, related_inquiry_ids, related_assessment_ids, related_verification_ids,
		status, superseded_by, author_id, reviewer_id, reviewed_at, created_at, updated_at
		FROM investigative_analysis_notes WHERE id = $1`, id).
		Scan(&n.ID, &n.CaseID, &n.Title, &n.AnalysisType, &n.Content, &n.Methodology,
			&n.RelatedEvidenceIDs, &n.RelatedInquiryIDs, &n.RelatedAssessmentIDs,
			&n.RelatedVerificationIDs, &n.Status, &n.SupersededBy, &n.AuthorID,
			&n.ReviewerID, &n.ReviewedAt, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AnalysisNote{}, ErrNotFound
	}
	return n, err
}

func (r *PGRepository) UpdateAnalysisNote(ctx context.Context, id uuid.UUID, note AnalysisNote) (AnalysisNote, error) {
	note.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `UPDATE investigative_analysis_notes SET
		title=$2, analysis_type=$3, content=$4, methodology=$5,
		related_evidence_ids=$6, related_inquiry_ids=$7, related_assessment_ids=$8,
		related_verification_ids=$9, status=$10, updated_at=$11 WHERE id=$1`,
		id, note.Title, note.AnalysisType, note.Content, note.Methodology,
		note.RelatedEvidenceIDs, note.RelatedInquiryIDs, note.RelatedAssessmentIDs,
		note.RelatedVerificationIDs, note.Status, note.UpdatedAt)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("update analysis note: %w", err)
	}
	return r.GetAnalysisNote(ctx, id)
}

func (r *PGRepository) SupersedeAnalysisNote(ctx context.Context, oldID uuid.UUID, newNote AnalysisNote) (AnalysisNote, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	newNote.ID = uuid.New()
	now := time.Now().UTC()
	newNote.CreatedAt = now
	newNote.UpdatedAt = now
	newNote.Status = AnalysisStatusDraft

	_, err = tx.Exec(ctx, `INSERT INTO investigative_analysis_notes
		(id, case_id, title, analysis_type, content, methodology,
		 related_evidence_ids, related_inquiry_ids, related_assessment_ids,
		 related_verification_ids, status, author_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		newNote.ID, newNote.CaseID, newNote.Title, newNote.AnalysisType, newNote.Content,
		newNote.Methodology, newNote.RelatedEvidenceIDs, newNote.RelatedInquiryIDs,
		newNote.RelatedAssessmentIDs, newNote.RelatedVerificationIDs,
		newNote.Status, newNote.AuthorID, newNote.CreatedAt, newNote.UpdatedAt)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("create superseding note: %w", err)
	}

	_, err = tx.Exec(ctx, `UPDATE investigative_analysis_notes SET
		status = 'superseded', superseded_by = $2, updated_at = $3 WHERE id = $1`,
		oldID, newNote.ID, now)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("supersede old note: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AnalysisNote{}, fmt.Errorf("commit tx: %w", err)
	}
	return newNote, nil
}

// --- Templates ---

func (r *PGRepository) ListTemplates(ctx context.Context, templateType string) ([]InvestigationTemplate, error) {
	query := `SELECT id, template_type, name, description, version, is_default,
		schema_definition, created_by, is_system_template, created_at, updated_at
		FROM investigation_templates`
	var args []any
	if templateType != "" {
		query += ` WHERE template_type = $1`
		args = append(args, templateType)
	}
	query += ` ORDER BY is_default DESC, name`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []InvestigationTemplate
	for rows.Next() {
		var t InvestigationTemplate
		var schemaJSON []byte
		if err := rows.Scan(&t.ID, &t.TemplateType, &t.Name, &t.Description, &t.Version,
			&t.IsDefault, &schemaJSON, &t.CreatedBy, &t.IsSystemTemplate,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		if schemaJSON != nil {
			_ = json.Unmarshal(schemaJSON, &t.SchemaDefinition)
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (r *PGRepository) GetTemplate(ctx context.Context, id uuid.UUID) (InvestigationTemplate, error) {
	var t InvestigationTemplate
	var schemaJSON []byte
	err := r.pool.QueryRow(ctx, `SELECT id, template_type, name, description, version, is_default,
		schema_definition, created_by, is_system_template, created_at, updated_at
		FROM investigation_templates WHERE id = $1`, id).
		Scan(&t.ID, &t.TemplateType, &t.Name, &t.Description, &t.Version,
			&t.IsDefault, &schemaJSON, &t.CreatedBy, &t.IsSystemTemplate,
			&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return InvestigationTemplate{}, ErrNotFound
	}
	if err != nil {
		return InvestigationTemplate{}, fmt.Errorf("get template: %w", err)
	}
	if schemaJSON != nil {
		_ = json.Unmarshal(schemaJSON, &t.SchemaDefinition)
	}
	return t, nil
}

func (r *PGRepository) CreateTemplateInstance(ctx context.Context, inst TemplateInstance) (TemplateInstance, error) {
	inst.ID = uuid.New()
	now := time.Now().UTC()
	inst.CreatedAt = now
	inst.UpdatedAt = now
	inst.Status = InstanceStatusDraft

	contentJSON, err := json.Marshal(inst.Content)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("marshal content: %w", err)
	}

	_, err = r.pool.Exec(ctx, `INSERT INTO investigation_template_instances
		(id, template_id, case_id, content, status, prepared_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		inst.ID, inst.TemplateID, inst.CaseID, contentJSON, inst.Status,
		inst.PreparedBy, inst.CreatedAt, inst.UpdatedAt)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("create template instance: %w", err)
	}
	return inst, nil
}

func (r *PGRepository) ListTemplateInstances(ctx context.Context, caseID uuid.UUID) ([]TemplateInstance, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, template_id, case_id, content, status,
		prepared_by, approved_by, approved_at, created_at, updated_at
		FROM investigation_template_instances WHERE case_id = $1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list template instances: %w", err)
	}
	defer rows.Close()

	var instances []TemplateInstance
	for rows.Next() {
		var inst TemplateInstance
		var contentJSON []byte
		if err := rows.Scan(&inst.ID, &inst.TemplateID, &inst.CaseID, &contentJSON,
			&inst.Status, &inst.PreparedBy, &inst.ApprovedBy, &inst.ApprovedAt,
			&inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template instance: %w", err)
		}
		if contentJSON != nil {
			_ = json.Unmarshal(contentJSON, &inst.Content)
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

func (r *PGRepository) GetTemplateInstance(ctx context.Context, id uuid.UUID) (TemplateInstance, error) {
	var inst TemplateInstance
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, `SELECT id, template_id, case_id, content, status,
		prepared_by, approved_by, approved_at, created_at, updated_at
		FROM investigation_template_instances WHERE id = $1`, id).
		Scan(&inst.ID, &inst.TemplateID, &inst.CaseID, &contentJSON,
			&inst.Status, &inst.PreparedBy, &inst.ApprovedBy, &inst.ApprovedAt,
			&inst.CreatedAt, &inst.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TemplateInstance{}, ErrNotFound
	}
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("get template instance: %w", err)
	}
	if contentJSON != nil {
		_ = json.Unmarshal(contentJSON, &inst.Content)
	}
	return inst, nil
}

func (r *PGRepository) UpdateTemplateInstance(ctx context.Context, id, caseID uuid.UUID, content map[string]any, status string) (TemplateInstance, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("marshal content: %w", err)
	}
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `UPDATE investigation_template_instances SET
		content=$2, status=$3, updated_at=$4 WHERE id=$1 AND case_id=$5`,
		id, contentJSON, status, now, caseID)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("update template instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return TemplateInstance{}, ErrNotFound
	}
	return r.GetTemplateInstance(ctx, id)
}

// --- Reports ---

func (r *PGRepository) CreateReport(ctx context.Context, report InvestigationReport) (InvestigationReport, error) {
	report.ID = uuid.New()
	now := time.Now().UTC()
	report.CreatedAt = now
	report.UpdatedAt = now
	report.Status = ReportStatusDraft

	sectionsJSON, err := json.Marshal(report.Sections)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("marshal sections: %w", err)
	}

	_, err = r.pool.Exec(ctx, `INSERT INTO investigation_reports
		(id, case_id, title, report_type, sections, limitations, caveats, assumptions,
		 referenced_evidence_ids, referenced_analysis_ids, status, author_id,
		 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		report.ID, report.CaseID, report.Title, report.ReportType, sectionsJSON,
		report.Limitations, report.Caveats, report.Assumptions,
		report.ReferencedEvidenceIDs, report.ReferencedAnalysisIDs,
		report.Status, report.AuthorID, report.CreatedAt, report.UpdatedAt)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("create report: %w", err)
	}
	return report, nil
}

func (r *PGRepository) ListReports(ctx context.Context, caseID uuid.UUID) ([]InvestigationReport, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, case_id, title, report_type, sections,
		limitations, caveats, assumptions, referenced_evidence_ids, referenced_analysis_ids,
		status, author_id, reviewer_id, reviewed_at, approved_by, approved_at, created_at, updated_at
		FROM investigation_reports WHERE case_id = $1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []InvestigationReport
	for rows.Next() {
		rpt, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rpt)
	}
	return reports, rows.Err()
}

func (r *PGRepository) GetReport(ctx context.Context, id uuid.UUID) (InvestigationReport, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, case_id, title, report_type, sections,
		limitations, caveats, assumptions, referenced_evidence_ids, referenced_analysis_ids,
		status, author_id, reviewer_id, reviewed_at, approved_by, approved_at, created_at, updated_at
		FROM investigation_reports WHERE id = $1`, id)

	rpt, err := scanReportRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return InvestigationReport{}, ErrNotFound
	}
	return rpt, err
}

func (r *PGRepository) UpdateReport(ctx context.Context, id, caseID uuid.UUID, report InvestigationReport) (InvestigationReport, error) {
	report.UpdatedAt = time.Now().UTC()
	sectionsJSON, err := json.Marshal(report.Sections)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("marshal sections: %w", err)
	}

	var tag pgconn.CommandTag
	tag, err = r.pool.Exec(ctx, `UPDATE investigation_reports SET
		title=$2, report_type=$3, sections=$4, limitations=$5, caveats=$6,
		assumptions=$7, referenced_evidence_ids=$8, referenced_analysis_ids=$9,
		status=$10, reviewer_id=$11, reviewed_at=$12, approved_by=$13, approved_at=$14,
		updated_at=$15 WHERE id=$1 AND case_id=$16`,
		id, report.Title, report.ReportType, sectionsJSON, report.Limitations,
		report.Caveats, report.Assumptions, report.ReferencedEvidenceIDs,
		report.ReferencedAnalysisIDs, report.Status, report.ReviewerID,
		report.ReviewedAt, report.ApprovedBy, report.ApprovedAt, report.UpdatedAt, caseID)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("update report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return InvestigationReport{}, ErrNotFound
	}
	return r.GetReport(ctx, id)
}

func scanReport(rows pgx.Rows) (InvestigationReport, error) {
	var rpt InvestigationReport
	var sectionsJSON []byte
	if err := rows.Scan(&rpt.ID, &rpt.CaseID, &rpt.Title, &rpt.ReportType, &sectionsJSON,
		&rpt.Limitations, &rpt.Caveats, &rpt.Assumptions,
		&rpt.ReferencedEvidenceIDs, &rpt.ReferencedAnalysisIDs,
		&rpt.Status, &rpt.AuthorID, &rpt.ReviewerID, &rpt.ReviewedAt,
		&rpt.ApprovedBy, &rpt.ApprovedAt, &rpt.CreatedAt, &rpt.UpdatedAt); err != nil {
		return InvestigationReport{}, fmt.Errorf("scan report: %w", err)
	}
	if sectionsJSON != nil {
		_ = json.Unmarshal(sectionsJSON, &rpt.Sections)
	}
	if rpt.Sections == nil {
		rpt.Sections = []ReportSection{}
	}
	if rpt.Limitations == nil {
		rpt.Limitations = []string{}
	}
	if rpt.Caveats == nil {
		rpt.Caveats = []string{}
	}
	if rpt.Assumptions == nil {
		rpt.Assumptions = []string{}
	}
	return rpt, nil
}

func scanReportRow(row pgx.Row) (InvestigationReport, error) {
	var rpt InvestigationReport
	var sectionsJSON []byte
	if err := row.Scan(&rpt.ID, &rpt.CaseID, &rpt.Title, &rpt.ReportType, &sectionsJSON,
		&rpt.Limitations, &rpt.Caveats, &rpt.Assumptions,
		&rpt.ReferencedEvidenceIDs, &rpt.ReferencedAnalysisIDs,
		&rpt.Status, &rpt.AuthorID, &rpt.ReviewerID, &rpt.ReviewedAt,
		&rpt.ApprovedBy, &rpt.ApprovedAt, &rpt.CreatedAt, &rpt.UpdatedAt); err != nil {
		return InvestigationReport{}, err
	}
	if sectionsJSON != nil {
		_ = json.Unmarshal(sectionsJSON, &rpt.Sections)
	}
	if rpt.Sections == nil {
		rpt.Sections = []ReportSection{}
	}
	if rpt.Limitations == nil {
		rpt.Limitations = []string{}
	}
	if rpt.Caveats == nil {
		rpt.Caveats = []string{}
	}
	if rpt.Assumptions == nil {
		rpt.Assumptions = []string{}
	}
	return rpt, nil
}

// --- Safety Profiles ---

func (r *PGRepository) UpsertSafetyProfile(ctx context.Context, profile SafetyProfile) (SafetyProfile, error) {
	now := time.Now().UTC()
	profile.UpdatedAt = now
	if profile.ID == uuid.Nil {
		profile.ID = uuid.New()
		profile.CreatedAt = now
	}

	_, err := r.pool.Exec(ctx, `INSERT INTO investigator_safety_profiles
		(id, case_id, user_id, pseudonym, use_pseudonym, opsec_level, required_vpn,
		 required_tor, approved_devices, prohibited_platforms, threat_level, threat_notes,
		 safety_briefing_completed, safety_briefing_date, safety_officer_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (case_id, user_id) DO UPDATE SET
		pseudonym = EXCLUDED.pseudonym, use_pseudonym = EXCLUDED.use_pseudonym,
		opsec_level = EXCLUDED.opsec_level, required_vpn = EXCLUDED.required_vpn,
		required_tor = EXCLUDED.required_tor, approved_devices = EXCLUDED.approved_devices,
		prohibited_platforms = EXCLUDED.prohibited_platforms, threat_level = EXCLUDED.threat_level,
		threat_notes = EXCLUDED.threat_notes, safety_briefing_completed = EXCLUDED.safety_briefing_completed,
		safety_briefing_date = EXCLUDED.safety_briefing_date, safety_officer_id = EXCLUDED.safety_officer_id,
		updated_at = EXCLUDED.updated_at`,
		profile.ID, profile.CaseID, profile.UserID, profile.Pseudonym, profile.UsePseudonym,
		profile.OpsecLevel, profile.RequiredVPN, profile.RequiredTor, profile.ApprovedDevices,
		profile.ProhibitedPlatforms, profile.ThreatLevel, profile.ThreatNotes,
		profile.SafetyBriefingCompleted, profile.SafetyBriefingDate, profile.SafetyOfficerID,
		profile.CreatedAt, profile.UpdatedAt)
	if err != nil {
		return SafetyProfile{}, fmt.Errorf("upsert safety profile: %w", err)
	}
	return r.GetSafetyProfile(ctx, profile.CaseID, profile.UserID)
}

func (r *PGRepository) GetSafetyProfile(ctx context.Context, caseID, userID uuid.UUID) (SafetyProfile, error) {
	var p SafetyProfile
	err := r.pool.QueryRow(ctx, `SELECT id, case_id, user_id, pseudonym, use_pseudonym,
		opsec_level, required_vpn, required_tor, approved_devices, prohibited_platforms,
		threat_level, threat_notes, safety_briefing_completed, safety_briefing_date,
		safety_officer_id, created_at, updated_at
		FROM investigator_safety_profiles WHERE case_id = $1 AND user_id = $2`, caseID, userID).
		Scan(&p.ID, &p.CaseID, &p.UserID, &p.Pseudonym, &p.UsePseudonym,
			&p.OpsecLevel, &p.RequiredVPN, &p.RequiredTor, &p.ApprovedDevices,
			&p.ProhibitedPlatforms, &p.ThreatLevel, &p.ThreatNotes,
			&p.SafetyBriefingCompleted, &p.SafetyBriefingDate, &p.SafetyOfficerID,
			&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SafetyProfile{}, ErrNotFound
	}
	if p.ApprovedDevices == nil {
		p.ApprovedDevices = []string{}
	}
	if p.ProhibitedPlatforms == nil {
		p.ProhibitedPlatforms = []string{}
	}
	return p, err
}

func (r *PGRepository) ListSafetyProfiles(ctx context.Context, caseID uuid.UUID) ([]SafetyProfile, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, case_id, user_id, pseudonym, use_pseudonym,
		opsec_level, required_vpn, required_tor, approved_devices, prohibited_platforms,
		threat_level, threat_notes, safety_briefing_completed, safety_briefing_date,
		safety_officer_id, created_at, updated_at
		FROM investigator_safety_profiles WHERE case_id = $1 ORDER BY created_at`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list safety profiles: %w", err)
	}
	defer rows.Close()

	var profiles []SafetyProfile
	for rows.Next() {
		var p SafetyProfile
		if err := rows.Scan(&p.ID, &p.CaseID, &p.UserID, &p.Pseudonym, &p.UsePseudonym,
			&p.OpsecLevel, &p.RequiredVPN, &p.RequiredTor, &p.ApprovedDevices,
			&p.ProhibitedPlatforms, &p.ThreatLevel, &p.ThreatNotes,
			&p.SafetyBriefingCompleted, &p.SafetyBriefingDate, &p.SafetyOfficerID,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan safety profile: %w", err)
		}
		if p.ApprovedDevices == nil {
			p.ApprovedDevices = []string{}
		}
		if p.ProhibitedPlatforms == nil {
			p.ProhibitedPlatforms = []string{}
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// --- Bulk queries by case (for export) ---

func (r *PGRepository) ListAssessmentsByCase(ctx context.Context, caseID uuid.UUID) ([]EvidenceAssessment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, evidence_id, case_id, relevance_score, relevance_rationale,
		reliability_score, reliability_rationale, source_credibility, misleading_indicators,
		recommendation, methodology, assessed_by, reviewed_by, reviewed_at, created_at, updated_at
		FROM evidence_assessments WHERE case_id = $1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list assessments by case: %w", err)
	}
	defer rows.Close()

	var assessments []EvidenceAssessment
	for rows.Next() {
		var a EvidenceAssessment
		if err := rows.Scan(&a.ID, &a.EvidenceID, &a.CaseID, &a.RelevanceScore, &a.RelevanceRationale,
			&a.ReliabilityScore, &a.ReliabilityRationale, &a.SourceCredibility,
			&a.MisleadingIndicators, &a.Recommendation, &a.Methodology,
			&a.AssessedBy, &a.ReviewedBy, &a.ReviewedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan assessment: %w", err)
		}
		if a.MisleadingIndicators == nil {
			a.MisleadingIndicators = []string{}
		}
		assessments = append(assessments, a)
	}
	return assessments, rows.Err()
}

func (r *PGRepository) ListVerificationsByCase(ctx context.Context, caseID uuid.UUID) ([]VerificationRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, evidence_id, case_id, verification_type, methodology,
		tools_used, sources_consulted, finding, finding_rationale, confidence_level,
		limitations, caveats, verified_by, reviewer, reviewer_approved, reviewer_notes,
		reviewed_at, created_at, updated_at
		FROM evidence_verification_records WHERE case_id = $1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list verifications by case: %w", err)
	}
	defer rows.Close()

	var records []VerificationRecord
	for rows.Next() {
		var rec VerificationRecord
		if err := rows.Scan(&rec.ID, &rec.EvidenceID, &rec.CaseID, &rec.VerificationType,
			&rec.Methodology, &rec.ToolsUsed, &rec.SourcesConsulted, &rec.Finding,
			&rec.FindingRationale, &rec.ConfidenceLevel, &rec.Limitations, &rec.Caveats,
			&rec.VerifiedBy, &rec.Reviewer, &rec.ReviewerApproved, &rec.ReviewerNotes,
			&rec.ReviewedAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan verification record: %w", err)
		}
		if rec.ToolsUsed == nil {
			rec.ToolsUsed = []string{}
		}
		if rec.SourcesConsulted == nil {
			rec.SourcesConsulted = []string{}
		}
		if rec.Caveats == nil {
			rec.Caveats = []string{}
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
