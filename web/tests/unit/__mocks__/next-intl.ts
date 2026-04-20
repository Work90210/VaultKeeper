import { vi } from 'vitest';

export const useTranslations = vi.fn((namespace: string) => {
  return (key: string) => `${namespace}.${key}`;
});
