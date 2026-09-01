import { GduModelProvider, useGduModelState } from './model';
import { Header } from './components/Header';
import { DonutView } from './components/DonutView';
import { FileTableView } from './components/FileTableView';
import { TreeMapView } from './components/TreeMapView';
import { HelpModal } from './components/HelpModal';
import { ProgressBar } from './components/ProgressBar';

export function App() {
  const model = useGduModelState();
  const { status, currentPath, nodeResp, useSIPrefix, loadError, view } = model;

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
    <GduModelProvider model={{ ...model, status, currentPath }}>
      <div className="app">
        <Header />

        {loadError && <div className="banner error">{loadError}</div>}

        <main className={`content ${view === 'treemap' ? 'treemap-view' : ''}`}>
          {view === 'donut' ? (
            <>
              <DonutView />
              <FileTableView />
            </>
          ) : (
            <TreeMapView />
          )}
        </main>

        <HelpModal />
      </div>
    </GduModelProvider>
  );
}
