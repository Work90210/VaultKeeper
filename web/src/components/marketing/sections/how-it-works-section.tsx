'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

const STEPS = ['ingest', 'organize', 'collaborate', 'disclose'] as const;

export function HowItWorksSection() {
  const t = useTranslations('marketing.howItWorks');

  return (
    <section className="section">
      <div className="wrap">
        <motion.div
          className="how"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.7 }}
        >
          <div style={{ maxWidth: '600px' }}>
            <span className="eyebrow">{t('eyebrow')}</span>
            <h2 style={{ marginTop: '20px' }}>
              {t('title')}
            </h2>
          </div>

          <div className="how-steps">
            {STEPS.map((step, i) => (
              <motion.div
                key={step}
                className="how-step"
                initial={{ opacity: 0, y: 16 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1, duration: 0.5 }}
              >
                <h4>{t(`${step}.title`)}</h4>
                <p>{t(`${step}.description`)}</p>
              </motion.div>
            ))}
          </div>
        </motion.div>
      </div>
    </section>
  );
}
