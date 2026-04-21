interface BarItem {
  label: string;
  value: number;
  max?: number;
}

interface BarChartProps {
  bars: BarItem[];
}

export function BarChart({ bars }: BarChartProps) {
  const globalMax = Math.max(...bars.map((b) => b.max ?? b.value), 1);

  return (
    <div className="bar-chart">
      {bars.map((bar) => {
        const max = bar.max ?? globalMax;
        const pct = max > 0 ? Math.min((bar.value / max) * 100, 100) : 0;

        return (
          <div key={bar.label} className="bar-row">
            <span className="bar-label mono">{bar.label}</span>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${pct}%` }} />
            </div>
            <span className="bar-value mono">{bar.value}</span>
          </div>
        );
      })}
    </div>
  );
}
