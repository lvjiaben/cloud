import type { RouteRecordRaw } from 'vue-router';

import type { PluginDynamicState } from '#/api/system/plugin/model';
import type { RegisteredPluginPage } from '#/plugins/page-registry';

import { describe, expect, it } from 'vitest';

import { appendCustomerPluginSurfaceRoutes } from './customer-surface-routes';

function baseRoutes(): RouteRecordRaw[] {
  return [
    {
      children: [],
      name: 'CustomerProductsGroup',
      path: '/account/products',
    },
    {
      children: [],
      name: 'CustomerFinanceGroup',
      path: '/account/finance-center',
    },
  ];
}

function dynamicPlugin(
  overrides: Partial<PluginDynamicState> = {},
): PluginDynamicState {
  return {
    customerSurfaces: [
      {
        bridgeCapabilities: ['phone-orders.query-own'],
        entry: 'customer/index.html',
        entryUrl: '/x-assets/phone-provider/v1.0.0/customer/index.html',
        icon: 'lucide:smartphone',
        key: 'phone-orders',
        pluginId: 'phone-provider',
        pluginType: 'dynamic',
        routePath: '/account/plugins/phone-provider/phone-orders',
        slot: 'customer.products',
        title: '手机订单',
        version: 'v1',
      },
    ],
    enabled: 1,
    generation: 1,
    id: 'phone-provider',
    installed: 1,
    runtimeState: 'normal',
    statusKey: 'sys_plugin.status:phone-provider',
    type: 'dynamic',
    version: 'v1.0.0',
    ...overrides,
  };
}

describe('customer plugin surface routes', () => {
  it('mounts an enabled dynamic page below the published customer group', () => {
    const plugin = dynamicPlugin();
    const surface = plugin.customerSurfaces?.[0];
    if (!surface) {
      throw new Error('customer surface fixture is missing');
    }
    surface.permission = 'phone-orders-customer';
    const routes = appendCustomerPluginSurfaceRoutes(
      baseRoutes(),
      [plugin],
      [],
    );
    const products = routes.find(
      (route) => route.name === 'CustomerProductsGroup',
    );
    expect(products?.children).toHaveLength(1);
    expect(products?.children?.[0]?.path).toBe(
      '/account/plugins/phone-provider/phone-orders',
    );
    expect(products?.children?.[0]?.meta?.pluginSandboxSrc).toBe(
      '/x-assets/phone-provider/v1.0.0/customer/index.html',
    );
    expect(products?.children?.[0]?.meta?.pluginSurfaceCapabilities).toEqual([
      'phone-orders.query-own',
    ]);
    expect(products?.children?.[0]?.meta?.pluginSurfaceHost).toBe('customer');
    expect(products?.children?.[0]?.meta?.pluginSurfaceId).toBe(
      'phone-provider',
    );
    expect(products?.children?.[0]?.meta?.pluginSurfaceKey).toBe(
      'phone-orders',
    );
    expect(products?.children?.[0]?.meta?.authority).toEqual([
      'phone-orders-customer',
    ]);
    expect(baseRoutes()[0]?.children).toHaveLength(0);
  });

  it('does not mount disabled, failed, unknown-slot, or admin-only surfaces', () => {
    const unknownSurface = dynamicPlugin().customerSurfaces?.[0];
    if (!unknownSurface) {
      throw new Error('customer surface fixture is missing');
    }
    const unknownSlot = dynamicPlugin({
      customerSurfaces: [
        {
          ...unknownSurface,
          slot: 'customer.top-level',
        },
      ],
    });
    const adminOnly = dynamicPlugin({
      adminSurfaces: dynamicPlugin().customerSurfaces,
      customerSurfaces: [],
    });
    const routes = appendCustomerPluginSurfaceRoutes(
      baseRoutes(),
      [
        dynamicPlugin({ enabled: 0 }),
        dynamicPlugin({ runtimeState: 'upgrade_failed' }),
        unknownSlot,
        adminOnly,
      ],
      [],
    );
    expect(routes.flatMap((route) => route.children ?? [])).toHaveLength(0);
  });

  it('mounts a compiled source page without turning it into an iframe', () => {
    const component = {};
    const page: RegisteredPluginPage = {
      capabilities: [],
      component,
      filePath: '/plugins/server-owner/frontend/pages/servers.vue',
      key: 'server-owner:servers',
      pluginId: 'server-owner',
      routePath: 'server-owner-servers',
      title: '云服务器',
    };
    const source = dynamicPlugin({
      customerSurfaces: [
        {
          entry: 'server-owner-servers',
          key: 'servers',
          pluginId: 'server-owner',
          pluginType: 'source',
          routePath: '/account/plugins/server-owner/servers',
          slot: 'customer.products',
          title: '云服务器',
          version: 'v1',
        },
      ],
      id: 'server-owner',
      type: 'source',
    });
    const routes = appendCustomerPluginSurfaceRoutes(
      baseRoutes(),
      [source],
      [page],
    );
    const mounted = routes[0]?.children?.[0];
    expect(mounted?.component).toBe(component);
    expect(mounted?.meta?.iframeSrc).toBeUndefined();
    expect(mounted?.meta?.pluginSandboxSrc).toBeUndefined();
  });
});
