<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { $t } from '@vben/locales';
import { preferences, usePreferences } from '@vben/preferences';
import { useTabbarStore } from '@vben/stores';

import {
  getPluginSandboxBridgeAdapter,
  PLUGIN_PAGE_BRIDGE_PROTOCOL,
  PLUGIN_PAGE_BRIDGE_VERSION,
  prewarmPluginSandboxSource,
} from './plugin-sandbox-bridge';

defineOptions({ name: 'PluginSandboxView' });

type EmbeddedSurfaceBinding = {
  capabilities?: string[];
  host?: 'admin' | 'customer';
  pluginId: string;
  pluginPath?: string;
  source: string;
  surfaceKey: string;
  surfaceVersion: string;
  title?: string;
};

const props = withDefaults(
  defineProps<{
    binding?: EmbeddedSurfaceBinding;
    embedded?: boolean;
  }>(),
  { embedded: false },
);

type BridgeEnvelope = {
  capability?: unknown;
  direction?: unknown;
  method?: unknown;
  payload?: unknown;
  pluginId?: unknown;
  protocol?: unknown;
  requestId?: unknown;
  surfaceKey?: unknown;
  version?: unknown;
};

type APIRequestPayload = {
  body?: unknown;
  download?: unknown;
  method?: unknown;
  params?: unknown;
  path?: unknown;
};

type PageBinding = {
  generation: number;
  host: 'admin' | 'customer';
  pluginId: string;
  surfaceKey: string;
  surfaceVersion: string;
  target: Window;
};

const route = useRoute();
const router = useRouter();
const tabbarStore = useTabbarStore();
const { isDark, locale } = usePreferences();
const frame = ref<HTMLIFrameElement>();
const frameHeight = ref(640);
const loadFailed = ref(false);
const loading = ref(true);
const reloadKey = ref(0);
const requestedTitle = ref('');
const embeddedPluginPath = ref('');
let loadTimeout: ReturnType<typeof setTimeout> | undefined;
let titleSyncTimer: ReturnType<typeof setTimeout> | undefined;
let generation = 0;
const inFlightRequests = new Map<string, AbortController>();
const seenRequestIds = new Set<string>();
const maxInFlightRequests = 8;
const maxRequestIdsPerGeneration = 2048;

const source = computed(() =>
  String(props.binding?.source ?? route.meta.pluginSandboxSrc ?? '').trim(),
);
const pluginId = computed(() =>
  String(props.binding?.pluginId ?? route.meta.pluginSurfaceId ?? '').trim(),
);
const surfaceKey = computed(() =>
  String(props.binding?.surfaceKey ?? route.meta.pluginSurfaceKey ?? '').trim(),
);
const surfaceVersion = computed(() =>
  String(
    props.binding?.surfaceVersion ?? route.meta.pluginSurfaceVersion ?? '',
  ).trim(),
);
const surfaceHost = computed<'admin' | 'customer'>(() =>
  (props.binding?.host ?? route.meta.pluginSurfaceHost) === 'customer'
    ? 'customer'
    : 'admin',
);
const declaredCapabilities = computed(
  () =>
    new Set<string>(
      props.binding?.capabilities ??
        (Array.isArray(route.meta.pluginSurfaceCapabilities)
          ? route.meta.pluginSurfaceCapabilities.map(String)
          : []),
    ),
);
const frameStyle = computed(() => ({ height: `${frameHeight.value}px` }));
const currentPluginPath = computed(() =>
  props.embedded
    ? embeddedPluginPath.value
    : String(route.query.pluginPath ?? ''),
);
const surfaceTitle = computed(() =>
  String(props.binding?.title ?? route.meta.title ?? ''),
);

function clearTimers() {
  if (loadTimeout) clearTimeout(loadTimeout);
  if (titleSyncTimer) clearTimeout(titleSyncTimer);
  loadTimeout = undefined;
  titleSyncTimer = undefined;
}

function cancelInFlightRequests() {
  for (const controller of inFlightRequests.values()) controller.abort();
  inFlightRequests.clear();
}

function beginLoad() {
  clearTimers();
  generation += 1;
  cancelInFlightRequests();
  seenRequestIds.clear();
  loadFailed.value = false;
  loading.value = true;
  requestedTitle.value = '';
  frameHeight.value = 640;
  prewarmPluginSandboxSource(source.value);
  loadTimeout = setTimeout(() => {
    loadFailed.value = true;
    loading.value = false;
  }, 20_000);
}

function retry() {
  reloadKey.value += 1;
  beginLoad();
}

