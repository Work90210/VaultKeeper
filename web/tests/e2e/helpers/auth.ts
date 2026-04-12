import type { Page } from '@playwright/test';

/**
 * Default Sprint 10 test user. Must exist in the local Keycloak realm
 * with the `system_admin` role assigned. Created via:
 *
 *   POST /admin/realms/vaultkeeper/users
 *   { username: 'sprint10-tester',
 *     credentials: [{ type: 'password', value: SPRINT10_PASSWORD }] }
 *
 * then the `system_admin` realm role is mapped onto the user.
 */
export const SPRINT10_USERNAME = 'sprint10-tester';
export const SPRINT10_PASSWORD = 'SprintTen-TestPass-2026!';

/**
 * Log the given user into VaultKeeper via the Keycloak OIDC flow.
 * Leaves the browser on the case-list page. Skips if the user is
 * already authenticated.
 */
export async function login(
  page: Page,
  username = SPRINT10_USERNAME,
  password = SPRINT10_PASSWORD,
): Promise<void> {
  await page.goto('/en/cases');

  // Already signed in — cookie-backed session redirected us to the
  // cases list instead of the Keycloak login page.
  if (/\/en\/cases(\?|$)/.test(page.url())) {
    return;
  }

  // Keycloak login form.
  await page.getByLabel(/username or email/i).fill(username);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole('button', { name: /sign in/i }).click();

  await page.waitForURL(/\/en\/cases/);
}
