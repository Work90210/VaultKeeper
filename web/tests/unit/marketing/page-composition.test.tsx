import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';

describe('Landing page composition', () => {
  const pageSource = readFileSync(
    resolve(__dirname, '../../../src/app/[locale]/(marketing)/page.tsx'),
    'utf-8',
  );

  it('does not import SocialProofSection', () => {
    expect(pageSource).not.toContain('SocialProofSection');
    expect(pageSource).not.toContain('social-proof-section');
  });

  it('does not import StatsSection', () => {
    expect(pageSource).not.toContain('StatsSection');
    expect(pageSource).not.toContain('stats-section');
  });

  it('imports CredibilitySignalsSection', () => {
    expect(pageSource).toContain('CredibilitySignalsSection');
    expect(pageSource).toContain('credibility-signals-section');
  });

  it('imports OpenSourceSection', () => {
    expect(pageSource).toContain('OpenSourceSection');
    expect(pageSource).toContain('open-source-section');
  });

  it('renders sections in correct order', () => {
    const heroIdx = pageSource.indexOf('<HeroSection');
    const credIdx = pageSource.indexOf('<CredibilitySignalsSection');
    const featIdx = pageSource.indexOf('<FeaturesSection');
    const howIdx = pageSource.indexOf('<HowItWorksSection');
    const osIdx = pageSource.indexOf('<OpenSourceSection');
    const faqIdx = pageSource.indexOf('<FaqSection');
    const ctaIdx = pageSource.indexOf('<CtaSection');

    expect(heroIdx).toBeLessThan(credIdx);
    expect(credIdx).toBeLessThan(featIdx);
    expect(featIdx).toBeLessThan(howIdx);
    expect(howIdx).toBeLessThan(osIdx);
    expect(osIdx).toBeLessThan(faqIdx);
    expect(faqIdx).toBeLessThan(ctaIdx);
  });

  it('metadata description targets documentation teams', () => {
    expect(pageSource).toContain('human rights documentation teams');
  });
});

describe('Pricing page metadata', () => {
  const source = readFileSync(
    resolve(
      __dirname,
      '../../../src/app/[locale]/(marketing)/pricing/page.tsx',
    ),
    'utf-8',
  );

  it('targets documentation teams in description', () => {
    expect(source).toContain('human rights documentation teams');
    expect(source).not.toContain('investigation teams of every size');
  });
});

describe('Contact page metadata', () => {
  const source = readFileSync(
    resolve(
      __dirname,
      '../../../src/app/[locale]/(marketing)/contact/page.tsx',
    ),
    'utf-8',
  );

  it('targets documentation teams in description', () => {
    expect(source).toContain('human rights documentation teams');
    expect(source).not.toContain('Join investigation teams');
  });
});

describe('Deleted files do not exist', () => {
  const files = [
    'src/components/marketing/sections/stats-section.tsx',
    'src/components/marketing/sections/social-proof-section.tsx',
    'src/components/ui/animated-number.tsx',
  ];

  for (const file of files) {
    it(`${file} is deleted`, () => {
      const exists = (() => {
        try {
          readFileSync(resolve(__dirname, '../../../', file));
          return true;
        } catch {
          return false;
        }
      })();
      expect(exists).toBe(false);
    });
  }
});
