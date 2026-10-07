import { createApp, h, shallowRef } from "vue";

import type {
  PluginHostMountContext,
  PluginHostMountInstance,
} from "./host-context";

import App from "./App.vue";
import styles from "./styles.css?inline";

export function mount(
  initialContext: PluginHostMountContext,
): PluginHostMountInstance {
  if (!initialContext.container) {
    throw new Error("The LinaPro plugin mount container is required.");
  }

  const context = shallowRef(initialContext);
  const mountPoint = document.createElement("div");
  mountPoint.dataset.pluginFrontendRoot = "idc-core";
  mountPoint.className = "idc-plugin-shell";
  const styleElement = document.createElement("style");
  styleElement.dataset.pluginFrontendStyle = "idc-core";
  styleElement.textContent = styles;
  mountPoint.append(styleElement);
  const appPoint = document.createElement("div");
  appPoint.dataset.pluginFrontendApp = "idc-core";
  mountPoint.append(appPoint);
  initialContext.container.replaceChildren(mountPoint);

  const app = createApp({
    render: () => h(App, { context: context.value }),
  });
  app.mount(appPoint);

  return {
    unmount() {
      app.unmount();
      mountPoint.remove();
    },
    update(nextContext) {
      context.value = nextContext;
    },
  };
}
