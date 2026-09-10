import { cn } from '@/lib/utils';

/**
 * DataTable column descriptor. The render() callback receives the
 * strongly typed row, so callers no longer need `as unknown as T`
 * casts at every render site.
 */
export interface DataTableColumn<T> {
  key: string;
  header: string;
  className?: string;
  render: (row: T) => React.ReactNode;
  /**
   * If set, the column header becomes a clickable sort toggle.
   * The wrapper itself doesn't manage sort state — that lives
   * on the page-level `useState` hook. The button's pressed
   * state + ARIA sort are derived from `sortKey` + current sort.
   */
  sortable?: boolean;
  sortKey?: keyof T;
}

export interface DataTableProps<T> {
  rows: T[];
  columns: DataTableColumn<T>[];
  empty?: React.ReactNode;
  className?: string;
  rowKey: (row: T, index: number) => string;
  onRowClick?: (row: T) => void;
  /** Caption rendered above the table for screen readers; visually optional. */
  caption?: string;
  sortState?: { key: string; direction: 'asc' | 'desc' };
  onSort?: (key: string) => void;
}

export function DataTable<T>({
  rows,
  columns,
  empty,
  className,
  rowKey,
  onRowClick,
  caption,
  sortState,
  onSort,
}: DataTableProps<T>) {
  if (rows.length === 0 && empty) {
    return (
      <div className={cn('rounded-xl border border-border-subtle bg-surface-1/60', className)}>
        {empty}
      </div>
    );
  }

  return (
    <div className={cn('overflow-hidden rounded-xl border border-border-subtle bg-surface-1', className)}>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <caption className="sr-only">{caption ?? 'Data table'}</caption>
          <thead className="bg-surface-2/60 text-xs uppercase tracking-wider text-text-subtle">
            <tr>
              {columns.map((col) => {
                const sortable = Boolean(col.sortable);
                const isSorted = sortState && col.sortKey && String(col.sortKey) === sortState.key;
                const ariaSort = isSorted
                  ? sortState?.direction === 'asc'
                    ? 'ascending'
                    : 'descending'
                  : sortable
                    ? 'none'
                    : undefined;
                return (
                  <th
                    key={col.key}
                    scope="col"
                    aria-sort={ariaSort}
                    className={cn('px-4 py-3 font-medium', col.className)}
                  >
                    {sortable && onSort ? (
                      <button
                        type="button"
                        onClick={() => onSort(String(col.sortKey))}
                        className={cn(
                          'inline-flex items-center gap-1.5',
                          'hover:text-text-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-highlight rounded',
                        )}
                      >
                        {col.header}
                        <span aria-hidden="true" className="text-text-muted">
                          {isSorted ? (sortState?.direction === 'asc' ? '↑' : '↓') : '↕'}
                        </span>
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                );
              })}
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
