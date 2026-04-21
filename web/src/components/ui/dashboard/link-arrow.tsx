import Link from 'next/link';

interface LinkArrowProps {
  readonly href: string;
  readonly children: React.ReactNode;
}

export function LinkArrow({ href, children }: LinkArrowProps) {
  return (
    <Link href={href} className="linkarrow">
      {children} <span>→</span>
    </Link>
  );
}
