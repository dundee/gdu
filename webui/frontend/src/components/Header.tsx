import { useGduModel } from '../model';
import { formatSize } from '../format';
import { Breadcrumbs } from './Breadcrumbs';

export function Header() {
  const { status, nodeResp, total, useSIPrefix, view, toggleView, effectiveApparent, toggleApparent, handleSelect } =
    useGduModel();

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
