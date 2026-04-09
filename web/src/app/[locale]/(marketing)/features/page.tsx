import type { Metadata } from 'next';
import { FeaturesSection } from '@/components/marketing/sections/features-section';
import { HowItWorksSection } from '@/components/marketing/sections/how-it-works-section';
import { CtaSection } from '@/components/marketing/sections/cta-section';
import { FeaturesPageHero } from './hero';

export const metadata: Metadata = {
  title: 'Features',
  description:
    'Explore VaultKeeper capabilities: secure evidence intake, chain-of-custody tracking, role-based access, disclosure management, collaborative redaction, and intelligent search.',
  alternates: {
    languages: {
      en: '/en/features',
      fr: '/fr/features',
    },
  },
};

export default function FeaturesPage({
  params,
}: {
  params: { locale: string };
}) {
  return (
    <>
      <FeaturesPageHero />
      <FeaturesSection />
      <HowItWorksSection />
      <CtaSection locale={params.locale} />
    </>
  );
}
