import { hierarchy, treemap } from 'd3-hierarchy';

export interface TreemapItem<T> {
  value: number;
  item: T;
}

export interface TreemapRect<T> {
  item: T;
  x: number;
  y: number;
  w: number;
  h: number;
}

interface LayoutDatum<T> {
  value?: number;
  item?: T;
  children?: LayoutDatum<T>[];
}

export function squarify<T>(
  items: TreemapItem<T>[],
  x: number,
  y: number,
  w: number,
  h: number,
): TreemapRect<T>[] {
  const total = items.reduce((acc, it) => acc + it.value, 0);
  if (!total || w <= 0 || h <= 0) {
    return [];
  }

  const root = hierarchy<LayoutDatum<T>>({
    children: items.map(({ value, item }) => ({ value, item })),
  }).sum((datum) => datum.value ?? 0);
  const laidOut = treemap<LayoutDatum<T>>().size([w, h]).padding(0)(root);

  return laidOut.leaves().map((leaf) => ({
    item: leaf.data.item as T,
    x: x + leaf.x0,
    y: y + leaf.y0,
    w: leaf.x1 - leaf.x0,
    h: leaf.y1 - leaf.y0,
  }));
}
