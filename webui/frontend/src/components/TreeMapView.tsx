import { useCallback, useEffect, useState } from 'react';
import type { Node } from '../types';
import { fetchTree } from '../api';
import { useGduModel } from '../model';
import { TreeMap } from './TreeMap';
import { DeleteConfirmModal } from './DeleteConfirmModal';
import { useLatest } from '../useLatest';
import { useItemActions } from '../useItemActions';

// TreeMapView owns everything specific to the treemap: fetching (and
// caching, via the model) the recursive tree, node selection, and the
// delete/reveal action workflow. None of this is needed by the donut/table
// views, so it does not live in the shared model.
export function TreeMapView() {
  const {
    currentPath,
    effectiveApparent,
    useSIPrefix,
    colorMap,
    hoveredPath,
    setHoveredPath,
    navigateToPath,
    status,
    view,
    treeRoot,
    treePath,
    setTree,
    clearTree,
  } = useGduModel();

  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);

  // Reset selection whenever the directory changes.
  useEffect(() => {
    setSelectedNode(null);
  }, [currentPath]);

  // Tracks the latest currentPath so an in-flight refreshTree() call (e.g.
  // one started before breadcrumb navigation) can tell its result is stale
  // once it resolves.
  const currentPathRef = useLatest(currentPath);

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

  const refreshTree = useCallback(async () => {
    if (currentPath === null) {
      return;
    }
    const path = currentPath;
    try {
      const root = await fetchTree(path);
      if (currentPathRef.current === path) {
        setTree(path, root);
        setTreeError(null);
      }
    } catch (err: unknown) {
      // Invalidate the cache instead of leaving the pre-mutation treeRoot/
      // treePath in place: otherwise they would still match currentPath and
      // the stale tree would keep rendering as if it were up to date, with
      // treeError never shown.
      if (currentPathRef.current === path) {
        setTreeError(err instanceof Error ? err.message : String(err));
        clearTree();
      }
    }
  }, [currentPath, setTree, clearTree]);

  const clearSelection = useCallback(() => setSelectedNode(null), []);

  const actions = useItemActions({ refreshExtra: refreshTree, onRefreshed: clearSelection });
  const { deleteCandidate, reveal, requestDelete: requestDeleteNode } = actions;

  const revealSelected = useCallback(async () => {
    if (selectedNode) {
      await reveal(selectedNode);
    }
  }, [reveal, selectedNode]);

  const requestDelete = useCallback(() => {
    if (selectedNode) {
      requestDeleteNode(selectedNode);
    }
  }, [requestDeleteNode, selectedNode]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) {
        return;
      }
      if (deleteCandidate) {
        return;
      }
      if (event.code === 'KeyO' && selectedNode) {
        event.preventDefault();
        void revealSelected();
      } else if (
        // Deliberately not Backspace: every desktop file manager binds it to
        // "go up one level", so a user reaching for it here would expect to
        // navigate, not to destroy the selected item. The help modal lists
        // only D and Delete.
        (event.code === 'KeyD' || event.key === 'Delete') &&
        selectedNode &&
        status.deleteAllowed
      ) {
        event.preventDefault();
        requestDelete();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [deleteCandidate, requestDelete, revealSelected, selectedNode, status.deleteAllowed]);

  return (
    <>
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

      <DeleteConfirmModal actions={actions} />
    </>
  );
}
