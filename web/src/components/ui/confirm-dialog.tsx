'use client';

interface ConfirmDialogProps {
  readonly title: string;
  readonly message: string;
  readonly confirmLabel: string;
  readonly variant: 'danger' | 'warning';
  readonly onConfirm: () => void;
  readonly onCancel: () => void;
}

export function ConfirmDialog({ title, message, confirmLabel, variant, onConfirm, onCancel }: ConfirmDialogProps) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ backgroundColor: 'rgba(0,0,0,0.4)' }}
      onClick={onCancel}
    >
      <div
        className="card p-[var(--space-lg)] max-w-sm w-full mx-[var(--space-md)]"
        onClick={(e) => e.stopPropagation()}
        role="alertdialog"
        aria-labelledby="confirm-title"
        aria-describedby="confirm-message"
      >
        <h3
          id="confirm-title"
          className="font-[family-name:var(--font-heading)] text-lg mb-[var(--space-xs)]"
          style={{ color: 'var(--text-primary)' }}
        >
          {title}
        </h3>
        <p id="confirm-message" className="text-sm mb-[var(--space-lg)]" style={{ color: 'var(--text-secondary)' }}>
          {message}
        </p>
        <div className="flex justify-end gap-[var(--space-sm)]">
          <button type="button" className="btn-ghost text-sm" onClick={onCancel}>Cancel</button>
          <button
            type="button"
            className={variant === 'danger' ? 'btn-danger text-sm' : 'btn-primary text-sm'}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
