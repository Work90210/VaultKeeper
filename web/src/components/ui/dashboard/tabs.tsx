'use client';

interface Tab {
  readonly key: string;
  readonly label: string;
}

interface TabsProps {
  readonly tabs: Tab[];
  readonly active: string;
  readonly onChange?: (key: string) => void;
}

export function Tabs({ tabs, active, onChange }: TabsProps) {
  return (
    <div className="tabs">
      {tabs.map((tab) => (
        <button
          key={tab.key}
          type="button"
          className={tab.key === active ? 'active' : undefined}
          onClick={() => onChange?.(tab.key)}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            padding: '14px 14px',
            fontSize: '13.5px',
            color: tab.key === active ? 'var(--ink)' : 'var(--muted)',
            borderBottom:
              tab.key === active
                ? '2px solid var(--accent)'
                : '2px solid transparent',
            marginBottom: -1,
            fontWeight: tab.key === active ? 500 : 400,
            fontFamily: 'inherit',
            transition: 'color .15s',
          }}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
