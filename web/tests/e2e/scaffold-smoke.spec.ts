import { test, expect } from '@playwright/test';

test('app loads at root', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/login/);
});

test('login page renders', async ({ page }) => {
  await page.goto('/en/login');
  await expect(page.getByText('VaultKeeper')).toBeVisible();
  await expect(page.getByText('Sign in with Keycloak')).toBeVisible();
});
