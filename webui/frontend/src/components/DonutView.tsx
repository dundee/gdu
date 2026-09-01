import { useGduModel } from '../model';
import { DonutChart } from './DonutChart';

export function DonutView() {
  const { children, effectiveApparent, useSIPrefix, hoveredPath, setHoveredPath, handleSelect } =
    useGduModel();

  return (
    <section className="chart-panel">
      <DonutChart
        children={children}
        apparent={effectiveApparent}
        useSIPrefix={useSIPrefix}
        hoveredPath={hoveredPath}
        onHover={setHoveredPath}
        onSelect={handleSelect}
      />
    </section>
  );
}
