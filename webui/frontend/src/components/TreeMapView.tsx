import { useCallback, useEffect, useRef, useState } from 'react';
import type { Node } from '../types';
import { deleteNode, fetchTree, revealNode } from '../api';
import { useGduModel } from '../model';
import { formatSize } from '../format';
import { TreeMap } from './TreeMap';
import { Modal } from './Modal';

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
    actionToken,
    view,
    treeRoot,
    treePath,
    setTree,
    clearTree,
    refreshNode,
    setLoadError,
    skipDeleteConfirm,
    setSkipDeleteConfirm,
  } = useGduModel();

  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Node | null>(null);
  const [skipDeleteChoice, setSkipDeleteChoice] = useState(false);
  const [actionPending, setActionPending] = useState(false);

  // Reset selection whenever the directory changes.
  useEffect(() => {
    setSelectedNode(null);
  }, [currentPath]);

  // Tracks the latest currentPath so an in-flight refreshTree() call (e.g.
  // one started before breadcrumb navigation) can tell its result is stale
  // once it resolves.
  const currentPathRef = useRef(currentPath);
  useEffect(() => {
    currentPathRef.current = currentPath;
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

  const refreshAfterMutation = useCallback(async () => {
    await Promise.all([refreshNode(), refreshTree()]);
    setSelectedNode(null);
    setHoveredPath(null);
  }, [refreshNode, refreshTree, setHoveredPath]);

  const performDelete = useCallback(
    async (node: Node) => {
      if (actionPending) {
        return;
      }
      setActionPending(true);
      setDeleteCandidate(null);
      try {
        await deleteNode(node.path, actionToken);
        await refreshAfterMutation();
        setLoadError(null);
      } catch (err: unknown) {
        if (typeof err === 'object' && err !== null && 'status' in err && err.status === 404) {
          try {
            await refreshAfterMutation();
          } catch {
            // Keep the original action error; the regular loader can retry later.
          }
        }
        setLoadError(err instanceof Error ? err.message : String(err));
      } finally {
        setActionPending(false);
      }
    },
    [actionPending, refreshAfterMutation, setLoadError, actionToken],
  );

  const revealSelected = useCallback(async () => {
    if (!selectedNode || actionPending) {
      return;
    }
    setActionPending(true);
    try {
      await revealNode(selectedNode.path, actionToken);
      setLoadError(null);
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : String(err));
    } finally {
      setActionPending(false);
    }
  }, [actionPending, selectedNode, setLoadError, actionToken]);

  const requestDelete = useCallback(() => {
    if (!selectedNode || actionPending || !status.deleteAllowed) {
      return;
    }
    if (skipDeleteConfirm) {
      void performDelete(selectedNode);
      return;
    }
    setSkipDeleteChoice(false);
    setDeleteCandidate(selectedNode);
  }, [actionPending, performDelete, selectedNode, skipDeleteConfirm, status.deleteAllowed]);

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
      } else if (event.code === 'KeyD' && selectedNode && status.deleteAllowed) {
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

      {deleteCandidate && (
        <Modal
          titleId="delete-title"
          title="Delete item"
          onClose={() => setDeleteCandidate(null)}
        >
          <p>This permanently deletes the selected item. This action cannot be undone.</p>
          <strong>{deleteCandidate.name}</strong>
          <code className="modal-path">{deleteCandidate.path}</code>
          <span className="muted">
            {formatSize(
              effectiveApparent ? deleteCandidate.size : deleteCandidate.usage,
              useSIPrefix,
            )}
          </span>
          <label className="modal-checkbox">
            <input
              type="checkbox"
              checked={skipDeleteChoice}
              onChange={(event) => setSkipDeleteChoice(event.target.checked)}
            />
            Do not ask again this session
          </label>
          <div className="modal-actions">
            <button type="button" autoFocus onClick={() => setDeleteCandidate(null)}>
              Cancel
            </button>
            <button
              type="button"
              className="danger"
              disabled={actionPending}
              onClick={() => {
                if (skipDeleteChoice) {
                  setSkipDeleteConfirm(true);
                }
                void performDelete(deleteCandidate);
              }}
            >
              Delete permanently
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
