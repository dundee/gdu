import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import * as api from './api';
import type { NodeResponse, Status, TreeNode } from './types';

vi.mock('./api');

const root = '/tmp/root';
const status: Status = {
  state: 'done',
  rootPath: root,
  progress: { currentItem: '', itemCount: 2, totalUsage: 100 },
  showApparentSize: false,
  showRelativeSize: false,
  useSIPrefix: false,
};
const nodeResponse: NodeResponse = {
  node: {
    name: 'root',
    path: root,
    isDir: true,
    size: 100,
    usage: 100,
    itemCount: 2,
    mtime: 0,
  },
  breadcrumbs: [],
  children: [
    {
      name: 'folder',
      path: `${root}/folder`,
      isDir: true,
      size: 70,
      usage: 70,
      itemCount: 1,
      mtime: 0,
    },
    {
      name: 'file.txt',
      path: `${root}/file.txt`,
      isDir: false,
      size: 30,
      usage: 30,
      itemCount: 1,
      mtime: 0,
    },
  ],
};
const treeResponse: TreeNode = {
  ...nodeResponse.node,
  children: [
    {
      ...nodeResponse.children[0],
      children: [
        {
          name: 'nested.bin',
          path: `${root}/folder/nested.bin`,
          isDir: false,
          size: 50,
          usage: 50,
          itemCount: 1,
          mtime: 0,
          children: [],
        },
      ],
    },
    { ...nodeResponse.children[1], children: [] },
  ],
};

beforeEach(() => {
  vi.mocked(api.fetchStatus).mockResolvedValue(status);
  vi.mocked(api.fetchNode).mockResolvedValue(nodeResponse);
  vi.mocked(api.fetchTree).mockResolvedValue(treeResponse);
  vi.mocked(api.subscribeStatus).mockReturnValue(() => undefined);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('chart view', () => {
  it('keeps the donut table and switches to a full treemap with an inverse label', async () => {
    render(<App />);

    const toggle = await screen.findByRole('button', { name: 'Treemap' });
    expect(screen.getByRole('table')).toBeTruthy();
    expect(api.fetchTree).not.toHaveBeenCalled();

    fireEvent.click(toggle);

    await waitFor(() => expect(screen.getByRole('button', { name: 'Donut' })).toBeTruthy());
    await waitFor(() => expect(api.fetchTree).toHaveBeenCalledWith(root));
    expect(screen.getByRole('img', { name: 'Directory size treemap' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /nested\.bin/ })).toBeTruthy();
    expect(screen.queryByRole('table')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Donut' }));
    fireEvent.click(screen.getByRole('button', { name: 'Treemap' }));
    expect(api.fetchTree).toHaveBeenCalledTimes(1);
  });
});

describe('treemap navigation', () => {
  it('opens shortcut help with question mark', async () => {
    render(<App />);
    await screen.findByRole('button', { name: 'Treemap' });

    fireEvent.keyDown(window, { key: '?' });

    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeTruthy();
  });

  it('opens a directory only on double-click', async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    const folder = await screen.findByRole('button', { name: /folder/ });

    fireEvent.click(folder);
    expect(api.fetchNode).toHaveBeenCalledTimes(1);

    fireEvent.doubleClick(folder);
    await waitFor(() =>
      expect(api.fetchNode).toHaveBeenLastCalledWith(`${root}/folder`, 'size', 'desc'),
    );
    await waitFor(() => expect(api.fetchTree).toHaveBeenLastCalledWith(`${root}/folder`));
  });

  it('opens the parent directory when double-clicking a nested file', async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    const nested = await screen.findByRole('button', { name: /nested\.bin/ });

    fireEvent.doubleClick(nested);
    await waitFor(() =>
      expect(api.fetchNode).toHaveBeenLastCalledWith(`${root}/folder`, 'size', 'desc'),
    );
    await waitFor(() => expect(api.fetchTree).toHaveBeenLastCalledWith(`${root}/folder`));
  });
});
