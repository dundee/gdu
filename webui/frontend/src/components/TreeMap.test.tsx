import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { TreeNode } from '../types';
import { TreeMap } from './TreeMap';

const nested: TreeNode = {
  name: 'nested.bin',
  path: '/tmp/root/folder/nested.bin',
  isDir: false,
  size: 70,
  usage: 70,
  itemCount: 1,
  mtime: 0,
  children: [],
};
const folder: TreeNode = {
  name: 'folder',
  path: '/tmp/root/folder',
  isDir: true,
  size: 70,
  usage: 70,
  itemCount: 4,
  mtime: 0,
  children: [nested],
};
const file: TreeNode = {
  name: 'file.txt',
  path: '/tmp/root/file.txt',
  isDir: false,
  size: 30,
  usage: 30,
  itemCount: 1,
  mtime: 0,
  children: [],
};
const root: TreeNode = {
  name: 'root',
  path: '/tmp/root',
  isDir: true,
  size: 100,
  usage: 100,
  itemCount: 5,
  mtime: 0,
  children: [folder, file],
};

afterEach(cleanup);

describe('TreeMap interaction', () => {
  it('selects files with one click and opens directories with a double click', () => {
    const onSelect = vi.fn();
    const onOpen = vi.fn();
    const { container } = render(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath={null}
        selectedPath={null}
        onHover={() => undefined}
        onSelect={onSelect}
        onOpen={onOpen}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /file\.txt/ }));
    expect(onSelect).toHaveBeenCalledWith(file);

    const rootTerritory = container.querySelector('.treemap-root');
    expect(rootTerritory).toBeTruthy();
    if (rootTerritory) {
      fireEvent.click(rootTerritory);
    }
    expect(onSelect).toHaveBeenCalledTimes(1);

    fireEvent.doubleClick(screen.getByRole('button', { name: /folder/ }));
    expect(onOpen).toHaveBeenCalledWith(folder.path);
  });

  it('opens the parent directory when double-clicking a file', () => {
    const onOpen = vi.fn();
    render(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath={null}
        selectedPath={null}
        onHover={() => undefined}
        onSelect={() => undefined}
        onOpen={onOpen}
      />,
    );

    fireEvent.doubleClick(screen.getByRole('button', { name: /file\.txt/ }));
    expect(onOpen).toHaveBeenCalledWith(root.path);

    fireEvent.doubleClick(screen.getByRole('button', { name: /nested\.bin/ }));
    expect(onOpen).toHaveBeenCalledWith(folder.path);
  });

  it('shows the full hover annotation and distinguishes hover from selection', () => {
    const { rerender } = render(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath={null}
        selectedPath="/tmp/root/file.txt"
        onHover={() => undefined}
        onSelect={() => undefined}
        onOpen={() => undefined}
      />,
    );

    const file = screen.getByRole('button', { name: /file\.txt/ });
    expect(file.classList.contains('selected')).toBe(true);

    rerender(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath="/tmp/root/folder"
        selectedPath="/tmp/root/file.txt"
        onHover={() => undefined}
        onSelect={() => undefined}
        onOpen={() => undefined}
      />,
    );

    expect(screen.getByRole('img', { name: 'Directory size treemap' }).classList.contains('hovering')).toBe(true);
    expect(screen.getByText('/tmp/root/folder')).toBeTruthy();
    expect(screen.getByText(/70 B · 70\.0% · 3 items/)).toBeTruthy();

    rerender(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath="/tmp/root/folder/nested.bin"
        selectedPath="/tmp/root/file.txt"
        onHover={() => undefined}
        onSelect={() => undefined}
        onOpen={() => undefined}
      />,
    );
    expect(screen.getByRole('button', { name: /folder/ }).classList.contains('ancestor')).toBe(true);
  });

  it('fills containers whose aspect ratio is not 3:2', () => {
    const { container } = render(
      <TreeMap
        root={root}
        apparent={false}
        useSIPrefix={false}
        colorMap={new Map()}
        hoveredPath={null}
        selectedPath={null}
        onHover={() => undefined}
        onSelect={() => undefined}
        onOpen={() => undefined}
      />,
    );

    expect(container.querySelector('svg')?.getAttribute('preserveAspectRatio')).toBe('none');
  });
});
