export default function CaseDetailPage({ params }: { params: { id: string } }) {
  return (
    <main className="container mx-auto py-8">
      <h1 className="text-2xl font-bold">Case: {params.id}</h1>
      <p className="text-muted-foreground mt-2">Case detail view will be implemented in Sprint 2.</p>
    </main>
  );
}
