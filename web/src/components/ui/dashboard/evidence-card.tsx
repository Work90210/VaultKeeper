interface EvidenceCardProps {
  thumbnail?: string;
  type: string;
  name: string;
  reference: string;
  meta?: string;
  tags?: string[];
  bpPhases?: { status: 'complete' | 'in_progress' | 'not_started' }[];
  statusPill?: string;
  className?: string;
}

export function EvidenceCard({
  thumbnail,
  type,
  name,
  reference,
  meta,
  tags,
  bpPhases,
  statusPill,
  className,
}: EvidenceCardProps) {
  return (
    <div className={`ev-card${className ? ` ${className}` : ''}`}>
      <div className="ev-thumb">
        {thumbnail ? (
          <img src={thumbnail} alt={name} />
        ) : (
          <div className="ev-thumb-empty" />
        )}
        <span className="ev-type-badge">{type}</span>
        {statusPill && <span className="ev-status-pill">{statusPill}</span>}
      </div>
      <div className="ev-name">{name}</div>
      <div className="ev-ref mono">
        {reference}
        {meta && <span className="ev-meta">{meta}</span>}
      </div>
      {tags && tags.length > 0 && (
        <div className="ev-tags">
          {tags.map((tag) => (
            <span key={tag} className="tag">
              {tag}
            </span>
          ))}
        </div>
      )}
      {bpPhases && bpPhases.length > 0 && (
        <div className="ev-bp">
          {bpPhases.map((phase, i) => (
            <span key={i} className={`bp-dot bp-${phase.status}`} />
          ))}
        </div>
      )}
    </div>
  );
}
