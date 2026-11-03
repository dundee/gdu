import type { CategoriesResponse, Category } from '../types';
import { formatSize, percent } from '../format';
import {
  AGE_BUCKETS,
  CATEGORY_COLORS,
  REVIEW_COLOR,
  SAFE_COLOR,
  amountValue,
  type Projection,
} from '../cleanup';

interface CategoryLegendProps {
  projection: Projection;
  categories: CategoriesResponse | null;
  apparent: boolean;
  useSIPrefix: boolean;
  filter: Category | null;
  onFilter: (category: Category | null) => void;
}

function Swatch({ color }: { color: string }) {
  return <span className="legend-swatch" style={{ background: color }} />;
}

// CategoryLegend explains the active projection and, for the cleanup ones,
// lists the space each category takes within the current directory. Clicking
// a category highlights only the tiles holding it.
export function CategoryLegend({
  projection,
  categories,
  apparent,
  useSIPrefix,
  filter,
  onFilter,
}: CategoryLegendProps) {
  if (projection === 'location') {
    return null;
  }
  if (projection === 'age') {
    return (
      <div className="legend" aria-label="Age legend">
        <span className="legend-title">Last change</span>
        {AGE_BUCKETS.map((b) => (
          <span key={b.label} className="legend-key">
            <Swatch color={b.color} />
            {b.label}
          </span>
        ))}
      </div>
    );
  }
  if (!categories) {
    return <div className="legend legend-loading">Classifying…</div>;
  }

  const total = amountValue(categories.total, apparent);
  const present = categories.categories
    .map((c) => ({ ...c, bytes: amountValue(c, apparent) }))
    .filter((c) => c.bytes > 0);
  const reclaimable = present.reduce((sum, c) => sum + c.bytes, 0);

  return (
    <div className="legend" aria-label="Cleanup legend">
      {present.length === 0 ? (
        <span className="legend-title">No cleanup candidates here</span>
      ) : (
        <span className="legend-title">
          Candidates: {formatSize(reclaimable, useSIPrefix)} ({percent(reclaimable, total).toFixed(0)}%)
        </span>
      )}
      {projection === 'cleanup' && present.length > 0 && (
        <>
          <span className="legend-key">
            <Swatch color={SAFE_COLOR} />
            safe
          </span>
          <span className="legend-key">
            <Swatch color={REVIEW_COLOR} />
            review first
          </span>
        </>
      )}
      {present.map((c) => {
        const active = filter === c.id;
        const color = CATEGORY_COLORS[c.id];
        return (
          <button
            key={c.id}
            type="button"
            className={`legend-chip ${active ? 'active' : ''}`}
            aria-pressed={active}
            title={active ? 'Show all categories (Esc)' : `Highlight ${c.label.toLowerCase()}`}
            style={active ? { borderColor: color, boxShadow: `inset 0 0 0 1px ${color}` } : undefined}
            onClick={() => onFilter(active ? null : c.id)}
          >
            <Swatch color={color} />
            {c.label} · {formatSize(c.bytes, useSIPrefix)} · {percent(c.bytes, total).toFixed(0)}%
          </button>
        );
      })}
      {filter && (
        <button type="button" className="legend-clear" onClick={() => onFilter(null)}>
          Show all
        </button>
      )}
    </div>
  );
}
