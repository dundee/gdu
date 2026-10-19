import { useCallback, useState } from 'react';
import type { Node } from './types';
import { deleteNode, revealNode, type DeleteMode } from './api';
import { isBeingDeleted, useGduModel } from './model';
import { useLatest } from './useLatest';

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
  // actionPending is true while a reveal runs. Deletions never block: they
  // are optimistic, see performDelete.
  actionPending: boolean;
  deleteCandidate: Node | null;
  skipDeleteChoice: boolean;
  setSkipDeleteChoice: (value: boolean) => void;
  // Starts the delete workflow for a node: deletes right away when the user
  // opted out of confirmation this session, otherwise opens the modal.
  requestDelete: (node: Node) => void;
  cancelDelete: () => void;
  // performDelete marks the item as being deleted right away and sends the
  // request in the background, so more items can be deleted meanwhile. A
  // failed deletion puts the item back and reports why. Once the last
  // running request finishes, the data is refreshed once.
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
    deleting,
    beginDeletion,
    settleDeletion,
    clearDeleting,
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
    return nodeResp;
  }, [refreshNode, refreshExtra, clearTree, onRefreshed, setHoveredPath]);
  // A deletion can outlive the render that started it; settle it with the
  // latest refresh so the view that is shown when it finishes is updated.
  const refreshRef = useLatest(refreshAfterMutation);

  // finishDeletions refreshes once after a batch of deletions. Errors are
  // shown after the refresh, which would otherwise clear them.
  const finishDeletions = useCallback(async (errors: string[]) => {
    try {
      const refresh = refreshRef.current ?? refreshAfterMutation;
      const nodeResp = await refresh();
      clearDeleting();
      if (errors.length > 0) {
        setLoadError(errors.join('\n'));
      }
      // Deleting the last item leaves the current directory empty: staying
      // put would show a blank chart with no way back except the breadcrumbs,
      // so step up to the parent directory instead.
      if (nodeResp.children.length === 0 && nodeResp.breadcrumbs.length > 1) {
        const parent = nodeResp.breadcrumbs[nodeResp.breadcrumbs.length - 2];
        navigateToPath(parent.path);
      }
    } catch (err: unknown) {
      clearDeleting();
      setLoadError(err instanceof Error ? err.message : String(err));
    }
  }, [refreshRef, refreshAfterMutation, clearDeleting, navigateToPath, setLoadError]);

  const performDelete = useCallback(
    async (node: Node, mode: DeleteMode) => {
      setDeleteCandidate(null);
      if (!actionToken) {
        setLoadError(noActionTokenMessage);
        return;
      }
      if (isBeingDeleted(deleting, node.path)) {
        return;
      }
      beginDeletion(node.path);
      let error: string | null = null;
      try {
        await deleteNode(node.path, actionToken, mode);
      } catch (err: unknown) {
        error = `Could not delete ${node.name}: ${err instanceof Error ? err.message : String(err)}`;
        // show it right away; it is shown again after the batch refresh
        setLoadError(error);
      }
      const errors = settleDeletion(node.path, error);
      if (errors) {
        await finishDeletions(errors);
      }
    },
    [actionToken, deleting, beginDeletion, settleDeletion, finishDeletions, setLoadError],
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
      if (!status.deleteAllowed || isBeingDeleted(deleting, node.path)) {
        return;
      }
      if (skipDeleteConfirm) {
        void performDelete(node, skipDeleteConfirm);
        return;
      }
      setSkipDeleteChoice(false);
      setDeleteCandidate(node);
    },
    [deleting, performDelete, skipDeleteConfirm, status.deleteAllowed],
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
