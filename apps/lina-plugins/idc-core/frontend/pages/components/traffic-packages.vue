<script setup lang="ts">
import type { TrafficPackage } from "../traffic-package-client";
import { computed } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Tag } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { useAccess } from "../plugin-access";
import { minorToMajorAmount } from "../catalog-money";
import { listTrafficPackages } from "../traffic-package-client";
import TrafficPackageEditor from "./traffic-package-editor.vue";

const tr = (key: string) => $t(`plugin.idc-core.trafficPackages.${key}`);
const access = useAccess();
const canWrite = computed(() => access.hasAccessByCodes(["idc:admin:resource:accessory-price:write"]));
const [Editor, editorApi] = useVbenModal({ connectedComponent: TrafficPackageEditor });
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: { collapsed: false, showCollapseButton: false, schema: [
    { fieldName: "keyword", component: "Input", label: tr("name"), componentProps: { allowClear: true, maxlength: 120 } },
  ] },
  gridOptions: {
    id: "idc-traffic-packages", height: "100%", border: true, showOverflow: true,
    columns: [
      { field: "name", title: tr("name"), minWidth: 160, slots: { default: "name" } },
      { field: "trafficGb", title: tr("capacity"), minWidth: 100, align: "center", slots: { default: "capacity" } },
      { field: "amountMinor", title: tr("price"), minWidth: 155, align: "right", slots: { default: "price" } },
      { field: "stockQuantity", title: tr("totalStock"), minWidth: 110, align: "center", slots: { default: "stock" } },
      { field: "occupiedQuantity", title: tr("occupied"), minWidth: 90, align: "center" },
      { field: "remainingQuantity", title: tr("remaining"), minWidth: 90, align: "center", slots: { default: "remaining" } },
      { field: "enabled", title: tr("status"), minWidth: 100, align: "center", slots: { default: "status" } },
      { field: "actions", title: tr("actions"), width: 90, fixed: "right", align: "center", slots: { default: "actions" } },
    ],
    rowConfig: { keyField: "packageNo" },
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10,20,50,100], layouts: ["Total","Sizes","PrevJump","PrevPage","Number","NextPage","NextJump","FullJump"] },
    proxyConfig: { ajax: { query: ({ page }: { page: { currentPage: number; pageSize: number } }, values: Record<string, unknown>) => listTrafficPackages({ page: page.currentPage, pageSize: page.pageSize, keyword: String(values.keyword || "") }) } },
  },
});
function edit(item?: TrafficPackage) { editorApi.setData({ item }).open(); }
</script>

<template>
  <div class="flex h-full min-h-[480px] flex-col gap-3" data-testid="traffic-package-management">
    <Alert :message="tr('stockHelp')" :description="tr('historyHelp')" type="info" show-icon />
    <div class="min-h-0 flex-1"><Grid>
      <template #toolbar-tools><Button v-if="canWrite" type="primary" @click="edit()">{{ tr('create') }}</Button></template>
      <template #name="{ row }">{{ preferences.app.locale === 'zh-CN' ? row.nameZhCN : row.nameEnUS }}</template>
      <template #capacity="{ row }">{{ row.trafficGb }} GB</template>
      <template #price="{ row }">{{ row.currencyCode }} {{ minorToMajorAmount(String(row.amountMinor), row.currencyCode) }}</template>
      <template #stock="{ row }">{{ row.stockQuantity == null ? tr('unlimited') : row.stockQuantity }}</template>
      <template #remaining="{ row }">{{ row.remainingQuantity == null ? tr('unlimited') : row.remainingQuantity }}</template>
      <template #status="{ row }"><Tag :color="row.enabled ? 'green' : 'default'">{{ tr(row.enabled ? 'onSale' : 'offSale') }}</Tag></template>
      <template #actions="{ row }"><Button v-if="canWrite" type="link" @click="edit(row)">{{ tr('edit') }}</Button></template>
    </Grid></div>
    <Editor @success="gridApi.query()" />
  </div>
</template>
