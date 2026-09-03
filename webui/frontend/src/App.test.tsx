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
  deleteAllowed: true,
  actionToken: 'test-token',
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

describe('treemap actions', () => {
  it('reveals and deletes only the selected item, with session-only confirmation', async () => {
    vi.mocked(api.revealNode).mockResolvedValue(undefined);
    vi.mocked(api.deleteNode).mockResolvedValue(undefined);
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    const file = await screen.findByRole('button', { name: /file\.txt/ });
    fireEvent.click(file);

    fireEvent.keyDown(window, { code: 'KeyO' });
    await waitFor(() => expect(api.revealNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token'));

    fireEvent.keyDown(window, { code: 'KeyD' });
    expect(screen.getByRole('dialog', { name: 'Delete item' })).toBeTruthy();
    expect(screen.getByText(`${root}/file.txt`)).toBeTruthy();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Do not ask again this session' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete permanently' }));
    await waitFor(() => expect(api.deleteNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token'));
    await waitFor(() => expect(api.fetchTree).toHaveBeenCalledTimes(2));

    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    await waitFor(() => expect(api.deleteNode).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole('dialog', { name: 'Delete item' })).toBeNull();
  });

  it('opens shortcut help with question mark', async () => {
    render(<App />);
    await screen.findByRole('button', { name: 'Treemap' });

    fireEvent.keyDown(window, { key: '?' });

    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeTruthy();
    expect(screen.getByText('Reveal selected item')).toBeTruthy();
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
    fireEvent.keyDown(window, { code: 'KeyD' });
    expect(screen.queryByRole('dialog', { name: 'Delete item' })).toBeNull();
    expect(api.deleteNode).not.toHaveBeenCalled();
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

  it('does not offer deletion when the backend disables it', async () => {
    vi.mocked(api.fetchStatus).mockResolvedValue({ ...status, deleteAllowed: false });
    vi.mocked(api.revealNode).mockResolvedValue(undefined);
    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));

    fireEvent.keyDown(window, { code: 'KeyO' });
    await waitFor(() => expect(api.revealNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token'));
    fireEvent.keyDown(window, { code: 'KeyD' });

    expect(screen.queryByRole('dialog', { name: 'Delete item' })).toBeNull();
    expect(api.deleteNode).not.toHaveBeenCalled();
  });

  it('invalidates the stale treemap instead of leaving it displayed when the post-delete refresh fails', async () => {
    vi.mocked(api.deleteNode).mockResolvedValue(undefined);
    // First call (initial load) succeeds; every call after that (the
    // post-delete refresh, and the automatic retry it unblocks by
    // invalidating the cache) fails, so the failure is not masked by an
    // immediate successful retry.
    vi.mocked(api.fetchTree)
      .mockResolvedValueOnce(treeResponse)
      .mockRejectedValue(new Error('tree refresh failed'));
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    await screen.findByRole('img', { name: 'Directory size treemap' });
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    fireEvent.click(screen.getByRole('button', { name: 'Delete permanently' }));

    await waitFor(() => expect(api.deleteNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token'));
    await screen.findByText('Treemap unavailable');
    expect(screen.queryByRole('img', { name: 'Directory size treemap' })).toBeNull();
  });

  it('refreshes the current directory when a selected item disappeared', async () => {
    vi.mocked(api.deleteNode).mockRejectedValue(
      Object.assign(new Error('node not found'), { status: 404 }),
    );
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    fireEvent.click(screen.getByRole('button', { name: 'Delete permanently' }));

    await waitFor(() => expect(api.fetchNode).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('node not found')).toBeTruthy();
  });
});
