import { metricValue } from '../../slices';
import { squarify } from './treemap';
import type { TreeNode } from '../../types';

export interface TreeLayoutOptions {
  width: number;
  height: number;
  apparent: boolean;
  minArea: number;
  maxRects: number;
  inset: number;
}

export interface TreeLayoutRect {
  node: TreeNode;
  x: number;
  y: number;
  w: number;
  h: number;
  depth: number;
  ancestorPaths: string[];
  colorPath: string;
}

interface Bounds {
  x: number;
  y: number;
  w: number;
  h: number;
}

export function layoutTree(
  root: TreeNode,
  options: TreeLayoutOptions,
): TreeLayoutRect[] {
  const output: TreeLayoutRect[] = [];

  const visit = (
    parent: TreeNode,
    bounds: Bounds,
    depth: number,
    ancestorPaths: string[],
    inheritedColorPath: string | null,
  ) => {
    if (output.length >= options.maxRects) {
      return;
    }
    const inset = Math.min(options.inset, bounds.w / 2, bounds.h / 2);
    const content = {
      x: bounds.x + inset,
      y: bounds.y + inset,
      w: Math.max(bounds.w - inset * 2, 0),
      h: Math.max(bounds.h - inset * 2, 0),
    };
    if (content.w <= 0 || content.h <= 0) {
      return;
    }

    // Zero-size children (empty files, or directories holding only empty
    // files) still get a sliver of layout weight instead of being dropped:
    // otherwise a directory containing nothing but empty files would render
    // as a totally blank tile, indistinguishable from a load in progress.
    const MIN_LAYOUT_VALUE = 1;
    const children = parent.children
      .map((child) => ({
        child,
        value: Math.max(metricValue(child, options.apparent), MIN_LAYOUT_VALUE),
      }))
      .sort((a, b) => b.value - a.value);
    const childrenTotal = children.reduce((total, child) => total + child.value, 0);
    const parentValue = metricValue(parent, options.apparent);
    const layoutTotal = Math.max(parentValue, childrenTotal);
    const residual = Math.max(layoutTotal - childrenTotal, 0);
    const items: Array<{ value: number; item: TreeNode | null }> = children.map(
      ({ child, value }) => ({ value, item: child }),
    );
    if (residual > 0) {
      items.push({ value: residual, item: null });
    }

    const rects = squarify(items, content.x, content.y, content.w, content.h);
    for (const rect of rects) {
      if (output.length >= options.maxRects) {
        break;
      }
      const child = rect.item;
      if (child === null || rect.w * rect.h < options.minArea) {
        continue;
      }
      const colorPath = inheritedColorPath ?? child.path;
      output.push({
        node: child,
        x: rect.x,
        y: rect.y,
        w: rect.w,
        h: rect.h,
        depth,
        ancestorPaths,
        colorPath,
      });
      if (child.isDir && child.children.length > 0) {
        visit(
          child,
          rect,
          depth + 1,
          [...ancestorPaths, child.path],
          colorPath,
        );
      }
    }
  };

  visit(
    root,
    { x: 0, y: 0, w: options.width, h: options.height },
    0,
    [root.path],
    null,
  );
  return output;
}
