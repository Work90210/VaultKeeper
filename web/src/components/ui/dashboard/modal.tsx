'use client';

import { useEffect, useCallback } from 'react';

interface ModalProps {
  readonly open: boolean;
  readonly onClose: () => void;
  readonly title: React.ReactNode;
  readonly children: React.ReactNode;
  readonly layout?: 'default' | 'split';
  readonly left?: React.ReactNode;
  readonly wide?: boolean;
}

export function Modal({
  open,
  onClose,
  title,
  children,
  layout = 'default',
  left,
  wide = false,
}: ModalProps) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    },
    [onClose],
  );

  useEffect(() => {
    if (!open) return;
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [open, handleKeyDown]);

  if (!open) return null;

  const isSplit = layout === 'split';
  const isWide = wide || isSplit;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.3)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: 'var(--paper)',
          borderRadius: 'var(--radius)',
          maxWidth: isWide ? '56rem' : '32rem',
          width: '100%',
          boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)',
          maxHeight: '90vh',
          overflow: 'hidden',
          display: 'flex',
          flexDirection: 'column',
        }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label={typeof title === 'string' ? title : 'Dialog'}
      >
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            padding: '18px 22px',
            borderBottom: '1px solid var(--line)',
          }}
        >
          <h3
            style={{
              fontFamily: '"Fraunces", serif',
              fontWeight: 400,
              fontSize: 20,
              letterSpacing: '-.01em',
              margin: 0,
            }}
          >
            {title}
          </h3>
          <button
            type="button"
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: 18,
              color: 'var(--muted)',
              padding: 4,
              lineHeight: 1,
            }}
            aria-label="Close"
          >
            &#x2715;
          </button>
        </div>

        {isSplit ? (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              flex: 1,
              overflow: 'auto',
            }}
          >
            <div style={{ padding: 22, borderRight: '1px solid var(--line)' }}>
              {left}
            </div>
            <div style={{ padding: 22 }}>{children}</div>
          </div>
        ) : (
          <div style={{ padding: 22, overflow: 'auto' }}>{children}</div>
        )}
      </div>
    </div>
  );
}
