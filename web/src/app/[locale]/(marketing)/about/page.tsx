import type { Metadata } from 'next';
import { AboutPageContent } from './content';
import { CtaSection } from '@/components/marketing/sections/cta-section';

export const metadata: Metadata = {
  title: 'About',
  description:
    'VaultKeeper is built on a single conviction: evidence integrity is the foundation of justice. Learn about our mission, principles, and commitment to sovereignty.',
  alternates: {
    languages: {
      en: '/en/about',
      fr: '/fr/about',
    },
  },
};

export default function AboutPage({
  params,
}: {
  params: { locale: string };
}) {
  return (
    <>
      <AboutPageContent />
      <CtaSection locale={params.locale} />
    </>
  );
}
