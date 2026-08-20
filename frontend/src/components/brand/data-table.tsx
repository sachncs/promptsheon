import { cn } from '@/lib/utils';

export interface DataTableColumn {
  key: string;
  header: string;
  className?: string;
  render: (row: Record<string, unknown>) => React.ReactNode;
}

export function DataTable({
  rows,
  columns,
  empty,
  className,
  rowKey,
  onRowClick,
}: {
  rows: Array<Record<string, unknown>>;
  columns: DataTableColumn[];
  empty?: React.ReactNode;
  className?: string;
  rowKey: (row: Record<string, unknown>, index: number) => string;
  onRowClick?: (row: Record<string, unknown>) => void;
}) {
  if (rows.length === 0 && empty) {
    return <div className={cn('rounded-xl border border-border-subtle bg-surface-1/60', className)}>{empty}</div>;
  }

  return (
    <div className={cn('overflow-hidden rounded-xl border border-border-subtle bg-surface-1', className)}>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="bg-surface-2/60 text-xs uppercase tracking-wider text-text-subtle">
            <tr>
              {columns.map((col) => (
                <th key={col.key} className={cn('px-4 py-3 font-medium', col.className)}>
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border-subtle">
            {rows.map((row, idx) => (
              <tr
                key={rowKey(row, idx)}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                className={cn(
                  'text-text-default transition-colors',
                  onRowClick && 'cursor-pointer hover:bg-surface-2/40',
                )}
              >
                {columns.map((col) => (
                  <td key={col.key} className={cn('px-4 py-3 align-middle', col.className)}>
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
