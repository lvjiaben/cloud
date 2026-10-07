<script setup lang="ts" generic="Row extends Record<string, unknown>">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  mountIsolatedVbenTable,
  type IsolatedTableColumn,
  type IsolatedTableHandle,
  type IsolatedTablePagination,
} from "@linapro/vben-plugin-adapter/isolated";

const emit = defineEmits<{ failed: []; pageChange: [page: { currentPage: number; pageSize: number }] }>();
const props = defineProps<{
  columns: IsolatedTableColumn<Row>[];
  rows: Row[];
  rowKey: string;
  locale: string;
  emptyText: string;
  loading: boolean;
  pagination?: Omit<IsolatedTablePagination, "onChange">;
}>();
const container = ref<HTMLElement>();
let table: IsolatedTableHandle<Row> | undefined;
let disposed = false;
function failed() { if (!disposed) emit("failed"); }
function options() {
  return { ...props, height: 420, pagination: props.pagination ? { ...props.pagination, onChange: (page: { currentPage: number; pageSize: number }) => emit("pageChange", page) } : undefined };
}
onMounted(() => {
  if (container.value) {
    try {
      table = mountIsolatedVbenTable(container.value, options());
      void table.ready.catch(failed);
    } catch { failed(); }
  }
});
watch(() => [props.columns, props.rows, props.locale, props.emptyText, props.loading, props.pagination], () => table?.update(options()));
onBeforeUnmount(() => { disposed = true; table?.destroy(); });
</script>

<template>
  <div ref="container" class="standalone-vben-table" />
</template>
