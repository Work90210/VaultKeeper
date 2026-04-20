import type { Metadata } from 'next';
import { ContactPageContent } from './content';

export const metadata: Metadata = {
  title: 'Contact',
  description:
    'Get in touch with the VaultKeeper team. Request a demo, discuss pricing, plan a migration, or ask about sovereign deployment options.',
  alternates: {
    languages: {
      en: '/en/contact',
      fr: '/fr/contact',
    },
  },
};

export default function ContactPage() {
  return <ContactPageContent />;
}
