'use client';

import { useRef, useState } from 'react';
import { motion, useInView } from 'motion/react';
import { useTranslations } from 'next-intl';
import { AnimatedNumber } from '@/components/ui/animated-number';

const STATS = [
  { key: 'evidenceItems', value: 2400000, suffix: '+' },
  { key: 'jurisdictions', value: 34, suffix: '' },
  { key: 'uptime', value: 99.99, suffix: '%', precision: 2 },
  { key: 'auditEvents', value: 18000000, suffix: '+' },
] as const;

export function StatsSection() {
  const t = useTranslations('marketing.stats');
  const ref = useRef<HTMLDivElement>(null);
  const isInView = useInView(ref, { once: true, margin: '-50px' });
  const [triggered, setTriggered] = useState(false);

  if (isInView && !triggered) {
    setTriggered(true);
  }

  return (
    <section
      className="py-16 md:py-20"
      ref={ref}
      style={{ borderTop: '1px solid var(--border-subtle)', borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="marketing-section">
        <div className="grid grid-cols-2 lg:grid-cols-4">
          {STATS.map((stat, i) => (
            <motion.div
              key={stat.key}
              className="text-center py-6 lg:py-0"
              style={{
                borderRight: i < 3 ? '1px solid var(--border-subtle)' : 'none',
              }}
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, amount: 0 }}
              transition={{ delay: i * 0.08, duration: 0.5 }}
            >
              <div
                className="font-[family-name:var(--font-heading)] text-3xl md:text-4xl mb-1"
                style={{ color: 'var(--text-primary)' }}
              >
                {triggered ? (
                  <>
                    <AnimatedNumber
                      value={stat.value}
                      precision={stat.key === 'uptime' ? 2 : 0}
                      stiffness={50}
                      damping={20}
                    />
                    {stat.suffix}
                  </>
                ) : (
                  <span style={{ color: 'var(--text-tertiary)' }}>0</span>
                )}
              </div>
              <p
                className="text-[10px] font-semibold uppercase tracking-[0.15em]"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {t(`${stat.key}Label`)}
              </p>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
