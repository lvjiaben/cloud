<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/member-navigation",
  title: "Member Navigation",
};
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { ProductDirectory } from "./product-directory-client";
import { computed, onMounted, ref, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import { preferences } from "@vben/preferences";
import { Button, InputNumber, Tag } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { useAccess } from "./plugin-access";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";
import { listProductDirectories, updateProductDirectory } from "./product-directory-client";
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
      title: $t(key + "displayName"),
      align: "left",
      treeNode: true,
      minWidth: 220,
      slots: { default: "name" },
    },
    {field:"navigationType",title:$t(key+"navigationType"),width:140,slots:{default:"navigationType"}},
    { field: "sortOrder", title: $t(key + "sortOrder"), width: 150, slots: {default:"sortOrder"} },
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
    treeConfig: {transform:true,rowField:"directoryNo",parentField:"parentDirectoryNo",expandAll:true,showLine:true},
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
        }) => {
          const result=await listProductDirectories(page.currentPage, page.pageSize, true);
          const ids=new Set(result.items.map(row=>row.directoryNo));
          return {...result,items:[...(result.parents || []).filter(row=>!ids.has(row.directoryNo)),...result.items]};
        },
      },
    },
    toolbarConfig: { refresh: true, zoom: true },
    id: "customer-product-directories",
  },
});
const pendingSort = ref<Record<string,number|null>>({});
const sorting = ref<Record<string,boolean>>({});
async function saveSort(row: ProductDirectory) {
  const value=pendingSort.value[row.directoryNo];
  if (value === undefined || sorting.value[row.directoryNo]) return;
  if (value === null || value === row.sortOrder) {delete pendingSort.value[row.directoryNo];return;}
  sorting.value[row.directoryNo]=true;
  try {
    const result=await updateProductDirectory(row.directoryNo,{
      navigationType:row.navigationType,targetUrl:row.targetUrl,icon:row.icon,
      parentDirectoryNo:row.parentDirectoryNo || "",
      nameZhCn:row.nameZhCn,nameEnUs:row.nameEnUs,sortOrder:value,
      statusCode:row.statusCode,expectedVersion:row.version,
    });
    Object.assign(row,result.directory);
    delete pendingSort.value[row.directoryNo];
    await gridApi.query();
  } catch {
    // The request client reports the failure; retain the edited value for retry.
  } finally {sorting.value[row.directoryNo]=false;}
}
function blurSort(event: KeyboardEvent) {(event.target as HTMLInputElement | null)?.blur();}
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
const layoutRoot = useAdminGridLayout(gridApi);
</script>
<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" auto-content-height>
    <div
      data-testid="merchant-product-directories"
      class="flex h-full flex-col gap-3"
    >
      <Grid class="min-h-0 flex-1">
        <template #toolbar-tools
          ><Button v-if="canCreate" type="primary" @click="edit()">{{
            $t(key + "create")
          }}</Button></template
        >
        <template #navigationType="{row}">{{ $t(key+"types."+(row.navigationType || "product")) }}</template>
        <template #name="{ row }"><span class="directory-name" :data-testid="`directory-name-${row.directoryNo}`"><IconifyIcon :icon="row.icon || 'lucide:folder'" class="size-4 shrink-0" aria-hidden="true" :data-testid="`directory-icon-${row.directoryNo}`"/><span>{{ name(row) }}</span></span></template>
        <template #sortOrder="{row}"><InputNumber :value="pendingSort[row.directoryNo] === undefined ? row.sortOrder : pendingSort[row.directoryNo]" :min="0" :max="1000000" :precision="0" :disabled="!canUpdate || sorting[row.directoryNo]" :aria-label="`${name(row)} ${$t(key+'sortOrder')}`" :data-testid="`directory-sort-${row.directoryNo}`" class="w-full" @update:value="value=>pendingSort[row.directoryNo]=typeof value==='number'?value:null" @blur="saveSort(row)" @press-enter="blurSort"/></template>
        <template #status="{ row }"
          ><Tag :color="row.statusCode === 'enabled' ? 'green' : 'default'">{{
            $t(key + row.statusCode)
          }}</Tag></template
        >
        <template #action="{ row }"
          ><ghost-button v-if="canUpdate" :disabled="sorting[row.directoryNo]" @click="edit(row)">{{
            $t(key + "edit")
          }}</ghost-button></template
        >
      </Grid>
      <Editor @success="gridApi.query()" />
    </div>
  </Page>
</template>

<style scoped>
.directory-name {display:inline-flex;align-items:center;gap:8px;vertical-align:middle;min-width:0;}
.directory-name>span {overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
</style>
