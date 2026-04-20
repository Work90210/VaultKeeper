import { test, expect } from '@playwright/test';

test.describe('Landing page — credibility overhaul', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/en');
  });

  test('landing page loads without errors', async ({ page }) => {
    await expect(page).toHaveURL(/\/en/);
    await expect(page.locator('body')).toBeVisible();
  });

  test('hero shows Berkeley Protocol-aligned indicator', async ({ page }) => {
    await expect(page.getByText('Berkeley Protocol-aligned')).toBeVisible();
  });

  test('hero shows RFC 3161 indicator', async ({ page }) => {
    await expect(page.getByText('RFC 3161 trusted timestamps')).toBeVisible();
  });

  test('hero shows AGPL-3.0 indicator', async ({ page }) => {
    await expect(page.getByText('AGPL-3.0 open source')).toBeVisible();
  });

  test('hero targets documentation teams', async ({ page }) => {
    await expect(
      page.getByText('human rights documentation teams'),
    ).toBeVisible();
  });

  test('hero mentions sovereign deployment', async ({ page }) => {
    await expect(
      page.getByText('Supports deployment in your jurisdiction'),
    ).toBeVisible();
  });

  test('no fake stats section visible', async ({ page }) => {
    await expect(page.getByText('Evidence items managed')).not.toBeVisible();
    await expect(page.getByText('Jurisdictions served')).not.toBeVisible();
    await expect(page.getByText('Audit events recorded')).not.toBeVisible();
  });

  test('no "Trusted by" social proof', async ({ page }) => {
    await expect(
      page.getByText('Trusted by investigation teams worldwide'),
    ).not.toBeVisible();
  });

  test('credibility signals section visible', async ({ page }) => {
    await expect(
      page.getByText('Berkeley Protocol-aligned').first(),
    ).toBeVisible();
    await expect(page.getByText('Sovereign deployment')).toBeVisible();
    await expect(page.getByText('Immutable audit trails')).toBeVisible();
  });

  test('open source section visible', async ({ page }) => {
    await expect(page.getByText('Built in the open')).toBeVisible();
    await expect(page.getByText('AGPL-3.0')).toBeVisible();
  });

  test('GitHub link exists and points to correct repo', async ({ page }) => {
    const link = page.getByRole('link', { name: /View on GitHub/i });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute(
      'href',
      'https://github.com/KyleFuehri/VaultKeeper',
    );
  });

  test('CTA badges use honest compliance language', async ({ page }) => {
    await expect(page.getByText('Built for ISO 27001')).toBeVisible();
    await expect(page.getByText('SOC 2 Type II roadmap')).toBeVisible();
    await expect(page.getByText('GDPR by design')).toBeVisible();
  });

  test('CTA does not claim trust', async ({ page }) => {
    await expect(
      page.getByText('trust VaultKeeper'),
    ).not.toBeVisible();
  });

  test('FAQ accordion has multiple items open by default', async ({
    page,
  }) => {
    // Sovereignty, security, and pilot should be open
    const sovereigntyButton = page.getByRole('button', {
      name: /Where is my data stored/i,
    });
    await expect(sovereigntyButton).toHaveAttribute('aria-expanded', 'true');

    const securityButton = page.getByRole('button', {
      name: /security certifications/i,
    });
    await expect(securityButton).toHaveAttribute('aria-expanded', 'true');

    const pilotButton = page.getByRole('button', {
      name: /pilot program include/i,
    });
    await expect(pilotButton).toHaveAttribute('aria-expanded', 'true');
  });

  test('FAQ accordion can toggle items', async ({ page }) => {
    const button = page.getByRole('button', {
      name: /Where is my data stored/i,
    });
    await expect(button).toHaveAttribute('aria-expanded', 'true');

    await button.click();
    await expect(button).toHaveAttribute('aria-expanded', 'false');

    await button.click();
    await expect(button).toHaveAttribute('aria-expanded', 'true');
  });

  test('FAQ security answer uses honest language', async ({ page }) => {
    await expect(
      page.getByText('Certification roadmap available on request'),
    ).toBeVisible();
  });
});

test.describe('Features page CTA', () => {
  test('shows honest compliance badges', async ({ page }) => {
    await page.goto('/en/features');
    await expect(page.getByText('Built for ISO 27001')).toBeVisible();
    await expect(page.getByText('SOC 2 Type II roadmap')).toBeVisible();
  });
});

test.describe('Pricing page', () => {
  test('shows honest compliance badges', async ({ page }) => {
    await page.goto('/en/pricing');
    await expect(page.getByText('Built for ISO 27001')).toBeVisible();
    await expect(page.getByText('GDPR by design')).toBeVisible();
  });

  test('pricing professional targets documentation teams', async ({
    page,
  }) => {
    await page.goto('/en/pricing');
    await expect(
      page.getByText('documentation and legal teams', { exact: false }),
    ).toBeVisible();
  });
});

test.describe('Pilot registration', () => {
  test('contact page loads', async ({ page }) => {
    await page.goto('/en/contact');
    await expect(
      page.getByText('Register for pilot access'),
    ).toBeVisible();
  });

  test('form has all required fields', async ({ page }) => {
    await page.goto('/en/contact');
    await expect(page.getByPlaceholder('Your full name')).toBeVisible();
    await expect(
      page.getByPlaceholder('you@organization.gov'),
    ).toBeVisible();
    await expect(
      page.getByPlaceholder('Your organization or institution'),
    ).toBeVisible();
  });

  test('form has honeypot hidden from view', async ({ page }) => {
    await page.goto('/en/contact');
    const honeypot = page.locator('input[name="honeypot"]');
    await expect(honeypot).toBeHidden();
  });
});

test.describe('French locale', () => {
  test('landing page renders in French', async ({ page }) => {
    await page.goto('/fr');
    await expect(
      page.getByText('Conforme au Protocole de Berkeley'),
    ).toBeVisible();
  });

  test('French CTA badges use correct translations', async ({ page }) => {
    await page.goto('/fr');
    await expect(page.getByText('Conçu pour ISO 27001')).toBeVisible();
    await expect(
      page.getByText('Feuille de route SOC 2 Type II'),
    ).toBeVisible();
    await expect(page.getByText('RGPD par conception')).toBeVisible();
  });

  test('French open source section visible', async ({ page }) => {
    await page.goto('/fr');
    await expect(
      page.getByText('Construit en toute transparence'),
    ).toBeVisible();
  });
});
