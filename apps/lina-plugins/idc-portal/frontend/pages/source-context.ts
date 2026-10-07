import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute } from "vue-router";

export interface PortalMountContext {
  accessToken: string;
  assetURL: string;
  baseURL: string;
  container: HTMLElement;
  locale: string;
  messages: Record<string, unknown>;
  query: Record<string, string>;
  route: ReturnType<typeof useRoute>;
  routePath: string;
  t: (key: string, fallback?: string) => string;
  title: string;
}

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

export function usePortalMountContext() {
  const route = useRoute();
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

  return computed<PortalMountContext>(() => ({
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
    routePath: route.path,
    t: (_key, fallback = _key) => fallback,
    title: String(route.meta.title ?? route.path),
  }));
}
