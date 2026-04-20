'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { Shield, Eye, Scale, Lock } from 'lucide-react';

const PRINCIPLES = [
  { key: 'sovereignty', icon: Shield },
  { key: 'transparency', icon: Eye },
  { key: 'integrity', icon: Scale },
  { key: 'security', icon: Lock },
] as const;

export function AboutPageContent() {
  const t = useTranslations('marketing.aboutPage');

  return (
    <>
      {/* Hero */}
      <section className="pt-32 pb-16 md:pt-40 md:pb-20">
        <div className="marketing-section">
          <div className="max-w-3xl">
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
              className="mt-6 text-balance leading-relaxed"
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
        </div>
      </section>

      {/* Mission statement */}
      <section
        className="py-16 md:py-24"
        style={{ backgroundColor: 'var(--bg-secondary)' }}
      >
        <div className="marketing-section">
          <div className="max-w-3xl mx-auto text-center">
            <blockquote
              className="font-[family-name:var(--font-heading)] text-balance leading-snug"
              style={{
                color: 'var(--text-primary)',
                fontSize: 'clamp(1.25rem, 1rem + 1.5vw, 2rem)',
                letterSpacing: '-0.01em',
              }}
            >
              &ldquo;{t('mission')}&rdquo;
            </blockquote>
          </div>
        </div>
      </section>

      {/* Principles */}
      <section className="py-20 md:py-28">
        <div className="marketing-section">
          <motion.div
            className="text-center mb-16"
            initial={{ opacity: 0, y: 16 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6 }}
          >
            <h2
              className="font-[family-name:var(--font-heading)] text-balance"
              style={{
                color: 'var(--text-primary)',
                fontSize: 'clamp(1.75rem, 1.2rem + 2vw, 3rem)',
                letterSpacing: '-0.02em',
              }}
            >
              {t('principlesTitle')}
            </h2>
          </motion.div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-4xl mx-auto">
            {PRINCIPLES.map((principle, i) => {
              const Icon = principle.icon;
              return (
                <motion.div
                  key={principle.key}
                  className="group p-8 rounded-xl"
                  style={{
                    backgroundColor: 'var(--bg-elevated)',
                    border: '1px solid var(--border-subtle)',
                  }}
                  initial={{ opacity: 0, y: 20 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  viewport={{ once: true }}
                  transition={{
                    delay: i * 0.1,
                    duration: 0.6,
                    ease: [0.16, 1, 0.3, 1],
                  }}
                  whileHover={{
                    borderColor: 'var(--border-strong)',
                    boxShadow: 'var(--shadow-md)',
                    y: -2,
                  }}
                >
                  <div
                    className="flex items-center justify-center w-11 h-11 rounded-xl mb-5"
                    style={{
                      backgroundColor: 'var(--amber-subtle)',
                      border: '1px solid oklch(0.750 0.080 75 / 0.2)',
                    }}
                  >
                    <Icon
                      size={20}
                      strokeWidth={1.5}
                      style={{ color: 'var(--amber-accent)' }}
                    />
                  </div>
                  <h3
                    className="font-[family-name:var(--font-heading)] text-lg mb-2"
                    style={{ color: 'var(--text-primary)' }}
                  >
                    {t(`principles.${principle.key}.title`)}
                  </h3>
                  <p
                    className="text-sm leading-relaxed"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t(`principles.${principle.key}.description`)}
                  </p>
                </motion.div>
              );
            })}
          </div>
        </div>
      </section>
    </>
  );
}
