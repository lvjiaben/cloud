<script lang="ts">
import { router } from "#/router";
import { pluginSlotKeys } from "#/plugins/plugin-slots";
import { bindOpenPlatformMenu } from "../../../../pages/customer/open-platform-menu";
export default {};
export const pluginSlotMeta = {
  slotKey: pluginSlotKeys.layoutHeaderActionsBefore,
  order: 99,
};
// Source modules are loaded by the public registry even where this outlet is not mounted.
// Defer access until router readiness, after the host has installed its Pinia instance.
queueMicrotask(() => {
  void router.isReady().then(() => bindOpenPlatformMenu(router));
});
</script>
<template><span class="hidden" aria-hidden="true" /></template>
