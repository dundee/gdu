import { useCallback, useState } from 'react';
import type { Node } from './types';
import { deleteNode, revealNode, type DeleteMode } from './api';
import { useGduModel } from './model';

// Shown instead of firing a request that is certain to come back as a bare
// 403. The token is missing whenever this tab never received one - the page
// was opened without the token gdu put in the URL, or session storage is
// unavailable - and no API call can recover it.
const noActionTokenMessage =
  'Actions are unavailable: this page has no action token. Reopen the URL gdu printed, or restart gdu.';

export interface ItemActionsOptions {
  // Refreshes view-specific data after a mutation. Defaults to invalidating
  // the cached recursive tree, so the treemap re-fetches the next time it is
  // shown. The treemap passes a function that re-fetches it right away.
  refreshExtra?: () => Promise<void>;
  // Runs after a successful refresh, e.g. to clear a view-local selection.
  onRefreshed?: () => void;
}

export interface ItemActions {
  actionPending: boolean;
  deleteCandidate: Node | null;
  skipDeleteChoice: boolean;
  setSkipDeleteChoice: (value: boolean) => void;
  // Starts the delete workflow for a node: deletes right away when the user
  // opted out of confirmation this session, otherwise opens the modal.
  requestDelete: (node: Node) => void;
  cancelDelete: () => void;
  performDelete: (node: Node, mode: DeleteMode) => Promise<void>;
  reveal: (node: Node) => Promise<void>;
}

// useItemActions is the delete/reveal workflow shared by every view that can
// act on a single item (the treemap and the file table). The confirmation
// modal is rendered by DeleteConfirmModal from the state returned here.
export function useItemActions(options: ItemActionsOptions = {}): ItemActions {
  const { refreshExtra, onRefreshed } = options;
  const {
    actionToken,
    status,
    skipDeleteConfirm,
    setHoveredPath,
    navigateToPath,
    refreshNode,
    clearTree,
    setLoadError,
  } = useGduModel();

  const [deleteCandidate, setDeleteCandidate] = useState<Node | null>(null);
  const [skipDeleteChoice, setSkipDeleteChoice] = useState(false);
  const [actionPending, setActionPending] = useState(false);

  const refreshAfterMutation = useCallback(async () => {
    const [nodeResp] = await Promise.all([
      refreshNode(),
      refreshExtra ? refreshExtra() : Promise.resolve(clearTree()),
    ]);
    onRefreshed?.();
    setHoveredPath(null);
    // Deleting the last item leaves the current directory empty: staying
    // put would show a blank chart with no way back except the breadcrumbs,
    // so step up to the parent directory instead.
    if (nodeResp.children.length === 0 && nodeResp.breadcrumbs.length > 1) {
      const parent = nodeResp.breadcrumbs[nodeResp.breadcrumbs.length - 2];
      navigateToPath(parent.path);
    }
  }, [refreshNode, refreshExtra, clearTree, onRefreshed, setHoveredPath, navigateToPath]);

  const performDelete = useCallback(
    async (node: Node, mode: DeleteMode) => {
      if (actionPending) {
        return;
      }
      if (!actionToken) {
        setDeleteCandidate(null);
        setLoadError(noActionTokenMessage);
        return;
      }
      setActionPending(true);
      setDeleteCandidate(null);
      try {
        await deleteNode(node.path, actionToken, mode);
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

  const reveal = useCallback(
    async (node: Node) => {
      if (actionPending) {
        return;
      }
      if (!actionToken) {
        setLoadError(noActionTokenMessage);
        return;
      }
      setActionPending(true);
      try {
        await revealNode(node.path, actionToken);
        setLoadError(null);
      } catch (err: unknown) {
        setLoadError(err instanceof Error ? err.message : String(err));
      } finally {
        setActionPending(false);
      }
    },
    [actionPending, setLoadError, actionToken],
  );

  const requestDelete = useCallback(
    (node: Node) => {
      if (actionPending || !status.deleteAllowed) {
        return;
      }
      if (skipDeleteConfirm) {
        void performDelete(node, skipDeleteConfirm);
        return;
      }
      setSkipDeleteChoice(false);
      setDeleteCandidate(node);
    },
    [actionPending, performDelete, skipDeleteConfirm, status.deleteAllowed],
  );

  const cancelDelete = useCallback(() => setDeleteCandidate(null), []);

  return {
    actionPending,
    deleteCandidate,
    skipDeleteChoice,
    setSkipDeleteChoice,
    requestDelete,
    cancelDelete,
    performDelete,
    reveal,
  };
}
