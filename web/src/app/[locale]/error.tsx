'use client';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[50vh] gap-[var(--space-md)]">
      <h2
        className="font-[family-name:var(--font-heading)] text-xl"
        style={{ color: 'var(--text-primary)' }}
      >
        Something went wrong
      </h2>
      <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
        {error.message || 'An unexpected error occurred.'}
      </p>
      <button onClick={reset} className="btn-primary">
        Try again
      </button>
    </div>
  );
}
