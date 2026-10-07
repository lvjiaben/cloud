<script setup lang="ts" generic="Row extends Record<string, unknown>">
import type {
  IsolatedTableColumn,
  IsolatedTableHandle,
  IsolatedTablePagination,
} from "@linapro/vben-plugin-adapter/isolated";
import { mountIsolatedVbenTable } from "@linapro/vben-plugin-adapter/isolated";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    columns: IsolatedTableColumn<Row>[];
    rows: Row[];
    rowKey: keyof Row & string;
    height?: number;
    locale: string;
    loading?: boolean;
    emptyText?: string;
    pagination?: IsolatedTablePagination;
  }>(),
  { height: 300, loading: false, emptyText: "" },
);
const emit = defineEmits<{ failed: []; ready: [] }>();
const container = ref<HTMLElement>();
let handle: IsolatedTableHandle<Row> | undefined;
let disposed = false;
const options = () => ({
  columns: props.columns,
  rows: props.rows,
  rowKey: props.rowKey,
  height: props.height,
  locale: props.locale,
  loading: props.loading,
  emptyText: props.emptyText,
  pagination: props.pagination,
});
onMounted(async () => {
  if (!container.value) return;
  try {
    handle = mountIsolatedVbenTable(container.value, options());
    await handle.ready;
    if (!disposed) emit("ready");
  } catch {
    if (!disposed) emit("failed");
  }
});
watch(
  () => [
    props.columns,
    props.rows,
    props.rowKey,
    props.height,
    props.locale,
    props.loading,
    props.emptyText,
    props.pagination,
  ],
  () => handle?.update(options()),
);
onBeforeUnmount(() => {
  disposed = true;
  handle?.destroy();
});
</script>

<template><div ref="container" class="idc-isolated-grid" /></template>
