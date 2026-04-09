import type { Metadata } from 'next';
import { PricingSection } from '@/components/marketing/sections/pricing-section';
import { FaqSection } from '@/components/marketing/sections/faq-section';
import { CtaSection } from '@/components/marketing/sections/cta-section';
import { PricingPageHero } from './hero';

export const metadata: Metadata = {
  title: 'Pricing',
  description:
    'VaultKeeper pricing plans for investigation teams of every size. Start with a free pilot, scale to enterprise-grade sovereign evidence management.',
  alternates: {
    languages: {
      en: '/en/pricing',
      fr: '/fr/pricing',
    },
  },
};

export default function PricingPage({
  params,
}: {
  params: { locale: string };
}) {
  return (
    <>
      <PricingPageHero />
      <PricingSection locale={params.locale} />
      <FaqSection />
      <CtaSection locale={params.locale} />
    </>
  );
}
