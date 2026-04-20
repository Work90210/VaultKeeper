const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

async function apiFetch<T>(path: string, token: string, options?: RequestInit): Promise<{ data: T | null; error: string | null }> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { data: null, error: body?.error || `Request failed (${res.status})` };
  }
  const body = await res.json();
  return { data: body.data ?? body, error: null };
}

// --- Inquiry Logs (Phase 1) ---

export function createInquiryLog(caseId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/inquiry-logs`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listInquiryLogs(caseId: string, token: string, limit = 50, offset = 0) {
  return apiFetch(`/api/cases/${caseId}/inquiry-logs?limit=${limit}&offset=${offset}`, token);
}

export function getInquiryLog(id: string, token: string) {
  return apiFetch(`/api/inquiry-logs/${id}`, token);
}

// --- Assessments (Phase 2) ---

export function createAssessment(evidenceId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/evidence/${evidenceId}/assessments`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listAssessments(evidenceId: string, token: string) {
  return apiFetch(`/api/evidence/${evidenceId}/assessments`, token);
}

export function getAssessment(id: string, token: string) {
  return apiFetch(`/api/assessments/${id}`, token);
}

// --- Verification Records (Phase 5) ---

export function createVerificationRecord(evidenceId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/evidence/${evidenceId}/verifications`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listVerificationRecords(evidenceId: string, token: string) {
  return apiFetch(`/api/evidence/${evidenceId}/verifications`, token);
}

export function getVerificationRecord(id: string, token: string) {
  return apiFetch(`/api/verifications/${id}`, token);
}

// --- Corroboration (Phase 5) ---

export function createCorroborationClaim(caseId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/corroborations`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listCorroborationClaims(caseId: string, token: string) {
  return apiFetch(`/api/cases/${caseId}/corroborations`, token);
}

export function getCorroborationClaim(id: string, token: string) {
  return apiFetch(`/api/corroborations/${id}`, token);
}

export function getClaimsByEvidence(evidenceId: string, token: string) {
  return apiFetch(`/api/evidence/${evidenceId}/corroborations`, token);
}

// --- Analysis Notes (Phase 6) ---

export function createAnalysisNote(caseId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/analysis-notes`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listAnalysisNotes(caseId: string, token: string, limit = 50, offset = 0) {
  return apiFetch(`/api/cases/${caseId}/analysis-notes?limit=${limit}&offset=${offset}`, token);
}

export function getAnalysisNote(id: string, token: string) {
  return apiFetch(`/api/analysis-notes/${id}`, token);
}

// --- Templates (Annexes 1-3) ---

export function listTemplates(token: string, type?: string) {
  const qs = type ? `?type=${type}` : '';
  return apiFetch(`/api/templates${qs}`, token);
}

export function getTemplate(id: string, token: string) {
  return apiFetch(`/api/templates/${id}`, token);
}

export function createTemplateInstance(caseId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/template-instances`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listTemplateInstances(caseId: string, token: string) {
  return apiFetch(`/api/cases/${caseId}/template-instances`, token);
}

export function getTemplateInstance(id: string, token: string) {
  return apiFetch(`/api/template-instances/${id}`, token);
}

export function updateTemplateInstance(id: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/template-instances/${id}`, token, { method: 'PUT', body: JSON.stringify(data) });
}

// --- Reports (R1, R3) ---

export function createReport(caseId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/reports`, token, { method: 'POST', body: JSON.stringify(data) });
}

export function listReports(caseId: string, token: string) {
  return apiFetch(`/api/cases/${caseId}/reports`, token);
}

export function getReport(id: string, token: string) {
  return apiFetch(`/api/reports/${id}`, token);
}

export function publishReport(id: string, token: string) {
  return apiFetch(`/api/reports/${id}/publish`, token, { method: 'POST' });
}

// --- Safety Profiles (P4, S2) ---

export function listSafetyProfiles(caseId: string, token: string) {
  return apiFetch(`/api/cases/${caseId}/safety-profiles`, token);
}

export function getMySafetyProfile(caseId: string, token: string) {
  return apiFetch(`/api/cases/${caseId}/safety-profiles/mine`, token);
}

export function upsertSafetyProfile(caseId: string, userId: string, data: Record<string, unknown>, token: string) {
  return apiFetch(`/api/cases/${caseId}/safety-profiles/${userId}`, token, { method: 'PUT', body: JSON.stringify(data) });
}
