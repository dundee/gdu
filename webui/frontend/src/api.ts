import type { NodeResponse, SortKey, SortOrder, Status } from './types';

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    let message = `request failed: ${res.status}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // ignore non-JSON error bodies
    }
    throw new Error(message);
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
