'use client';

import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useState } from 'react';

export default function NewCasePage() {
  const router = useRouter();
  const { data: session } = useSession();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const apiBase = process.env.NEXT_PUBLIC_API_URL || '';

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
      const res = await fetch(`${apiBase}/api/cases`, {
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
    <main className="container mx-auto max-w-2xl px-6 py-8">
      <h1 className="mb-6 text-2xl font-bold tracking-tight">Create Case</h1>

      {error && (
        <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="reference_code" className="mb-1 block text-sm font-medium">
            Reference Code
          </label>
          <input
            id="reference_code"
            name="reference_code"
            type="text"
            required
            placeholder="ICC-UKR-2024"
            className="w-full rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
          <p className="mt-1 text-xs text-zinc-500">
            Format: ABC-ABC-1234 (e.g., ICC-UKR-2024)
          </p>
        </div>

        <div>
          <label htmlFor="title" className="mb-1 block text-sm font-medium">
            Title
          </label>
          <input
            id="title"
            name="title"
            type="text"
            required
            maxLength={500}
            className="w-full rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>

        <div>
          <label htmlFor="description" className="mb-1 block text-sm font-medium">
            Description
          </label>
          <textarea
            id="description"
            name="description"
            rows={4}
            maxLength={10000}
            className="w-full rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>

        <div>
          <label htmlFor="jurisdiction" className="mb-1 block text-sm font-medium">
            Jurisdiction
          </label>
          <input
            id="jurisdiction"
            name="jurisdiction"
            type="text"
            maxLength={200}
            className="w-full rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={loading}
            className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
          >
            {loading ? 'Creating...' : 'Create Case'}
          </button>
          <button
            type="button"
            onClick={() => router.back()}
            className="rounded-md border px-4 py-2 text-sm hover:bg-zinc-50"
          >
            Cancel
          </button>
        </div>
      </form>
    </main>
  );
}
