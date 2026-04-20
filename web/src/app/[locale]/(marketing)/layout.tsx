import type { Metadata } from 'next';
import { MarketingHeader } from '@/components/marketing/layout/marketing-header';
import { MarketingFooter } from '@/components/marketing/layout/marketing-footer';

export const metadata: Metadata = {
  title: {
    template: '%s | VaultKeeper',
    default: 'VaultKeeper — Sovereign Evidence Management',
  },
  description:
    'Authoritative control over evidence workflows for legal and criminal investigations. Secure intake, chain-of-custody, and court-ready disclosure.',
  openGraph: {
    type: 'website',
    siteName: 'VaultKeeper',
  },
};

export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <MarketingHeader />
      <main>{children}</main>
      <MarketingFooter />
    </>
  );
}
