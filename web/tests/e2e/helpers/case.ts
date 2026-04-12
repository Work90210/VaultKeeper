import type { Page } from '@playwright/test';

/**
 * Known-good test case in the local dev database. Seeded in Phase 1
 * of the Sprint 10 smoke test; kept around for the lifetime of the
 * dev stack. Replace this with a fresh-case fixture once a case-seed
 * helper exists.
 */
export const SPRINT10_CASE_ID = '44b283ca-3ede-4a8b-bdef-86dddb3e9c51';

/**
 * Navigates to the case's Settings tab. Waits for the Data import
 * section to be visible before returning.
 */
export async function gotoCaseSettings(page: Page, caseId = SPRINT10_CASE_ID): Promise<void> {
  await page.goto(`/en/cases/${caseId}`);
  // The case-detail page is a client component with tab state; wait
  // for the tab list to exist before clicking the Settings tab.
  await page.getByRole('tab', { name: /settings/i }).click();
  await page.getByText(/data import/i).waitFor({ state: 'visible' });
}
