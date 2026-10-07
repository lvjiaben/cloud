import { initPreferences, updatePreferences } from '@vben/preferences';
import { unmountGlobalLoading } from '@vben/utils';

import { enforcedVisualPreferences, overridesPreferences } from './preferences';
import { syncPublicFrontendSettings } from './runtime/public-frontend';

/**
 * 应用初始化完成之后再进行页面加载渲染
 */
async function initApplication() {
  // name用于指定项目唯一标识
  // 用于区分不同项目的偏好设置以及存储数据的key前缀以及其他一些需要隔离的数据
  const env = import.meta.env.PROD ? 'prod' : 'dev';
  const appVersion = import.meta.env.VITE_APP_VERSION;
  const namespace = `${import.meta.env.VITE_APP_NAMESPACE}-${appVersion}-${env}`;

  // app偏好设置初始化
  await initPreferences({
    namespace,
    overrides: overridesPreferences,
  });

  // Product visual rules are authoritative. Re-apply them after cached user
  // preferences are restored so an old rounded/light-sidebar snapshot cannot
  // bring back the mixed theme after a restart or upgrade.
  updatePreferences(enforcedVisualPreferences, {
    markUserThemePreference: false,
  });

  await syncPublicFrontendSettings();

  // 启动应用并挂载
  // vue应用主要逻辑及视图
  const { bootstrap } = await import('./bootstrap');
  await bootstrap(namespace);

  // 移除并销毁loading
  unmountGlobalLoading();
}

initApplication();
