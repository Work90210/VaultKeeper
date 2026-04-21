interface ChainVisualProps {
  nodes: number;
  breaks?: number[];
}

export function ChainVisual({ nodes, breaks = [] }: ChainVisualProps) {
  const breakSet = new Set(breaks);
  const elements: React.ReactNode[] = [];

  for (let i = 0; i < nodes; i++) {
    if (i > 0) {
      elements.push(<div key={`seg-${i}`} className="seg" />);
    }

    const isBroken = breakSet.has(i);
    const nodeClass = isBroken ? 'node br' : 'node on';
    elements.push(<div key={`node-${i}`} className={nodeClass} />);
  }

  return <div className="chain">{elements}</div>;
}
