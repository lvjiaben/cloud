import type { PluginHostMountContext } from '../src/host-context';

import '../src/styles.css';

import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

function storedAccessToken() {
  for (const key of Object.keys(localStorage)) {
    if (!key.endsWith('-core-access')) continue;
    try {
      const value = JSON.parse(localStorage.getItem(key) ?? '{}') as {
        accessToken?: string;
      };
      if (typeof value.accessToken === 'string') return value.accessToken;
    } catch {
      // Ignore unrelated invalid browser storage owned by another feature.
    }
  }
  return '';
}

export function useSourcePluginContext(pagePath: string) {
  const route = useRoute();
  const locale = ref(document.documentElement.lang || navigator.language || 'zh-CN');
  const localeObserver = new MutationObserver(() => {
    locale.value = document.documentElement.lang || 'zh-CN';
  });
  onMounted(() => localeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['lang'] }));
  onBeforeUnmount(() => localeObserver.disconnect());

  return computed<PluginHostMountContext>(() => ({
    accessToken: storedAccessToken(),
    assetURL: '',
    baseURL: '',
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
    routePath: pagePath,
    t: (_key, fallback = _key) => fallback,
    title: String(route.meta.title ?? route.path),
  }));
}