function postToPlugin(capability: string, method: string, payload: unknown) {
  frame.value?.contentWindow?.postMessage(
    {
      capability,
      direction: 'host-to-plugin',
      method,
      payload,
      pluginId: pluginId.value,
      protocol: PLUGIN_PAGE_BRIDGE_PROTOCOL,
      surfaceKey: surfaceKey.value,
      version: PLUGIN_PAGE_BRIDGE_VERSION,
    },
    '*',
  );
}

function syncHostContext() {
  if (!frame.value?.contentWindow) return;
  const rootStyle = getComputedStyle(document.documentElement);
  const bodyStyle = getComputedStyle(document.body);
  postToPlugin('host.context.v1', 'sync', {
    host: surfaceHost.value,
    locale: locale.value,
    route: {
      path: currentPluginPath.value,
      title: surfaceTitle.value,
    },
    theme: {
      colorPrimary:
        rootStyle.getPropertyValue('--primary').trim() ||
        preferences.theme.colorPrimary,
      cssVariables: Object.fromEntries(
        [
          '--background',
          '--border',
          '--card',
          '--card-foreground',
          '--foreground',
          '--muted',
          '--muted-foreground',
          '--primary',
          '--primary-foreground',
        ].map((name) => [name, rootStyle.getPropertyValue(name).trim()]),
      ),
      fontFamily: bodyStyle.fontFamily,
      fontSize: preferences.theme.fontSize,
      mode: isDark.value ? 'dark' : 'light',
      radius: rootStyle.getPropertyValue('--radius').trim(),
      spacing: rootStyle.getPropertyValue('--spacing').trim() || '0.25rem',
    },
  });
}

function bindingIsCurrent(binding: PageBinding) {
  return (
    binding.generation === generation &&
    frame.value?.contentWindow === binding.target &&
    binding.pluginId === pluginId.value &&
    binding.surfaceKey === surfaceKey.value &&
    binding.surfaceVersion === surfaceVersion.value
  );
}

function sendResponse(
  binding: PageBinding,
  request: BridgeEnvelope,
  ok: boolean,
  result?: unknown,
  errorCode?: string,
) {
  if (!bindingIsCurrent(binding)) return;
  binding.target.postMessage(
    {
      capability: request.capability,
      direction: 'host-to-plugin',
      error: errorCode ? { code: errorCode } : undefined,
      method: request.method,
      ok,
      pluginId: binding.pluginId,
      protocol: PLUGIN_PAGE_BRIDGE_PROTOCOL,
      requestId: request.requestId,
      result,
      surfaceKey: binding.surfaceKey,
      version: PLUGIN_PAGE_BRIDGE_VERSION,
    },
    '*',
  );
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isSafeRequestId(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    value.length <= 80 &&
    /^[\w-]+$/.test(value)
  );
}

function isSafePluginPath(value: unknown): value is string {
  if (typeof value !== 'string' || value.length === 0 || value.length > 300) {
    return false;
  }
  return (
    !value.startsWith('/') &&
    !value.includes('..') &&
    !value.includes('//') &&
    !value.includes('%') &&
    /^[\w/-]+$/.test(value)
  );
}

function payloadWithinLimit(payload: unknown) {
  try {
    return JSON.stringify(payload).length <= 256 * 1024;
  } catch {
    return false;
  }
}

function safeDownloadFileName(value: unknown) {
  if (typeof value !== 'string') return undefined;
  const normalized = value.trim();
  return /^[\w.-]{1,128}$/.test(normalized) ? normalized : undefined;
}

function cloneJSONValue(payload: unknown) {
  if (payload === undefined) return undefined;
  // The bridge deliberately reduces structured-clone values to plain JSON so
  // Blob/File and other browser objects cannot reach the authenticated client.
  // eslint-disable-next-line unicorn/prefer-structured-clone
  return JSON.parse(JSON.stringify(payload)) as unknown;
}

