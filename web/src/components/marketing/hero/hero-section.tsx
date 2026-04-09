'use client';

import { useRef } from 'react';
import { motion, useScroll, useTransform } from 'motion/react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { TextureButton } from '@/components/ui/texture-button';

/** Fake product UI — a miniature custody chain timeline */
function CustodyChainVisual() {
  const entries = [
    { action: 'UPLOADED', actor: 'M. Laurent', time: '09:14', hash: 'a7c3…f1e2' },
    { action: 'VERIFIED', actor: 'System', time: '09:14', hash: 'a7c3…f1e2' },
    { action: 'ACCESSED', actor: 'J. Okafor', time: '11:32', hash: '—' },
    { action: 'REDACTED', actor: 'S. Voss', time: '14:07', hash: 'b91d…c4a8' },
    { action: 'DISCLOSED', actor: 'M. Laurent', time: '16:45', hash: 'b91d…c4a8' },
  ];

  return (
    <motion.div
      className="relative w-full max-w-md"
      initial={{ opacity: 0, x: 40 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.9, delay: 0.5, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* Browser chrome */}
      <div
        className="rounded-xl overflow-hidden"
        style={{
          border: '1px solid var(--border-default)',
          boxShadow: 'var(--shadow-xl)',
        }}
      >
        <div
          className="flex items-center gap-2 px-4 py-2.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderBottom: '1px solid var(--border-subtle)',
          }}
        >
          <div className="flex gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: 'var(--stone-600)' }} />
            <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: 'var(--stone-600)' }} />
            <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: 'var(--stone-600)' }} />
          </div>
          <div
            className="flex-1 text-center text-[10px] font-[family-name:var(--font-mono)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            Chain of Custody — EV-2024-0847
          </div>
        </div>

        {/* Content */}
        <div
          className="px-5 py-4 space-y-0"
          style={{ backgroundColor: 'var(--bg-primary)' }}
        >
          {/* Header row */}
          <div
            className="grid grid-cols-[80px_1fr_60px_90px] gap-2 pb-2 mb-1 text-[9px] font-semibold uppercase tracking-[0.15em]"
            style={{ color: 'var(--text-tertiary)', borderBottom: '1px solid var(--border-subtle)' }}
          >
            <span>Action</span>
            <span>Actor</span>
            <span>Time</span>
            <span>Hash</span>
          </div>

          {entries.map((entry, i) => (
            <motion.div
              key={i}
              className="grid grid-cols-[80px_1fr_60px_90px] gap-2 py-2 items-center"
              style={{ borderBottom: i < entries.length - 1 ? '1px solid var(--border-subtle)' : 'none' }}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.8 + i * 0.12, duration: 0.5 }}
            >
              <span
                className="text-[10px] font-semibold uppercase tracking-wider"
                style={{
                  color: entry.action === 'DISCLOSED'
                    ? 'var(--status-active)'
                    : entry.action === 'REDACTED'
                    ? 'var(--amber-accent)'
                    : 'var(--text-secondary)',
                }}
              >
                {entry.action}
              </span>
              <span className="text-xs" style={{ color: 'var(--text-primary)' }}>
                {entry.actor}
              </span>
              <span
                className="text-[10px] font-[family-name:var(--font-mono)]"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {entry.time}
              </span>
              <span
                className="text-[10px] font-[family-name:var(--font-mono)]"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {entry.hash}
              </span>
            </motion.div>
          ))}
        </div>
      </div>

      {/* Floating verification badge */}
      <motion.div
        className="absolute -bottom-4 -left-4 px-3 py-1.5 rounded-lg text-[10px] font-semibold"
        style={{
          backgroundColor: 'var(--status-active-bg)',
          color: 'var(--status-active)',
          border: '1px solid var(--status-active)',
          boxShadow: 'var(--shadow-md)',
        }}
        initial={{ opacity: 0, scale: 0.8 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ delay: 1.6, duration: 0.4, type: 'spring' }}
      >
        INTEGRITY VERIFIED
      </motion.div>
    </motion.div>
  );
}

export function HeroSection({ locale }: { locale: string }) {
  const t = useTranslations('marketing.hero');
  const ref = useRef<HTMLElement>(null);
  const { scrollYProgress } = useScroll({
    target: ref,
    offset: ['start start', 'end start'],
  });
  const opacity = useTransform(scrollYProgress, [0, 0.8], [1, 0]);

  return (
    <section
      ref={ref}
      className="relative min-h-[100svh] flex items-center overflow-hidden"
      style={{ backgroundColor: 'var(--bg-primary)' }}
    >
      {/* Subtle top-edge accent line */}
      <div
        className="absolute top-0 left-0 right-0 h-px"
        style={{ background: 'linear-gradient(90deg, transparent 10%, var(--amber-accent) 50%, transparent 90%)' }}
      />

      <motion.div
        className="marketing-section relative z-10 pt-28 pb-20 md:pt-36 md:pb-28"
        style={{ opacity }}
      >
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-16 lg:gap-20 items-center">
          {/* Left — copy */}
          <div>
            <motion.div
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6 }}
            >
              <span
                className="text-xs font-semibold uppercase tracking-[0.15em]"
                style={{ color: 'var(--amber-accent)' }}
              >
                {t('badge')}
              </span>
            </motion.div>

            <motion.h1
              className="font-[family-name:var(--font-heading)] mt-5 mb-6 leading-[1.08]"
              style={{
                color: 'var(--text-primary)',
                fontSize: 'clamp(2.25rem, 1.5rem + 3.5vw, 4rem)',
                letterSpacing: '-0.025em',
              }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.12 }}
            >
              {t('title')}
            </motion.h1>

            <motion.p
              className="max-w-lg leading-relaxed mb-10"
              style={{
                color: 'var(--text-secondary)',
                fontSize: 'clamp(1rem, 0.9rem + 0.4vw, 1.1875rem)',
              }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.25 }}
            >
              {t('description')}
            </motion.p>

            <motion.div
              className="flex flex-col sm:flex-row items-start gap-4"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.38 }}
            >
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
            </motion.div>

            {/* Trust line — minimal, no dots */}
            <motion.div
              className="flex items-center gap-6 mt-12"
              style={{ color: 'var(--text-tertiary)' }}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.6, delay: 0.6 }}
            >
              {(['indicator1', 'indicator2', 'indicator3'] as const).map((key, i) => (
                <span key={key} className="text-[11px] font-medium uppercase tracking-[0.1em]">
                  {i > 0 && (
                    <span className="mr-6" style={{ color: 'var(--border-default)' }}>
                      /
                    </span>
                  )}
                  {t(key)}
                </span>
              ))}
            </motion.div>
          </div>

          {/* Right — product visual */}
          <div className="hidden lg:flex justify-end">
            <CustodyChainVisual />
          </div>
        </div>
      </motion.div>

      <div className="absolute bottom-0 left-0 right-0">
        <div className="marketing-divider" />
      </div>
    </section>
  );
}
