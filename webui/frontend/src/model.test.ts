import { describe, expect, it } from 'vitest';
import type { Node } from './types';
import { deletionStateOf, freedBytes, type Deletion } from './model';

function entry(path: string, usage: number, state: Deletion['state'] = 'deleted'): [string, Deletion] {
  const node: Node = { name: path, path, isDir: true, size: usage / 2, usage, itemCount: 1, mtime: 0 };
  return [path, { state, node }];
}

describe('deletionStateOf', () => {
  it('covers the item and everything inside it, but not name prefixes', () => {
    const deletions = new Map([entry('/a/b', 1, 'pending')]);
    expect(deletionStateOf(deletions, '/a/b')).toBe('pending');
    expect(deletionStateOf(deletions, '/a/b/c')).toBe('pending');
    expect(deletionStateOf(deletions, '/a/bc')).toBeNull();
  });
});

describe('freedBytes', () => {
  it('sums confirmed deletions in the active metric', () => {
    const deletions = new Map([entry('/a', 100), entry('/b', 40), entry('/c', 999, 'pending')]);
    expect(freedBytes(deletions, false)).toBe(140);
    expect(freedBytes(deletions, true)).toBe(70);
  });

  it('counts a directory once when something inside it was deleted first', () => {
    const deletions = new Map([entry('/a/inner', 30), entry('/a', 100)]);
    expect(freedBytes(deletions, false)).toBe(100);
  });
});
