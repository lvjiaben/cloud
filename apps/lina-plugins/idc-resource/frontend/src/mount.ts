import { createApp,h,shallowRef } from "vue";
import type { PluginHostMountContext,PluginHostMountInstance } from "./host-context";
import App from "./ProviderConfiguration.vue";
import styles from "./styles.css?inline";
export function mount(initial:PluginHostMountContext):PluginHostMountInstance{if(!initial.container)throw new Error("mount container required");const context=shallowRef(initial);const root=document.createElement("div");root.dataset.pluginFrontendRoot="idc-resource";const style=document.createElement("style");style.textContent=styles;const appRoot=document.createElement("div");root.append(style,appRoot);initial.container.replaceChildren(root);const app=createApp({render:()=>h(App,{context:context.value})});app.mount(appRoot);return{unmount(){app.unmount();root.remove()},update(next){context.value=next}}}
