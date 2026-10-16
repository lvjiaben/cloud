<script lang="ts">
import { pluginSlotKeys } from "#/plugins/plugin-slots";
export const pluginSlotMeta = { order: -100, slotKey: pluginSlotKeys.authLoginAfter };
</script>

<script setup lang="ts">
import { onMounted } from "vue";
import { useRouter } from "vue-router";
import { clearPendingAssistance, readPendingAssistance } from "../../../../assistance-launch";

const router = useRouter();
onMounted(() => {
  const pending = readPendingAssistance();
  if (!pending) return;
  // A revoked previous session has now been cleared by the normal host workflow.
  // Remove before navigating: failures or reused codes never restart a loop.
  try { sessionStorage.setItem("idc-core:resumed-assistance", pending.sessionNo); }
  catch { /* Storage restrictions must not prevent the actual exchange. */ }
  clearPendingAssistance();
  void router.replace({ path: "/account/assistance", query: { token: pending.token } });
});
</script>

<template><span v-if="false" /></template>
