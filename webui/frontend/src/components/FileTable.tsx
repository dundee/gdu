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
            </tr>
          );
        })}
        {children.length === 0 && (
          <tr>
            <td colSpan={4} className="empty">
              Empty directory
            </td>
          </tr>
        )}
      </tbody>
      <tfoot>
        <tr>
          <td className="muted">{children.length} items</td>
          <td className="num">{formatSize(total, useSIPrefix)}</td>
          <td className="num muted" colSpan={2}>
            {percent(total, total) > 0 ? '100%' : ''}
          </td>
        </tr>
      </tfoot>
    </table>
  );
}
