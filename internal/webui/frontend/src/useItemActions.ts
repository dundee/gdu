import { useCallback, useState } from 'react';
import type { Node } from './types';
import { deleteNode, revealNode, type DeleteMode } from './api';
import { deletionStateOf, useGduModel } from './model';

// Shown instead of firing a request that is certain to come back as a bare
// 403. The token is missing whenever this tab never received one - the page
// was opened without the token gdu put in the URL, or session storage is
// unavailable - and no API call can recover it.
const noActionTokenMessage =
  'Actions are unavailable: this page has no action token. Reopen the URL gdu printed, or restart gdu.';

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
  // deleted item stays in place, marked, until the user navigates; a failed
  // deletion puts the item back and reports why.
  performDelete: (node: Node, mode: DeleteMode) => Promise<void>;
  reveal: (node: Node) => Promise<void>;
}

// useItemActions is the delete/reveal workflow shared by every view that can
// act on a single item (the treemap and the file table). The confirmation
// modal is rendered by DeleteConfirmModal from the state returned here.
export function useItemActions(): ItemActions {
  const {
    actionToken,
    status,
    skipDeleteConfirm,
    setLoadError,
    deletions,
    beginDeletion,
    settleDeletion,
  } = useGduModel();

  const [deleteCandidate, setDeleteCandidate] = useState<Node | null>(null);
  const [skipDeleteChoice, setSkipDeleteChoice] = useState(false);
  const [actionPending, setActionPending] = useState(false);

  const performDelete = useCallback(
    async (node: Node, mode: DeleteMode) => {
      setDeleteCandidate(null);
      if (!actionToken) {
        setLoadError(noActionTokenMessage);
        return;
      }
      if (deletionStateOf(deletions, node.path)) {
        return;
      }
      beginDeletion(node);
      let error: string | null = null;
      try {
        await deleteNode(node.path, actionToken, mode);
      } catch (err: unknown) {
        error = `Could not delete ${node.name}: ${err instanceof Error ? err.message : String(err)}`;
      }
      const errors = settleDeletion(node.path, error);
      if (errors && errors.length > 0) {
        setLoadError(errors.join('\n'));
      } else if (error) {
        setLoadError(error);
      }
    },
    [actionToken, deletions, beginDeletion, settleDeletion, setLoadError],
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
      if (!status.deleteAllowed || deletionStateOf(deletions, node.path)) {
        return;
      }
      if (skipDeleteConfirm) {
        void performDelete(node, skipDeleteConfirm);
        return;
      }
      setSkipDeleteChoice(false);
      setDeleteCandidate(node);
    },
    [deletions, performDelete, skipDeleteConfirm, status.deleteAllowed],
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
