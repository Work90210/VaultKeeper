'use client';

import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import {
  Upload,
  Link as LinkIcon,
  ShieldCheck,
  FileOutput,
  Scissors,
  Search,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

const FEATURES: {
  key: string;
  icon: LucideIcon;
  area: string;
}[] = [
  { key: 'intake', icon: Upload, area: 'md:col-span-7 md:row-span-2' },
  { key: 'custody', icon: LinkIcon, area: 'md:col-span-5' },
  { key: 'access', icon: ShieldCheck, area: 'md:col-span-5' },
  { key: 'disclosure', icon: FileOutput, area: 'md:col-span-4' },
  { key: 'redaction', icon: Scissors, area: 'md:col-span-4' },
  { key: 'search', icon: Search, area: 'md:col-span-4' },
];

function FeatureCard({
  icon: Icon,
  title,
  description,
  area,
  index,
  isLarge,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  area: string;
  index: number;
  isLarge: boolean;
}) {
  return (
    <motion.div
      className={`group relative rounded-xl overflow-hidden ${area}`}
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-subtle)',
      }}
      initial={{ opacity: 0, y: 24 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.1 }}
      transition={{
        delay: index * 0.07,
        duration: 0.6,
        ease: [0.16, 1, 0.3, 1],
      }}
      whileHover={{
        borderColor: 'var(--border-strong)',
        y: -2,
        transition: { duration: 0.2 },
      }}
    >
      {/* Subtle top accent on hover */}
      <div
        className="absolute top-0 left-0 right-0 h-px opacity-0 group-hover:opacity-100 transition-opacity duration-300"
        style={{ backgroundColor: 'var(--amber-accent)' }}
      />

      <div className={`relative z-10 ${isLarge ? 'p-10' : 'p-7'}`}>
        {/* Number + icon row */}
        <div className="flex items-center justify-between mb-6">
          <div
            className="flex items-center justify-center w-10 h-10 rounded-lg"
            style={{
              backgroundColor: 'var(--amber-subtle)',
              border: '1px solid oklch(0.750 0.080 75 / 0.15)',
            }}
          >
            <Icon
              size={18}
              strokeWidth={1.5}
              style={{ color: 'var(--amber-accent)' }}
            />
          </div>
          <span
            className="text-[10px] font-bold tracking-[0.2em]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {String(index + 1).padStart(2, '0')}
          </span>
        </div>

        <h3
          className={`font-[family-name:var(--font-heading)] mb-3 ${isLarge ? 'text-2xl' : 'text-lg'}`}
          style={{ color: 'var(--text-primary)', letterSpacing: '-0.01em' }}
        >
          {title}
        </h3>
        <p
          className={`leading-relaxed ${isLarge ? 'text-base max-w-md' : 'text-sm'}`}
          style={{ color: 'var(--text-secondary)' }}
        >
          {description}
        </p>
      </div>
    </motion.div>
  );
}

export function FeaturesSection() {
  const t = useTranslations('marketing.features');

  return (
    <section className="py-20 md:py-28">
      <div className="marketing-section">
        <motion.div
          className="mb-14 max-w-2xl"
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
              fontSize: 'clamp(1.75rem, 1.2rem + 2vw, 3rem)',
              letterSpacing: '-0.02em',
            }}
          >
            {t('title')}
          </h2>
          <p
            className="mt-4"
            style={{
              color: 'var(--text-secondary)',
              fontSize: 'var(--text-base)',
            }}
          >
            {t('subtitle')}
          </p>
        </motion.div>

        <div className="grid grid-cols-1 md:grid-cols-12 gap-4">
          {FEATURES.map((feature, i) => (
            <FeatureCard
              key={feature.key}
              icon={feature.icon}
              title={t(`${feature.key}.title`)}
              description={t(`${feature.key}.description`)}
              area={feature.area}
              index={i}
              isLarge={i === 0}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
