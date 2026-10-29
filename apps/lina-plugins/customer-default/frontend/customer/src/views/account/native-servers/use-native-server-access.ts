// Both console entry pages use the current plugin snapshot and lifecycle signal.
import { onBeforeUnmount, onMounted, ref } from "vue";
import {
  getPluginStateMap,
  onPluginRegistryChanged,
} from "#/plugins/slot-registry";

export function useNativeServerAccess() {
  const available = ref(false);
  const ready = ref(false);
  let disposed = false;
  let sequence = 0;
  const enabled = (value: unknown) =>
    value === true || value === 1 || value === "1";
  async function refresh() {
    const current = ++sequence;
    try {
      const states = await getPluginStateMap();
      if (disposed || current !== sequence) return;
      const state = states.get("idc-resource");
      available.value =
        !!state &&
        enabled(state.installed) &&
        enabled(state.enabled) &&
        (!state.runtimeState || state.runtimeState === "normal");
    } catch {
      if (!disposed && current === sequence) available.value = false;
    } finally {
      if (!disposed && current === sequence) ready.value = true;
    }
  }
  const unsubscribe = onPluginRegistryChanged(refresh);
  onMounted(refresh);
  onBeforeUnmount(() => {
    disposed = true;
    sequence++;
    unsubscribe();
  });
  return { available, ready };
}
