<script lang="ts">
import { pluginSlotKeys } from "#/plugins/plugin-slots";
export const pluginSlotMeta = {
  order: 20,
  slotKey: pluginSlotKeys.layoutHeaderActionsBefore,
};
</script>

<script setup lang="ts">
// This lifecycle slot keeps employee notifications current and renders no header action.
import { computed, onBeforeUnmount, onMounted, watch } from "vue";
import { preferences } from "@vben/preferences";
import { useAccessStore, useUserStore } from "@vben/stores";
import { useTenantStore } from "#/store/tenant";
import { useAccess } from "../../../../pages/plugin-access";
import { syncAccessoryRenewalNotifications } from "./accessory-renewal-client";

const access = useAccessStore();
const user = useUserStore();
const tenant = useTenantStore();
const { hasAccessByCodes } = useAccess();
const allowed = computed(() =>
  Boolean(
    access.accessToken &&
    user.userInfo?.userId &&
    !access.loginExpired &&
    hasAccessByCodes(["idc:admin:resource:accessory-renewal:handle"]),
  ),
);
const locale = computed(() =>
  preferences.app.locale === "en-US" ? "en-US" : "zh-CN",
);
let mounted = false;
let generation = 0;
let syncing = false;
let timer: ReturnType<typeof setInterval> | undefined;
async function synchronize() {
  if (
    !mounted ||
    !allowed.value ||
    syncing ||
    document.visibilityState !== "visible" ||
    !navigator.onLine
  )
    return;
  const scope = generation;
  syncing = true;
  try {
    await syncAccessoryRenewalNotifications(locale.value);
  } catch {
    // The server retains pending intent; retry on the next interval or from the work-items page.
  } finally {
    if (scope === generation) syncing = false;
  }
}
watch(
  () => [
    access.accessToken,
    access.loginExpired,
    access.accessCodes.join("\u0000"),
    user.userInfo?.userId,
    tenant.currentTenant?.id,
    locale.value,
  ],
  () => {
    generation += 1;
    syncing = false;
    void synchronize();
  },
  { flush: "sync" },
);
onMounted(() => {
  mounted = true;
  void synchronize();
  timer = setInterval(synchronize, 60_000);
  document.addEventListener("visibilitychange", synchronize);
  window.addEventListener("online", synchronize);
});
onBeforeUnmount(() => {
  mounted = false;
  generation += 1;
  clearInterval(timer);
  document.removeEventListener("visibilitychange", synchronize);
  window.removeEventListener("online", synchronize);
});
</script>
<template />
