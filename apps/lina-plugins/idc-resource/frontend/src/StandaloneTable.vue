<script setup lang="ts" generic="Row extends Record<string, unknown>">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  mountIsolatedVbenTable,
  type IsolatedTableColumn,
  type IsolatedTableHandle,
  type IsolatedTablePagination,
  type IsolatedTableOptions,
} from "@linapro/vben-plugin-adapter/isolated";

const emit = defineEmits<{ failed: [] }>();
const props = defineProps<{
  columns: IsolatedTableColumn<Row>[];
  rows: Row[];
  rowKey: string;
  locale: string;
  emptyText: string;
  loading: boolean;
  pagination?: IsolatedTablePagination;
  search?: IsolatedTableOptions<Row>["search"];
  actions?: IsolatedTableOptions<Row>["actions"];
  height?: number;
}>();
const container = ref<HTMLElement>();
let table: IsolatedTableHandle<Row> | undefined;
let disposed = false;
function failed() { if (!disposed) emit("failed"); }
function options() {
  return { ...props, height: props.height ?? 420 };
}
onMounted(() => {
  if (container.value) {
    try {
      table = mountIsolatedVbenTable(container.value, options());
      void table.ready.catch(failed);
    } catch { failed(); }
  }
});
watch(() => [props.columns, props.rows, props.locale, props.emptyText, props.loading, props.pagination, props.search, props.actions, props.height], () => table?.update(options()));
onBeforeUnmount(() => { disposed = true; table?.destroy(); });
</script>

<template>
  <div ref="container" class="standalone-vben-table" />
</template>
