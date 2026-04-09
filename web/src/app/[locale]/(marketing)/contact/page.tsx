import type { Metadata } from 'next';
import { ContactPageContent } from './content';

export const metadata: Metadata = {
  title: 'Contact — Pilot Program',
  description:
    'Register for the VaultKeeper pilot program. Join investigation teams already using sovereign evidence management for secure intake, custody tracking, and court-ready disclosure.',
  alternates: {
    languages: {
      en: '/en/contact',
      fr: '/fr/contact',
    },
  },
};

export default function ContactPage({
  params,
}: {
  params: { locale: string };
}) {
  return <ContactPageContent locale={params.locale} />;
}
