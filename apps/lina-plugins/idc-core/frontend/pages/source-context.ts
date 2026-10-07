import type { PluginHostMountContext } from "../src/host-context";

import "../src/styles.css";

import type { InjectionKey, Ref } from "vue";

import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute } from "vue-router";

export type SourcePluginRouteOverride = Readonly<
  Ref<{ path: string; title: string }>
>;

export const sourcePluginRouteOverrideKey: InjectionKey<SourcePluginRouteOverride> =
  Symbol("idc-core-source-plugin-route-override");

function storedAccessToken() {
  for (const key of Object.keys(localStorage)) {
    if (!key.endsWith("-core-access")) continue;
    try {
      const value = JSON.parse(localStorage.getItem(key) ?? "{}") as {
        accessToken?: string;
      };
      if (typeof value.accessToken === "string") return value.accessToken;
    } catch {
      // Ignore unrelated invalid browser storage owned by another feature.
    }
  }
  return "";
}

export function useSourcePluginContext() {
  const route = useRoute();
  const routeOverride = inject(sourcePluginRouteOverrideKey, undefined);
  const locale = ref(
    document.documentElement.lang || navigator.language || "zh-CN",
  );
  const localeObserver = new MutationObserver(() => {
    locale.value = document.documentElement.lang || "zh-CN";
  });
  onMounted(() =>
    localeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["lang"],
    }),
  );
  onBeforeUnmount(() => localeObserver.disconnect());

  return computed<PluginHostMountContext>(() => {
    const targetPath = routeOverride?.value.path || route.path;
    const targetTitle =
      routeOverride?.value.title || String(route.meta.title ?? targetPath);
    return {
      accessToken: storedAccessToken(),
      assetURL: "",
      baseURL: "",
      container: document.body,
      locale: locale.value,
      messages: {},
      query: Object.fromEntries(
        Object.entries(route.query).flatMap(([key, value]) => {
          const normalized = Array.isArray(value) ? value[0] : value;
          return normalized == null ? [] : [[key, String(normalized)]];
        }),
      ),
      route,
      routePath: targetPath,
      t: (_key, fallback = _key) => fallback,
      title: targetTitle,
    };
  });
}
