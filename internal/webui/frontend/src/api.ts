import type { CategoriesResponse, NodeResponse, SortKey, SortOrder, Status, TreeNode } from './types';

async function responseError(res: Response): Promise<Error & { status: number }> {
  let message = `request failed: ${res.status}`;
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error) {
      message = body.error;
    }
  } catch {
    // ignore non-JSON error bodies
  }
  return Object.assign(new Error(message), { status: res.status });
}

// The tab-scoped session storage key index.html keeps the per-process token
// in, taken from the URL gdu printed (see actionTokenParam in
// webui/action_token.go).
const actionTokenStorageKey = 'gdu.actionToken';

// storedActionToken returns the token of this tab, or '' when it has none.
// Every API request needs it: the server refuses to list the tree to anyone
// who has not seen the URL gdu printed. sessionStorage can throw rather than
// return null (private-mode browsers, storage disabled by policy); losing
// the token then costs the data, which the page says.
export function storedActionToken(): string {
  try {
    return window.sessionStorage.getItem(actionTokenStorageKey) ?? '';
  } catch {
    return '';
  }
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: { 'X-GDU-Action': storedActionToken() } });
  if (!res.ok) {
    throw await responseError(res);
  }
  return (await res.json()) as T;
}

export function fetchStatus(): Promise<Status> {
  return getJSON<Status>('api/v1/status');
}

export function fetchNode(
  path: string,
  sort: SortKey,
  order: SortOrder,
): Promise<NodeResponse> {
  const params = new URLSearchParams({ path, sort, order });
  return getJSON<NodeResponse>(`api/v1/nodes?${params.toString()}`);
}

// fetchTree loads the recursive tree below path. With byCategory, it is the
// tree of the "By category" projection: one synthetic node per cleanup
// category (its path starts with CATEGORY_NODE_PREFIX, see cleanup.ts)
// holding the candidates with their real paths.
export function fetchTree(path: string, byCategory = false): Promise<TreeNode> {
  const params = new URLSearchParams({ path });
  if (byCategory) {
    params.set('projection', 'category');
  }
  return getJSON<TreeNode>(`api/v1/tree?${params.toString()}`);
}

export function fetchCategories(path: string): Promise<CategoriesResponse> {
  const params = new URLSearchParams({ path });
  return getJSON<CategoriesResponse>(`api/v1/categories?${params.toString()}`);
}

// requestAction sends a mutating request carrying the per-process action
// token the server printed into this page's own URL (see
// GduModel.actionToken in model.tsx). The server rejects the request unless
// this exact token is echoed back, which keeps a page on another origin (or
// another browser tab, or another local user who never saw this URL) from
// triggering a delete/reveal even though it may still be able to make the
// browser send a request: it does not know the token, since the token is
// never served over any API response.
async function requestAction(url: string, token: string, init: RequestInit): Promise<void> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'X-GDU-Action': token,
      ...init.headers,
    },
  });
  if (res.ok) {
    return;
  }
  throw await responseError(res);
}

export function revealNode(path: string, token: string): Promise<void> {
  return requestAction('api/v1/reveal', token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  });
}

export type DeleteMode = 'trash' | 'permanent';

export function deleteNode(path: string, token: string, mode: DeleteMode): Promise<void> {
  const params = new URLSearchParams({ path });
  if (mode === 'trash') {
    params.set('mode', 'trash');
  }
  return requestAction(`api/v1/nodes?${params.toString()}`, token, { method: 'DELETE' });
}

// subscribeStatus opens an SSE connection that yields status updates. Returns
// an unsubscribe function. EventSource cannot send headers, so the token goes
// in the query, which the server accepts on this route only. Without a token
// nothing is opened: the server would refuse it, and EventSource retry it
// forever.
export function subscribeStatus(onStatus: (status: Status) => void): () => void {
  const token = storedActionToken();
  if (!token) {
    return () => undefined;
  }
  const params = new URLSearchParams({ token });
  const source = new EventSource(`api/v1/events?${params.toString()}`);
  source.onmessage = (event: MessageEvent<string>) => {
    try {
      onStatus(JSON.parse(event.data) as Status);
    } catch {
      // ignore malformed frames
    }
  };
  return () => source.close();
}
