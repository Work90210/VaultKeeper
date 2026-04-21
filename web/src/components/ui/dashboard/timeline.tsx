interface TimelineItem {
  content: React.ReactNode;
  subline?: string;
  time?: string;
  accent?: boolean;
}

interface TimelineProps {
  items: TimelineItem[];
}

export function Timeline({ items }: TimelineProps) {
  return (
    <div className="tl-list">
      {items.map((item, i) => (
        <div key={i} className={`tl-item${item.accent ? ' accent' : ''}`}>
          <div>
            <div className="what">{item.content}</div>
            {item.subline && <div className="who-line">{item.subline}</div>}
          </div>
          {item.time && <div className="sig">{item.time}</div>}
        </div>
      ))}
    </div>
  );
}
