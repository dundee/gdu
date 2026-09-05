import { describe, expect, it } from 'vitest';
import type { TreeNode } from '../../types';
import { layoutTree } from './treeLayout';

function node(
  name: string,
  usage: number,
  children: TreeNode[] = [],
): TreeNode {
  return {
    name,
    path: `/root/${name}`,
    isDir: children.length > 0,
    size: usage,
    usage,
    itemCount: children.length,
    mtime: 0,
    children,
  };
}

const tiny = node('tiny', 10);
const big = node('big', 70);
const directory = node('directory', 80, [big, tiny]);
const sibling = node('sibling', 20);
const root: TreeNode = {
  ...node('root', 100, [directory, sibling]),
  path: '/root',
};

describe('layoutTree', () => {
  it('leaves sub-pixel details as parent territory', () => {
    const rects = layoutTree(root, {
      width: 100,
      height: 100,
      apparent: false,
      minArea: 1500,
      maxRects: 2000,
      inset: 0,
    });

    expect(rects.map((rect) => rect.node.name)).toEqual([
      'directory',
      'big',
      'sibling',
    ]);
    const bigRect = rects.find((rect) => rect.node === big);
    expect(bigRect).toBeDefined();
    if (!bigRect) {
      throw new Error('big rectangle missing');
    }
    expect(bigRect.w * bigRect.h).toBeCloseTo(7000, 5);
    expect(bigRect.ancestorPaths).toEqual(['/root', '/root/directory']);
  });

  it('adapts visibility to viewport area', () => {
    const small = layoutTree(root, {
      width: 100,
      height: 100,
      apparent: false,
      minArea: 1500,
      maxRects: 2000,
      inset: 0,
    });
    const large = layoutTree(root, {
      width: 200,
      height: 200,
      apparent: false,
      minArea: 1500,
      maxRects: 2000,
      inset: 0,
    });

    expect(small.some((rect) => rect.node === tiny)).toBe(false);
    expect(large.some((rect) => rect.node === tiny)).toBe(true);
  });

  it('still renders a lone zero-size file instead of leaving the directory blank', () => {
    const emptyFile = node('empty.txt', 0);
    const emptyDir: TreeNode = { ...node('root', 0, [emptyFile]), path: '/root' };

    const rects = layoutTree(emptyDir, {
      width: 100,
      height: 100,
      apparent: false,
      minArea: 1,
      maxRects: 2000,
      inset: 0,
    });

    expect(rects.map((rect) => rect.node.name)).toEqual(['empty.txt']);
    expect(rects[0].w * rects[0].h).toBeCloseTo(10000, 5);
  });

  it('never exceeds the DOM rectangle limit', () => {
    const children = Array.from({ length: 3000 }, (_, index) => node(`file-${index}`, 1));
    const many: TreeNode = {
      ...node('many', children.length, children),
      path: '/root',
    };

    const rects = layoutTree(many, {
      width: 1000,
      height: 1000,
      apparent: false,
      minArea: 0,
      maxRects: 50,
      inset: 0,
    });

    expect(rects).toHaveLength(50);
  });
});
