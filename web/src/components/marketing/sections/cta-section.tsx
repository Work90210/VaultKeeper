'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { TextureButton } from '@/components/ui/texture-button';

export function CtaSection({ locale }: { locale: string }) {
  const t = useTranslations('marketing.cta');

  return (
    <section className="py-20 md:py-28">
      <div className="marketing-section max-w-3xl text-center">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.7 }}
        >
          <h2
            className="font-[family-name:var(--font-heading)] text-balance mb-5"
            style={{
              color: 'var(--text-primary)',
              fontSize: 'clamp(1.75rem, 1.2rem + 2vw, 2.75rem)',
              letterSpacing: '-0.02em',
            }}
          >
            {t('title')}
          </h2>
          <p
            className="max-w-lg mx-auto mb-10"
            style={{
              color: 'var(--text-secondary)',
              fontSize: 'var(--text-base)',
            }}
          >
            {t('description')}
          </p>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link href={`/${locale}/contact`}>
              <TextureButton variant="primary" size="lg">
                {t('primaryCta')}
                <ArrowRight size={16} />
              </TextureButton>
            </Link>
            <Link href={`/${locale}/features`}>
              <TextureButton variant="minimal" size="lg">
                {t('secondaryCta')}
              </TextureButton>
            </Link>
          </div>

          {/* Trust line */}
          <div className="flex items-center justify-center gap-6 mt-12">
            {(['badge1', 'badge2', 'badge3'] as const).map((key, i) => (
              <span
                key={key}
                className="text-[10px] font-medium uppercase tracking-[0.12em]"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {i > 0 && (
                  <span className="mr-6" style={{ color: 'var(--border-default)' }}>
                    /
                  </span>
                )}
                {t(key)}
              </span>
            ))}
          </div>
        </motion.div>
      </div>
    </section>
  );
}
