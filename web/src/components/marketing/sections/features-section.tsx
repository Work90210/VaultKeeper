'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

const FEATURES = [
  { key: 'intake' },
  { key: 'custody' },
  { key: 'access' },
  { key: 'disclosure' },
  { key: 'redaction' },
  { key: 'search' },
];

export function FeaturesSection() {
  const t = useTranslations('marketing.features');

  return (
    <section className="section">
      <div className="wrap">
        <motion.div
          style={{ marginBottom: '48px', maxWidth: '600px' }}
          initial={{ opacity: 0, y: 16 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6 }}
        >
          <span className="eyebrow">{t('eyebrow')}</span>
          <h2 style={{ marginTop: '20px' }}>{t('title')}</h2>
          <p className="lead" style={{ marginTop: '16px' }}>{t('subtitle')}</p>
        </motion.div>

        {/* Pillar grid — 4 columns on desktop matching the design */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(4, 1fr)',
            gap: '20px',
          }}
          className="pillars-grid max-sm:!grid-cols-1 max-lg:!grid-cols-2"
        >
          {FEATURES.slice(0, 4).map((feature, i) => (
            <motion.div
              key={feature.key}
              className="pillar"
              initial={{ opacity: 0, y: 24 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.08, duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
            >
              <span className="num">{String(i + 1).padStart(2, '0')}</span>
              <h3>{t(`${feature.key}.title`)}</h3>
              <p>{t(`${feature.key}.description`)}</p>
            </motion.div>
          ))}
        </div>

        {/* Remaining features as a 2-column row below */}
        {FEATURES.length > 4 && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              gap: '20px',
              marginTop: '20px',
            }}
            className="pillars-grid-2 max-sm:!grid-cols-1"
          >
            {FEATURES.slice(4).map((feature, i) => (
              <motion.div
                key={feature.key}
                className="pillar"
                initial={{ opacity: 0, y: 24 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: (i + 4) * 0.08, duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
              >
                <span className="num">{String(i + 5).padStart(2, '0')}</span>
                <h3>{t(`${feature.key}.title`)}</h3>
                <p>{t(`${feature.key}.description`)}</p>
              </motion.div>
            ))}
          </div>
        )}

      </div>
    </section>
  );
}
