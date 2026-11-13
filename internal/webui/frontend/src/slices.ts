import type { Node } from './types';
import { OTHER_COLOR, colorAt } from './colors';

// Maximum number of individually colored slices/rows before the remainder is
// aggregated into a single "Other" bucket.
export const MAX_SLICES = 11;

// metricValue returns the number a node contributes to charts/bars: disk usage
// by default, apparent size when the user toggles it.
export function metricValue(node: Node, showApparent: boolean): number {
  return Math.max(showApparent ? node.size : node.usage, 0);
}

export interface Slice {
  key: string;
  label: string;
  value: number;
  color: string;
  node?: Node;
}

// colorMapFor assigns a stable color to each child by descending metric rank so
// that donut slices and table rows always agree regardless of table sort order.
export function colorMapFor(children: Node[], showApparent: boolean): Map<string, string> {
  const map = new Map<string, string>();
  const ranked = [...children].sort(
    (a, b) => metricValue(b, showApparent) - metricValue(a, showApparent),
  );
  ranked.forEach((child, index) => {
    map.set(child.path, index < MAX_SLICES ? colorAt(index) : OTHER_COLOR);
  });
  return map;
}

// computeSlices builds the donut slices for a directory's children, aggregating
// everything beyond MAX_SLICES into a single "Other" slice. Slice colors are
// assigned by descending metric rank, matching colorMapFor.
export function computeSlices(
  children: Node[],
  showApparent: boolean,
): { slices: Slice[]; total: number } {
  const withValue = children
    .map((n) => ({ node: n, value: metricValue(n, showApparent) }))
    .filter((s) => s.value > 0)
    .sort((a, b) => b.value - a.value);

  const total = withValue.reduce((acc, s) => acc + s.value, 0);

  const head = withValue.slice(0, MAX_SLICES);
  const tail = withValue.slice(MAX_SLICES);

  const slices: Slice[] = head.map((s, index) => ({
    key: s.node.path,
    label: s.node.name,
    value: s.value,
    color: colorAt(index),
    node: s.node,
  }));

  if (tail.length > 0) {
    slices.push({
      key: '__other__',
      label: `Other (${tail.length})`,
      value: tail.reduce((acc, s) => acc + s.value, 0),
      color: OTHER_COLOR,
    });
  }

  return { slices, total };
}
