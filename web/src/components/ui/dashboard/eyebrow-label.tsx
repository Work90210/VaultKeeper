interface EyebrowLabelProps {
  readonly children: React.ReactNode;
}

export function EyebrowLabel({ children }: EyebrowLabelProps) {
  return (
    <span
      className="eyebrow-m"
      style={{
        fontFamily: '"JetBrains Mono", monospace',
        fontSize: 10,
        letterSpacing: '.08em',
        textTransform: 'uppercase',
        color: 'var(--muted)',
      }}
    >
      {children}
    </span>
  );
}
