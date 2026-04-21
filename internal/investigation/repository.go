package investigation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the data access interface for the investigation subsystem.
type Repository interface {
	// Inquiry Logs (Phase 1)
	CreateInquiryLog(ctx context.Context, log InquiryLog) (InquiryLog, error)
	ListInquiryLogs(ctx context.Context, caseID uuid.UUID, limit, offset int) ([]InquiryLog, int, error)
	GetInquiryLog(ctx context.Context, id uuid.UUID) (InquiryLog, error)
	UpdateInquiryLog(ctx context.Context, id, caseID uuid.UUID, log InquiryLog) (InquiryLog, error)
	DeleteInquiryLog(ctx context.Context, id, caseID uuid.UUID) error
	SetInquiryLogSealedStatus(ctx context.Context, id uuid.UUID, status string, sealedAt *time.Time, notes *string) (InquiryLog, error)

	// Assessments (Phase 2)
	CreateAssessment(ctx context.Context, assessment EvidenceAssessment) (EvidenceAssessment, error)
	GetAssessmentsByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceAssessment, error)
	GetAssessment(ctx context.Context, id uuid.UUID) (EvidenceAssessment, error)
	UpdateAssessment(ctx context.Context, id, evidenceID uuid.UUID, assessment EvidenceAssessment) (EvidenceAssessment, error)

	// Verification Records (Phase 5)
	CreateVerificationRecord(ctx context.Context, record VerificationRecord) (VerificationRecord, error)
	ListVerificationRecords(ctx context.Context, evidenceID uuid.UUID) ([]VerificationRecord, error)
	GetVerificationRecord(ctx context.Context, id uuid.UUID) (VerificationRecord, error)

	// Corroboration (Phase 5)
	CreateCorroborationClaim(ctx context.Context, claim CorroborationClaim) (CorroborationClaim, error)
	ListCorroborationClaims(ctx context.Context, caseID uuid.UUID) ([]CorroborationClaim, error)
	GetCorroborationClaim(ctx context.Context, id uuid.UUID) (CorroborationClaim, error)
	AddEvidenceToClaim(ctx context.Context, evidence CorroborationEvidence) error
	RemoveEvidenceFromClaim(ctx context.Context, claimID, evidenceID uuid.UUID) error
	GetClaimsByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]CorroborationClaim, error)

	// Analysis Notes (Phase 6)
	CreateAnalysisNote(ctx context.Context, note AnalysisNote) (AnalysisNote, error)
	ListAnalysisNotes(ctx context.Context, caseID uuid.UUID, limit, offset int) ([]AnalysisNote, int, error)
	GetAnalysisNote(ctx context.Context, id uuid.UUID) (AnalysisNote, error)
	UpdateAnalysisNote(ctx context.Context, id uuid.UUID, note AnalysisNote) (AnalysisNote, error)
	SupersedeAnalysisNote(ctx context.Context, oldID uuid.UUID, newNote AnalysisNote) (AnalysisNote, error)

	// Templates (Annexes 1-3)
	ListTemplates(ctx context.Context, templateType string) ([]InvestigationTemplate, error)
	GetTemplate(ctx context.Context, id uuid.UUID) (InvestigationTemplate, error)
	CreateTemplateInstance(ctx context.Context, instance TemplateInstance) (TemplateInstance, error)
	ListTemplateInstances(ctx context.Context, caseID uuid.UUID) ([]TemplateInstance, error)
	GetTemplateInstance(ctx context.Context, id uuid.UUID) (TemplateInstance, error)
	UpdateTemplateInstance(ctx context.Context, id, caseID uuid.UUID, content map[string]any, status string) (TemplateInstance, error)

	// Reports (R1, R3)
	CreateReport(ctx context.Context, report InvestigationReport) (InvestigationReport, error)
	ListReports(ctx context.Context, caseID uuid.UUID) ([]InvestigationReport, error)
	GetReport(ctx context.Context, id uuid.UUID) (InvestigationReport, error)
	UpdateReport(ctx context.Context, id, caseID uuid.UUID, report InvestigationReport) (InvestigationReport, error)

	// Bulk queries by case (for export)
	ListAssessmentsByCase(ctx context.Context, caseID uuid.UUID) ([]EvidenceAssessment, error)
	ListVerificationsByCase(ctx context.Context, caseID uuid.UUID) ([]VerificationRecord, error)

	// Safety Profiles (P4, S2)
	UpsertSafetyProfile(ctx context.Context, profile SafetyProfile) (SafetyProfile, error)
	GetSafetyProfile(ctx context.Context, caseID, userID uuid.UUID) (SafetyProfile, error)
	ListSafetyProfiles(ctx context.Context, caseID uuid.UUID) ([]SafetyProfile, error)
}
