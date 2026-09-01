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

async function requestAction(url: string, init: RequestInit): Promise<void> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'X-GDU-Action': '1',
      ...init.headers,
    },
  });
  if (res.ok) {
    return;
  }
  throw await responseError(res);
}

export function revealNode(path: string): Promise<void> {
  return requestAction('api/v1/reveal', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  });
}

export function deleteNode(path: string): Promise<void> {
  const params = new URLSearchParams({ path });
  return requestAction(`api/v1/nodes?${params.toString()}`, { method: 'DELETE' });
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
