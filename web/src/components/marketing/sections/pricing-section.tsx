'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { Check } from 'lucide-react';

const TIERS = ['pilot', 'professional', 'enterprise'] as const;

export function PricingSection({ locale }: { locale: string }) {
  const t = useTranslations('marketing.pricing');

  return (
    <section className="py-20 md:py-28">
      <div className="marketing-section">
        <motion.div
          className="text-center mb-16"
          initial={{ opacity: 0, y: 16 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6 }}
        >
          <span
            className="text-xs font-semibold uppercase tracking-[0.15em]"
            style={{ color: 'var(--amber-accent)' }}
          >
            {t('eyebrow')}
          </span>
          <h2
            className="font-[family-name:var(--font-heading)] mt-4 text-balance"
            style={{
              color: 'var(--text-primary)',
              fontSize: 'clamp(1.75rem, 1.2rem + 2vw, 3rem)',
              letterSpacing: '-0.02em',
            }}
          >
            {t('title')}
          </h2>
          <p
            className="mt-4 max-w-lg mx-auto"
            style={{
              color: 'var(--text-secondary)',
              fontSize: 'var(--text-base)',
            }}
          >
            {t('subtitle')}
          </p>
        </motion.div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
          {TIERS.map((tier, i) => {
            const isMiddle = i === 1;
            return (
              <motion.div
                key={tier}
                className="relative flex flex-col rounded-xl overflow-hidden"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: isMiddle
                    ? '2px solid var(--amber-accent)'
                    : '1px solid var(--border-subtle)',
                  boxShadow: isMiddle ? 'var(--shadow-lg)' : 'var(--shadow-xs)',
                }}
                initial={{ opacity: 0, y: 24 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{
                  delay: i * 0.1,
                  duration: 0.6,
                  ease: [0.16, 1, 0.3, 1],
                }}
                whileHover={{ y: -4, boxShadow: 'var(--shadow-xl)' }}
              >
                {/* Popular badge */}
                {isMiddle && (
                  <div
                    className="text-center py-2 text-xs font-semibold uppercase tracking-wider"
                    style={{
                      backgroundColor: 'var(--amber-accent)',
                      color: 'var(--stone-950)',
                    }}
                  >
                    {t('popular')}
                  </div>
                )}

                <div className="p-8 flex-1 flex flex-col">
                  <h3
                    className="font-[family-name:var(--font-heading)] text-xl mb-1"
                    style={{ color: 'var(--text-primary)' }}
                  >
                    {t(`${tier}.name`)}
                  </h3>
                  <p
                    className="text-sm mb-6"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t(`${tier}.description`)}
                  </p>

                  <div className="mb-8">
                    <span
                      className="font-[family-name:var(--font-heading)] text-3xl"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {t(`${tier}.price`)}
                    </span>
                    {tier !== 'enterprise' && (
                      <span
                        className="text-sm ml-1"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        {t(`${tier}.period`)}
                      </span>
                    )}
                  </div>

                  {/* Features list */}
                  <ul className="space-y-3 mb-8 flex-1">
                    {([1, 2, 3, 4, 5] as const).map((n) => {
                      const featureKey = `${tier}.feature${n}`;
                      return (
                        <li key={n} className="flex items-start gap-3">
                          <Check
                            size={16}
                            className="mt-0.5 shrink-0"
                            style={{ color: 'var(--amber-accent)' }}
                          />
                          <span
                            className="text-sm"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            {t(featureKey)}
                          </span>
                        </li>
                      );
                    })}
                  </ul>

                  <Link
                    href={`/${locale}/contact`}
                    className={
                      isMiddle
                        ? 'btn-marketing-primary text-center'
                        : 'btn-marketing-secondary text-center'
                    }
                  >
                    {t(`${tier}.cta`)}
                  </Link>
                </div>
              </motion.div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
