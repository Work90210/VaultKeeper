'use client';

import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useState } from 'react';
import { Shell } from '@/components/layout/shell';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export default function NewCasePage() {
  const router = useRouter();
  const { data: session } = useSession();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const formData = new FormData(e.currentTarget);
    const body = {
      reference_code: formData.get('reference_code'),
      title: formData.get('title'),
      description: formData.get('description'),
      jurisdiction: formData.get('jurisdiction'),
    };

    try {
      const res = await fetch(`${API_BASE}/api/cases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(session?.accessToken
            ? { Authorization: `Bearer ${session.accessToken}` }
            : {}),
        },
        body: JSON.stringify(body),
      });

      const data = await res.json();
      if (!res.ok) {
        setError(data.error || 'Failed to create case');
        setLoading(false);
        return;
      }

      router.push(`/en/cases/${data.data.id}`);
    } catch {
      setError('Failed to create case');
      setLoading(false);
    }
  };

  return (
    <Shell>
      <div className="max-w-xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <a
          href="/en/cases"
          className="link-subtle text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-lg)] inline-block"
        >
          &larr; Cases
        </a>

        <h1
          className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)] mb-[var(--space-lg)]"
          style={{ color: 'var(--text-primary)' }}
        >
          New Case
        </h1>

        {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}

        <div className="card p-[var(--space-lg)]">
          <form onSubmit={handleSubmit} className="space-y-[var(--space-lg)]">
            <FormField
              label="Reference Code"
              name="reference_code"
              required
              placeholder="ICC-UKR-2024"
              hint="Format: ABC-ABC-1234"
            />
            <FormField label="Title" name="title" required maxLength={500} />
            <FormField
              label="Description"
              name="description"
              multiline
              maxLength={10000}
            />
            <FormField
              label="Jurisdiction"
              name="jurisdiction"
              maxLength={200}
            />

            <div
              className="flex gap-[var(--space-md)] pt-[var(--space-md)]"
              style={{ borderTop: '1px solid var(--border-subtle)' }}
            >
              <button
                type="submit"
                disabled={loading}
                className="btn-primary"
              >
                {loading ? 'Creating...' : 'Create case'}
              </button>
              <button
                type="button"
                onClick={() => router.back()}
                className="btn-ghost"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </Shell>
  );
}

function FormField({
  label,
  name,
  required,
  placeholder,
  hint,
  multiline,
  maxLength,
}: {
  label: string;
  name: string;
  required?: boolean;
  placeholder?: string;
  hint?: string;
  multiline?: boolean;
  maxLength?: number;
}) {
  return (
    <div>
      <label htmlFor={name} className="field-label">
        {label}
      </label>
      {multiline ? (
        <textarea
          id={name}
          name={name}
          rows={4}
          maxLength={maxLength}
          className="input-field resize-y"
        />
      ) : (
        <input
          id={name}
          name={name}
          type="text"
          required={required}
          placeholder={placeholder}
          maxLength={maxLength}
          className="input-field"
        />
      )}
      {hint && (
        <p
          className="mt-[var(--space-xs)] text-[var(--text-xs)]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {hint}
        </p>
      )}
    </div>
  );
}
