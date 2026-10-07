import type { RouteRecordStringComponent } from '@vben/types';

import { describe, expect, it } from 'vitest';

import { organizeAdminMenus } from './menu-organization';

describe('admin menu organization', () => {
  it('hides framework internals, groups configuration, and moves staff access', () => {
    const routes: RouteRecordStringComponent[] = [
      {
        component: 'BasicLayout',
        children: [
          {
            component: '#/views/system/user/index.vue',
            meta: { title: 'Users' },
            name: 'Users',
            path: '/system/user',
          },
          {
            component: '#/views/system/menu/index.vue',
            meta: { title: 'Menus' },
            name: 'Menus',
            path: '/system/menu',
          },
        ],
        meta: { title: 'Access' },
        name: 'IAM',
        path: '/iam',
      },
      {
        component: 'BasicLayout',
        children: [
          {
            component: '#/views/system/plugin/dynamic-page.vue',
            meta: { title: 'Staff access' },
            name: 'StaffAccess',
            path: '/system/staff-access',
          },
        ],
        meta: { title: 'Business access' },
        name: 'BusinessAccess',
        path: '/business-access',
      },
      {
        component: 'BasicLayout',
        children: [
          {
            component: '#/views/system/file/index.vue',
            meta: { title: 'Files' },
            name: 'Files',
            path: '/system/file',
          },
        ],
        meta: { title: 'Settings' },
        name: 'Setting',
        path: '/setting',
      },
      {
        component: 'BasicLayout',
        children: [
          {
            component: '#/views/system/plugin/index.vue',
            meta: { title: 'Plugins' },
            name: 'Plugins',
            path: '/system/plugin',
          },
        ],
        meta: { title: 'Extensions' },
        name: 'Extension',
        path: '/extension',
      },
      {
        component: '#/views/system/plugin/dynamic-page.vue',
        meta: { title: 'Website brand' },
        name: 'PortalBrand',
        path: '/idc/portal-brand',
      },
      {
        component: '#/views/platform/tenant/index.vue',
        meta: { title: 'Tenants' },
        name: 'Tenants',
        path: '/platform/tenants',
      },
      {
        children: [
          {
            component: '#/views/system/plugin/dynamic-page.vue',
            meta: { title: 'AI' },
            name: 'AI',
            path: '/idc/ai-copilot',
          },
        ],
        component: 'BasicLayout',
        meta: { title: 'Operations' },
        name: 'Operations',
        path: '/idc/operations/workbench',
        redirect: '/idc/ai-copilot',
      },
    ];

    const result = organizeAdminMenus(routes, {
      systemConfigurationTitle: '系统配置',
    });

    const iam = result.find((route) => route.path === '/iam');
    expect(
      iam?.children?.some((route) => route.path === '/system/staff-access'),
    ).toBe(true);
    expect(
      result.find((route) => route.path === '/business-access')?.meta
        ?.hideInMenu,
    ).toBe(true);
    expect(
      result.find((route) => route.path === '/idc/portal-brand')?.meta
        ?.hideInMenu,
    ).toBe(true);
    expect(
      result.find((route) => route.path === '/platform/tenants')?.meta
        ?.hideInMenu,
    ).toBe(true);
    expect(
      result
        .find((route) => route.path === '/setting')
        ?.children?.find(
          (route) => route.path === '/system/configuration-center',
        )?.meta?.title,
    ).toBe('系统配置');
    expect(
      result.find((route) => route.path === '/setting')?.redirect,
    ).toBe('/system/configuration-center');
    expect(
      result
        .find((route) => route.path === '/setting')
        ?.children?.some((route) => route.path === '/system/plugin'),
    ).toBe(true);
    expect(result.find((route) => route.path === '/extension')).toBeUndefined();
    expect(
      result
        .find((route) => route.path === '/iam')
        ?.children?.find((route) => route.path === '/system/menu')?.meta
        ?.hideInMenu,
    ).toBe(true);
    expect(
      result.find((route) => route.path === '/idc/operations/workbench')?.meta
        ?.hideInMenu,
    ).toBeUndefined();
    expect(
      result
        .find((route) => route.path === '/idc/operations/workbench')
        ?.children?.find((route) => route.path === '/idc/ai-copilot'),
    ).toBeUndefined();
    expect(
      result.find((route) => route.path === '/idc/ai-copilot')?.meta
        ?.hideInMenu,
    ).toBe(true);
    expect(routes[1]?.children).toHaveLength(1);
  });
});
