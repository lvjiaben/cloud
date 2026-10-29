<script setup lang="ts">
import type { NativeServerItem } from "#/api/idc/native-servers";
import { computed, watch } from "vue";
import { useRouter } from "vue-router";
import { preferences } from "@vben/preferences";
import { Card, Tag } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { listNativeServers } from "#/api/idc/native-servers";
import { $t } from "#/locales";
const props = defineProps<{ compact?: boolean }>();
const router = useRouter();
const emit = defineEmits<{ total: [value: number] }>();
const columns = computed(() => [
  {
    field: "product",
    title: $t("nativeServers.product"),
    minWidth: 260,
    align: "left" as const,
    slots: { default: "product" },
  },
  {
    field: "state",
    title: $t("nativeServers.resourceState"),
    width: 140,
    slots: { default: "state" },
  },
  {
    field: "action",
    title: $t("nativeServers.actions"),
    width: 110,
    fixed: "right" as const,
    slots: { default: "action" },
  },
]);
const [Grid, grid] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    rowConfig: { keyField: "serverNo" },
    height: "auto",
    minHeight: props.compact ? 160 : 240,
    maxHeight: props.compact ? 380 : 600,
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50] },
    toolbarConfig: {
      enabled: true,
      refresh: true,
      custom: false,
      zoom: false,
      export: false,
      print: false,
    },
    proxyConfig: {
      ajax: {
        query: async ({
          page,
        }: {
          page: { currentPage: number; pageSize: number };
        }) => {
          const result = await listNativeServers(
            page.currentPage,
            page.pageSize,
          );
          emit("total", result.total);
          return result;
        },
      },
      props: { result: "items", total: "total" },
    },
  },
});
watch(columns, () => grid.setGridOptions({ columns: columns.value }), {
  immediate: true,
});
function title(item: NativeServerItem) {
  return preferences.app.locale === "zh-CN"
    ? item.productNameZhCn
    : item.productNameEnUs;
}
function state(item: NativeServerItem) {
  return $t(
    `nativeServers.mapping.${["active", "provisioning", "quarantined", "released", "releasing"].includes(item.state) ? item.state : "unknown"}`,
  );
}
</script>
<template>
  <Card
    class="mb-4"
    :title="$t('nativeServers.title')"
    data-testid="native-server-list"
  >
    <Grid>
      <template #product="{ row }"
        ><div class="native-list-name">
          <strong>{{ title(row) }}</strong
          ><small>{{ row.serverNo }}</small>
        </div></template
      >
      <template #state="{ row }"
        ><Tag>{{ state(row) }}</Tag></template
      >
      <template #action="{ row }"
        ><ghost-button
          :aria-label="$t('nativeServers.manage')"
          @click="
            router.push(
              `/account/native-servers/${encodeURIComponent(row.serverNo)}`,
            )
          "
          >{{ $t("nativeServers.manage") }}</ghost-button
        ></template
      >
    </Grid>
  </Card>
</template>
<style scoped>
.native-list-name {
  display: grid;
  gap: 6px;
  padding: 6px 0;
}
.native-list-name small {
  font-size: 11px;
  color: hsl(var(--muted-foreground));
  font-family: ui-monospace, monospace;
  overflow-wrap: anywhere;
}
.native-list-name strong {
  font-weight: 500;
}
</style>
