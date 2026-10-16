import type {
  ComponentRecordType,
  GenerateMenuAndRoutesOptions,
} from '@vben/types';

import { generateAccessible } from '@vben/access';
import { preferences } from '@vben/preferences';

import { listCustomerProductDirectories } from '#/api/idc/product-directories';
import { BasicLayout, IFrameView, PluginSandboxView } from '#/layouts';
import { appendCustomerPluginSurfaceRoutes } from '#/plugins/customer-surface-routes';
import { getPluginPages } from '#/plugins/page-registry';
import { getPluginStateMap } from '#/plugins/slot-registry';

import { appendProductDirectoryRoutes } from './product-directory-routes';

const forbiddenComponent = () => import('#/views/_core/fallback/forbidden.vue');

async function generateAccess(
  options: GenerateMenuAndRoutesOptions,
  runtimeOptions: {
    pluginStateMap?: Map<
      string,
      import('#/api/system/plugin/model').PluginDynamicState
    >;
    showLoadingToast?: boolean;
  } = {},
) {
  const pageMap: ComponentRecordType = import.meta.glob([
    '../views/_core/**/*.vue',
    '../views/account/**/*.vue',
  ]);
  const layoutMap: ComponentRecordType = {
    BasicLayout,
    IFrameView,
    PluginSandboxView,
  };

  const pluginStateMap =
    runtimeOptions.pluginStateMap ?? (await getPluginStateMap());
  const routeStates = new Map(
    [...pluginStateMap.values()].map((item) => [item.id, item]),
  );
  const surfaceRoutes = appendCustomerPluginSurfaceRoutes(
    options.routes,
    routeStates.values(),
    getPluginPages(),
  );

  // Each request is bounded to 100 entries; fetch remaining pages in batches.
  // A directory outage must not prevent login or access to all purchased services.
  const directories = await (async () => {
    try {
      const first = await listCustomerProductDirectories(1, 100);
      const items = [...first.items];
      const pages = Math.ceil(first.total / 100);
      for (let page = 2; page <= pages; page += 4) {
        const batch = await Promise.all(
          Array.from({ length: Math.min(4, pages - page + 1) }, (_, offset) =>
            listCustomerProductDirectories(page + offset, 100),
          ),
        );
        items.push(...batch.flatMap((result) => result.items));
      }
      return items;
    } catch {
      return [];
    }
  })();
  const routes = appendProductDirectoryRoutes(
    surfaceRoutes,
    directories,
    preferences.app.locale,
  );

  return generateAccessible(preferences.app.accessMode, {
    ...options,
    routes,
    fetchMenuListAsync: async () => [],
    forbiddenComponent,
    layoutMap,
    pageMap,
  });
}

export { generateAccess };
