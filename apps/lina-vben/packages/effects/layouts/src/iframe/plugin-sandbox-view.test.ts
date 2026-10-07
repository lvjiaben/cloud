import { mount } from '@vue/test-utils';
import { ref } from 'vue';

import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  configurePluginSandboxBridge,
  PLUGIN_PAGE_BRIDGE_PROTOCOL,
  PLUGIN_PAGE_BRIDGE_VERSION,
} from './plugin-sandbox-bridge';
import PluginSandboxView from './plugin-sandbox-view.vue';

const mocks = vi.hoisted(() => ({
  beforeResolve: vi.fn(),
  replace: vi.fn(),
  resolve: vi.fn(),
  request: vi.fn(),
  setTabTitle: vi.fn(),
  setUpdateTime: vi.fn(),
  tabs: [] as Array<Record<string, unknown>>,
  route: {
    fullPath: '/account/plugins/phone-provider/orders',
    meta: {
      pluginSandboxSrc: 'about:blank',
      pluginSurfaceCapabilities: ['phone-orders.query-own'],
      pluginSurfaceHost: 'customer',
      pluginSurfaceId: 'phone-provider',
      pluginSurfaceKey: 'phone-orders',
      pluginSurfaceVersion: 'v1',
      title: 'Phone orders',
    },
    name: 'PhoneOrders',
    path: '/account/plugins/phone-provider/orders',
    query: {},
  },
}));

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({
    beforeResolve: mocks.beforeResolve,
    replace: mocks.replace,
    resolve: mocks.resolve,
  }),
}));

vi.mock('@vben/locales', () => ({ $t: (key: string) => key }));

vi.mock('@vben/preferences', () => ({
  preferences: {
    app: { name: 'Customer center' },
    theme: { colorPrimary: '#1677ff', fontSize: 16, radius: '0.5' },
  },
  usePreferences: () => ({ isDark: ref(false), locale: ref('en-US') }),
}));

vi.mock('@vben/stores', () => ({
  useTabbarStore: () => ({
    getTabs: mocks.tabs,
    setTabTitle: mocks.setTabTitle,
    setUpdateTime: mocks.setUpdateTime,
  }),
}));

function pluginMessage(
  capability: string,
  pluginId = 'phone-provider',
  requestId = 'request-1',
) {
  return {
    capability,
    direction: 'plugin-to-host',
    method: 'request',
    payload: { method: 'GET', path: 'phone-orders' },
    pluginId,
    protocol: PLUGIN_PAGE_BRIDGE_PROTOCOL,
    requestId,
    surfaceKey: 'phone-orders',
    version: PLUGIN_PAGE_BRIDGE_VERSION,
  };
}

