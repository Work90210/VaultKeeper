/**
 * Sprint 10 — end-to-end UI verification of the unified archive import
 * flow (case Settings → Data import).
 *
 * These specs require the full dev stack to be running:
 *   - Next.js dev server on :3000 (Playwright starts this itself)
 *   - Go API on :8080 with MIGRATION_STAGING_ROOT set and
 *     VAULTKEEPER_ALLOW_EPHEMERAL_SIGNING=1 for non-production key
 *   - Keycloak on :8180 with the sprint10-tester user + system_admin role
 *   - Postgres on :5433, MinIO on :9000
 *
 * Each spec self-skips if the backend is unreachable so CI without
 * the infra still gets a clean run.
 */

import { test, expect, type Page } from '@playwright/test';
import { login } from './helpers/auth';
import { gotoCaseSettings, SPRINT10_CASE_ID } from './helpers/case';
import {
  buildBulkUploadZip,
  buildVerifiedMigrationZip,
  buildBadArchive,
} from './helpers/zip-fixtures';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

/**
 * Skips the current test if the Go API isn't running. The Data import
 * flow requires the API; there is no useful fallback.
 */
async function skipIfBackendUnavailable(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/health`);
    return res.ok;
  } catch {
    return false;
  }
}

test.describe('Sprint 10 — Import archive (case Settings → Data import)', () => {
  test.beforeEach(async ({ page }) => {
    const healthy = await skipIfBackendUnavailable();
    test.skip(!healthy, 'Go API on :8080 is not reachable — skipping Sprint 10 import E2E');
    await login(page);
  });

  test('Data import section is on Settings, not Evidence', async ({ page }) => {
    // Evidence tab should NOT have any import-related button.
    await page.goto(`/en/cases/${SPRINT10_CASE_ID}`);
    await page.getByRole('tab', { name: /evidence/i }).click();

    // The only primary upload action on Evidence is "Upload evidence".
    await expect(
      page.getByRole('button', { name: /^upload evidence$/i }),
    ).toBeVisible();

    // No "Bulk upload" or "Import archive" on the Evidence tab.
    await expect(
      page.getByRole('button', { name: /bulk upload/i }),
    ).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: /import archive/i }),
    ).toHaveCount(0);
    await expect(page.getByRole('button', { name: /migrations/i })).toHaveCount(
      0,
    );

    // Now flip to Settings and confirm Data import lives there.
    await gotoCaseSettings(page);
    await expect(page.getByText(/data import/i).first()).toBeVisible();
    await expect(page.getByText(/relativityone export/i).first()).toBeVisible();
    await expect(page.getByText(/drag a \.zip archive/i)).toBeVisible();
  });

  test('Bulk-upload ZIP (no manifest) → bulk success card', async ({ page }) => {
    await gotoCaseSettings(page);

    const zipPath = buildBulkUploadZip();
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles(zipPath);

    // Success card for the bulk path.
    const card = page.getByText(/bulk upload · no hash verification/i);
    await card.waitFor({ state: 'visible', timeout: 15_000 });

    await expect(page.getByText(/imported/i).first()).toBeVisible();
    // Total files and processed counts should both be 2.
    const stats = page.locator('p', { hasText: /^2$/ });
    await expect(stats).toHaveCount(2);
  });

  test('Verified-migration ZIP (manifest.csv at root) → migration success card + TSA', async ({
    page,
  }) => {
    await gotoCaseSettings(page);

    const { zipPath } = buildVerifiedMigrationZip();
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles(zipPath);

    // Success card for the verified migration path.
    const card = page.getByText(/verified migration · hash-bridged/i);
    await card.waitFor({ state: 'visible', timeout: 30_000 });

    // Every matched count assertion — 2 files, all matched.
    await expect(page.getByText(/status/i).first()).toBeVisible();

    // RFC 3161 timestamp label appears inside the success card. The
    // explainer paragraphs also mention "RFC 3161", so target the
    // exact card label ("RFC 3161 timestamp") which only the
    // MigrationResultCard renders.
    await expect(
      page.getByText(/^RFC 3161 timestamp$/).first(),
    ).toBeVisible();

    // Download button is rendered for completed migrations.
    await expect(
      page.getByRole('button', { name: /download attestation pdf/i }),
    ).toBeVisible();
  });

  test('Download attestation PDF triggers a real PDF download', async ({
    page,
  }) => {
    await gotoCaseSettings(page);

    const { zipPath } = buildVerifiedMigrationZip();
    await page.locator('input[type="file"]').setInputFiles(zipPath);
    await page
      .getByText(/verified migration · hash-bridged/i)
      .waitFor({ state: 'visible', timeout: 30_000 });

    const downloadPromise = page.waitForEvent('download');
    await page.getByRole('button', { name: /download attestation pdf/i }).click();
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toMatch(
      /^migration-attestation-[0-9a-f-]+\.pdf$/,
    );
  });

  test('Malformed archive renders an error card', async ({ page }) => {
    await gotoCaseSettings(page);

    const zipPath = buildBadArchive();
    await page.locator('input[type="file"]').setInputFiles(zipPath);

    await page
      .getByText(/import failed/i)
      .waitFor({ state: 'visible', timeout: 10_000 });
    await expect(page.getByRole('button', { name: /dismiss/i })).toBeVisible();
  });
});

/**
 * Smaller spec: verify the Evidence tab is clean (no Sprint 10 buttons)
 * and the single-file upload control is still present. This catches
 * regressions if the Settings reorg accidentally drops the Upload button.
 */
test.describe('Sprint 10 — Evidence tab toolbar regression guard', () => {
  test.beforeEach(async ({ page }) => {
    const healthy = await skipIfBackendUnavailable();
    test.skip(!healthy, 'Go API on :8080 is not reachable — skipping');
    await login(page);
  });

  test('Evidence tab shows only the single-file Upload button', async ({
    page,
  }: {
    page: Page;
  }) => {
    await page.goto(`/en/cases/${SPRINT10_CASE_ID}`);
    await page.getByRole('tab', { name: /evidence/i }).click();
    // Primary action: Upload evidence.
    await expect(
      page.getByRole('button', { name: /^upload evidence$/i }),
    ).toBeVisible();
    // No Sprint 10 import entry points leaking in.
    await expect(
      page.getByRole('button', { name: /bulk upload/i }),
    ).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: /import archive/i }),
    ).toHaveCount(0);
  });
});
