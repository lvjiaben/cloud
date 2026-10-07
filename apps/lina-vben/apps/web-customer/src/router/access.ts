import type {
  ComponentRecordType,
  GenerateMenuAndRoutesOptions,
} from '@vben/types';

import { generateAccessible } from '@vben/access';
import { preferences } from '@vben/preferences';

import { BasicLayout, IFrameView, PluginSandboxView } from '#/layouts';
import { appendCustomerPluginSurfaceRoutes } from '#/plugins/customer-surface-routes';
import { getPluginPages } from '#/plugins/page-registry';
import { getPluginStateMap } from '#/plugins/slot-registry';

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
  const routes = appendCustomerPluginSurfaceRoutes(
    options.routes,
    routeStates.values(),
    getPluginPages(),
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
