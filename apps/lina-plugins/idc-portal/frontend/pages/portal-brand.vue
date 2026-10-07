<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/portal-brand",
  title: "Website Brand Settings",
};
</script>

<script setup lang="ts">
import { Page } from "@vben/common-ui";
import { onBeforeUnmount, onMounted, ref } from "vue";

import { $t } from "#/locales";

import { usePortalMountContext } from "./source-context";

type MountedPortal = { unmount?: () => void };
type PortalModule = {
  mount: (
    context: ReturnType<typeof usePortalMountContext>["value"],
  ) => MountedPortal;
};

const mountPoint = ref<HTMLElement>();
const loadError = ref("");
const context = usePortalMountContext();
let mountedPortal: MountedPortal | undefined;

onMounted(async () => {
  if (!mountPoint.value) return;
  try {
    const source = "/x-assets/idc-portal/v0.1.10/mount.js";
    const module = (await import(/* @vite-ignore */ source)) as PortalModule;
    mountedPortal = module.mount({
      ...context.value,
      container: mountPoint.value,
    });
  } catch {
    loadError.value = $t("plugin.idc-portal.loadError");
  }
});

onBeforeUnmount(() => mountedPortal?.unmount?.());
</script>

<template>
  <Page :auto-content-height="true">
    <div
      ref="mountPoint"
      class="portal-brand-page h-full min-h-0"
      data-plugin-frontend-root="idc-portal"
    >
      <p v-if="loadError" class="portal-brand-error">{{ loadError }}</p>
    </div>
  </Page>
</template>

<style scoped>
.portal-brand-page {
  min-height: 460px;
  overflow: auto;
  background: hsl(var(--background));
}

.portal-brand-error {
  margin: 0;
  padding: 32px 24px;
  color: hsl(var(--destructive));
}
</style>
