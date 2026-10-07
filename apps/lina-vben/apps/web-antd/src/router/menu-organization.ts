import type { RouteRecordStringComponent } from '@vben/types';

const hiddenPaths = new Set([
  '/about/api-docs',
  '/about/system-info',
  '/developer',
  '/platform',
  '/platform/tenants',
  '/scheduler',
  '/system/config',
  '/system/dict',
  '/system/job',
  '/system/job-group',
  '/system/job-log',
  '/system/menu',
]);

const configurationPaths = new Set([
  '/idc/ai-copilot',
  '/idc/catalog/automation-interfaces',
  '/idc/compliance/providers/aliyun-face',
  '/idc/compliance/suppliers',
  '/idc/customer-external-menus',
  '/idc/migration',
  '/idc/notifications/channels',
  '/idc/notifications/templates',
  '/idc/payment-channels',
  '/idc/portal-brand',
  '/idc/resource/ip-change-policy',
  '/idc/resource/provider-configuration',
  '/idc/resource/server-operation-security',
  '/idc/support/settings',
  '/plugins/idc-zjmfmigrate/migration-checks',
]);

interface OrganizationOptions {
  systemConfigurationTitle: string;
}

function cloneRoute(
  route: RouteRecordStringComponent,
): RouteRecordStringComponent {
  return {
    ...route,
    children: route.children?.map(cloneRoute),
    meta: route.meta ? { ...route.meta } : undefined,
  };
}

function visit(
  routes: RouteRecordStringComponent[],
  visitor: (route: RouteRecordStringComponent) => void,
) {
  for (const route of routes) {
    visitor(route);
    visit(route.children || [], visitor);
  }
}

function detachByPath(
  routes: RouteRecordStringComponent[],
  path: string,
): RouteRecordStringComponent | undefined {
  for (let index = 0; index < routes.length; index += 1) {
    const route = routes[index];
    if (!route) continue;
    if (route.path === path) return routes.splice(index, 1)[0];
    const nested = detachByPath(route.children || [], path);
    if (nested) return nested;
  }
  return undefined;
}

function findByPath(
  routes: RouteRecordStringComponent[],
  path: string,
): RouteRecordStringComponent | undefined {
  for (const route of routes) {
    if (route.path === path) return route;
    const nested = findByPath(route.children || [], path);
    if (nested) return nested;
  }
  return undefined;
}

export function organizeAdminMenus(
  source: RouteRecordStringComponent[],
  options: OrganizationOptions,
) {
  const routes = source.map(cloneRoute);

  visit(routes, (route) => {
    if (!hiddenPaths.has(route.path) && !configurationPaths.has(route.path)) {
      return;
    }
    route.meta = {
      ...(route.meta || {}),
      hideInMenu: true,
      title: route.meta?.title || String(route.name || route.path),
    };
  });

  const staffAccess = detachByPath(routes, '/system/staff-access');
  const businessAccess = routes.find(
    (route) => route.path === '/business-access',
  );
  if (businessAccess) {
    businessAccess.meta = {
      ...(businessAccess.meta || {}),
      hideInMenu: true,
      title:
        businessAccess.meta?.title ||
        String(businessAccess.name || businessAccess.path),
    };
  }
  if (staffAccess) {
    const iam = routes.find((route) => route.path === '/iam');
    if (iam) {
      iam.children = [...(iam.children || []), staffAccess];
    } else {
      routes.push(staffAccess);
    }
  }

  // Older synchronized menu rows may still nest configuration pages below
  // the former operations workbench. Keep the routes reachable for the
  // configuration center, but remove the obsolete menu entry.
  const aiCopilot = detachByPath(routes, '/idc/ai-copilot');
  if (aiCopilot) {
    aiCopilot.meta = {
      ...(aiCopilot.meta || {}),
      hideInMenu: true,
      title: aiCopilot.meta?.title || String(aiCopilot.name || aiCopilot.path),
    };
    routes.push(aiCopilot);
  }
  const operationsWorkbench = findByPath(routes, '/idc/operations/workbench');
  if (operationsWorkbench) {
    operationsWorkbench.component = '#/views/system/plugin/dynamic-page.vue';
    delete operationsWorkbench.redirect;
  }

  let configurationCenter = detachByPath(
    routes,
    '/system/configuration-center',
  );
  if (!configurationCenter) {
    configurationCenter = {
      component: '#/views/system/plugin/dynamic-page.vue',
      meta: {
        icon: 'lucide:settings-2',
        order: 10,
        title: options.systemConfigurationTitle,
      },
      name: 'SystemConfigurationCenter',
      path: '/system/configuration-center',
    };
  }

  const pluginManagement = detachByPath(routes, '/system/plugin');
  // Plugin management now belongs to system settings. The former extension
  // container has no remaining merchant workflow and must not survive as an
  // empty top-level menu.
  detachByPath(routes, '/extension');
  const setting = findByPath(routes, '/setting');
  if (setting) {
    const children = setting.children || [];
    setting.redirect = '/system/configuration-center';
    setting.children = [
      configurationCenter,
      ...children,
      ...(pluginManagement ? [pluginManagement] : []),
    ];
  } else {
    routes.push(configurationCenter);
    if (pluginManagement) routes.push(pluginManagement);
  }

  visit(routes, (route) => {
    if (
      route.component !== 'BasicLayout' ||
      route.children?.some((child) => !child.meta?.hideInMenu)
    ) {
      return;
    }
    route.meta = {
      ...(route.meta || {}),
      hideInMenu: true,
      title: route.meta?.title || String(route.name || route.path),
    };
  });

  return routes;
}
