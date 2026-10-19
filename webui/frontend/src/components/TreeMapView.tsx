import { useCallback, useEffect, useMemo, useState } from 'react';
import type { CategoriesResponse, Category, Node } from '../types';
import { fetchCategories, fetchTree } from '../api';
import { useGduModel } from '../model';
import { TreeMap } from './TreeMap';
import { DeleteConfirmModal } from './DeleteConfirmModal';
import { useLatest } from '../useLatest';
import { useItemActions } from '../useItemActions';
import { CategoryLegend } from './CategoryLegend';
import type { TreeLayoutRect } from './treemap/treeLayout';
import {
  CATEGORY_COLORS,
  NEUTRAL_COLOR,
  PIVOT_PATH_PREFIX,
  PROJECTIONS,
  ageColor,
  ageText,
  candidateSummary,
  cleanupColor,
  containsCategory,
  isPivotNode,
} from '../cleanup';

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
    projection,
    setProjection,
  } = useGduModel();

  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [categories, setCategories] = useState<CategoriesResponse | null>(null);
  const [categoryFilter, setCategoryFilter] = useState<Category | null>(null);

  const pivot = projection === 'categories';
  const showsCategories = projection === 'cleanup' || pivot;
  // The pivoted tree is cached under its own key, so switching projections
  // back and forth does not mix up the two shapes of the same directory.
  const treeKey = pivot ? PIVOT_PATH_PREFIX + currentPath : currentPath;

  // Reset selection whenever the directory changes.
  useEffect(() => {
    setSelectedNode(null);
    setCategoryFilter(null);
  }, [currentPath]);

  // Category totals are scoped to the current directory.
  useEffect(() => {
    if (view !== 'treemap' || !showsCategories || currentPath === null) {
      return;
    }
    let cancelled = false;
    setCategories(null);
    fetchCategories(currentPath)
      .then((resp) => {
        if (!cancelled) {
          setCategories(resp);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [currentPath, showsCategories, view]);

  // Tracks the latest currentPath so an in-flight refreshTree() call (e.g.
  // one started before breadcrumb navigation) can tell its result is stale
  // once it resolves.
  const currentPathRef = useLatest(currentPath);

  // Load (and cache, via the model) the recursive tree for the current
  // directory. Cached in the model rather than here so toggling back to this
  // view for the same path does not re-fetch.
  useEffect(() => {
    if (view !== 'treemap' || currentPath === null || treePath === treeKey) {
      return;
    }
    let cancelled = false;
    setTreeLoading(true);
    setTreeError(null);
    fetchTree(currentPath, pivot)
      .then((root) => {
        if (!cancelled) {
          setTree(treeKey, root);
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
  }, [currentPath, treePath, treeKey, pivot, view, setTree]);

  const refreshTree = useCallback(async () => {
    if (currentPath === null) {
      return;
    }
    const path = currentPath;
    if (showsCategories) {
      fetchCategories(path)
        .then((resp) => {
          if (currentPathRef.current === path) {
            setCategories(resp);
          }
        })
        .catch(() => undefined);
    }
    try {
      const root = await fetchTree(path, pivot);
      if (currentPathRef.current === path) {
        setTree(treeKey, root);
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
  }, [currentPath, pivot, showsCategories, treeKey, setTree, clearTree]);

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
      if (event.code === 'KeyP' && !event.metaKey && !event.ctrlKey) {
        event.preventDefault();
        const index = PROJECTIONS.findIndex((p) => p.id === projection);
        setProjection(PROJECTIONS[(index + 1) % PROJECTIONS.length].id);
      } else if (event.code === 'KeyO' && selectedNode) {
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
  }, [
    deleteCandidate,
    requestDelete,
    revealSelected,
    selectedNode,
    status.deleteAllowed,
    projection,
    setProjection,
  ]);

  const labels = useMemo(
    () => new Map((categories?.categories ?? []).map((c) => [c.id, c.label])),
    [categories],
  );
  const now = useMemo(() => Date.now() / 1000, [treeRoot]);

  const colorFor = useMemo(() => {
    switch (projection) {
      case 'cleanup':
        return (rect: TreeLayoutRect) => cleanupColor(rect.node, effectiveApparent);
      case 'age':
        return (rect: TreeLayoutRect) => ageColor(rect.node.mtime, now);
      case 'categories':
        return (rect: TreeLayoutRect) => {
          const category = rect.colorPath.slice(PIVOT_PATH_PREFIX.length) as Category;
          return CATEGORY_COLORS[category] ?? NEUTRAL_COLOR;
        };
      default:
        return undefined;
    }
  }, [projection, effectiveApparent, now]);

  const muted = useMemo(
    () =>
      showsCategories && categoryFilter
        ? (node: Node) => !isPivotNode(node) && !containsCategory(node, categoryFilter)
        : undefined,
    [showsCategories, categoryFilter],
  );

  const tooltipDetail = useMemo(() => {
    if (projection === 'location') {
      return undefined;
    }
    if (projection === 'age') {
      return (node: Node) => ageText(node.mtime, now);
    }
    return (node: Node) => {
      if (isPivotNode(node)) {
        return null;
      }
      const summary = candidateSummary(node, effectiveApparent, useSIPrefix, labels, now);
      return [summary, node.hint].filter(Boolean).join(' — ') || null;
    };
  }, [projection, effectiveApparent, useSIPrefix, labels, now]);

  // Synthetic category nodes of the pivoted tree are not items: they can be
  // neither selected for an action nor opened.
  const selectNode = useCallback((node: Node) => {
    if (!isPivotNode(node)) {
      setSelectedNode(node);
    }
  }, []);
  const openPath = useCallback(
    (path: string) => {
      if (!path.startsWith(PIVOT_PATH_PREFIX)) {
        navigateToPath(path);
      }
    },
    [navigateToPath],
  );

  return (
    <>
      <section className={`chart-panel ${projection === 'location' ? '' : 'with-legend'}`}>
        <CategoryLegend
          projection={projection}
          categories={categories}
          apparent={effectiveApparent}
          useSIPrefix={useSIPrefix}
          filter={categoryFilter}
          onFilter={setCategoryFilter}
        />
        {treeRoot && treePath === treeKey ? (
          <TreeMap
            root={treeRoot}
            apparent={effectiveApparent}
            useSIPrefix={useSIPrefix}
            colorMap={colorMap}
            hoveredPath={hoveredPath}
            selectedPath={selectedNode?.path ?? null}
            onHover={setHoveredPath}
            onSelect={selectNode}
            onOpen={openPath}
            colorFor={colorFor}
            muted={muted}
            tooltipDetail={tooltipDetail}
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
