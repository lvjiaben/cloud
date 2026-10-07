// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { startCustomerServersPage } from './page';

const serverNo = `SVR-${'A'.repeat(32)}`;
const item = { serverNo, boundAt: 1_777_000_000_000, channelName: '华东渠道', kind: 'cloud_server', latestOperation: 'idle', poolName: '上海资源池', productNameEnUs: 'Business Cloud 4C8G', productNameZhCn: '商务云 4核8G', state: 'ready', updatedAt: 1_777_000_100_000 };

function fixture() {
  document.body.innerHTML = `<main><span data-testid="server-summary"></span><button data-testid="server-refresh"></button><section data-testid="server-skeleton"></section><section data-testid="server-error" hidden><b data-role="title"></b><p data-role="description"></p><button data-testid="server-retry"></button></section><section data-testid="server-empty" hidden><b data-role="title"></b><p data-role="description"></p></section><section data-testid="server-content" hidden><ul data-testid="server-list"></ul><div data-testid="server-pager" hidden><button data-testid="server-more"></button></div></section><section data-testid="server-detail" hidden><h2 data-testid="server-detail-title"></h2><button data-testid="server-detail-close"></button><div data-testid="server-detail-skeleton"></div><div data-testid="server-detail-error" hidden><p data-role="description"></p><button data-testid="server-detail-retry"></button></div><dl data-testid="server-detail-grid" hidden></dl></section></main>`;
  Object.defineProperty(document.documentElement, 'scrollHeight', { configurable: true, value: 600 });
}

function hostMessage(payload: Record<string, unknown>) {
  window.dispatchEvent(new MessageEvent('message', { source: window.parent, data: { protocol: 'linapro.plugin-page-bridge', version: '1.0', direction: 'host-to-plugin', pluginId: 'idc-zjmfcloud', surfaceKey: 'servers', ...payload } }));
}

describe('customer server page bridge', () => {
  beforeEach(() => { fixture(); vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} }); });

  it('loads once on repeated context sync and refreshes only on request', () => {
    const postMessage = vi.fn(); Object.defineProperty(window, 'parent', { configurable: true, value: { postMessage } });
    const stop = startCustomerServersPage();
    hostMessage({ capability: 'host.context.v1', payload: { locale: 'zh-CN', theme: { mode: 'light', cssVariables: {} } } });
    hostMessage({ capability: 'host.context.v1', payload: { locale: 'en-US', theme: { mode: 'dark', cssVariables: {} } } });
    const requests = () => postMessage.mock.calls.map(([message]) => message).filter((message) => message.capability === 'customer-servers.read');
    expect(requests()).toHaveLength(1);
    expect(requests()[0].payload).toEqual({ method: 'GET', path: 'account/plugin-servers', params: { cursor: '', limit: 20 } });
    hostMessage({ capability: 'customer-servers.read', requestId: requests()[0].requestId, ok: true, result: { items: [item], nextCursor: '' } });
    expect(document.querySelector('[data-testid="server-list"]')?.textContent).toContain('Business Cloud 4C8G');
    expect(document.body.textContent).not.toContain(serverNo);
    document.querySelector<HTMLButtonElement>('[data-testid="server-refresh"]')?.click();
    expect(requests()).toHaveLength(2);
    stop();
  });

  it('requests detail by server key without rendering the key', () => {
    const postMessage = vi.fn(); Object.defineProperty(window, 'parent', { configurable: true, value: { postMessage } });
    const stop = startCustomerServersPage();
    hostMessage({ capability: 'host.context.v1', payload: { locale: 'zh-CN', theme: { mode: 'light', cssVariables: {} } } });
    const messages = () => postMessage.mock.calls.map(([message]) => message).filter((message) => message.capability === 'customer-servers.read');
    hostMessage({ capability: 'customer-servers.read', requestId: messages()[0].requestId, ok: true, result: { items: [item], nextCursor: '' } });
    document.querySelector<HTMLButtonElement>('[data-server-no]')?.click();
    expect(messages().at(-1).payload).toEqual({ method: 'GET', path: `account/plugin-servers/${serverNo}`, params: {} });
    hostMessage({ capability: 'customer-servers.read', requestId: messages().at(-1).requestId, ok: true, result: { item } });
    expect(document.querySelector('[data-testid="server-detail-grid"]')?.textContent).toContain('华东渠道 · 上海资源池');
    expect(document.body.textContent).not.toContain(serverNo);
    stop();
  });

  it('shows recoverable error and empty states without inventing servers', () => {
    const postMessage = vi.fn(); Object.defineProperty(window, 'parent', { configurable: true, value: { postMessage } });
    const stop = startCustomerServersPage();
    hostMessage({ capability: 'host.context.v1', payload: { locale: 'zh-CN', theme: { mode: 'light', cssVariables: {} } } });
    const requests = () => postMessage.mock.calls.map(([message]) => message).filter((message) => message.capability === 'customer-servers.read');
    hostMessage({ capability: 'customer-servers.read', requestId: requests()[0].requestId, ok: false });
    expect(document.querySelector<HTMLElement>('[data-testid="server-error"]')?.hidden).toBe(false);
    document.querySelector<HTMLButtonElement>('[data-testid="server-retry"]')?.click();
    hostMessage({ capability: 'customer-servers.read', requestId: requests().at(-1).requestId, ok: true, result: { items: [], nextCursor: '' } });
    expect(document.querySelector<HTMLElement>('[data-testid="server-empty"]')?.hidden).toBe(false);
    expect(document.querySelector('[data-testid="server-list"]')?.children).toHaveLength(0);
    stop();
  });
});
