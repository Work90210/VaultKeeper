import { useEffect } from 'react';

export function useDirtyForm<T>(initialState: T, currentState: T): boolean {
  const isDirty = JSON.stringify(initialState) !== JSON.stringify(currentState);

  useEffect(() => {
    if (!isDirty) return;

    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [isDirty]);

  return isDirty;
}
