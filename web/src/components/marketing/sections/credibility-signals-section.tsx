'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

const SIGNALS = ['berkeleyProtocol', 'rfc3161', 'sovereignty', 'auditTrails'] as const;

export function CredibilitySignalsSection() {
  const t = useTranslations('marketing.credibilitySignals');

  return (
    <section className="section-tight">
      <div className="wrap">
        {/* Stats strip — matching the design's 4-cell horizontal bar */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(4, 1fr)',
            border: '1px solid var(--line)',
            borderRadius: 'var(--radius)',
            background: 'var(--paper)',
            overflow: 'hidden',
          }}
          className="stats-grid max-sm:!grid-cols-1 max-md:!grid-cols-2"
        >
          {SIGNALS.map((key, i) => (
            <motion.div
              key={key}
              className="stat-cell"
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.08, duration: 0.5 }}
            >
              <div className="k">{t(`${key}.label`)}</div>
              <div className="v" style={{ fontSize: '28px', marginTop: '8px' }}>
                {t(`${key}.title`)}
              </div>
              <div className="sub" style={{ marginTop: '6px' }}>
                {t(`${key}.description`)}
              </div>
            </motion.div>
          ))}
        </div>

      </div>
    </section>
  );
}
