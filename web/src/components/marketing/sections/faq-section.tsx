'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useTranslations } from 'next-intl';
import { ChevronDown } from 'lucide-react';

const FAQ_KEYS = [
  'sovereignty',
  'security',
  'integration',
  'pilot',
  'pricing',
  'languages',
  'training',
  'migration',
] as const;

function FaqItem({
  id,
  question,
  answer,
  isOpen,
  onToggle,
}: {
  id: string;
  question: string;
  answer: string;
  isOpen: boolean;
  onToggle: () => void;
}) {
  const triggerId = `faq-trigger-${id}`;
  const panelId = `faq-panel-${id}`;
  return (
    <div
      className="rounded-xl overflow-hidden transition-colors"
      style={{
        border: `1px solid ${isOpen ? 'var(--border-strong)' : 'var(--border-subtle)'}`,
        backgroundColor: isOpen ? 'var(--bg-elevated)' : 'transparent',
      }}
    >
      <button
        type="button"
        id={triggerId}
        className="w-full flex items-center justify-between p-6 text-left"
        onClick={onToggle}
        aria-expanded={isOpen}
        aria-controls={panelId}
      >
        <span
          className="font-medium pr-4"
          style={{
            color: isOpen ? 'var(--text-primary)' : 'var(--text-secondary)',
            fontSize: 'var(--text-base)',
          }}
        >
          {question}
        </span>
        <motion.div
          animate={{ rotate: isOpen ? 180 : 0 }}
          transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
          className="shrink-0"
        >
          <ChevronDown
            size={18}
            style={{
              color: isOpen ? 'var(--amber-accent)' : 'var(--text-tertiary)',
            }}
          />
        </motion.div>
      </button>
      <AnimatePresence initial={false}>
        {isOpen && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
          >
            <div
              id={panelId}
              role="region"
              aria-labelledby={triggerId}
              className="px-6 pb-6 text-sm leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {answer}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export function FaqSection() {
  const t = useTranslations('marketing.faq');
  const [openKeys, setOpenKeys] = useState<Set<string>>(
    () => new Set(['sovereignty', 'security', 'pilot']),
  );

  return (
    <section className="section">
      <div className="wrap" style={{ maxWidth: '780px' }}>
        <motion.div
          style={{ marginBottom: '48px' }}
          initial={{ opacity: 0, y: 16 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6 }}
        >
          <span className="eyebrow">{t('eyebrow')}</span>
          <h2 style={{ marginTop: '20px' }}>{t('title')}</h2>
        </motion.div>

        <div className="space-y-3">
          {FAQ_KEYS.map((key, i) => (
            <motion.div
              key={key}
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, amount: 0 }}
              transition={{
                delay: i * 0.05,
                duration: 0.5,
                ease: [0.16, 1, 0.3, 1],
              }}
            >
              <FaqItem
                id={key}
                question={t(`${key}.question`)}
                answer={t(`${key}.answer`)}
                isOpen={openKeys.has(key)}
                onToggle={() => {
                  const next = new Set(openKeys);
                  if (next.has(key)) {
                    next.delete(key);
                  } else {
                    next.add(key);
                  }
                  setOpenKeys(next);
                }}
              />
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
