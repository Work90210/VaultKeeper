interface TagProps {
  children: React.ReactNode;
  accent?: boolean;
  className?: string;
}

export function Tag({ children, accent, className }: TagProps) {
  const classes = [
    'tag',
    accent ? 'a' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  return <span className={classes}>{children}</span>;
}
