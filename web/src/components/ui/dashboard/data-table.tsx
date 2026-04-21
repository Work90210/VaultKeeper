interface Column {
  key: string;
  label: string;
  align?: 'left' | 'right' | 'center';
  className?: string;
}

interface DataTableProps {
  columns: Column[];
  children: React.ReactNode;
  className?: string;
}

export function DataTable({ columns, children, className }: DataTableProps) {
  return (
    <table className={`tbl${className ? ` ${className}` : ''}`}>
      <thead>
        <tr>
          {columns.map((col) => (
            <th
              key={col.key}
              style={col.align ? { textAlign: col.align } : undefined}
              className={col.className}
            >
              {col.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}
