'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { Upload, FolderOpen, Users, Gavel } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

const STEPS: { key: string; icon: LucideIcon }[] = [
  { key: 'ingest', icon: Upload },
  { key: 'organize', icon: FolderOpen },
  { key: 'collaborate', icon: Users },
  { key: 'disclose', icon: Gavel },
];

export function HowItWorksSection() {
  const t = useTranslations('marketing.howItWorks');

  return (
    <section
      className="py-20 md:py-28"
      style={{ backgroundColor: 'var(--bg-secondary)' }}
    >
      <div className="marketing-section">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-16">
          {/* Left — heading (sticky on desktop) */}
          <div className="lg:col-span-4">
            <div className="lg:sticky lg:top-28">
              <motion.div
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
                  className="font-[family-name:var(--font-heading)] mt-4"
                  style={{
                    color: 'var(--text-primary)',
                    fontSize: 'clamp(1.75rem, 1.2rem + 2vw, 2.75rem)',
                    letterSpacing: '-0.02em',
                  }}
                >
                  {t('title')}
                </h2>
              </motion.div>
            </div>
          </div>

          {/* Right — steps as vertical timeline */}
          <div className="lg:col-span-8">
            <div className="relative">
              {/* Vertical line */}
              <div
                className="absolute left-5 top-0 bottom-0 w-px hidden md:block"
                style={{ backgroundColor: 'var(--border-default)' }}
              />

              {STEPS.map((step, i) => {
                const Icon = step.icon;
                return (
                  <motion.div
                    key={step.key}
                    className="relative flex gap-8 md:gap-12 pb-14 last:pb-0"
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true, amount: 0.3 }}
                    transition={{
                      delay: i * 0.1,
                      duration: 0.6,
                      ease: [0.16, 1, 0.3, 1],
                    }}
                  >
                    {/* Step marker */}
                    <div className="relative z-10 shrink-0">
                      <div
                        className="flex items-center justify-center w-10 h-10 rounded-lg"
                        style={{
                          backgroundColor: 'var(--bg-elevated)',
                          border: '1px solid var(--border-default)',
                        }}
                      >
                        <Icon
                          size={18}
                          strokeWidth={1.5}
                          style={{ color: 'var(--amber-accent)' }}
                        />
                      </div>
                    </div>

                    {/* Content */}
                    <div className="pt-1">
                      <span
                        className="text-[10px] font-bold tracking-[0.2em] uppercase"
                        style={{ color: 'var(--amber-muted)' }}
                      >
                        Step {String(i + 1).padStart(2, '0')}
                      </span>
                      <h3
                        className="font-[family-name:var(--font-heading)] text-lg mt-1 mb-2"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {t(`${step.key}.title`)}
                      </h3>
                      <p
                        className="text-sm leading-relaxed max-w-md"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {t(`${step.key}.description`)}
                      </p>
                    </div>
                  </motion.div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
