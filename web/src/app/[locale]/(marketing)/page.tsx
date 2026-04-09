import type { Metadata } from 'next';
import { HeroSection } from '@/components/marketing/hero/hero-section';
import { SocialProofSection } from '@/components/marketing/sections/social-proof-section';
import { StatsSection } from '@/components/marketing/sections/stats-section';
import { FeaturesSection } from '@/components/marketing/sections/features-section';
import { HowItWorksSection } from '@/components/marketing/sections/how-it-works-section';
import { FaqSection } from '@/components/marketing/sections/faq-section';
import { CtaSection } from '@/components/marketing/sections/cta-section';

export const metadata: Metadata = {
  title: 'VaultKeeper — Sovereign Evidence Management',
  description:
    'Authoritative control over evidence workflows for legal and criminal investigations. Secure intake, chain-of-custody, and court-ready disclosure.',
  alternates: {
    languages: {
      en: '/en',
      fr: '/fr',
    },
  },
};

export default function HomePage({
  params,
}: {
  params: { locale: string };
}) {
  return (
    <>
      <HeroSection locale={params.locale} />
      <SocialProofSection />
      <StatsSection />
      <FeaturesSection />
      <HowItWorksSection />
      <FaqSection />
      <CtaSection locale={params.locale} />
    </>
  );
}
