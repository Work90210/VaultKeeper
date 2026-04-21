interface KPIItem {
  label: string;
  value: string | number;
  sub?: string;
  delta?: string;
  deltaNegative?: boolean;
}

interface KPIStripProps {
  items: KPIItem[];
}

function formatValue(value: string | number): React.ReactNode {
  const str = String(value);
  const match = str.match(/^([\d,.]+)(.+)$/);
  if (match) {
    return (
      <>
        {match[1]}
        <em>{match[2]}</em>
      </>
    );
  }
  return str;
}

export function KPIStrip({ items }: KPIStripProps) {
  return (
    <div className="d-kpis">
      {items.map((item) => (
        <div key={item.label} className="d-kpi">
          <div className="k">{item.label}</div>
          <div className="v">{formatValue(item.value)}</div>
          {item.sub && <div className="sub">{item.sub}</div>}
          {item.delta && (
            <div className={`delta${item.deltaNegative ? ' n' : ''}`}>
              {item.delta}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
