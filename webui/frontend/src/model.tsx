import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import type { Node, NodeResponse, SortKey, SortOrder, Status, TreeNode } from './types';
import { fetchNode, fetchStatus, subscribeStatus, type DeleteMode } from './api';
import { colorMapFor, computeSlices } from './slices';
import { useLatest } from './useLatest';
import { amountValue, type Projection } from './cleanup';

export type ChartView = 'donut' | 'treemap';

const actionTokenStorageKey = 'gdu.actionToken';
const deleteModeStorageKey = 'gdu.skipDeleteConfirm';

// sessionStorage can throw rather than return null (private-mode browsers,
// storage disabled by policy). Losing the token only costs the actions, so
// degrade to "no token" instead of taking the whole app down with it.
function readSessionStorage(key: string): string | null {
  try {
    return window.sessionStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeSessionStorage(key: string, value: string | null) {
  try {
    if (value === null) {
      window.sessionStorage.removeItem(key);
    } else {
      window.sessionStorage.setItem(key, value);
    }
  } catch {
    // Non-fatal: the preference simply will not survive a reload.
  }
}

// GduModel is the shared app-level data: scan status, the current
// directory's node data, and display preferences. Directory-specific
// features (the recursive tree, selection, delete/reveal) live in the view
// that needs them (see TreeMapView) and are not part of this model.
export interface GduModel {
  status: Status;
  // Per-process secret the server prints/opens the page with (see
  // actionTokenParam in webui/action_token.go), read from this page's own URL
  // and retained in tab-scoped session storage. Required on every
  // delete/reveal request; see api.ts.
  actionToken: string;
  currentPath: string;
  nodeResp: NodeResponse | null;
  children: Node[];
  effectiveApparent: boolean;
  useSIPrefix: boolean;
  total: number;
  colorMap: Map<string, string>;
  sort: SortKey;
  order: SortOrder;
  onSortChange: (key: SortKey) => void;
  view: ChartView;
  toggleView: () => void;
  // projection is how the treemap is colored (and, for "categories", shaped).
  // Kept here so it survives toggling the view.
  projection: Projection;
  setProjection: (projection: Projection) => void;
  toggleApparent: () => void;
  hoveredPath: string | null;
  setHoveredPath: (path: string | null) => void;
  loadError: string | null;
  setLoadError: (message: string | null) => void;
  handleSelect: (node: Node) => void;
  navigateToPath: (path: string) => void;
  refreshNode: () => Promise<NodeResponse>;
  showHelp: boolean;
  setShowHelp: (show: boolean) => void;
  // Recursive-tree cache, keyed by path. Lives here (rather than inside
  // TreeMapView) so it survives the view toggling donut/treemap without a
  // redundant re-fetch; TreeMapView owns fetching it and writes back here.
  treeRoot: TreeNode | null;
  treePath: string | null;
  setTree: (path: string, root: TreeNode | null) => void;
  // "Do not ask again this session" for delete confirmation, remembering
  // which of the two delete modes to repeat without asking. Lives here
  // (rather than in TreeMapView) so it survives the view toggling
  // donut/treemap, which unmounts TreeMapView and would otherwise reset it.
  skipDeleteConfirm: DeleteMode | null;
  setSkipDeleteConfirm: (mode: DeleteMode | null) => void;
  // Optimistic deletion: items whose deletion is running ("pending") or was
  // done in the current directory ("deleted"). Deleted items keep their place
  // and are only marked, so the layout does not jump under the cursor; the
  // directory is reloaded once the user navigates. Lives here so every view
  // sees the same state.
  deletions: ReadonlyMap<string, Deletion>;
  // Requests still running. Navigating meanwhile keeps their items marked
  // wherever they show up; an item being deleted cannot be opened.
  deletionsInFlight: number;
  beginDeletion: (node: Node) => void;
  // settleDeletion records a finished request; a failed one (error set) is
  // dropped from deletions so its item is drawn normally again. Once no
  // request is running it returns the errors of the whole batch, otherwise
  // null.
  settleDeletion: (path: string, error: string | null) => string[] | null;
}

export type DeletionState = 'pending' | 'deleted';

export interface Deletion {
  state: DeletionState;
  node: Node;
}

// deletionStateOf tells whether path, or a directory containing it, is being
// or has been deleted.
export function deletionStateOf(
  deletions: ReadonlyMap<string, Deletion>,
  path: string,
): DeletionState | null {
  if (deletions.size === 0) {
    return null;
  }
  const own = deletions.get(path);
  if (own) {
    return own.state;
  }
  for (const [deleted, { state }] of deletions) {
    const separator = path[deleted.length];
    if (path.startsWith(deleted) && (separator === '/' || separator === '\\')) {
      return state;
    }
  }
  return null;
}

// freedBytes is the space freed by the confirmed deletions. An item deleted
// after something inside it was is counted once, with the size it had when
// the user deleted it.
export function freedBytes(deletions: ReadonlyMap<string, Deletion>, apparent: boolean): number {
  let total = 0;
  for (const [path, { state, node }] of deletions) {
    if (state !== 'deleted') {
      continue;
    }
    const parent = path.slice(0, Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')));
    if (parent && deletionStateOf(deletions, parent) === 'deleted') {
      continue;
    }
    total += amountValue(node, apparent);
  }
  return total;
}

const GduModelContext = createContext<GduModel | null>(null);

export function useGduModel(): GduModel {
  const model = useContext(GduModelContext);
  if (!model) {
    throw new Error('useGduModel must be used within GduModelProvider');
  }
  return model;
}

export function GduModelProvider({ model, children }: { model: GduModel; children: ReactNode }) {
  return <GduModelContext.Provider value={model}>{children}</GduModelContext.Provider>;
}

// useGduModelState owns the top-level state and effects (SSE status, node
// fetching) and returns the raw hook result. App calls it directly so it can
// still render the pre-scan loading/error/progress screens before a
// GduModelProvider (which needs a non-null status/currentPath) makes sense.
export function useGduModelState() {
  // index.html strips a fresh token from the URL before loading the application
  // bundle and retains it in tab-scoped storage so reloads keep actions working.
  const [actionToken] = useState(() => readSessionStorage(actionTokenStorageKey) ?? '');
  const [status, setStatus] = useState<Status | null>(null);
  const [currentPath, setCurrentPath] = useState<string | null>(null);
  const [nodeResp, setNodeResp] = useState<NodeResponse | null>(null);
  const [sort, setSort] = useState<SortKey>('size');
  const [order, setOrder] = useState<SortOrder>('desc');
  const [apparent, setApparent] = useState<boolean | null>(null);
  const [view, setView] = useState<ChartView>('donut');
  const [projection, setProjection] = useState<Projection>('location');
  const [hoveredPath, setHoveredPath] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const [treeRoot, setTreeRoot] = useState<TreeNode | null>(null);
  const [treePath, setTreePath] = useState<string | null>(null);
  const [deletions, setDeletions] = useState<ReadonlyMap<string, Deletion>>(() => new Map());
  const [deletionsInFlight, setDeletionsInFlight] = useState(0);
  const inFlightRef = useRef(0);
  // Set once anything was deleted: the shown data is stale from then on and
  // is reloaded on the next navigation.
  const staleRef = useRef(false);
  const deletionsRef = useLatest(deletions);
  const [reloadKey, setReloadKey] = useState(0);
  const deletionErrorsRef = useRef<string[]>([]);
  const [skipDeleteConfirm, setSkipDeleteConfirmState] = useState<DeleteMode | null>(() => {
    const mode = readSessionStorage(deleteModeStorageKey);
    return mode === 'trash' || mode === 'permanent' ? mode : null;
  });

  // Tracks the latest currentPath so an in-flight refreshNode() call (e.g.
  // one started before a breadcrumb navigation) can tell its result is
  // stale once it resolves, instead of unconditionally overwriting nodeResp
  // with data for a directory the user has since navigated away from.
  const currentPathRef = useLatest(currentPath);

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
  }, [currentPath, sort, order, status?.state, reloadKey]);

  // Data loaded while the classification ran lacks cleanup candidates:
  // reload it once the classification is done.
  const classifying = status?.classifying ?? false;
  const wasClassifyingRef = useRef(classifying);
  useEffect(() => {
    if (wasClassifyingRef.current && !classifying) {
      setTreePath(null);
      setReloadKey((k) => k + 1);
    }
    wasClassifyingRef.current = classifying;
  }, [classifying]);

  // Global "?" shortcut to open the help overlay; works from any view.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) {
        return;
      }
      if (event.key === '?') {
        event.preventDefault();
        setShowHelp(true);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

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

  const onSortChange = useCallback(
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

  const navigateToPath = useCallback((path: string) => {
    // the contents of an item being deleted are going away
    if (deletionsRef.current && deletionStateOf(deletionsRef.current, path)) {
      return;
    }
    if (staleRef.current) {
      // running deletions stay marked in the reloaded data, which is stale
      // again once they finish
      staleRef.current = inFlightRef.current > 0;
      setDeletions((prev) => new Map([...prev].filter(([, d]) => d.state === 'pending')));
      setTreePath(null);
      setTreeRoot(null);
      // reload even when navigating to the directory already shown
      setReloadKey((k) => k + 1);
    }
    setCurrentPath(path);
    setHoveredPath(null);
  }, [deletionsRef]);

  const beginDeletion = useCallback((node: Node) => {
    inFlightRef.current += 1;
    staleRef.current = true;
    setDeletionsInFlight(inFlightRef.current);
    setDeletions((prev) => new Map(prev).set(node.path, { state: 'pending', node }));
  }, []);

  const settleDeletion = useCallback((path: string, error: string | null) => {
    inFlightRef.current = Math.max(inFlightRef.current - 1, 0);
    setDeletionsInFlight(inFlightRef.current);
    setDeletions((prev) => {
      const next = new Map(prev);
      const entry = next.get(path);
      if (error !== null) {
        next.delete(path);
      } else if (entry) {
        next.set(path, { ...entry, state: 'deleted' });
      }
      return next;
    });
    if (error === null) {
      staleRef.current = true;
    }
    if (error !== null) {
      deletionErrorsRef.current.push(error);
    }
    if (inFlightRef.current > 0) {
      return null;
    }
    const errors = deletionErrorsRef.current;
    deletionErrorsRef.current = [];
    return errors;
  }, []);


  const handleSelect = useCallback(
    (node: Node) => {
      if (node.isDir) {
        navigateToPath(node.path);
      }
    },
    [navigateToPath],
  );

  const toggleView = useCallback(() => {
    setView((v) => (v === 'donut' ? 'treemap' : 'donut'));
    setHoveredPath(null);
  }, []);

  const toggleApparent = useCallback(() => {
    setApparent((a) => !(a ?? status?.showApparentSize ?? false));
  }, [status?.showApparentSize]);

  const refreshNode = useCallback(async () => {
    if (currentPath === null) {
      throw new Error('no current path');
    }
    const path = currentPath;
    const resp = await fetchNode(path, sort, order);
    if (currentPathRef.current === path) {
      setNodeResp(resp);
      setLoadError(null);
    }
    return resp;
  }, [currentPath, sort, order]);

  const setTree = useCallback((path: string, root: TreeNode | null) => {
    setTreePath(path);
    setTreeRoot(root);
  }, []);

  const setSkipDeleteConfirm = useCallback((mode: DeleteMode | null) => {
    setSkipDeleteConfirmState(mode);
    writeSessionStorage(deleteModeStorageKey, mode);
  }, []);

  return {
    status,
    actionToken,
    currentPath,
    nodeResp,
    children,
    effectiveApparent,
    useSIPrefix,
    total,
    colorMap,
    sort,
    order,
    onSortChange,
    view,
    toggleView,
    projection,
    setProjection,
    toggleApparent,
    hoveredPath,
    setHoveredPath,
    loadError,
    setLoadError,
    handleSelect,
    navigateToPath,
    refreshNode,
    showHelp,
    setShowHelp,
    treeRoot,
    treePath,
    setTree,
    skipDeleteConfirm,
    setSkipDeleteConfirm,
    deletions,
    deletionsInFlight,
    beginDeletion,
    settleDeletion,
  };
}
