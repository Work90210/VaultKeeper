'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';

export function CtaSection({ locale }: { locale: string }) {
  const t = useTranslations('marketing.cta');

  return (
    <section className="section">
      <div className="wrap">
        <motion.div
          className="cta-banner"
          style={{
            display: 'grid',
            gridTemplateColumns: '1.4fr auto',
            gap: '40px',
            alignItems: 'center',
          }}
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.7 }}
        >
          <div>
            <h2
              style={{
                fontSize: 'clamp(32px, 4vw, 52px)',
                lineHeight: 1.1,
              }}
            >
              {t('title')}
            </h2>
            <p
              style={{
                color: 'var(--muted-2)',
                fontSize: '16px',
                marginTop: '16px',
                maxWidth: '48ch',
                lineHeight: 1.55,
              }}
            >
              {t('description')}
            </p>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <Link
              href={`/${locale}/contact`}
              className="btn"
              style={{ background: 'var(--bg)', color: 'var(--ink)' }}
            >
              {t('primaryCta')} <span className="arr">&rarr;</span>
            </Link>
            <Link
              href={`/${locale}/features`}
              className="btn ghost"
              style={{ color: 'var(--bg)', borderColor: 'rgba(255,255,255,0.2)' }}
            >
              {t('secondaryCta')}
            </Link>
          </div>
        </motion.div>
      </div>
    </section>
  );
}
