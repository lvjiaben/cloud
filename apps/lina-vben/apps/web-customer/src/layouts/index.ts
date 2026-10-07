const BasicLayout = () => import('./basic.vue');
const AuthPageLayout = () => import('./auth.vue');

const IFrameView = () => import('@vben/layouts').then((m) => m.IFrameView);
const PluginSandboxView = () =>
  import('@vben/layouts').then((m) => m.PluginSandboxView);

export { AuthPageLayout, BasicLayout, IFrameView, PluginSandboxView };
