import type { NodeResponse, SortKey, SortOrder, Status, TreeNode } from './types';

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

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
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

export function fetchTree(path: string): Promise<TreeNode> {
  const params = new URLSearchParams({ path });
  return getJSON<TreeNode>(`api/v1/tree?${params.toString()}`);
}

// requestAction sends a mutating request carrying the per-process action
// token the server issued over /api/v1/status (see Status.actionToken). The
// server rejects the request unless this exact token is echoed back, which
// keeps a page on another origin (or another browser tab) from triggering a
// delete/reveal even though it may still be able to make the browser send a
// request: it does not know the token, since the server serves it only to
// same-origin readers.
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

export function deleteNode(path: string, token: string): Promise<void> {
  const params = new URLSearchParams({ path });
  return requestAction(`api/v1/nodes?${params.toString()}`, token, { method: 'DELETE' });
}

// subscribeStatus opens an SSE connection that yields status updates. Returns
// an unsubscribe function.
export function subscribeStatus(onStatus: (status: Status) => void): () => void {
  const source = new EventSource('api/v1/events');
  source.onmessage = (event: MessageEvent<string>) => {
    try {
      onStatus(JSON.parse(event.data) as Status);
    } catch {
      // ignore malformed frames
    }
  };
  return () => source.close();
}
