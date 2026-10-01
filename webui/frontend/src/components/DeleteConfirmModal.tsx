import { useGduModel } from '../model';
import { formatSize } from '../format';
import type { ItemActions } from '../useItemActions';
import { Modal } from './Modal';

// Confirmation dialog for the delete workflow in useItemActions. Renders
// nothing until a delete has been requested.
export function DeleteConfirmModal({ actions }: { actions: ItemActions }) {
  const { effectiveApparent, useSIPrefix, setSkipDeleteConfirm } = useGduModel();
  const {
    deleteCandidate,
    skipDeleteChoice,
    setSkipDeleteChoice,
    actionPending,
    cancelDelete,
    performDelete,
  } = actions;

  if (!deleteCandidate) {
    return null;
  }

  return (
    <Modal titleId="delete-title" title="Delete item" onClose={cancelDelete}>
      <p>Move the selected item to the trash, or delete it permanently right away.</p>
      <code className="modal-path">{deleteCandidate.path}</code>
      <span className="muted">
        {formatSize(effectiveApparent ? deleteCandidate.size : deleteCandidate.usage, useSIPrefix)}
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
        <button type="button" autoFocus onClick={cancelDelete}>
          Cancel
        </button>
        <button
          type="button"
          disabled={actionPending}
          onClick={() => {
            if (skipDeleteChoice) {
              setSkipDeleteConfirm('trash');
            }
            void performDelete(deleteCandidate, 'trash');
          }}
        >
          Move to Trash
        </button>
        <button
          type="button"
          className="danger"
          disabled={actionPending}
          onClick={() => {
            if (skipDeleteChoice) {
              setSkipDeleteConfirm('permanent');
            }
            void performDelete(deleteCandidate, 'permanent');
          }}
        >
          Delete Permanently
        </button>
      </div>
    </Modal>
  );
}
