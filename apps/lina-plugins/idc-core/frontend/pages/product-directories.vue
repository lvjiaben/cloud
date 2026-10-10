<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog/product-directories",
  title: "Customer Product Directories",
};
</script>
<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { ProductDirectory } from "./product-directory-client";
import { computed, onMounted, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Button, Tag } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { useAccess } from "./plugin-access";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";
import { listProductDirectories } from "./product-directory-client";
import DirectoryEditor from "./components/product-directory-editor.vue";
const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes(["idc:admin:product-directory:create"]),
);
const canUpdate = computed(() =>
  hasAccessByCodes(["idc:admin:product-directory:update"]),
);
const key = "plugin.idc-core.productDirectory.";
const [Editor, editorApi] = useVbenModal({
  connectedComponent: DirectoryEditor,
});
function name(row: ProductDirectory) {
  return preferences.app.locale === "zh-CN"
    ? row.nameZhCn || row.nameEnUs
    : row.nameEnUs || row.nameZhCn;
}
function columns(): VxeGridProps<ProductDirectory>["columns"] {
  return [
    {
      field: "name",
      title: $t(key + "name"),
      minWidth: 220,
      slots: { default: "name" },
    },
    { field: "productCount", title: $t(key + "productCount"), width: 140 },
    { field: "sortOrder", title: $t(key + "sortOrder"), width: 120 },
    {
      field: "statusCode",
      title: $t(key + "status"),
      width: 130,
      slots: { default: "status" },
    },
    fixedRightActionColumn({
      field: "action",
      title: $t(key + "actions"),
      width: 100,
      slots: { default: "action" },
    }),
  ];
}
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    height: "100%",
    rowConfig: { keyField: "directoryNo" },
    pagerConfig: {
      enabled: true,
      pageSize: 20,
      pageSizes: [20, 50, 100],
      layouts: [
        "Total",
        "Sizes",
        "PrevJump",
        "PrevPage",
        "Number",
        "NextPage",
        "NextJump",
        "FullJump",
      ],
    },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({
          page,
        }: {
          page: { currentPage: number; pageSize: number };
        }) => listProductDirectories(page.currentPage, page.pageSize),
      },
    },
    toolbarConfig: { refresh: true, zoom: true },
    id: "customer-product-directories",
  },
});
function edit(row?: ProductDirectory) {
  editorApi.setData(row ? { directory: row } : {});
  editorApi.open();
}
watch(
  () => preferences.app.locale,
  () => gridApi.setGridOptions({ columns: columns() }),
);
onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
</script>
<template>
  <Page auto-content-height>
    <div
      data-testid="merchant-product-directories"
      class="flex h-full flex-col gap-3"
    >
      <p class="text-muted-foreground">{{ $t(key + "note") }}</p>
      <Grid class="min-h-0 flex-1">
        <template #toolbar-tools
          ><Button v-if="canCreate" type="primary" @click="edit()">{{
            $t(key + "create")
          }}</Button></template
        >
        <template #name="{ row }">{{ name(row) }}</template>
        <template #status="{ row }"
          ><Tag :color="row.statusCode === 'enabled' ? 'green' : 'default'">{{
            $t(key + row.statusCode)
          }}</Tag></template
        >
        <template #action="{ row }"
          ><ghost-button v-if="canUpdate" @click="edit(row)">{{
            $t(key + "edit")
          }}</ghost-button></template
        >
      </Grid>
      <Editor @success="gridApi.query()" />
    </div>
  </Page>
</template>
