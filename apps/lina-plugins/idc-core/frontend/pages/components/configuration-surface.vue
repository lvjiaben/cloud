<script setup lang="ts">
import { computed, provide, ref, watch } from "vue";
import { useRouter } from "vue-router";

import { PluginSandboxView } from "@vben/layouts";

import { Result, Spin } from "ant-design-vue";

import { getAllMenusApi } from "#/api/core/menu";
import { $t } from "#/locales";
import { getPluginPageByRoute } from "#/plugins/page-registry";

import { sourcePluginRouteOverrideKey } from "../source-context";

const props = defineProps<{ path: string }>();
const router = useRouter();

interface AuthorizedMenuRoute {
  children?: AuthorizedMenuRoute[];
  meta?: Record<string, unknown>;
  path?: string;
}

let authorizedMenusPromise: Promise<AuthorizedMenuRoute[]> | undefined;

function authorizedMenus() {
  if (!authorizedMenusPromise) {
    authorizedMenusPromise = getAllMenusApi()
      .then((menus) => menus as unknown as AuthorizedMenuRoute[])
      .catch((error) => {
        authorizedMenusPromise = undefined;
        throw error;
      });
  }
  return authorizedMenusPromise;
}

function findAuthorizedMeta(
  menus: AuthorizedMenuRoute[],
  path: string,
): Record<string, unknown> | undefined {
  const target = router.resolve(path).path;
  for (const menu of menus) {
    if (menu.path && router.resolve(menu.path).path === target) return menu.meta;
    const childMeta = menu.children
      ? findAuthorizedMeta(menu.children, path)
      : undefined;
    if (childMeta) return childMeta;
  }
  return undefined;
}

function hasSandboxBinding(meta: Record<string, unknown>) {
  return [
    meta.pluginSandboxSrc,
    meta.pluginSurfaceId,
    meta.pluginSurfaceKey,
    meta.pluginSurfaceVersion,
  ].every((value) => String(value ?? "").trim());
}

const sourceComponent = computed(() => {
  const routePath = props.path.replace(/^\/+|\/+$/gu, "");
  return getPluginPageByRoute(routePath)?.component;
});

const authorizedMeta = ref<Record<string, unknown>>();
const metadataLoading = ref(false);
let metadataGeneration = 0;

watch(
  () => props.path,
  async (path) => {
    const generation = ++metadataGeneration;
    authorizedMeta.value = undefined;
    const routeMeta = router.resolve(path).meta as Record<string, unknown>;
    if (sourceComponent.value || hasSandboxBinding(routeMeta)) return;
    metadataLoading.value = true;
    try {
      const menus = await authorizedMenus();
      if (generation !== metadataGeneration) return;
      authorizedMeta.value = findAuthorizedMeta(menus, path);
    } finally {
      if (generation === metadataGeneration) metadataLoading.value = false;
    }
  },
  { immediate: true },
);

const surfaceMeta = computed(() => {
  const routeMeta = router.resolve(props.path).meta as Record<string, unknown>;
  return hasSandboxBinding(routeMeta) ? routeMeta : authorizedMeta.value;
});

provide(
  sourcePluginRouteOverrideKey,
  computed(() => {
    const resolved = router.resolve(props.path);
    return {
      path: resolved.path,
      title: String(surfaceMeta.value?.title ?? resolved.meta.title ?? resolved.path),
    };
  }),
);

const sandboxBinding = computed(() => {
  const meta = surfaceMeta.value;
  if (!meta) return null;
  const source = String(meta.pluginSandboxSrc ?? "").trim();
  const pluginId = String(meta.pluginSurfaceId ?? "").trim();
  const surfaceKey = String(meta.pluginSurfaceKey ?? "").trim();
  const surfaceVersion = String(meta.pluginSurfaceVersion ?? "").trim();
  if (!source || !pluginId || !surfaceKey || !surfaceVersion) return null;
  return {
    capabilities: Array.isArray(meta.pluginSurfaceCapabilities)
      ? meta.pluginSurfaceCapabilities.map(String)
      : [],
    host:
      meta.pluginSurfaceHost === "customer"
        ? ("customer" as const)
        : ("admin" as const),
    pluginId,
    pluginPath: "",
    source,
    surfaceKey,
    surfaceVersion,
    title: String(meta.title ?? ""),
  };
});
</script>

<template>
  <div class="configuration-surface" :data-surface-path="path">
    <component :is="sourceComponent" v-if="sourceComponent" />
    <div v-else-if="metadataLoading" class="configuration-surface__loading">
      <Spin />
    </div>
    <PluginSandboxView
      v-else-if="sandboxBinding"
      :binding="sandboxBinding"
      embedded
    />
    <Result
      v-else
      status="404"
      :title="$t('page.plugin.dynamicPage.notFoundTitle')"
      :sub-title="$t('page.plugin.dynamicPage.notFoundSubtitle')"
    />
  </div>
</template>

<style scoped>
.configuration-surface {
  height: 100%;
  min-height: 520px;
  min-width: 0;
}

.configuration-surface :deep(.vben-page) {
  height: 100%;
  min-height: 0;
}

/* The drawer already names the selected configuration. Keep its actions. */
.configuration-surface :deep(.workspace-toolbar h1),
.configuration-surface :deep(.workspace-toolbar h2) {
  display: none;
}

.configuration-surface :deep(.workspace-toolbar) {
  padding: 12px;
}

.configuration-surface__loading {
  display: grid;
  min-height: 320px;
  place-items: center;
}
</style>
