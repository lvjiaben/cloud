import { createApp, h, shallowRef } from "vue";
import App from "./App.vue";
import type { PluginHostMountContext, PluginHostMountInstance } from "./host-context";
import styles from "./styles.css?inline";

export function mount(initialContext: PluginHostMountContext): PluginHostMountInstance {
  if (!initialContext.container) throw new Error("The LinaPro plugin mount container is required.");
  const context = shallowRef(initialContext);
  const root = document.createElement("div"); root.dataset.pluginFrontendRoot = "idc-payment"; root.className = "payment-reconciliation-shell";
  const style = document.createElement("style"); style.textContent = styles; root.append(style);
  const appRoot = document.createElement("div"); root.append(appRoot); initialContext.container.replaceChildren(root);
  const app = createApp({ render: () => h(App, { context: context.value }) }); app.mount(appRoot);
  return { unmount() { app.unmount(); root.remove(); }, update(next) { context.value = next; } };
}
