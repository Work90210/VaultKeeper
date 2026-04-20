'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { ExternalLink } from 'lucide-react';

export function OpenSourceSection() {
  const t = useTranslations('marketing.openSource');

  return (
    <section
      className="py-20 md:py-28"
      style={{ backgroundColor: 'var(--bg-secondary)' }}
    >
      <div className="marketing-section max-w-3xl">
        <motion.div
          className="text-center"
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
            className="mt-4 max-w-xl mx-auto leading-relaxed"
            style={{
              color: 'var(--text-secondary)',
              fontSize: 'var(--text-base)',
            }}
          >
            {t('description')}
          </p>

          <motion.div
            className="mt-8 flex items-center justify-center gap-4"
            initial={{ opacity: 0, y: 12 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.15 }}
          >
            <a
              href="https://github.com/KyleFuehri/VaultKeeper"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-medium transition-colors"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                color: 'var(--text-primary)',
                border: '1px solid var(--border-default)',
              }}
            >
              {t('cta')}
              <ExternalLink size={14} aria-hidden />
            </a>
          </motion.div>

          <p
            className="mt-6 text-xs font-[family-name:var(--font-mono)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {t('license')}
          </p>
        </motion.div>
      </div>
    </section>
  );
}
