import type { CategoriesResponse, Category } from '../types';
import { formatSize, percent } from '../format';
import {
  AGE_BUCKETS,
  CATEGORY_COLORS,
  REVIEW_COLOR,
  SAFE_COLOR,
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

  const total = apparent ? categories.total.size : categories.total.usage;
  const present = categories.categories
    .map((c) => ({ ...c, bytes: apparent ? c.size : c.usage }))
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
      {present.map((c) => (
        <button
          key={c.id}
          type="button"
          className={`legend-chip ${filter === c.id ? 'active' : ''}`}
          aria-pressed={filter === c.id}
          onClick={() => onFilter(filter === c.id ? null : c.id)}
        >
          {projection === 'categories' && <Swatch color={CATEGORY_COLORS[c.id]} />}
          {c.label} · {formatSize(c.bytes, useSIPrefix)} · {percent(c.bytes, total).toFixed(0)}%
        </button>
      ))}
    </div>
  );
}
