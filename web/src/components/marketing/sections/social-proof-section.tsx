'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

export function SocialProofSection() {
  const t = useTranslations('marketing.socialProof');

  return (
    <section className="py-12 md:py-14">
      <div className="marketing-section">
        <motion.p
          className="text-center text-xs font-medium uppercase tracking-[0.2em]"
          style={{ color: 'var(--text-tertiary)' }}
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true }}
        >
          {t('title')}
        </motion.p>
      </div>
    </section>
  );
}
