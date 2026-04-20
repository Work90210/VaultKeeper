'use client';

import { useRef } from 'react';
import { motion, useScroll, useTransform } from 'motion/react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';

/** Mock case panel — matches the design's hero visual */
function HeroVisual() {
  const cases = [
    { ref: 'ICC-UKR-2024', sub: 'Crimes against humanity, Butcha', status: 'sealed', label: 'sealed' },
    { ref: 'KSC-23-042', sub: 'Witness intimidation, Pristina', status: 'sealed', label: 'sealed' },
    { ref: 'RSCSL-12', sub: 'Sierra Leone residual', status: 'sealed', label: 'sealed' },
    { ref: 'IRMCT-99', sub: 'Archival re-verification', status: 'hold', label: 'hold' },
  ];

  return (
    <motion.div
      className="rise d2"
      style={{
        position: 'relative',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
        background: 'var(--paper)',
        border: '1px solid var(--line)',
        boxShadow: 'var(--shadow-lg)',
      }}
      initial={{ opacity: 0, x: 40 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.9, delay: 0.4, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* Top bar */}
      <div
        style={{
          padding: '20px 22px',
          borderBottom: '1px solid var(--line)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          fontSize: '12px',
          color: 'var(--muted)',
          background: 'color-mix(in srgb, var(--paper) 70%, var(--bg-2))',
        }}
      >
        <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span style={{ width: '7px', height: '7px', background: 'var(--ok)', borderRadius: '50%' }} />
          Chain intact &middot; 48,217 events
        </span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px' }}>
          EV-2024-0847
        </span>
      </div>

      {/* Case rows */}
      <div style={{ padding: '24px 22px 120px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
        {cases.map((c, i) => (
          <motion.div
            key={c.ref}
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '14px 0',
              borderBottom: '1px dashed var(--line-2)',
              fontSize: '14px',
            }}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.7 + i * 0.1, duration: 0.5 }}
          >
            <div>
              <div style={{ fontFamily: 'var(--font-heading)', fontSize: '18px', letterSpacing: '-0.01em' }}>
                {c.ref}
              </div>
              <div style={{ color: 'var(--muted)', fontFamily: 'var(--font-body)', fontSize: '11.5px', marginTop: '3px' }}>
                {c.sub}
              </div>
            </div>
            <span className={`pl ${c.status === 'hold' ? 'hold' : 'sealed'}`}>
              {c.label}
            </span>
          </motion.div>
        ))}

        {/* Chain of custody dots */}
        <div
          style={{
            marginTop: '6px',
            padding: '16px',
            borderRadius: '14px',
            background: 'var(--bg-2)',
            border: '1px solid var(--line)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            {[1, 2, 3, 4, 5, 6].map((n) => (
              <span key={n}>
                <span
                  style={{
                    display: 'inline-grid',
                    placeItems: 'center',
                    width: '22px',
                    height: '22px',
                    borderRadius: '50%',
                    background: n <= 5 ? 'var(--accent)' : 'var(--paper)',
                    border: '1.5px solid var(--accent)',
                    color: n <= 5 ? '#fff' : 'var(--accent)',
                    fontSize: '10px',
                    fontWeight: 500,
                  }}
                >
                  {n}
                </span>
                {n < 6 && (
                  <span
                    style={{
                      display: 'inline-block',
                      width: '12px',
                      height: '2px',
                      background: 'repeating-linear-gradient(to right, var(--accent) 0 4px, transparent 4px 8px)',
                      verticalAlign: 'middle',
                      margin: '0 2px',
                    }}
                  />
                )}
              </span>
            ))}
          </div>
          <div
            style={{
              marginTop: '8px',
              fontFamily: 'var(--font-mono)',
              fontSize: '10.5px',
              color: 'var(--muted)',
              display: 'flex',
              justifyContent: 'space-between',
            }}
          >
            <span>6 / 6 phases signed</span>
            <span>f208...bc91</span>
          </div>
        </div>
      </div>

      {/* Seal stamp */}
      <motion.div
        style={{
          position: 'absolute',
          bottom: '20px',
          right: '20px',
          width: '72px',
          height: '72px',
          borderRadius: '50%',
          border: '1px solid var(--line)',
          display: 'grid',
          placeItems: 'center',
          background: 'var(--paper)',
          textAlign: 'center',
          fontSize: '9px',
          color: 'var(--muted)',
          lineHeight: 1.3,
        }}
        initial={{ opacity: 0, scale: 0.8, rotate: -10 }}
        animate={{ opacity: 1, scale: 1, rotate: 0 }}
        transition={{ delay: 1.4, duration: 0.6, type: 'spring' }}
      >
        <span>
          eIDAS<br />QES<br />
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: '7px' }}>v3.1</span>
        </span>
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
      className="relative overflow-hidden"
      style={{ padding: '56px 0 80px' }}
    >
      {/* Decorative blobs */}
      <div
        style={{
          position: 'absolute',
          width: '520px',
          height: '520px',
          borderRadius: '50%',
          background: 'var(--accent)',
          opacity: 0.06,
          filter: 'blur(100px)',
          right: '-120px',
          top: '-100px',
          pointerEvents: 'none',
        }}
      />

      <motion.div className="wrap" style={{ opacity }}>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1.2fr 0.9fr',
            gap: '64px',
            alignItems: 'center',
            position: 'relative',
            zIndex: 1,
          }}
          className="max-lg:!grid-cols-1 max-lg:!gap-8"
        >
          {/* Left — copy */}
          <div className="rise">
            <motion.span
              className="eyebrow"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5 }}
            >
              {t('badge')}
            </motion.span>

            <motion.h1
              className="font-heading"
              style={{
                marginTop: '24px',
                fontSize: 'clamp(48px, 6.2vw, 98px)',
                fontWeight: 400,
                lineHeight: 1.05,
                letterSpacing: '-0.035em',
              }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.1 }}
            >
              {t('title').split(' ').map((word, i) =>
                word.toLowerCase() === 'government' || word.toLowerCase() === 'evidence' ? (
                  <em key={i}>{word} </em>
                ) : (
                  <span key={i}>{word} </span>
                ),
              )}
            </motion.h1>

            <motion.p
              className="lead"
              style={{
                marginTop: '28px',
                fontSize: '20px',
                color: 'var(--muted)',
                maxWidth: '52ch',
                lineHeight: 1.55,
              }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.2 }}
            >
              {t('description')}
            </motion.p>

            <motion.div
              style={{ display: 'flex', gap: '12px', marginTop: '36px', flexWrap: 'wrap', alignItems: 'center' }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.7, delay: 0.3 }}
            >
              <Link href={`/${locale}/contact`} className="btn">
                {t('primaryCta')} <span className="arr">&rarr;</span>
              </Link>
              <Link href={`/${locale}/features`} className="btn ghost">
                {t('secondaryCta')}
              </Link>
            </motion.div>

            {/* Trust indicators */}
            <motion.div
              style={{
                display: 'flex',
                gap: '14px',
                marginTop: '18px',
                fontSize: '13px',
                color: 'var(--muted)',
                flexWrap: 'wrap',
              }}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.6, delay: 0.5 }}
            >
              {(['indicator1', 'indicator2', 'indicator3'] as const).map((key) => (
                <span key={key} style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                  <span
                    style={{
                      width: '5px',
                      height: '5px',
                      borderRadius: '50%',
                      background: 'var(--ok)',
                    }}
                  />
                  {t(key)}
                </span>
              ))}
            </motion.div>
          </div>

          {/* Right — product visual */}
          <div className="hidden lg:block">
            <HeroVisual />
          </div>
        </div>
      </motion.div>

    </section>
  );
}
