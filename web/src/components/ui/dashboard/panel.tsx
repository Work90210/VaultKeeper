interface PanelProps {
  title?: string;
  titleAccent?: string;
  meta?: React.ReactNode;
  headerRight?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function Panel({
  title,
  titleAccent,
  meta,
  headerRight,
  children,
  className,
}: PanelProps) {
  const hasHeader = title || meta || headerRight;
  return (
    <div className={`panel${className ? ` ${className}` : ''}`}>
      {hasHeader && (
        <div className="panel-h">
          {title && (
            <h3>
              {title}
              {titleAccent && <em> {titleAccent}</em>}
            </h3>
          )}
          {meta && <span className="meta">{meta}</span>}
          {headerRight}
        </div>
      )}
      <div className="panel-body">{children}</div>
    </div>
  );
}