async function handleAPIRequest(request: BridgeEnvelope, binding: PageBinding) {
  if (
    request.method !== 'request' ||
    typeof request.capability !== 'string' ||
    !declaredCapabilities.value.has(request.capability) ||
    !isSafeRequestId(request.requestId) ||
    !isPlainRecord(request.payload) ||
    !payloadWithinLimit(request.payload)
  ) {
    sendResponse(binding, request, false, undefined, 'bridge_request_denied');
    return;
  }
  const payload = request.payload as APIRequestPayload;
  const method = String(payload.method ?? 'GET').toUpperCase();
  const download =
    payload.download === undefined
      ? undefined
      : isPlainRecord(payload.download)
        ? payload.download
        : null;
  const downloadFileName =
    download === undefined
      ? undefined
      : safeDownloadFileName(download?.fileName);
  if (
    !['DELETE', 'GET', 'PATCH', 'POST', 'PUT'].includes(method) ||
    !isSafePluginPath(payload.path) ||
    (payload.params !== undefined && !isPlainRecord(payload.params)) ||
    download === null ||
    (download !== undefined &&
      (method !== 'GET' || downloadFileName === undefined))
  ) {
    sendResponse(binding, request, false, undefined, 'bridge_request_invalid');
    return;
  }
  const adapter = getPluginSandboxBridgeAdapter();
  if (!adapter) {
    sendResponse(binding, request, false, undefined, 'bridge_unavailable');
    return;
  }
  if (
    seenRequestIds.has(request.requestId) ||
    seenRequestIds.size >= maxRequestIdsPerGeneration ||
    inFlightRequests.size >= maxInFlightRequests
  ) {
    sendResponse(binding, request, false, undefined, 'bridge_request_busy');
    return;
  }
  seenRequestIds.add(request.requestId);
  const controller = new AbortController();
  inFlightRequests.set(request.requestId, controller);
  const timeout = setTimeout(() => controller.abort(), 30_000);
  try {
    const result = await adapter.request({
      body: cloneJSONValue(payload.body),
      capability: request.capability,
      downloadFileName,
      host: binding.host,
      method: method as 'DELETE' | 'GET' | 'PATCH' | 'POST' | 'PUT',
      params: cloneJSONValue(payload.params) as
        | Record<string, unknown>
        | undefined,
      path: payload.path,
      pluginId: binding.pluginId,
      signal: controller.signal,
      surfaceKey: binding.surfaceKey,
      surfaceVersion: binding.surfaceVersion,
    });
    sendResponse(binding, request, true, result);
  } catch {
    sendResponse(
      binding,
      request,
      false,
      undefined,
      controller.signal.aborted
        ? 'bridge_request_timeout'
        : 'bridge_request_failed',
    );
  } finally {
    clearTimeout(timeout);
    if (inFlightRequests.get(request.requestId) === controller) {
      inFlightRequests.delete(request.requestId);
    }
  }
}

async function handleHostViewMessage(
  request: BridgeEnvelope,
  binding: PageBinding,
) {
  if (!bindingIsCurrent(binding)) return false;
  if (request.capability !== 'host.view.v1') return false;
  if (request.method === 'ready') {
    clearTimers();
    loading.value = false;
    syncHostContext();
    return true;
  }
  if (request.method === 'resize' && isPlainRecord(request.payload)) {
    const nextHeight = Number(request.payload.height);
    if (Number.isFinite(nextHeight)) {
      frameHeight.value = Math.min(
        100_000,
        Math.max(460, Math.ceil(nextHeight)),
      );
    }
    return true;
  }
  if (request.method === 'error') {
    clearTimers();
    loadFailed.value = true;
    loading.value = false;
    return true;
  }
  if (request.method === 'set-title' && isPlainRecord(request.payload)) {
    const title = String(request.payload.title ?? '').trim();
    if (title && title.length <= 80) {
      requestedTitle.value = title;
      if (props.embedded) return true;
      route.meta.title = title;
      let attempts = 0;
      const applyTabTitle = async () => {
        if (!bindingIsCurrent(binding)) return;
        const currentTab = tabbarStore.getTabs.find(
          (tab) =>
            tab.fullPath === route.fullPath ||
            tab.path === route.path ||
            tab.name === route.name,
        );
        if (currentTab) {
          await tabbarStore.setTabTitle(currentTab, title);
          if (!bindingIsCurrent(binding)) return;
          tabbarStore.setUpdateTime();
          return;
        }
        attempts += 1;
        if (attempts < 20 && bindingIsCurrent(binding)) {
          titleSyncTimer = setTimeout(applyTabTitle, 100);
        }
      };
      await applyTabTitle();
      if (bindingIsCurrent(binding)) {
        document.title = `${title} - ${preferences.app.name}`;
      }
    }
    return true;
  }
  if (request.method === 'navigate' && isPlainRecord(request.payload)) {
    const pluginPath = request.payload.path;
    if (pluginPath === '' || isSafePluginPath(pluginPath)) {
      if (!bindingIsCurrent(binding)) return true;
      if (props.embedded) {
        embeddedPluginPath.value = String(pluginPath);
        syncHostContext();
        return true;
      }
      const navigationTarget = {
        path: route.path,
        query: {
          ...route.query,
          pluginPath: String(pluginPath) || undefined,
        },
      };
      const targetFullPath = router.resolve(navigationTarget).fullPath;
      const removeGenerationGuard = router.beforeResolve((to) => {
        if (to.fullPath !== targetFullPath) return true;
        return bindingIsCurrent(binding);
      });
      try {
        await router.replace(navigationTarget);
      } finally {
        removeGenerationGuard();
      }
    }
    return true;
  }
  return false;
}

