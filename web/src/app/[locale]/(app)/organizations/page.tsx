import { redirect } from 'next/navigation';

export default async function OrganizationsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  redirect(`/${locale}/settings?tab=organization`);
}
