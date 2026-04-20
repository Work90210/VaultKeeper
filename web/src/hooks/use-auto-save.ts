import { useEffect, useRef, useState } from 'react';

export function useAutoSave(
  data: unknown,
  saveFn: () => Promise<void>,
  delayMs: number = 3000,
  enabled: boolean = true,
): { lastSavedAt: Date | null; saving: boolean } {
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null);
  const [saving, setSaving] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const dataRef = useRef(data);

  useEffect(() => {
    dataRef.current = data;
  }, [data]);

  useEffect(() => {
    if (!enabled) return;

    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }

    timerRef.current = setTimeout(async () => {
      setSaving(true);
      try {
        await saveFn();
        setLastSavedAt(new Date());
      } catch {
        // Auto-save failures are silent — user can save manually
      } finally {
        setSaving(false);
      }
    }, delayMs);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, [data, saveFn, delayMs, enabled]);

  return { lastSavedAt, saving };
}
