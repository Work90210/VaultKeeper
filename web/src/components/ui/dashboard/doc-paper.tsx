interface DocPaperProps {
  children: React.ReactNode;
  className?: string;
}

export function DocPaper({ children, className }: DocPaperProps) {
  return (
    <div className={`doc-paper${className ? ` ${className}` : ''}`}>
      {children}
    </div>
  );
}
