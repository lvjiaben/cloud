<script lang="ts">
import { pluginSlotKeys } from "#/plugins/plugin-slots";
export const pluginSlotMeta = { order: -100, slotKey: pluginSlotKeys.layoutHeaderActionsBefore };
</script>

<script setup lang="ts">
import { onMounted, watch } from "vue";
import { message } from "ant-design-vue";
import { useAuthStore } from "#/store";
import { clearPendingAssistance, readPendingAssistance } from "../../../../assistance-launch";

const authStore = useAuthStore() as ReturnType<typeof useAuthStore> & {
  customerAssistanceSession?: { sessionNo: string };
};
function clearCompletedHandoff() {
  const pending = readPendingAssistance();
  let resumed: string | null = null;
  try { resumed = sessionStorage.getItem("idc-core:resumed-assistance"); }
  catch { /* Ordinary customer login works without plugin handoff storage. */ }
  if (authStore.customerAssistanceSession?.sessionNo &&
      (pending?.sessionNo === authStore.customerAssistanceSession.sessionNo ||
       resumed === authStore.customerAssistanceSession.sessionNo)) {
    // Successful new login makes the previous session's expiry hint obsolete.
    message.destroy();
    clearPendingAssistance();
    try { sessionStorage.removeItem("idc-core:resumed-assistance"); }
    catch { /* Successful authentication does not depend on hint cleanup. */ }
  }
}
onMounted(clearCompletedHandoff);
watch(() => authStore.customerAssistanceSession?.sessionNo, clearCompletedHandoff);
</script>

<template><span v-if="false" /></template>
