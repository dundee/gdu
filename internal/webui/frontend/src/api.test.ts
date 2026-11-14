import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchNode, subscribeStatus } from './api';

// Every API request carries the token of the tab: the server lists nothing
// to anyone who has not seen the URL gdu printed.
describe('api token', () => {
  const fetchMock = vi.fn();
  const eventSources: string[] = [];

  beforeEach(() => {
    fetchMock.mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    vi.stubGlobal(
      'EventSource',
      class {
        constructor(url: string) {
          eventSources.push(url);
        }
        close() {}
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
    eventSources.length = 0;
    window.sessionStorage.clear();
  });

  it('sends the token on reads', async () => {
    window.sessionStorage.setItem('gdu.actionToken', 'secret');

    await fetchNode('/root', 'size', 'desc');

    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.headers).toEqual({ 'X-GDU-Action': 'secret' });
  });

  it('subscribes to the events with the token in the query', () => {
    window.sessionStorage.setItem('gdu.actionToken', 'se cret');

    subscribeStatus(() => undefined);

    expect(eventSources).toEqual(['api/v1/events?token=se+cret']);
  });

  it('does not subscribe without a token, which the server would refuse forever', () => {
    subscribeStatus(() => undefined);

    expect(eventSources).toEqual([]);
  });
});
