import { describe, it, expect } from 'vitest';
import en from '@/messages/en.json';
import fr from '@/messages/fr.json';

function getKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = [];
  for (const key of Object.keys(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (typeof obj[key] === 'object' && obj[key] !== null) {
      keys.push(
        ...getKeys(obj[key] as Record<string, unknown>, fullKey),
      );
    } else {
      keys.push(fullKey);
    }
  }
  return keys.sort();
}

describe('Translation completeness', () => {
  const enMarketing = (en as Record<string, unknown>).marketing as Record<string, unknown>;
  const frMarketing = (fr as Record<string, unknown>).marketing as Record<string, unknown>;

  const enKeys = getKeys(enMarketing);
  const frKeys = getKeys(frMarketing);

  it('EN and FR marketing keys are identical', () => {
    expect(enKeys).toEqual(frKeys);
  });

  it('EN has credibilitySignals keys', () => {
    const csKeys = enKeys.filter((k) =>
      k.startsWith('credibilitySignals'),
    );
    expect(csKeys.length).toBeGreaterThanOrEqual(8); // 4 items x (title + description)
  });

  it('FR has credibilitySignals keys', () => {
    const csKeys = frKeys.filter((k) =>
      k.startsWith('credibilitySignals'),
    );
    expect(csKeys.length).toBeGreaterThanOrEqual(8);
  });

  it('EN has openSource keys', () => {
    const osKeys = enKeys.filter((k) => k.startsWith('openSource'));
    expect(osKeys.length).toBeGreaterThanOrEqual(5); // eyebrow, title, description, cta, license
  });

  it('FR has openSource keys', () => {
    const osKeys = frKeys.filter((k) => k.startsWith('openSource'));
    expect(osKeys.length).toBeGreaterThanOrEqual(5);
  });

  it('does not contain socialProof keys', () => {
    expect(enKeys.filter((k) => k.startsWith('socialProof'))).toHaveLength(0);
    expect(frKeys.filter((k) => k.startsWith('socialProof'))).toHaveLength(0);
  });

  it('does not contain stats keys', () => {
    expect(enKeys.filter((k) => k.startsWith('stats'))).toHaveLength(0);
    expect(frKeys.filter((k) => k.startsWith('stats'))).toHaveLength(0);
  });

  it('hero indicators reference Berkeley Protocol, RFC 3161, AGPL', () => {
    const hero = enMarketing.hero as Record<string, string>;
    expect(hero.indicator1).toContain('Berkeley');
    expect(hero.indicator2).toContain('RFC 3161');
    expect(hero.indicator3).toContain('AGPL');
  });

  it('hero description targets documentation teams', () => {
    const hero = enMarketing.hero as Record<string, string>;
    expect(hero.description).toContain('human rights documentation teams');
    expect(hero.description).toContain('legal case builders');
  });

  it('hero description mentions sovereign deployment', () => {
    const hero = enMarketing.hero as Record<string, string>;
    expect(hero.description).toContain('jurisdiction');
  });

  it('CTA badges use honest compliance language', () => {
    const cta = enMarketing.cta as Record<string, string>;
    expect(cta.badge1).toBe('Built for ISO 27001');
    expect(cta.badge2).toBe('SOC 2 Type II roadmap');
    expect(cta.badge3).toBe('GDPR by design');
  });

  it('CTA description does not claim trust', () => {
    const cta = enMarketing.cta as Record<string, string>;
    expect(cta.description).not.toContain('trust VaultKeeper');
    expect(cta.description).not.toContain('Trusted');
  });

  it('FAQ security answer mentions certification roadmap', () => {
    const faq = enMarketing.faq as Record<string, Record<string, string>>;
    expect(faq.security.answer).toContain('Certification roadmap');
    expect(faq.security.answer).not.toContain('We undergo regular');
  });

  it('pricing professional description targets documentation teams', () => {
    const pricing = enMarketing.pricing as Record<string, Record<string, string>>;
    expect(pricing.professional.description).toContain('documentation and legal teams');
    expect(pricing.professional.description).not.toContain('investigation teams');
  });

  it('no translation value is empty string', () => {
    const checkEmpty = (keys: string[], obj: Record<string, unknown>, lang: string) => {
      for (const key of keys) {
        const parts = key.split('.');
        let val: unknown = obj;
        for (const part of parts) {
          val = (val as Record<string, unknown>)?.[part];
        }
        // Allow empty period fields for pricing
        if (key.includes('.period')) continue;
        if (typeof val === 'string' && val.trim() === '') {
          // Skip known empty strings (pricing periods)
          if (!key.includes('period')) {
            throw new Error(`Empty translation: ${lang}.marketing.${key}`);
          }
        }
      }
    };

    expect(() => checkEmpty(enKeys, enMarketing, 'EN')).not.toThrow();
    expect(() => checkEmpty(frKeys, frMarketing, 'FR')).not.toThrow();
  });
});
