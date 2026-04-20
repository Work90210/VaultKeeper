export function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return <div className={`skeleton rounded ${className || ''}`} {...props} />;
}

export function CardSkeleton() {
  return (
    <div className="rounded-md border p-[var(--space-md)]" style={{ borderColor: 'var(--border-default)' }}>
      <Skeleton className="h-4 w-3/4 mb-[var(--space-sm)]" />
      <Skeleton className="h-3 w-1/2 mb-[var(--space-xs)]" />
      <Skeleton className="h-3 w-full" />
    </div>
  );
}

export function TableRowSkeleton() {
  return (
    <div className="flex gap-[var(--space-md)] py-[var(--space-sm)]">
      <Skeleton className="h-3 w-1/4" />
      <Skeleton className="h-3 w-1/3" />
      <Skeleton className="h-3 w-1/6" />
      <Skeleton className="h-3 w-1/6" />
    </div>
  );
}
