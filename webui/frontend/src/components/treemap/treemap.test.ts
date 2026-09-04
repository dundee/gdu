import { describe, expect, it } from 'vitest';
import { squarify } from './treemap';

describe('squarify', () => {
  it('tiles the full box with no gaps or overlaps in area', () => {
    const items = [
      { value: 50, item: 'a' },
      { value: 30, item: 'b' },
      { value: 20, item: 'c' },
    ];
    const rects = squarify(items, 0, 0, 100, 50);
    const totalArea = rects.reduce((acc, r) => acc + r.w * r.h, 0);
    expect(totalArea).toBeCloseTo(100 * 50, 5);
    expect(rects.map((r) => r.item)).toEqual(['a', 'b', 'c']);
  });

  it('returns nothing for an empty or zero-value input', () => {
    expect(squarify([], 0, 0, 100, 50)).toEqual([]);
    expect(squarify([{ value: 0, item: 'a' }], 0, 0, 100, 50)).toEqual([]);
  });

  it('returns nothing for a degenerate box', () => {
    expect(squarify([{ value: 10, item: 'a' }], 0, 0, 0, 50)).toEqual([]);
  });
});
