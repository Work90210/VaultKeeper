'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

export function PricingPageHero() {
  const t = useTranslations('marketing.pricingPage');

  return (
    <section className="pt-32 pb-8 md:pt-40 md:pb-12">
      <div className="marketing-section text-center">
        <motion.span
          className="text-xs font-semibold uppercase tracking-[0.15em]"
          style={{ color: 'var(--amber-accent)' }}
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
        >
          {t('eyebrow')}
        </motion.span>
        <motion.h1
          className="font-[family-name:var(--font-heading)] mt-4 text-balance"
          style={{
            color: 'var(--text-primary)',
            fontSize: 'clamp(2rem, 1.5rem + 2.5vw, 3.5rem)',
            letterSpacing: '-0.02em',
          }}
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.1 }}
        >
          {t('title')}
        </motion.h1>
        <motion.p
          className="mt-4 max-w-xl mx-auto"
          style={{
            color: 'var(--text-secondary)',
            fontSize: 'clamp(1rem, 0.9rem + 0.4vw, 1.1875rem)',
          }}
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
        >
          {t('description')}
        </motion.p>
      </div>
    </section>
  );
}
