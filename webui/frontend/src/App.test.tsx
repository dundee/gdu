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
  // The action token lives only in this page's own URL (see model.tsx),
  // mirroring how the server delivers it in practice.
  window.history.pushState({}, '', '/?token=test-token');
  vi.mocked(api.fetchStatus).mockResolvedValue(status);
  vi.mocked(api.fetchNode).mockResolvedValue(nodeResponse);
  vi.mocked(api.fetchTree).mockResolvedValue(treeResponse);
  vi.mocked(api.subscribeStatus).mockReturnValue(() => undefined);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.history.pushState({}, '', '/');
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

describe('action token', () => {
  it('strips the token from the address bar after reading it', async () => {
    render(<App />);
    await screen.findByRole('table');

    expect(window.location.search).toBe('');
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
    expect(screen.queryByText('file.txt', { selector: 'strong' })).toBeNull();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Do not ask again this session' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete Permanently' }));
    await waitFor(() =>
      expect(api.deleteNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token', 'permanent'),
    );
    await waitFor(() => expect(api.fetchTree).toHaveBeenCalledTimes(2));

    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    await waitFor(() => expect(api.deleteNode).toHaveBeenCalledTimes(2));
    expect(api.deleteNode).toHaveBeenLastCalledWith(`${root}/file.txt`, 'test-token', 'permanent');
    expect(screen.queryByRole('dialog', { name: 'Delete item' })).toBeNull();
  });

  it('moves focus between the delete modal buttons with the arrow keys', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });

    const cancel = screen.getByRole('button', { name: 'Cancel' });
    const trash = screen.getByRole('button', { name: 'Move to Trash' });
    const permanent = screen.getByRole('button', { name: 'Delete Permanently' });
    expect(document.activeElement).toBe(cancel);

    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(trash);

    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(permanent);

    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(cancel);

    fireEvent.keyDown(window, { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(permanent);
  });

  it('opens the delete confirmation via the Delete key, same as the D shortcut', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));

    fireEvent.keyDown(window, { key: 'Delete' });
    expect(screen.getByRole('dialog', { name: 'Delete item' })).toBeTruthy();
  });

  it('moves the item to the trash without affecting the permanent-delete skip choice', async () => {
    vi.mocked(api.deleteNode).mockResolvedValue(undefined);
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    fireEvent.click(screen.getByRole('button', { name: 'Move to Trash' }));

    await waitFor(() =>
      expect(api.deleteNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token', 'trash'),
    );
  });

  it('opens shortcut help with question mark, without treemap-only shortcuts in donut view', async () => {
    render(<App />);
    await screen.findByRole('button', { name: 'Treemap' });

    fireEvent.keyDown(window, { key: '?' });

    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeTruthy();
    // Reveal/Delete/Click only work in the treemap view (see TreeMapView),
    // so listing them here would be misleading while looking at the donut.
    expect(screen.queryByText('Reveal selected item')).toBeNull();
  });

  it('includes the treemap-only shortcuts in help when viewing the treemap', async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    await screen.findByRole('img', { name: 'Directory size treemap' });

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
    fireEvent.click(screen.getByRole('button', { name: 'Delete Permanently' }));

    await waitFor(() =>
      expect(api.deleteNode).toHaveBeenCalledWith(`${root}/file.txt`, 'test-token', 'permanent'),
    );
    await screen.findByText('Treemap unavailable');
    expect(screen.queryByRole('img', { name: 'Directory size treemap' })).toBeNull();
  });

  it('navigates up to the parent directory when the last item in it is deleted', async () => {
    const folderPath = `${root}/folder`;
    const onlyChild = {
      name: 'onlyfile.bin',
      path: `${folderPath}/onlyfile.bin`,
      isDir: false,
      size: 5,
      usage: 5,
      itemCount: 1,
      mtime: 0,
    };
    const folderNodeResponse: NodeResponse = {
      node: nodeResponse.children[0],
      breadcrumbs: [nodeResponse.node, nodeResponse.children[0]],
      children: [onlyChild],
    };
    const emptyFolderNodeResponse: NodeResponse = { ...folderNodeResponse, children: [] };
    const folderTreeResponse: TreeNode = {
      ...folderNodeResponse.node,
      children: [{ ...onlyChild, children: [] }],
    };

    let folderEmptied = false;
    vi.mocked(api.fetchNode).mockImplementation((path) =>
      Promise.resolve(
        path === folderPath
          ? folderEmptied
            ? emptyFolderNodeResponse
            : folderNodeResponse
          : nodeResponse,
      ),
    );
    vi.mocked(api.fetchTree).mockImplementation((path) =>
      Promise.resolve(path === folderPath ? folderTreeResponse : treeResponse),
    );
    vi.mocked(api.deleteNode).mockImplementation(async () => {
      folderEmptied = true;
    });

    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.doubleClick(await screen.findByRole('button', { name: /folder/ }));
    await waitFor(() => expect(api.fetchNode).toHaveBeenLastCalledWith(folderPath, 'size', 'desc'));

    fireEvent.click(await screen.findByRole('button', { name: /onlyfile\.bin/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    fireEvent.click(await screen.findByRole('button', { name: 'Delete Permanently' }));

    await waitFor(() =>
      expect(api.fetchNode).toHaveBeenLastCalledWith(root, 'size', 'desc'),
    );
  });

  it('refreshes the current directory when a selected item disappeared', async () => {
    vi.mocked(api.deleteNode).mockRejectedValue(
      Object.assign(new Error('node not found'), { status: 404 }),
    );
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: 'Treemap' }));
    fireEvent.click(await screen.findByRole('button', { name: /file\.txt/ }));
    fireEvent.keyDown(window, { code: 'KeyD' });
    fireEvent.click(screen.getByRole('button', { name: 'Delete Permanently' }));

    await waitFor(() => expect(api.fetchNode).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('node not found')).toBeTruthy();
  });
});
