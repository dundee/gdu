import { useGduModel } from '../model';
import { FileTable } from './FileTable';

export function FileTableView() {
  const {
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
      />
    </section>
  );
}
