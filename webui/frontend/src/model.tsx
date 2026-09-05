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

export type ChartView = 'donut' | 'treemap';

// GduModel is the shared app-level data: scan status, the current
// directory's node data, and display preferences. Directory-specific
// features (the recursive tree, selection, delete/reveal) live in the view
// that needs them (see TreeMapView) and are not part of this model.
export interface GduModel {
  status: Status;
  // Per-process secret the server prints/opens the page with (see
  // actionTokenParam in webui/action_token.go), read once from this page's
  // own URL rather than from any API response. Required on every
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
  // Invalidates the cached tree (clears both treeRoot and treePath) so a
  // failed refresh does not leave a stale tree displayed as current, and the
  // loader effect above treats the path as not-yet-loaded, eligible for a
  // retry.
  clearTree: () => void;
  // "Do not ask again this session" for delete confirmation, remembering
  // which of the two delete modes to repeat without asking. Lives here
  // (rather than in TreeMapView) so it survives the view toggling
  // donut/treemap, which unmounts TreeMapView and would otherwise reset it.
  skipDeleteConfirm: DeleteMode | null;
  setSkipDeleteConfirm: (mode: DeleteMode | null) => void;
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
  // Read once at mount: the token lives only in this page's own URL (see
  // GduModel.actionToken), not in any state the server pushes afterwards.
  // Strip it from the visible address bar immediately after reading it, so
  // it does not linger somewhere a shoulder-surfer, browser history entry,
  // or copy-pasted URL could pick it up.
  const [actionToken] = useState(() => {
    const url = new URL(window.location.href);
    const token = url.searchParams.get('token') ?? '';
    if (url.searchParams.has('token')) {
      url.searchParams.delete('token');
      window.history.replaceState(window.history.state, '', url);
    }
    return token;
  });
  const [status, setStatus] = useState<Status | null>(null);
  const [currentPath, setCurrentPath] = useState<string | null>(null);
  const [nodeResp, setNodeResp] = useState<NodeResponse | null>(null);
  const [sort, setSort] = useState<SortKey>('size');
  const [order, setOrder] = useState<SortOrder>('desc');
  const [apparent, setApparent] = useState<boolean | null>(null);
  const [view, setView] = useState<ChartView>('donut');
  const [hoveredPath, setHoveredPath] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const [treeRoot, setTreeRoot] = useState<TreeNode | null>(null);
  const [treePath, setTreePath] = useState<string | null>(null);
  const [skipDeleteConfirm, setSkipDeleteConfirm] = useState<DeleteMode | null>(null);

  // Tracks the latest currentPath so an in-flight refreshNode() call (e.g.
  // one started before a breadcrumb navigation) can tell its result is
  // stale once it resolves, instead of unconditionally overwriting nodeResp
  // with data for a directory the user has since navigated away from.
  const currentPathRef = useRef(currentPath);
  useEffect(() => {
    currentPathRef.current = currentPath;
  }, [currentPath]);

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
    setCurrentPath(path);
    setHoveredPath(null);
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

  const clearTree = useCallback(() => {
    setTreePath(null);
    setTreeRoot(null);
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
    clearTree,
    skipDeleteConfirm,
    setSkipDeleteConfirm,
  };
}
