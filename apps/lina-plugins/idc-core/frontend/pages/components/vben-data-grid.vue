<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, watch } from "vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";

type SourceColumn = {
  dataIndex?: string;
  fixed?: "left" | "right";
  key?: string;
  minWidth?: number;
  title?: string;
  width?: number;
};

const props = withDefaults(
  defineProps<{
    columns: unknown[];
    data: unknown[];
    height?: number;
    loading?: boolean;
    pagination?: { currentPage: number; pageSize: number; total: number };
    rowKey: string;
  }>(),
  { height: 360, loading: false },
);

const emit = defineEmits<{
  pageChange: [page: { currentPage: number; pageSize: number }];
}>();

const normalizedColumns = computed<VxeGridProps["columns"]>(() =>
  (props.columns as SourceColumn[]).map((column) => ({
    field: column.key || column.dataIndex || "value",
    fixed: column.fixed,
    minWidth: column.minWidth,
    showOverflow: "tooltip",
    slots: { default: "cell" },
    title: column.title,
    width: column.width,
  })),
);

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: (page: { currentPage: number; pageSize: number }) =>
      emit("pageChange", page),
  },
  gridOptions: {
    border: true,
    columns: normalizedColumns.value,
    data: [],
    height: props.height,
    loading: props.loading,
    pagerConfig: props.pagination
      ? { enabled: true, ...props.pagination }
      : { enabled: false },
    rowConfig: { keyField: props.rowKey },
    showOverflow: true,
  },
});

watch(
  () => props.data,
  (rows) => gridApi.setGridOptions({ data: rows as Record<string, unknown>[] }),
  { deep: true, immediate: true },
);

watch(
  () => props.pagination,
  (pagination) =>
    gridApi.setGridOptions({
      pagerConfig: pagination
        ? { enabled: true, ...pagination }
        : { enabled: false },
    }),
  { deep: true },
);

watch(
  () => [normalizedColumns.value, props.height, props.loading] as const,
  ([columns, height, loading]) =>
    gridApi.setGridOptions({ columns, height, loading }),
  { deep: true },
);
</script>

<template>
  <Grid>
    <template #cell="{ column, row }">
      <slot
        name="cell"
        :column="{ key: column.field }"
        :record="row"
        :value="row[column.field]"
      />
    </template>
  </Grid>
</template>
