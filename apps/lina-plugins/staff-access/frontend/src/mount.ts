import { createApp, h, shallowRef } from 'vue';
import App from './App.vue';
import type { PluginHostMountContext, PluginHostMountInstance } from './host-context';
import styles from './styles.css?inline';
import themeOverrides from './theme-overrides.css?inline';

export function mount(initialContext: PluginHostMountContext): PluginHostMountInstance {
  if (!initialContext.container) throw new Error('The LinaPro plugin mount container is required.');
  const context = shallowRef(initialContext);
  const root = document.createElement('div'); root.dataset.pluginFrontendRoot = 'staff-access'; root.className = 'staff-access-shell';
  const shadow = root.attachShadow({ mode: 'open' });
  const style = document.createElement('style'); style.textContent = `${styles}\n${themeOverrides}`; shadow.append(style);
  const point = document.createElement('div'); shadow.append(point); initialContext.container.replaceChildren(root);
  const app = createApp({ render: () => h(App, { context: context.value }) }); app.mount(point);
  return { unmount() { app.unmount(); root.remove(); }, update(next) { context.value = next; } };
}
