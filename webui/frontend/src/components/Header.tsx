import { useGduModel } from '../model';
import { formatSize } from '../format';
import { Breadcrumbs } from './Breadcrumbs';
import { PROJECTIONS, type Projection } from '../cleanup';

export function Header() {
  const {
    status,
    nodeResp,
    total,
    useSIPrefix,
    view,
    toggleView,
    projection,
    setProjection,
    effectiveApparent,
    toggleApparent,
    handleSelect,
  } = useGduModel();

  return (
    <header className="app-header">
      <div className="app-title">
        <span className="logo">gdu</span>
        {nodeResp && <Breadcrumbs breadcrumbs={nodeResp.breadcrumbs} onNavigate={handleSelect} />}
      </div>
      <div className="app-actions">
        <span className="total">{formatSize(total, useSIPrefix)}</span>
        <button
          type="button"
          className="toggle"
          onClick={toggleView}
          title="Toggle between donut and treemap charts"
        >
          {view === 'donut' ? 'Treemap' : 'Donut'}
        </button>
        {view === 'treemap' && (
          <select
            className="toggle"
            aria-label="Treemap projection"
            title="Color the treemap by location, cleanup candidates or age (P)"
            value={projection}
            onChange={(event) => setProjection(event.target.value as Projection)}
          >
            {PROJECTIONS.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        )}
        <button
          type="button"
          className="toggle"
          onClick={toggleApparent}
          title="Toggle between disk usage and apparent size"
        >
          {effectiveApparent ? 'Apparent size' : 'Disk usage'}
        </button>
        {status.state === 'scanning' && <span className="scanning-badge">scanning…</span>}
      </div>
    </header>
  );
}
