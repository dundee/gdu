import { useGduModel } from '../model';
import { Modal } from './Modal';

export function HelpModal() {
  const { showHelp, setShowHelp, view } = useGduModel();
  if (!showHelp) {
    return null;
  }

  return (
    <Modal
      titleId="shortcuts-title"
      title="Keyboard shortcuts"
      className="shortcuts-modal"
      onClose={() => setShowHelp(false)}
    >
      <dl className="shortcuts">
        {view === 'treemap' && (
          <>
            <dt>O</dt>
            <dd>Reveal selected item</dd>
            <dt>D / Delete</dt>
            <dd>Delete selected item</dd>
            <dt>Click</dt>
            <dd>Select a treemap item</dd>
            <dt>Double-click</dt>
            <dd>Open a directory</dd>
          </>
        )}
        <dt>?</dt>
        <dd>Show this help</dd>
        <dt>Esc</dt>
        <dd>Close a dialog</dd>
      </dl>
      <div className="modal-actions">
        <button type="button" autoFocus onClick={() => setShowHelp(false)}>
          Close
        </button>
      </div>
    </Modal>
  );
}
