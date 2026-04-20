'use client';

import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { motion, AnimatePresence } from 'motion/react';
import { useTranslations } from 'next-intl';
import {
  Send,
  CheckCircle2,
  AlertCircle,
  Loader2,
} from 'lucide-react';
import {
  pilotRegistrationSchema,
  ROLE_OPTIONS,
  type PilotRegistrationFormData,
} from './pilot-registration-schema';

export function PilotRegistrationForm({ locale }: { locale: string }) {
  const t = useTranslations('marketing.contact.form');
  const [submitted, setSubmitted] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<PilotRegistrationFormData>({
    resolver: zodResolver(pilotRegistrationSchema),
    defaultValues: {
      name: '',
      email: '',
      organization: '',
      role: 'investigator',
      message: '',
      locale: locale as 'en' | 'fr',
      honeypot: '',
    },
  });

  async function onSubmit(data: PilotRegistrationFormData) {
    setServerError(null);

    if (data.honeypot) return;

    try {
      const response = await fetch('/api/pilot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: data.name,
          email: data.email,
          organization: data.organization,
          role: data.role,
          message: data.message,
          locale: data.locale,
        }),
      });

      if (!response.ok) {
        await response.json().catch(() => ({}));
        const safeMessages: Record<number, string> = {
          429: 'Too many requests. Please try again later.',
          400: 'Please check your input and try again.',
        };
        throw new Error(
          safeMessages[response.status] || 'Submission failed. Please try again.',
        );
      }

      setSubmitted(true);
    } catch (err) {
      setServerError(
        err instanceof Error ? err.message : 'An unexpected error occurred'
      );
    }
  }

  if (submitted) {
    return (
      <motion.div
        className="p-12 rounded-xl text-center"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
        }}
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
      >
        <div
          className="flex items-center justify-center w-16 h-16 rounded-full mx-auto mb-6"
          style={{
            backgroundColor: 'var(--status-active-bg)',
          }}
        >
          <CheckCircle2
            size={32}
            style={{ color: 'var(--status-active)' }}
          />
        </div>
        <h3
          className="font-[family-name:var(--font-heading)] text-xl mb-2"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('successTitle')}
        </h3>
        <p
          className="text-sm max-w-sm mx-auto"
          style={{ color: 'var(--text-secondary)' }}
        >
          {t('successDescription')}
        </p>
      </motion.div>
    );
  }

  return (
    <form
      onSubmit={handleSubmit(onSubmit)}
      className="space-y-6"
      noValidate
    >
      {/* Honeypot — invisible to humans */}
      <div className="absolute -left-[9999px]" aria-hidden="true">
        <input
          type="text"
          tabIndex={-1}
          autoComplete="off"
          {...register('honeypot')}
        />
      </div>

      {/* Name & Email row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
        <div>
          <label className="field-label">{t('name')}</label>
          <input
            type="text"
            className="input-field"
            placeholder={t('namePlaceholder')}
            {...register('name')}
          />
          {errors.name && (
            <p
              className="mt-1.5 text-xs flex items-center gap-1"
              style={{ color: 'var(--status-hold)' }}
            >
              <AlertCircle size={12} />
              {errors.name.message}
            </p>
          )}
        </div>
        <div>
          <label className="field-label">{t('email')}</label>
          <input
            type="email"
            className="input-field"
            placeholder={t('emailPlaceholder')}
            {...register('email')}
          />
          {errors.email && (
            <p
              className="mt-1.5 text-xs flex items-center gap-1"
              style={{ color: 'var(--status-hold)' }}
            >
              <AlertCircle size={12} />
              {errors.email.message}
            </p>
          )}
        </div>
      </div>

      {/* Organization & Role row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
        <div>
          <label className="field-label">{t('organization')}</label>
          <input
            type="text"
            className="input-field"
            placeholder={t('organizationPlaceholder')}
            {...register('organization')}
          />
          {errors.organization && (
            <p
              className="mt-1.5 text-xs flex items-center gap-1"
              style={{ color: 'var(--status-hold)' }}
            >
              <AlertCircle size={12} />
              {errors.organization.message}
            </p>
          )}
        </div>
        <div>
          <label className="field-label">{t('role')}</label>
          <select
            className="input-field"
            {...register('role')}
          >
            {ROLE_OPTIONS.map((role) => (
              <option key={role} value={role}>
                {t(`roles.${role}`)}
              </option>
            ))}
          </select>
          {errors.role && (
            <p
              className="mt-1.5 text-xs flex items-center gap-1"
              style={{ color: 'var(--status-hold)' }}
            >
              <AlertCircle size={12} />
              {errors.role.message}
            </p>
          )}
        </div>
      </div>

      {/* Message */}
      <div>
        <label className="field-label">{t('message')}</label>
        <textarea
          className="input-field min-h-[140px] resize-y"
          placeholder={t('messagePlaceholder')}
          {...register('message')}
        />
        {errors.message && (
          <p
            className="mt-1.5 text-xs flex items-center gap-1"
            style={{ color: 'var(--status-hold)' }}
          >
            <AlertCircle size={12} />
            {errors.message.message}
          </p>
        )}
      </div>

      {/* Server error */}
      <AnimatePresence>
        {serverError && (
          <motion.div
            className="banner-error"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
          >
            {serverError}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Submit */}
      <button
        type="submit"
        className="btn-marketing-primary w-full sm:w-auto"
        disabled={isSubmitting}
      >
        {isSubmitting ? (
          <>
            <Loader2 size={16} className="animate-spin" />
            {t('submitting')}
          </>
        ) : (
          <>
            <Send size={16} />
            {t('submit')}
          </>
        )}
      </button>

      <p
        className="text-xs"
        style={{ color: 'var(--text-tertiary)' }}
      >
        {t('disclaimer')}
      </p>
    </form>
  );
}
