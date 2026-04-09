'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { Mail, Clock, ShieldCheck } from 'lucide-react';
import { PilotRegistrationForm } from '@/components/marketing/forms/pilot-registration-form';

const INFO_ITEMS = [
  { key: 'response', icon: Clock },
  { key: 'security', icon: ShieldCheck },
  { key: 'email', icon: Mail },
] as const;

export function ContactPageContent({ locale }: { locale: string }) {
  const t = useTranslations('marketing.contact');

  return (
    <section className="pt-32 pb-20 md:pt-40 md:pb-28">
      <div className="marketing-section">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 lg:gap-16">
          {/* Left column — info */}
          <div className="lg:col-span-5">
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
                fontSize: 'clamp(2rem, 1.5rem + 2vw, 3rem)',
                letterSpacing: '-0.02em',
              }}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: 0.1 }}
            >
              {t('title')}
            </motion.h1>
            <motion.p
              className="mt-4 leading-relaxed"
              style={{
                color: 'var(--text-secondary)',
                fontSize: 'var(--text-base)',
              }}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: 0.2 }}
            >
              {t('description')}
            </motion.p>

            {/* Info cards */}
            <div className="mt-10 space-y-4">
              {INFO_ITEMS.map((item, i) => {
                const Icon = item.icon;
                return (
                  <motion.div
                    key={item.key}
                    className="flex items-start gap-4 p-4 rounded-xl"
                    style={{
                      backgroundColor: 'var(--bg-elevated)',
                      border: '1px solid var(--border-subtle)',
                    }}
                    initial={{ opacity: 0, x: -12 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{
                      delay: 0.4 + i * 0.1,
                      duration: 0.5,
                      ease: [0.16, 1, 0.3, 1],
                    }}
                  >
                    <div
                      className="flex items-center justify-center w-9 h-9 rounded-lg shrink-0"
                      style={{
                        backgroundColor: 'var(--amber-subtle)',
                      }}
                    >
                      <Icon
                        size={16}
                        style={{ color: 'var(--amber-accent)' }}
                      />
                    </div>
                    <div>
                      <p
                        className="text-sm font-medium"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {t(`info.${item.key}.title`)}
                      </p>
                      <p
                        className="text-xs mt-0.5"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        {t(`info.${item.key}.description`)}
                      </p>
                    </div>
                  </motion.div>
                );
              })}
            </div>
          </div>

          {/* Right column — form */}
          <motion.div
            className="lg:col-span-7"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.3 }}
          >
            <div
              className="p-8 md:p-10 rounded-xl relative"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                boxShadow: 'var(--shadow-md)',
              }}
            >
              <h2
                className="font-[family-name:var(--font-heading)] text-xl mb-6"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('form.heading')}
              </h2>
              <PilotRegistrationForm locale={locale} />
            </div>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
