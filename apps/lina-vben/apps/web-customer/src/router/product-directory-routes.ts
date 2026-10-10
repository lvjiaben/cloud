import type { RouteRecordRaw } from 'vue-router';

import type { CustomerProductDirectory } from '#/api/idc/product-directories';

/** Project merchant service menus into the existing product navigation group. */
export function appendProductDirectoryRoutes(
  routes: RouteRecordRaw[],
  directories: CustomerProductDirectory[],
  locale: string,
): RouteRecordRaw[] {
  return routes.map((route) => {
    if (route.name !== 'CustomerProductsGroup') return route;
    const children = [...(route.children ?? [])];
    const insertAt = children.findIndex(
      (child) => child.name === 'CustomerProductDirectories',
    );
    const entries: RouteRecordRaw[] = directories.map((directory, index) => ({
      component: () => import('#/views/account/product-directories/index.vue'),
      meta: {
        icon: 'lucide:folder',
        productDirectoryNo: directory.directoryNo,
        productDirectoryPage: Math.floor(index / 100) + 1,
        title:
          locale === 'zh-CN'
            ? directory.nameZhCn || directory.nameEnUs
            : directory.nameEnUs || directory.nameZhCn,
      },
      name: `CustomerProductDirectory_${directory.directoryNo}`,
      path: `/account/product-directories/${encodeURIComponent(directory.directoryNo)}`,
    }));
    children.splice(
      insertAt === -1 ? children.length : insertAt + 1,
      0,
      ...entries,
    );
    return { ...route, children };
  });
}
