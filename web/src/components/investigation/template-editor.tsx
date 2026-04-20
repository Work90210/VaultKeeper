'use client';

import { useEffect, useState } from 'react';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface SchemaSection {
  readonly title: string;
  readonly description?: string;
}

interface TemplateSchema {
  readonly id: string;
  readonly name: string;
  readonly description?: string;
  readonly schema_definition: {
    readonly sections: readonly SchemaSection[];
  };
}

export function TemplateEditor({
  caseId,
  templateId,
  accessToken,
  onSaved,
  existingInstance,
}: {
  caseId: string;
  templateId: string;
  accessToken: string;
  onSaved: () => void;
  existingInstance?: { id: string; content: Record<string, unknown>; status: string };
}) {
  const [template, setTemplate] = useState<TemplateSchema | null>(null);
  const [sectionContents, setSectionContents] = useState<
    Record<string, string>
  >({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchTemplate = async () => {
      setLoading(true);
      setError(null);

      try {
        const res = await fetch(`${API_BASE}/api/templates/${templateId}`, {
          headers: { Authorization: `Bearer ${accessToken}` },
        });

        if (!res.ok) {
          throw new Error(`Failed to load template (${res.status})`);
        }

        const data = await res.json();
        const tmpl: TemplateSchema = data.data || data;

        if (!cancelled) {
          setTemplate(tmpl);

          const initial: Record<string, string> = {};
          for (const section of tmpl.schema_definition.sections) {
            initial[section.title] = existingInstance?.content?.[section.title] as string || '';
          }
          setSectionContents(initial);
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : 'Failed to load template.',
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    fetchTemplate();

    return () => {
      cancelled = true;
    };
  }, [templateId, accessToken]);

  const updateSection = (title: string, value: string) => {
    setSectionContents((prev) => ({ ...prev, [title]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    try {
      const content: Record<string, string> = {};
      for (const [key, val] of Object.entries(sectionContents)) {
        content[key] = val;
      }

      const isEditing = !!existingInstance;
      const url = isEditing
        ? `${API_BASE}/api/template-instances/${existingInstance.id}`
        : `${API_BASE}/api/cases/${caseId}/template-instances`;
      const body = isEditing
        ? { content, status: existingInstance.status }
        : { template_id: templateId, content };

      const res = await fetch(url, {
        method: isEditing ? 'PUT' : 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        throw new Error(
          data?.error || `Failed to save template instance (${res.status})`,
        );
      }

      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred.');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="card" style={{ padding: 'var(--space-lg)' }}>
        <p style={{ color: 'var(--text-secondary)', fontSize: 'var(--text-sm)' }}>
          Loading template...
        </p>
      </div>
    );
  }

  if (!template) {
    return (
      <div className="card" style={{ padding: 'var(--space-lg)' }}>
        {error && <div className="banner-error">{error}</div>}
        {!error && (
          <p style={{ color: 'var(--text-secondary)', fontSize: 'var(--text-sm)' }}>
            Template not found.
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="card" style={{ padding: 'var(--space-lg)' }}>
      <h2
        style={{
          fontSize: 'var(--text-xl)',
          fontWeight: 600,
          color: 'var(--text-primary)',
          margin: 0,
          marginBottom: 'var(--space-xs)',
        }}
      >
        {template.name}
      </h2>

      {template.description && (
        <p
          style={{
            fontSize: 'var(--text-sm)',
            color: 'var(--text-secondary)',
            margin: 0,
            marginBottom: 'var(--space-lg)',
          }}
        >
          {template.description}
        </p>
      )}

      {error && (
        <div className="banner-error" style={{ marginBottom: 'var(--space-md)' }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-lg)' }}>
        {template.schema_definition.sections.map((section, idx) => (
          <div key={section.title + idx}>
            <h3
              style={{
                fontSize: 'var(--text-lg)',
                fontWeight: 600,
                color: 'var(--text-primary)',
                margin: 0,
                marginBottom: 'var(--space-xs)',
              }}
            >
              {section.title}
            </h3>

            {section.description && (
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-tertiary)',
                  margin: 0,
                  marginBottom: 'var(--space-sm)',
                }}
              >
                {section.description}
              </p>
            )}

            <textarea
              className="input-field"
              rows={6}
              placeholder={`Enter ${section.title.toLowerCase()} content...`}
              value={sectionContents[section.title] ?? ''}
              onChange={(e) => updateSection(section.title, e.target.value)}
              style={{ resize: 'vertical' }}
            />
          </div>
        ))}

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 'var(--space-sm)' }}>
          <button
            type="button"
            className="btn-primary"
            disabled={saving}
            onClick={handleSave}
          >
            {saving ? 'Saving...' : existingInstance ? 'Update Instance' : 'Save Instance'}
          </button>
        </div>
      </div>
    </div>
  );
}
