<script setup lang="ts">
import type {
  IsolatedEchartsHandle,
  IsolatedEchartsOption,
} from "@linapro/vben-plugin-adapter/isolated";
import { mountIsolatedVbenEcharts } from "@linapro/vben-plugin-adapter/isolated";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    option: IsolatedEchartsOption;
    height?: number;
  }>(),
  { height: 300 },
);
const container = ref<HTMLElement>();
let handle: IsolatedEchartsHandle | undefined;

onMounted(() => {
  if (!container.value) return;
  handle = mountIsolatedVbenEcharts(container.value, {
    option: props.option,
    height: props.height,
  });
});

watch(
  () => props.option,
  (next) => {
    handle?.update({ option: next });
  },
  { deep: true },
);

onBeforeUnmount(() => {
  handle?.destroy();
  handle = undefined;
});
</script>

<template>
  <div ref="container" :style="{ width: '100%' }"></div>
</template>
