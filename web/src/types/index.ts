export interface Case {
  id: string;
  organization_id: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: 'active' | 'closed' | 'archived';
  created_by: string;
  created_at: string;
  updated_at: string;
}

// --- Organizations ---

export type OrgRole = 'owner' | 'admin' | 'member';

export interface Organization {
  id: string;
  name: string;
  slug: string;
  description: string;
  logo_asset_id: string | null;
  settings: Record<string, unknown>;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface OrgWithRole extends Organization {
  role: OrgRole;
  member_count: number;
  case_count: number;
}

export interface OrgMembership {
  id: string;
  organization_id: string;
  user_id: string;
  role: OrgRole;
  status: 'active' | 'invited' | 'suspended' | 'removed';
  joined_at: string | null;
  created_at: string;
  updated_at: string;
  display_name?: string;
  email?: string;
}

export interface OrgInvitation {
  id: string;
  organization_id: string;
  email: string;
  role: OrgRole;
  status: 'pending' | 'accepted' | 'declined' | 'expired' | 'revoked';
  expires_at: string;
  invited_by: string;
  created_at: string;
}

export interface UserProfile {
  user_id: string;
  display_name: string;
  avatar_url: string;
  bio: string;
  timezone: string;
  updated_at: string;
}

export interface EvidenceItem {
  id: string;
  case_id: string;
  evidence_number: string;
  version: number;
  parent_id: string | null;
  title: string;
  description: string;
  filename: string;
  original_name: string;
  size_bytes: number;
  mime_type: string;
  sha256_hash: string;
  tsa_token: string | null;
  tsa_timestamp: string | null;
  tsa_name: string;
  classification: 'public' | 'restricted' | 'confidential' | 'ex_parte';
  tags: string[];
  source: string;
  source_date: string | null;
  created_at: string;
  uploaded_by: string;
  uploaded_by_name: string;
  is_current: boolean;
  destroyed_at: string | null;
  metadata: Record<string, unknown>;
  storage_key: string;
  thumbnail_key: string | null;

  // Berkeley Protocol capture metadata (present when online capture)
  capture_metadata?: CaptureMetadata;

