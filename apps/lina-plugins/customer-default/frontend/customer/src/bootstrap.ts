import { createApp, watch, watchEffect } from 'vue';

import { registerAccessDirective } from '@vben/access';
import { registerLoadingDirective } from '@vben/common-ui/es/loading';
import {
  configurePluginSandboxBridge,
  resolvePluginSandboxRequestOwner,
  unwrapPluginSandboxResponseBody,
} from '@vben/layouts';
import { preferences } from '@vben/preferences';
import { initStores } from '@vben/stores';
import '@vben/styles';
import '@vben/styles/antd';

import { useTitle } from '@vueuse/core';

import { pluginApiPath, requestClient } from '#/api/request';
import { $t, setupI18n } from '#/locales';
import { useDictStore } from '#/store/dict';

import { initComponentAdapter } from './adapter/component';
import { initSetupVbenForm } from './adapter/form';
import App from './app.vue';
import { setupGlobalComponent } from './components/global';
import { loadPluginSvgIcons } from './plugins/icon-registry';
import { registerConsoleIcons } from './plugins/console-icons';
import { router } from './router';
import { syncPublicFrontendSettings } from './runtime/public-frontend';
import './styles/customer-console.css';
import { downloadBlob } from './utils/download';

async function bootstrap(namespace: string) {
  registerConsoleIcons();
  document.documentElement.dataset.customerConsole = 'true';
  document.body.classList.add('customer-console-app');

  // Register source-plugin frontend/icons SVGs before the shell renders menus.
  loadPluginSvgIcons();

  // 初始化组件适配器
  await initComponentAdapter();

  // 初始化表单组件
  await initSetupVbenForm();

  // // 设置弹窗的默认配置
  // setDefaultModalProps({
  //   fullscreenButton: false,
  // });
  // // 设置抽屉的默认配置
  // setDefaultDrawerProps({
  //   zIndex: 1020,
  // });

  const app = createApp(App);

  // 注册全局组件
  setupGlobalComponent(app);

  // 注册v-loading指令
  registerLoadingDirective(app, {
    loading: 'loading', // 在这里可以自定义指令名称，也可以明确提供false表示不注册这个指令
    spinning: 'spinning',
  });

  // 配置 pinia store
  await initStores(app, { namespace });

  // 国际化 i18n 配置
  await setupI18n(app);

  configurePluginSandboxBridge({
    request: async ({
      body,
      capability,
      downloadFileName,
      host,
      method,
      params,
      path,
      pluginId,
      signal,
      surfaceKey,
      surfaceVersion,
    }) => {
      const { grant, ownerPluginId } = await requestClient.post<{
        grant: string;
        ownerPluginId?: string;
      }>(
        '/plugins/page-bridge/grant',
        {
          capability,
          host,
          method,
          path,
          pluginId,
          surfaceKey,
          surfaceVersion,
        },
        { signal, timeout: 30_000 },
      );
      const targetPluginId = resolvePluginSandboxRequestOwner(
        pluginId,
        ownerPluginId,
      );
      const requestPath = pluginApiPath(targetPluginId, path);
      const requestOptions = {
        data: body,
        headers: { 'X-Lina-Plugin-Page-Grant': grant },
        method,
        params,
        // Runtime-plugin routes deliberately return the guest response body
        // unchanged instead of the host { code, data } envelope.
        responseReturn: 'body' as const,
        signal,
        timeout: 30_000,
      };
      if (downloadFileName) {
        const blob = await requestClient.download<Blob>(
          requestPath,
          requestOptions,
        );
        downloadBlob(blob, downloadFileName, blob.type);
        return { downloaded: true };
      }
      const response = await requestClient.request(requestPath, requestOptions);
      return unwrapPluginSandboxResponseBody(response);
    },
  });

  // 安装权限指令
  registerAccessDirective(app);

  // 初始化 tippy
  const { initTippy } = await import('@vben/common-ui/es/tippy');
  initTippy(app);

  // 配置路由及路由守卫
  app.use(router);

  // 配置Motion插件
  const { MotionPlugin } = await import('@vben/plugins/motion');
  app.use(MotionPlugin);

  // 动态更新标题
  watchEffect(() => {
    if (preferences.app.dynamicTitle) {
      const routeTitle = router.currentRoute.value.meta?.title;
      const pageTitle =
        (routeTitle ? `${$t(routeTitle)} - ` : '') + preferences.app.name;
      useTitle(pageTitle);
    }
  });

  watch(
    () => preferences.app.locale,
    async (locale, previousLocale) => {
      if (!previousLocale || locale === previousLocale) {
        return;
      }
      await syncPublicFrontendSettings(locale);
      useDictStore().resetCache();
    },
  );

  app.mount('#app');
}

export { bootstrap };
