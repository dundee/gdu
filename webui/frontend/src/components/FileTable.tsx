import { useMemo } from 'react';
import type { Node, SortKey, SortOrder } from '../types';
import { metricValue } from '../slices';
import { formatCount, formatMtime, formatSize, percent } from '../format';

interface FileTableProps {
  children: Node[];
  colorMap: Map<string, string>;
  apparent: boolean;
  useSIPrefix: boolean;
  total: number;
  sort: SortKey;
  order: SortOrder;
  onSortChange: (key: SortKey) => void;
  hoveredPath: string | null;
  onHover: (path: string | null) => void;
  onSelect: (node: Node) => void;
  onReveal: (node: Node) => void;
  onDelete: (node: Node) => void;
  // Delete is hidden entirely (not just disabled) when the server does not
  // allow it; reveal stays available.
  canDelete: boolean;
  // True while an action request is in flight; disables the buttons.
  actionPending: boolean;
}

const COLUMNS: { key: SortKey; label: string; numeric: boolean }[] = [
  { key: 'name', label: 'Name', numeric: false },
  { key: 'size', label: 'Size', numeric: true },
  { key: 'itemCount', label: 'Items', numeric: true },
  { key: 'mtime', label: 'Modified', numeric: true },
];

export function FileTable({
  children,
  colorMap,
  apparent,
  useSIPrefix,
  total,
  sort,
  order,
  onSortChange,
  hoveredPath,
  onHover,
  onSelect,
  onReveal,
  onDelete,
  canDelete,
  actionPending,
}: FileTableProps) {
  const maxValue = useMemo(
    () => children.reduce((max, n) => Math.max(max, metricValue(n, apparent)), 0),
    [children, apparent],
  );

  return (
    <table className="file-table">
      <thead>
        <tr>
          {COLUMNS.map((col) => (
            <th
              key={col.key}
              className={col.numeric ? 'num' : ''}
              onClick={() => onSortChange(col.key)}
            >
              {col.label}
              {sort === col.key && (
                <span className="sort-arrow">{order === 'asc' ? ' ▲' : ' ▼'}</span>
              )}
            </th>
          ))}
          <th className="actions-col" aria-label="Actions" />
        </tr>
      </thead>
      <tbody>
        {children.map((node) => {
          const value = metricValue(node, apparent);
          const barWidth = maxValue > 0 ? (value / maxValue) * 100 : 0;
          const color = colorMap.get(node.path) ?? 'var(--other)';
          const isHovered = node.path === hoveredPath;

          return (
            <tr
              key={node.path}
              className={isHovered ? 'hovered' : ''}
              onMouseEnter={() => onHover(node.path)}
              onMouseLeave={() => onHover(null)}
              onClick={() => node.isDir && onSelect(node)}
              style={{ cursor: node.isDir ? 'pointer' : 'default' }}
            >
              <td className="name-cell">
                <span className="swatch" style={{ backgroundColor: color }} />
                <span className="name">
                  {node.isDir ? '📁' : '📄'} {node.name}
                  {node.flag === '!' && <span className="flag-error" title="Access error"> !</span>}
                </span>
                <span className="bar" style={{ width: `${barWidth}%`, backgroundColor: color }} />
              </td>
              <td className="num">{formatSize(value, useSIPrefix)}</td>
              <td className="num">{formatCount(node.itemCount)}</td>
              <td className="num muted">{formatMtime(node.mtime)}</td>
              <td className="actions-cell" onClick={(event) => event.stopPropagation()}>
                <button
                  type="button"
                  className="row-action"
                  aria-label={`Reveal ${node.name}`}
                  title="Reveal in file manager"
                  disabled={actionPending}
                  onClick={() => onReveal(node)}
                >
                  📂
                </button>
                {canDelete && (
                  <button
                    type="button"
                    className="row-action"
                    aria-label={`Delete ${node.name}`}
                    title="Delete"
                    disabled={actionPending}
                    onClick={() => onDelete(node)}
                  >
                    🗑
                  </button>
                )}
              </td>
            </tr>
          );
        })}
        {children.length === 0 && (
          <tr>
            <td colSpan={5} className="empty">
              Empty directory
            </td>
          </tr>
        )}
      </tbody>
      <tfoot>
        <tr>
          <td className="muted">{children.length} items</td>
          <td className="num">{formatSize(total, useSIPrefix)}</td>
          <td className="num muted" colSpan={3}>
            {percent(total, total) > 0 ? '100%' : ''}
          </td>
        </tr>
      </tfoot>
    </table>
  );
}
