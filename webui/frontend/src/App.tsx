import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Node, NodeResponse, SortKey, SortOrder, Status } from './types';
import { fetchNode, fetchStatus, subscribeStatus } from './api';
import { colorMapFor, computeSlices } from './slices';
import { formatSize } from './format';
import { DonutChart } from './components/DonutChart';
import { FileTable } from './components/FileTable';
import { Breadcrumbs } from './components/Breadcrumbs';
import { ProgressBar } from './components/ProgressBar';

export function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [currentPath, setCurrentPath] = useState<string | null>(null);
  const [nodeResp, setNodeResp] = useState<NodeResponse | null>(null);
  const [sort, setSort] = useState<SortKey>('size');
  const [order, setOrder] = useState<SortOrder>('desc');
  const [apparent, setApparent] = useState<boolean | null>(null);
  const [hoveredPath, setHoveredPath] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Initial status + live updates over SSE.
  useEffect(() => {
    fetchStatus().then(setStatus).catch(() => undefined);
    return subscribeStatus(setStatus);
  }, []);

  // Adopt the display default for size metric once, from the server flags.
  useEffect(() => {
    if (status && apparent === null) {
      setApparent(status.showApparentSize);
    }
  }, [status, apparent]);

  // Once a scan is done, default into the root directory.
  useEffect(() => {
    if (status?.state === 'done' && currentPath === null && status.rootPath) {
      setCurrentPath(status.rootPath);
    }
  }, [status, currentPath]);

  // Load the current node whenever the path, sort, or scan completion changes.
  useEffect(() => {
    if (currentPath === null) {
      return;
    }
    let cancelled = false;
    fetchNode(currentPath, sort, order)
      .then((resp) => {
        if (!cancelled) {
          setNodeResp(resp);
          setLoadError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoadError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [currentPath, sort, order, status?.state]);

  const effectiveApparent = apparent ?? status?.showApparentSize ?? false;
  const useSIPrefix = status?.useSIPrefix ?? false;
  const children = nodeResp?.children ?? [];

  const { total } = useMemo(
    () => computeSlices(children, effectiveApparent),
    [children, effectiveApparent],
  );
  const colorMap = useMemo(
    () => colorMapFor(children, effectiveApparent),
    [children, effectiveApparent],
  );

  const handleSortChange = useCallback(
    (key: SortKey) => {
      if (key === sort) {
        setOrder((o) => (o === 'asc' ? 'desc' : 'asc'));
      } else {
        setSort(key);
        setOrder(key === 'name' ? 'asc' : 'desc');
      }
    },
    [sort],
  );

  const handleSelect = useCallback((node: Node) => {
    if (node.isDir) {
      setCurrentPath(node.path);
      setHoveredPath(null);
    }
  }, []);

  const handleNavigate = useCallback((node: Node) => {
    setCurrentPath(node.path);
    setHoveredPath(null);
  }, []);

  if (!status) {
    return <div className="loading">Connecting…</div>;
  }

  if (status.state === 'error') {
    return (
      <div className="error-screen">
        <h2>Scan failed</h2>
        <p>{status.error ?? 'Unknown error'}</p>
      </div>
    );
  }

  if (currentPath === null || (status.state === 'scanning' && !nodeResp)) {
    return <ProgressBar progress={status.progress} useSIPrefix={useSIPrefix} />;
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="app-title">
          <span className="logo">gdu</span>
          {nodeResp && (
            <Breadcrumbs breadcrumbs={nodeResp.breadcrumbs} onNavigate={handleNavigate} />
          )}
        </div>
        <div className="app-actions">
          <span className="total">{formatSize(total, useSIPrefix)}</span>
          <button
            type="button"
            className="toggle"
            onClick={() => setApparent((a) => !(a ?? status.showApparentSize))}
            title="Toggle between disk usage and apparent size"
          >
            {effectiveApparent ? 'Apparent size' : 'Disk usage'}
          </button>
          {status.state === 'scanning' && <span className="scanning-badge">scanning…</span>}
        </div>
      </header>

      {loadError && <div className="banner error">{loadError}</div>}

      <main className="content">
        <section className="chart-panel">
          <DonutChart
            children={children}
            apparent={effectiveApparent}
            useSIPrefix={useSIPrefix}
            hoveredPath={hoveredPath}
            onHover={setHoveredPath}
            onSelect={handleSelect}
          />
        </section>
        <section className="table-panel">
          <FileTable
            children={children}
            colorMap={colorMap}
            apparent={effectiveApparent}
            useSIPrefix={useSIPrefix}
            total={total}
            sort={sort}
            order={order}
            onSortChange={handleSortChange}
            hoveredPath={hoveredPath}
            onHover={setHoveredPath}
            onSelect={handleSelect}
          />
        </section>
      </main>
    </div>
  );
}
