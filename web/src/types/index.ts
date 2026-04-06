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
  filename: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
  sha256_hash: string;
  classification: 'public' | 'restricted' | 'confidential' | 'ex_parte';
  uploaded_by: string;
  is_current: boolean;
  version: number;
  tsa_token: string | null;
  created_at: string;
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
  pseudonym: string;
  protection_status: 'standard' | 'protected' | 'high_risk';
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
  evidence_id: string;
  disclosed_to: string;
  disclosed_by: string;
  disclosed_at: string;
  notes: string;
}

export interface Notification {
  id: string;
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
