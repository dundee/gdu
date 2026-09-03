import { useEffect, useState } from 'react';
import type { Node } from '../types';
import { fetchTree } from '../api';
import { useGduModel } from '../model';
import { TreeMap } from './TreeMap';

// TreeMapView owns everything specific to the treemap: fetching (and
// caching, via the model) the recursive tree and node selection. None of
// this is needed by the donut/table views, so it does not live in the
// shared model.
export function TreeMapView() {
  const {
    currentPath,
    effectiveApparent,
    useSIPrefix,
    colorMap,
    hoveredPath,
    setHoveredPath,
    navigateToPath,
    view,
    treeRoot,
    treePath,
    setTree,
  } = useGduModel();

  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);

  // Reset selection whenever the directory changes.
  useEffect(() => {
    setSelectedNode(null);
  }, [currentPath]);

  // Load (and cache, via the model) the recursive tree for the current
  // directory. Cached in the model rather than here so toggling back to this
  // view for the same path does not re-fetch.
  useEffect(() => {
    if (view !== 'treemap' || currentPath === null || treePath === currentPath) {
      return;
    }
    let cancelled = false;
    setTreeLoading(true);
    setTreeError(null);
    fetchTree(currentPath)
      .then((root) => {
        if (!cancelled) {
          setTree(currentPath, root);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setTreeError(err instanceof Error ? err.message : String(err));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setTreeLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [currentPath, treePath, view, setTree]);

  return (
    <section className="chart-panel">
      {treeRoot && treePath === currentPath ? (
        <TreeMap
          root={treeRoot}
          apparent={effectiveApparent}
          useSIPrefix={useSIPrefix}
          colorMap={colorMap}
          hoveredPath={hoveredPath}
          selectedPath={selectedNode?.path ?? null}
          onHover={setHoveredPath}
          onSelect={setSelectedNode}
          onOpen={navigateToPath}
        />
      ) : (
        <div className="treemap-loading">
          {treeLoading && <span className="spinner compact" />}
          <span>{treeError ? 'Treemap unavailable' : 'Loading treemap…'}</span>
        </div>
      )}
    </section>
  );
}
