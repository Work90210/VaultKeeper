export interface Case {
  id: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: 'active' | 'closed' | 'archived';
  created_by: string;
  created_at: string;
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
