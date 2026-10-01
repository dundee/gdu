import { useGduModel } from '../model';
import { useItemActions } from '../useItemActions';
import { FileTable } from './FileTable';
import { DeleteConfirmModal } from './DeleteConfirmModal';

export function FileTableView() {
  const {
    status,
    children,
    colorMap,
    effectiveApparent,
    useSIPrefix,
    total,
    sort,
    order,
    onSortChange,
    hoveredPath,
    setHoveredPath,
    handleSelect,
  } = useGduModel();
  const actions = useItemActions();

  return (
    <section className="table-panel">
      <FileTable
        children={children}
        colorMap={colorMap}
        apparent={effectiveApparent}
        useSIPrefix={useSIPrefix}
        total={total}
        sort={sort}
        order={order}
        onSortChange={onSortChange}
        hoveredPath={hoveredPath}
        onHover={setHoveredPath}
        onSelect={handleSelect}
        onReveal={(node) => void actions.reveal(node)}
        onDelete={actions.requestDelete}
        canDelete={status.deleteAllowed}
        actionPending={actions.actionPending}
      />
      <DeleteConfirmModal actions={actions} />
    </section>
  );
}
