import type { RouteRecordRaw } from 'vue-router';

import type {
  PluginDynamicState,
  PluginRuntimeFrontendSurface,
} from '#/api/system/plugin/model';
import type { RegisteredPluginPage } from '#/plugins/page-registry';

import { prewarmPluginSandboxSource } from '@vben/layouts';

import { PluginSandboxView } from '#/layouts';

const customerSurfaceParents = {
  'customer.account': 'CustomerAccountGroup',
  'customer.compliance': 'CustomerComplianceGroup',
  'customer.finance': 'CustomerFinanceGroup',
  'customer.products': 'CustomerProductsGroup',
  'customer.security': 'CustomerSecurityGroup',
  'customer.support': 'CustomerServiceGroup',
} as const;

function pluginCanRenderSurfaces(item: PluginDynamicState) {
  return (
    item.installed === 1 &&
    item.enabled === 1 &&
    (!item.runtimeState || item.runtimeState === 'normal')
  );
}

function normalizeRoutePath(value: string) {
  return value.replace(/^\/+/, '');
}

function resolveSourcePage(
  surface: PluginRuntimeFrontendSurface,
  pages: RegisteredPluginPage[],
) {
  const entry = normalizeRoutePath(surface.entry);
  const routePath = normalizeRoutePath(surface.routePath);
  return (
    pages.find(
      (page) =>
        page.pluginId === surface.pluginId &&
        (page.routePath === entry || page.routePath === routePath),
    ) ?? null
  );
}

function buildSurfaceRoute(
  surface: PluginRuntimeFrontendSurface,
  pages: RegisteredPluginPage[],
): null | RouteRecordRaw {
  const isDynamic = surface.pluginType === 'dynamic';
  const sourcePage = isDynamic ? null : resolveSourcePage(surface, pages);
  if (isDynamic && !surface.entryUrl) {
    return null;
  }
  if (!isDynamic && !sourcePage) {
    return null;
  }

  const component = isDynamic ? PluginSandboxView : sourcePage?.component;
  if (!component) {
    return null;
  }

  return {
    component,
    meta: {
      authority: surface.permission ? [surface.permission] : undefined,
      icon: surface.icon,
      keepAlive: true,
      pluginSandboxSrc: isDynamic ? surface.entryUrl : undefined,
      pluginSurfaceCapabilities: isDynamic
        ? surface.bridgeCapabilities
        : undefined,
      pluginSurfaceHost: isDynamic ? 'customer' : undefined,
      pluginSurfaceId: isDynamic ? surface.pluginId : undefined,
      pluginSurfaceKey: isDynamic ? surface.key : undefined,
      pluginSurfaceVersion: isDynamic ? surface.version : undefined,
      title: surface.title,
    },
    name: `CustomerPluginSurface_${surface.pluginId}_${surface.key}`.replaceAll(
      /\W/g,
      '_',
    ),
    path: surface.routePath,
  };
}

function appendRouteToParent(
  routes: RouteRecordRaw[],
  parentName: string,
  route: RouteRecordRaw,
): boolean {
  for (const item of routes) {
    if (item.name === parentName) {
      item.children = [...(item.children ?? []), route];
      return true;
    }
    if (
      item.children &&
      appendRouteToParent(item.children, parentName, route)
    ) {
      return true;
    }
  }
  return false;
}

/**
 * Adds effective customer surfaces below host-published business groups. The
 * input route tree is cloned so refreshes cannot accumulate duplicate entries.
 */
export function appendCustomerPluginSurfaceRoutes(
  routes: RouteRecordRaw[],
  pluginStates: Iterable<PluginDynamicState>,
  pages: RegisteredPluginPage[],
) {
  const result = structuredCloneRoutes(routes);
  const seenPaths = new Set<string>();
  const states = [...pluginStates].toSorted((a, b) => a.id.localeCompare(b.id));

  for (const state of states) {
    if (!pluginCanRenderSurfaces(state)) {
      continue;
    }
    const surfaces = [...(state.customerSurfaces ?? [])].toSorted((a, b) => {
      if (a.slot !== b.slot) return a.slot.localeCompare(b.slot);
      return a.key.localeCompare(b.key);
    });
    for (const surface of surfaces) {
      const parentName =
        customerSurfaceParents[
          surface.slot as keyof typeof customerSurfaceParents
        ];
      if (!parentName || seenPaths.has(surface.routePath)) {
        continue;
      }
      const route = buildSurfaceRoute(surface, pages);
      if (!route || !appendRouteToParent(result, parentName, route)) {
        continue;
      }
      if (surface.entryUrl) prewarmPluginSandboxSource(surface.entryUrl);
      seenPaths.add(surface.routePath);
    }
  }
  return result;
}

function structuredCloneRoutes(routes: RouteRecordRaw[]): RouteRecordRaw[] {
  return routes.map((route) => {
    const cloned = {
      ...route,
      meta: route.meta ? { ...route.meta } : undefined,
    } as RouteRecordRaw;
    if (route.children) {
      cloned.children = structuredCloneRoutes(route.children);
    }
    return cloned;
  });
}