async function handleMessage(event: MessageEvent) {
  // Sandboxed plugin documents normally report the opaque `null` origin. Some
  // dev-server/browser combinations preserve the host origin while the
  // sandbox flags still prevent DOM access; the window identity check remains
  // mandatory so messages can only come from this exact iframe.
  if (
    event.source !== frame.value?.contentWindow ||
    (event.origin !== 'null' && event.origin !== window.location.origin)
  ) {
    return;
  }
  if (!isPlainRecord(event.data)) return;
  const binding: PageBinding = {
    generation,
    host: surfaceHost.value,
    pluginId: pluginId.value,
    surfaceKey: surfaceKey.value,
    surfaceVersion: surfaceVersion.value,
    target: event.source as Window,
  };
  const request = event.data as BridgeEnvelope;
  if (
    request.protocol !== PLUGIN_PAGE_BRIDGE_PROTOCOL ||
    request.version !== PLUGIN_PAGE_BRIDGE_VERSION ||
    request.direction !== 'plugin-to-host' ||
    request.pluginId !== pluginId.value ||
    request.surfaceKey !== surfaceKey.value
  ) {
    return;
  }
  if (await handleHostViewMessage(request, binding)) return;
  await handleAPIRequest(request, binding);
}

function handleFrameLoad() {
  if (loadTimeout) clearTimeout(loadTimeout);
  loadTimeout = undefined;
  syncHostContext();
}

watch(
  [source, pluginId, surfaceKey, surfaceVersion, surfaceHost],
  () => {
    embeddedPluginPath.value = props.binding?.pluginPath ?? '';
    reloadKey.value += 1;
    beginLoad();
  },
  { immediate: true },
);

watch(
  [
    locale,
    isDark,
    () => preferences.theme.colorPrimary,
    () => preferences.theme.fontSize,
    () => preferences.theme.radius,
    currentPluginPath,
  ],
  syncHostContext,
);

onMounted(() => window.addEventListener('message', handleMessage));
onBeforeUnmount(() => {
  clearTimers();
  generation += 1;
  cancelInFlightRequests();
  seenRequestIds.clear();
  window.removeEventListener('message', handleMessage);
});
</script>

<template>
  <section
    class="relative size-full min-h-[460px] overflow-hidden rounded-(--radius) bg-background"
    :data-plugin-title="requestedTitle"
    data-testid="plugin-sandbox-container"
  >
    <div
      v-if="loading && !loadFailed"
      class="absolute inset-0 z-10 space-y-4 bg-background p-5"
      data-testid="plugin-sandbox-skeleton"
    >
      <div class="h-8 w-48 animate-pulse rounded-sm bg-muted"></div>
      <div class="h-10 w-full animate-pulse rounded-sm bg-muted"></div>
      <div class="h-72 w-full animate-pulse rounded-sm bg-muted"></div>
    </div>

    <div
      v-if="loadFailed"
      class="absolute inset-0 z-20 flex-col-center gap-3 bg-background p-6 text-center"
      data-testid="plugin-sandbox-error"
    >
      <strong class="text-base text-foreground">{{
        $t('ui.fallback.internalError')
      }}</strong>
      <span class="text-sm text-muted-foreground">{{
        $t('ui.fallback.internalErrorDesc')
      }}</span>
      <button
        class="rounded-sm bg-primary px-4 py-2 text-sm text-primary-foreground"
        type="button"
        @click="retry"
      >
        {{ $t('common.refresh') }}
      </button>
    </div>

    <iframe
      v-if="source"
      :key="`${source}:${reloadKey}`"
      ref="frame"
      :src="source"
      :style="frameStyle"
      class="block min-h-[460px] w-full border-0 bg-transparent"
      data-testid="plugin-sandbox-frame"
      referrerpolicy="no-referrer"
      sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
      scrolling="no"
      @load="handleFrameLoad"
    ></iframe>
  </section>
</template>
