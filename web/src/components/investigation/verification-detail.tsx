'use client';

import type { VerificationRecord } from '@/types';
import {
  RecordDetailLayout,
  ContentSection,
  MetaBlock,
  KeywordBadge,
} from '@/components/investigation/record-detail-layout';

function toTitleCase(str: string): string {
  return str
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function VerificationDetail({
  record,
  accessToken: _accessToken,
}: {
  readonly record: VerificationRecord;
  readonly accessToken: string;
}) {
  return (
    <RecordDetailLayout
      backHref={`/en/cases/${record.case_id}?tab=verifications`}
      backLabel="Verifications"
      recordType="Verification"
      title={toTitleCase(record.verification_type)}
      subtitle={`${record.finding} \u00b7 ${record.confidence_level} confidence`}
      content={
        <>
          {/* Methodology */}
          <ContentSection label="Methodology">
            <p
              className="text-sm leading-relaxed whitespace-pre-wrap"
              style={{ color: 'var(--text-primary)' }}
            >
              {record.methodology}
            </p>
          </ContentSection>

          {/* Finding Rationale */}
          <ContentSection label="Finding Rationale">
            <p
              className="text-sm leading-relaxed whitespace-pre-wrap"
              style={{ color: 'var(--text-primary)' }}
            >
              {record.finding_rationale}
            </p>
          </ContentSection>

          {/* Tools Used */}
          {record.tools_used.length > 0 && (
            <ContentSection label="Tools Used">
              <div className="flex flex-wrap gap-[var(--space-xs)]">
                {record.tools_used.map((tool) => (
                  <KeywordBadge key={tool}>{tool}</KeywordBadge>
                ))}
              </div>
            </ContentSection>
          )}

          {/* Sources Consulted */}
          {record.sources_consulted.length > 0 && (
            <ContentSection label="Sources Consulted">
              <div className="flex flex-wrap gap-[var(--space-xs)]">
                {record.sources_consulted.map((source) => (
                  <KeywordBadge key={source}>{source}</KeywordBadge>
                ))}
              </div>
            </ContentSection>
          )}

          {/* Limitations */}
          {record.limitations && (
            <ContentSection label="Limitations">
              <p
                className="text-sm leading-relaxed whitespace-pre-wrap"
                style={{ color: 'var(--text-secondary)' }}
              >
                {record.limitations}
              </p>
            </ContentSection>
          )}

          {/* Caveats */}
          {record.caveats.length > 0 && (
            <ContentSection label="Caveats">
              <ul className="space-y-[var(--space-xs)]">
                {record.caveats.map((caveat) => (
                  <li
                    key={caveat}
                    className="text-sm leading-relaxed"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {caveat}
                  </li>
                ))}
              </ul>
            </ContentSection>
          )}

          {/* Reviewer Notes */}
          {record.reviewer_notes && (
            <ContentSection label="Reviewer Notes">
              <p
                className="text-sm leading-relaxed whitespace-pre-wrap"
                style={{ color: 'var(--text-secondary)' }}
              >
                {record.reviewer_notes}
              </p>
            </ContentSection>
          )}
        </>
      }
      sidebar={
        <>
          {/* Finding */}
          <MetaBlock label="Finding">
            <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
              {record.finding}
            </p>
          </MetaBlock>

          {/* Confidence Level */}
          <MetaBlock label="Confidence Level">
            <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
              {record.confidence_level}
            </p>
          </MetaBlock>

          {/* Verified By */}
          <MetaBlock label="Verified By">
            <p
              className="text-xs font-[family-name:var(--font-mono)] break-all"
              style={{ color: 'var(--text-secondary)' }}
            >
              {record.verified_by}
            </p>
          </MetaBlock>

          {/* Reviewer */}
          {record.reviewer && (
            <MetaBlock label="Reviewer">
              <p
                className="text-xs font-[family-name:var(--font-mono)] break-all"
                style={{ color: 'var(--text-secondary)' }}
              >
                {record.reviewer}
              </p>
            </MetaBlock>
          )}

          {/* Reviewer Approved */}
          {record.reviewer_approved != null && (
            <MetaBlock label="Reviewer Approved">
              <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                {record.reviewer_approved ? 'Yes' : 'No'}
              </p>
            </MetaBlock>
          )}
        </>
      }
      recordId={record.id}
      createdAt={record.created_at}
      updatedAt={record.updated_at}
      createdBy={record.verified_by}
    />
  );
}