  // Redaction metadata (populated only for finalized redacted derivatives)
  redaction_name?: string;
  redaction_purpose?: RedactionPurpose;
  redaction_area_count?: number;
  redaction_author_id?: string;
  redaction_finalized_at?: string;
}

export type RedactionPurpose =
  | 'disclosure_defence'
  | 'disclosure_prosecution'
  | 'public_release'
  | 'court_submission'
  | 'witness_protection'
  | 'internal_review';

export const REDACTION_PURPOSE_LABELS: Record<RedactionPurpose, string> = {
  disclosure_defence: 'Disclosure to Defence',
  disclosure_prosecution: 'Disclosure to Prosecution',
  public_release: 'Public Release',
  court_submission: 'Court Submission',
  witness_protection: 'Witness Protection',
  internal_review: 'Internal Review',
};

export const REDACTION_PURPOSE_CODES: Record<RedactionPurpose, string> = {
  disclosure_defence: 'DEFENCE',
  disclosure_prosecution: 'PROSECUTION',
  public_release: 'PUBLIC',
  court_submission: 'COURT',
  witness_protection: 'WITNESS',
  internal_review: 'INTERNAL',
};

export interface RedactionDraft {
  id: string;
  evidence_id: string;
  case_id: string;
  name: string;
  purpose: RedactionPurpose;
  area_count: number;
  created_by: string;
  status: 'draft' | 'applied' | 'discarded';
  last_saved_at: string;
  created_at: string;
}

export interface FinalizedRedaction {
  id: string;
  evidence_number: string;
  name: string;
  purpose: RedactionPurpose;
  area_count: number;
  author: string;
  finalized_at: string;
}

export interface RedactionManagementView {
  finalized: FinalizedRedaction[];
  drafts: RedactionDraft[];
}

export interface CustodyEntry {
  id: string;
  case_id: string;
  evidence_id: string;
  action: string;
  actor_user_id: string;
  detail: string;
  hash_value: string;
  previous_hash: string;
  timestamp: string;
}

export interface Witness {
  id: string;
  case_id: string;
  witness_code: string;
  full_name: string | null;
  contact_info: string | null;
  location: string | null;
  protection_status: 'standard' | 'protected' | 'high_risk';
  statement_summary: string;
  related_evidence: string[];
  identity_visible: boolean;
  created_at: string;
  updated_at: string;
}

export interface CaseRole {
  id: string;
  case_id: string;
  user_id: string;
  role: 'investigator' | 'prosecutor' | 'defence' | 'judge' | 'observer' | 'victim_representative';
  granted_by: string;
  granted_at: string;
}

export interface Disclosure {
  id: string;
  case_id: string;
  evidence_ids: string[];
  disclosed_to: string;
  disclosed_by: string;
  disclosed_at: string;
  notes: string;
  redacted: boolean;
}

export interface CaseAssignment {
  id: string;
  case_id: string;
  user_id: string;
  role: string;
  granted_by: string;
  granted_at: string;
  case_title: string;
  reference_code: string;
  case_status: string;
}

export interface Notification {
  id: string;
  type: string;
  case_id: string | null;
  user_id: string;
  title: string;
  body: string;
  read: boolean;
  created_at: string;
}

export interface ApiKey {
  id: string;
  user_id: string;
  name: string;
  permissions: 'read' | 'read_write';
  last_used_at: string | null;
  revoked_at: string | null;
  created_at: string;
}

export interface BackupLog {
  id: string;
  started_at: string;
  completed_at: string | null;
  status: 'started' | 'completed' | 'failed';
  size_bytes: number | null;
  destination: string;
  error_message: string | null;
}

// Berkeley Protocol capture metadata

export type Platform =
  | 'x' | 'facebook' | 'instagram' | 'youtube'
  | 'telegram' | 'tiktok' | 'whatsapp' | 'signal'
  | 'reddit' | 'web' | 'other';

export type CaptureMethod =
  | 'screenshot' | 'screen_recording' | 'web_archive'
  | 'api_export' | 'manual_download' | 'browser_save'
  | 'forensic_tool' | 'other';

export type AvailabilityStatus =
  | 'accessible' | 'deleted' | 'geo_blocked'
  | 'login_required' | 'account_suspended'
  | 'removed' | 'unavailable' | 'unknown';

export type VerificationStatus =
  | 'unverified' | 'partially_verified' | 'verified' | 'disputed';

export type GeoSource =
  | 'exif' | 'platform_metadata' | 'manual_entry' | 'derived' | 'unknown';

export type PlatformContentType =
  | 'post' | 'profile' | 'video' | 'image'
  | 'comment' | 'story' | 'livestream'
  | 'channel' | 'page' | 'other';

export const PLATFORMS: { value: Platform; label: string }[] = [
  { value: 'x', label: 'X (Twitter)' },
  { value: 'facebook', label: 'Facebook' },
  { value: 'instagram', label: 'Instagram' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'telegram', label: 'Telegram' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'signal', label: 'Signal' },
  { value: 'reddit', label: 'Reddit' },
  { value: 'web', label: 'Web' },
  { value: 'other', label: 'Other' },
];

export const CAPTURE_METHODS: { value: CaptureMethod; label: string }[] = [
  { value: 'screenshot', label: 'Screenshot' },
  { value: 'screen_recording', label: 'Screen Recording' },
  { value: 'web_archive', label: 'Web Archive' },
  { value: 'api_export', label: 'API Export' },
  { value: 'manual_download', label: 'Manual Download' },
  { value: 'browser_save', label: 'Browser Save' },
  { value: 'forensic_tool', label: 'Forensic Tool' },
  { value: 'other', label: 'Other' },
];

export const AVAILABILITY_STATUSES: { value: AvailabilityStatus; label: string }[] = [
  { value: 'accessible', label: 'Accessible' },
  { value: 'deleted', label: 'Deleted' },
  { value: 'geo_blocked', label: 'Geo-Blocked' },
  { value: 'login_required', label: 'Login Required' },
  { value: 'account_suspended', label: 'Account Suspended' },
  { value: 'removed', label: 'Removed' },
  { value: 'unavailable', label: 'Unavailable' },
  { value: 'unknown', label: 'Unknown' },
];

export const VERIFICATION_STATUSES: { value: VerificationStatus; label: string }[] = [
  { value: 'unverified', label: 'Unverified' },
  { value: 'partially_verified', label: 'Partially Verified' },
  { value: 'verified', label: 'Verified' },
  { value: 'disputed', label: 'Disputed' },
];

export interface CaptureMetadata {
  id: string;
  evidence_id: string;
  source_url?: string;
  canonical_url?: string;
  platform?: Platform;
  platform_content_type?: PlatformContentType;
  capture_method: CaptureMethod;
  capture_timestamp: string;
  publication_timestamp?: string;
  collector_user_id?: string;
  collector_display_name?: string;
  creator_account_handle?: string;
  creator_account_display_name?: string;
  creator_account_url?: string;
  creator_account_id?: string;
  content_description?: string;
  content_language?: string;
  geo_latitude?: number;
  geo_longitude?: number;
  geo_place_name?: string;
  geo_source?: GeoSource;
  availability_status?: AvailabilityStatus;
  was_live?: boolean;
  was_deleted?: boolean;
  capture_tool_name?: string;
  capture_tool_version?: string;
  browser_name?: string;
  browser_version?: string;
  browser_user_agent?: string;
  network_context?: Record<string, unknown>;
  preservation_notes?: string;
  verification_status: VerificationStatus;
  verification_notes?: string;
  metadata_schema_version: number;
  created_at: string;
  updated_at: string;
}

// --- Berkeley Protocol v2+v3 Investigation Types ---

export type SourceCredibility = 'established' | 'credible' | 'uncertain' | 'unreliable' | 'unassessed';
export type Recommendation = 'collect' | 'monitor' | 'deprioritize' | 'discard';
export type VerificationType =
  | 'source_authentication' | 'content_verification' | 'reverse_image_search'
  | 'geolocation_verification' | 'chronolocation' | 'metadata_analysis'
  | 'witness_corroboration' | 'expert_analysis' | 'open_source_cross_reference' | 'other';
export type Finding =
  | 'authentic' | 'likely_authentic' | 'inconclusive'
  | 'likely_manipulated' | 'manipulated' | 'unable_to_verify';
export type ConfidenceLevel = 'high' | 'medium' | 'low';
export type ClaimType =
  | 'event_occurrence' | 'identity_confirmation' | 'location_confirmation'
  | 'timeline_confirmation' | 'pattern_of_conduct' | 'contextual_corroboration' | 'other';
export type ClaimStrength = 'strong' | 'moderate' | 'weak' | 'contested';
export type RoleInClaim = 'primary' | 'supporting' | 'contextual' | 'contradicting';
export type AnalysisType =
  | 'factual_finding' | 'pattern_analysis' | 'timeline_reconstruction'
  | 'geographic_analysis' | 'network_analysis' | 'legal_assessment'
  | 'credibility_assessment' | 'gap_identification' | 'hypothesis_testing' | 'other';
export type AnalysisStatus = 'draft' | 'in_review' | 'approved' | 'superseded';
export type TemplateType = 'investigation_plan' | 'threat_assessment' | 'digital_landscape';
export type ReportType = 'interim' | 'final' | 'supplementary' | 'expert_opinion';
export type ReportStatus = 'draft' | 'in_review' | 'approved' | 'published' | 'withdrawn';
export type OpsecLevel = 'standard' | 'elevated' | 'high_risk';
export type ThreatLevel = 'low' | 'medium' | 'high' | 'critical';

export interface InquiryLog {
  id: string;
  case_id: string;
  evidence_id?: string;
  search_strategy: string;
  search_keywords: string[];
  search_operators?: string;
  search_tool: string;
  search_tool_version?: string;
  search_url?: string;
  search_started_at: string;
  search_ended_at?: string;
  results_count?: number;
  results_relevant?: number;
  results_collected?: number;
  objective: string;
  notes?: string;
  performed_by: string;
  created_at: string;
  updated_at: string;
}

export interface EvidenceAssessment {
  id: string;
  evidence_id: string;
  case_id: string;
  relevance_score: number;
  relevance_rationale: string;
  reliability_score: number;
  reliability_rationale: string;
  source_credibility: SourceCredibility;
  misleading_indicators: string[];
  recommendation: Recommendation;
  methodology?: string;
  assessed_by: string;
  reviewed_by?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface VerificationRecord {
  id: string;
  evidence_id: string;
  case_id: string;
  verification_type: VerificationType;
  methodology: string;
  tools_used: string[];
  sources_consulted: string[];
  finding: Finding;
  finding_rationale: string;
  confidence_level: ConfidenceLevel;
  limitations?: string;
  caveats: string[];
  verified_by: string;
  reviewer?: string;
  reviewer_approved?: boolean;
  reviewer_notes?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CorroborationClaim {
  id: string;
  case_id: string;
  claim_summary: string;
  claim_type: ClaimType;
  strength: ClaimStrength;
  analysis_notes?: string;
  evidence: CorroborationEvidence[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CorroborationEvidence {
  id: string;
  claim_id: string;
  evidence_id: string;
  role_in_claim: RoleInClaim;
  contribution_notes?: string;
  added_by: string;
  created_at: string;
}

export interface AnalysisNote {
  id: string;
  case_id: string;
  title: string;
  analysis_type: AnalysisType;
  content: string;
  methodology?: string;
  related_evidence_ids: string[];
  related_inquiry_ids: string[];
  related_assessment_ids: string[];
  related_verification_ids: string[];
  status: AnalysisStatus;
  superseded_by?: string;
  author_id: string;
  reviewer_id?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface InvestigationTemplate {
  id: string;
  template_type: TemplateType;
  name: string;
  description?: string;
  version: number;
  is_default: boolean;
  schema_definition: Record<string, unknown>;
  is_system_template: boolean;
  created_at: string;
  updated_at: string;
}

export interface TemplateInstance {
  id: string;
  template_id: string;
  case_id: string;
  content: Record<string, unknown>;
  status: 'draft' | 'active' | 'completed' | 'archived';
  prepared_by: string;
  approved_by?: string;
  approved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface InvestigationReport {
  id: string;
  case_id: string;
  title: string;
  report_type: ReportType;
  sections: ReportSection[];
  limitations: string[];
  caveats: string[];
  assumptions: string[];
  referenced_evidence_ids: string[];
  referenced_analysis_ids: string[];
  status: ReportStatus;
  author_id: string;
  reviewer_id?: string;
  reviewed_at?: string;
  approved_by?: string;
  approved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ReportSection {
  section_type: string;
  title: string;
  content: string;
  order: number;
}

export interface SafetyProfile {
  id: string;
  case_id: string;
  user_id: string;
  pseudonym?: string;
  use_pseudonym: boolean;
  opsec_level: OpsecLevel;
  required_vpn: boolean;
  required_tor: boolean;
  approved_devices: string[];
  prohibited_platforms: string[];
  threat_level: ThreatLevel;
  threat_notes?: string;
  safety_briefing_completed: boolean;
  safety_briefing_date?: string;
  safety_officer_id?: string;
  created_at: string;
  updated_at: string;
}

export const SOURCE_CREDIBILITIES: { value: SourceCredibility; label: string }[] = [
  { value: 'established', label: 'Established' },
  { value: 'credible', label: 'Credible' },
  { value: 'uncertain', label: 'Uncertain' },
  { value: 'unreliable', label: 'Unreliable' },
  { value: 'unassessed', label: 'Unassessed' },
];

export const RECOMMENDATIONS: { value: Recommendation; label: string }[] = [
  { value: 'collect', label: 'Collect' },
  { value: 'monitor', label: 'Monitor' },
  { value: 'deprioritize', label: 'Deprioritize' },
  { value: 'discard', label: 'Discard' },
];

export const VERIFICATION_TYPES: { value: VerificationType; label: string }[] = [
  { value: 'source_authentication', label: 'Source Authentication' },
  { value: 'content_verification', label: 'Content Verification' },
  { value: 'reverse_image_search', label: 'Reverse Image Search' },
  { value: 'geolocation_verification', label: 'Geolocation Verification' },
  { value: 'chronolocation', label: 'Chronolocation' },
  { value: 'metadata_analysis', label: 'Metadata Analysis' },
  { value: 'witness_corroboration', label: 'Witness Corroboration' },
  { value: 'expert_analysis', label: 'Expert Analysis' },
  { value: 'open_source_cross_reference', label: 'Open Source Cross-Reference' },
  { value: 'other', label: 'Other' },
];

export const FINDINGS: { value: Finding; label: string }[] = [
  { value: 'authentic', label: 'Authentic' },
  { value: 'likely_authentic', label: 'Likely Authentic' },
  { value: 'inconclusive', label: 'Inconclusive' },
  { value: 'likely_manipulated', label: 'Likely Manipulated' },
  { value: 'manipulated', label: 'Manipulated' },
  { value: 'unable_to_verify', label: 'Unable to Verify' },
];

export const CONFIDENCE_LEVELS: { value: ConfidenceLevel; label: string }[] = [
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
];

export const CLAIM_TYPES: { value: ClaimType; label: string }[] = [
  { value: 'event_occurrence', label: 'Event Occurrence' },
  { value: 'identity_confirmation', label: 'Identity Confirmation' },
  { value: 'location_confirmation', label: 'Location Confirmation' },
  { value: 'timeline_confirmation', label: 'Timeline Confirmation' },
  { value: 'pattern_of_conduct', label: 'Pattern of Conduct' },
  { value: 'contextual_corroboration', label: 'Contextual Corroboration' },
  { value: 'other', label: 'Other' },
];

export const CLAIM_STRENGTHS: { value: ClaimStrength; label: string }[] = [
  { value: 'strong', label: 'Strong' },
  { value: 'moderate', label: 'Moderate' },
  { value: 'weak', label: 'Weak' },
  { value: 'contested', label: 'Contested' },
];

export const ANALYSIS_TYPES: { value: AnalysisType; label: string }[] = [
  { value: 'factual_finding', label: 'Factual Finding' },
  { value: 'pattern_analysis', label: 'Pattern Analysis' },
  { value: 'timeline_reconstruction', label: 'Timeline Reconstruction' },
  { value: 'geographic_analysis', label: 'Geographic Analysis' },
  { value: 'network_analysis', label: 'Network Analysis' },
  { value: 'legal_assessment', label: 'Legal Assessment' },
  { value: 'credibility_assessment', label: 'Credibility Assessment' },
  { value: 'gap_identification', label: 'Gap Identification' },
  { value: 'hypothesis_testing', label: 'Hypothesis Testing' },
  { value: 'other', label: 'Other' },
];

export const REPORT_TYPES: { value: ReportType; label: string }[] = [
  { value: 'interim', label: 'Interim' },
  { value: 'final', label: 'Final' },
  { value: 'supplementary', label: 'Supplementary' },
  { value: 'expert_opinion', label: 'Expert Opinion' },
];

export const OPSEC_LEVELS: { value: OpsecLevel; label: string }[] = [
  { value: 'standard', label: 'Standard' },
  { value: 'elevated', label: 'Elevated' },
  { value: 'high_risk', label: 'High Risk' },
];

export const THREAT_LEVELS: { value: ThreatLevel; label: string }[] = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
];