describe('plugin sandbox view bridge authorization', () => {
  beforeEach(() => {
    mocks.beforeResolve.mockReset();
    mocks.beforeResolve.mockReturnValue(vi.fn());
    mocks.replace.mockReset();
    mocks.replace.mockResolvedValue(undefined);
    mocks.resolve.mockReset();
    mocks.resolve.mockImplementation(
      (target: { query?: { pluginPath?: string } }) => ({
        fullPath: `${mocks.route.path}?pluginPath=${target.query?.pluginPath ?? ''}`,
      }),
    );
    mocks.request.mockReset();
    mocks.request.mockResolvedValue({ owner: 'current-customer' });
    mocks.setTabTitle.mockReset();
    mocks.setUpdateTime.mockReset();
    mocks.tabs.length = 0;
    configurePluginSandboxBridge({ request: mocks.request });
  });

  it('proxies only a declared capability to the current plugin namespace', async () => {
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;

    window.dispatchEvent(
      new MessageEvent('message', {
        data: pluginMessage('phone-orders.query-own'),
        origin: 'null',
        source: iframe.contentWindow,
      }),
    );
    await vi.waitFor(() => expect(mocks.request).toHaveBeenCalledOnce());
    expect(mocks.request).toHaveBeenCalledWith(
      expect.objectContaining({
        capability: 'phone-orders.query-own',
        host: 'customer',
        path: 'phone-orders',
        pluginId: 'phone-provider',
        surfaceKey: 'phone-orders',
      }),
    );
    wrapper.unmount();
  });

  it('lets the host download an exact granted file without exposing credentials to the iframe', async () => {
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    const message = pluginMessage('phone-orders.query-own');
    message.payload = {
      download: { fileName: 'certificate.zip' },
      method: 'GET',
      path: 'account/service-downloads/ticket_123',
    };

    window.dispatchEvent(
      new MessageEvent('message', {
        data: message,
        origin: 'null',
        source: iframe.contentWindow,
      }),
    );
    await vi.waitFor(() => expect(mocks.request).toHaveBeenCalledOnce());
    expect(mocks.request).toHaveBeenCalledWith(
      expect.objectContaining({
        downloadFileName: 'certificate.zip',
        method: 'GET',
        path: 'account/service-downloads/ticket_123',
      }),
    );
    expect(JSON.stringify(mocks.request.mock.calls)).not.toMatch(
      /authorization|bearer|accessToken/i,
    );
    wrapper.unmount();
  });

  it.each([
    { download: { fileName: '../secret.zip' }, method: 'GET' },
    { download: { fileName: 'certificate.zip' }, method: 'POST' },
  ])('rejects an unsafe download request: %j', async (payload) => {
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    const message = pluginMessage('phone-orders.query-own');
    message.payload = {
      ...payload,
      path: 'account/service-downloads/ticket_123',
    };
    window.dispatchEvent(
      new MessageEvent('message', {
        data: message,
        origin: 'null',
        source: iframe.contentWindow,
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mocks.request).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('ignores another plugin and denies an undeclared capability', async () => {
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;

    for (const data of [
      pluginMessage('phone-orders.query-own', 'other-plugin'),
      pluginMessage('other-plugin.read'),
    ]) {
      window.dispatchEvent(
        new MessageEvent('message', {
          data,
          origin: 'null',
          source: iframe.contentWindow,
        }),
      );
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mocks.request).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('ignores messages from the wrong origin or window', async () => {
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    for (const event of [
      { origin: 'https://malicious.example', source: iframe.contentWindow },
      { origin: 'null', source: window },
    ]) {
      window.dispatchEvent(
        new MessageEvent('message', {
          data: pluginMessage('phone-orders.query-own'),
          ...event,
        }),
      );
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mocks.request).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('drops a delayed response after the iframe generation changes', async () => {
    let resolveRequest: ((value: unknown) => void) | undefined;
    mocks.request.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRequest = resolve;
        }),
    );
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const oldFrame = wrapper.get('iframe').element as HTMLIFrameElement;
    const oldWindow = oldFrame.contentWindow;
    if (!oldWindow) throw new Error('iframe content window is unavailable');
    const oldPostMessage = vi.spyOn(oldWindow, 'postMessage');

    window.dispatchEvent(
      new MessageEvent('message', {
        data: pluginMessage('phone-orders.query-own'),
        origin: 'null',
        source: oldFrame.contentWindow,
      }),
    );
    await vi.waitFor(() => expect(mocks.request).toHaveBeenCalledOnce());
    const requestSignal = (
      mocks.request.mock.calls[0]?.[0] as { signal?: AbortSignal }
    ).signal;
    (wrapper.vm as unknown as { retry: () => void }).retry();
    await wrapper.vm.$nextTick();
    expect(requestSignal?.aborted).toBe(true);
    resolveRequest?.({ private: 'old-plugin-result' });
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(
      oldPostMessage.mock.calls.some(
        ([message]) =>
          (message as { requestId?: string }).requestId === 'request-1',
      ),
    ).toBe(false);
    wrapper.unmount();
  });

  it('rejects duplicate request ids while the first request is in flight', async () => {
    mocks.request.mockImplementation(() => new Promise(() => {}));
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    const message = pluginMessage('phone-orders.query-own');

    for (let index = 0; index < 2; index += 1) {
      window.dispatchEvent(
        new MessageEvent('message', {
          data: message,
          origin: 'null',
          source: iframe.contentWindow,
        }),
      );
    }
    await vi.waitFor(() => expect(mocks.request).toHaveBeenCalledOnce());
    wrapper.unmount();
  });

  it('limits each isolated page to eight concurrent API requests', async () => {
    mocks.request.mockImplementation(() => new Promise(() => {}));
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    for (let index = 0; index < 9; index += 1) {
      window.dispatchEvent(
        new MessageEvent('message', {
          data: pluginMessage(
            'phone-orders.query-own',
            'phone-provider',
            `request-${index}`,
          ),
          origin: 'null',
          source: iframe.contentWindow,
        }),
      );
    }
    await vi.waitFor(() => expect(mocks.request).toHaveBeenCalledTimes(8));
    wrapper.unmount();
  });

  it('does not let an old async host-view title update the new iframe generation', async () => {
    let resolveTitle: (() => void) | undefined;
    mocks.tabs.push({ fullPath: mocks.route.fullPath });
    mocks.setTabTitle.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveTitle = resolve;
        }),
    );
    document.title = 'Current page';
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    window.dispatchEvent(
      new MessageEvent('message', {
        data: {
          ...pluginMessage('host.view.v1'),
          method: 'set-title',
          payload: { title: 'Old private page' },
          requestId: undefined,
        },
        origin: 'null',
        source: iframe.contentWindow,
      }),
    );
    await vi.waitFor(() => expect(mocks.setTabTitle).toHaveBeenCalledOnce());
    (wrapper.vm as unknown as { retry: () => void }).retry();
    await wrapper.vm.$nextTick();
    resolveTitle?.();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(document.title).toBe('Current page');
    expect(mocks.setUpdateTime).not.toHaveBeenCalled();
    expect(
      wrapper
        .get('[data-testid="plugin-sandbox-container"]')
        .attributes('data-plugin-title'),
    ).toBe('');
    wrapper.unmount();
  });

  it('does not let an old delayed navigation continue after retry', async () => {
    let resolveNavigation: (() => void) | undefined;
    let generationGuard: ((to: { fullPath: string }) => boolean) | undefined;
    mocks.beforeResolve.mockImplementation(
      (guard: (to: { fullPath: string }) => boolean) => {
        generationGuard = guard;
        return vi.fn();
      },
    );
    mocks.replace.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveNavigation = resolve;
        }),
    );
    const wrapper = mount(PluginSandboxView, { attachTo: document.body });
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement;
    window.dispatchEvent(
      new MessageEvent('message', {
        data: {
          ...pluginMessage('host.view.v1'),
          method: 'navigate',
          payload: { path: 'shipment/42' },
          requestId: undefined,
        },
        origin: 'null',
        source: iframe.contentWindow,
      }),
    );
    await vi.waitFor(() => expect(mocks.replace).toHaveBeenCalledOnce());
    const pluginTarget = `${mocks.route.path}?pluginPath=shipment/42`;
    expect(generationGuard?.({ fullPath: pluginTarget })).toBe(true);
    (wrapper.vm as unknown as { retry: () => void }).retry();
    await wrapper.vm.$nextTick();
    expect(generationGuard?.({ fullPath: pluginTarget })).toBe(false);
    expect(generationGuard?.({ fullPath: '/account/security' })).toBe(true);
    resolveNavigation?.();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(mocks.replace).toHaveBeenCalledWith({
      path: mocks.route.path,
      query: { pluginPath: 'shipment/42' },
    });
    wrapper.unmount();
  });
});
