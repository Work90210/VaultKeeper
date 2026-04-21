interface KVItem {
  readonly label: string;
  readonly value: React.ReactNode;
}

interface KeyValueListProps {
  readonly items: KVItem[];
}

export function KeyValueList({ items }: KeyValueListProps) {
  return (
    <dl className="kvs">
      {items.map((item) => (
        <div key={item.label} style={{ display: 'contents' }}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
