<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';

import { Result as AResult } from 'ant-design-vue';

import { $t } from '#/locales';
import {
  getPluginPageByOwnerAndEntry,
  getPluginPageByRoute,
} from '#/plugins/page-registry';

const governedSurfacePluginQueryKey = 'pluginSurfacePlugin';
const governedSurfaceEntryQueryKey = 'pluginSurfaceEntry';

const route = useRoute();

function queryValue(key: string) {
  const metaValue = (route.meta.query as Record<string, unknown> | undefined)?.[
    key
  ];
  const routeValue = route.query[key];
  const value = routeValue ?? metaValue;
  return Array.isArray(value)
    ? String(value[0] ?? '').trim()
    : String(value ?? '').trim();
}

const pageEntry = computed(() => {
  const pluginId = queryValue(governedSurfacePluginQueryKey);
  const entry = queryValue(governedSurfaceEntryQueryKey);
  if (pluginId || entry) {
    if (!pluginId || !entry) {
      return null;
    }
    return getPluginPageByOwnerAndEntry(pluginId, entry);
  }

  const normalizedPath = route.path.replace(/^\/+|\/+$/gu, '');
  const routeSegments = normalizedPath.split('/').filter(Boolean);
  for (const routePath of [normalizedPath, routeSegments.at(-1) ?? '']) {
    const candidate = getPluginPageByRoute(routePath);
    if (candidate) {
      return candidate;
    }
  }
  return null;
});
</script>

<template>
  <component :is="pageEntry.component" v-if="pageEntry" />
  <a-result
    v-else
    status="404"
    :title="$t('page.plugin.dynamicPage.notFoundTitle')"
    :sub-title="$t('page.plugin.dynamicPage.notFoundSubtitle')"
  />
</template>
