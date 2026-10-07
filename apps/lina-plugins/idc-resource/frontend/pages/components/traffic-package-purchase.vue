<script setup lang="ts">
import type { StaffTrafficPackage } from "../server-accessory-client";
import { ref } from "vue";
import { useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Descriptions, DescriptionsItem, message, Tag } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { createStaffServerAccessoryOperation, listStaffTrafficPackages } from "../server-accessory-client";

const emit = defineEmits<{ success: [] }>();
const serviceNo = ref("");
const selected = ref<StaffTrafficPackage>();
const requestNo = ref("");
const tr = (key: string) => $t(`plugin.idc-resource.trafficPackages.${key}`);
const packageName = (item: StaffTrafficPackage) => preferences.app.locale === "zh-CN" ? item.nameZhCN : item.nameEnUS;
function amount(item: StaffTrafficPackage) {
  const formatter = new Intl.NumberFormat(preferences.app.locale, { style: "currency", currency: item.currencyCode });
  return formatter.format(item.amountMinor / 10 ** (formatter.resolvedOptions().maximumFractionDigits ?? 2));
}
function columns() { return [
  { field: "name", title: tr("name"), minWidth: 180, slots: { default: "name" } },
  { field: "trafficGb", title: tr("capacity"), minWidth: 100, align: "center" as const, slots: { default: "capacity" } },
  { field: "amountMinor", title: tr("price"), minWidth: 150, align: "right" as const, slots: { default: "price" } },
  { field: "available", title: tr("availability"), minWidth: 100, align: "center" as const, slots: { default: "available" } },
  { field: "actions", title: tr("actions"), width: 120, fixed: "right" as const, align: "center" as const, slots: { default: "actions" } },
]; }
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: { collapsed: false, showCollapseButton: false, schema: [{ fieldName: "keyword", component: "Input", label: tr("name"), componentProps: { allowClear: true, maxlength: 120 } }] },
  gridOptions: {
    id: "staff-traffic-package-purchase", columns: columns(), height: "100%", border: true, showOverflow: true,
    rowConfig: { keyField: "packageNo" },
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100], layouts: ["Total", "Sizes", "PrevJump", "PrevPage", "Number", "NextPage", "NextJump", "FullJump"] },
    proxyConfig: { autoLoad: false, ajax: { query: ({ page }: { page: { currentPage: number; pageSize: number } }, values: Record<string, unknown>) => listStaffTrafficPackages(serviceNo.value, { page: page.currentPage, pageSize: page.pageSize, keyword: String(values.keyword || "") }) } },
  },
});
const [Drawer, drawerApi] = useVbenDrawer({
  class: "w-[min(94vw,1000px)]", footer: false,
  onOpenChange(open) {
    if (open) serviceNo.value = drawerApi.getData<{ serviceNo: string }>().serviceNo;
    else { selected.value = undefined; confirmApi.close(); }
  },
  onOpened: () => gridApi.reload(),
});
const [Confirm, confirmApi] = useVbenModal({ onConfirm: submit, closeOnClickModal: false });
function selectPackage(item: StaffTrafficPackage) {
  if (!item.available) return;
  selected.value = { ...item };
  requestNo.value = `staff-traffic-${crypto.randomUUID()}`;
  confirmApi.open();
}
async function submit() {
  if (!selected.value) return false;
  confirmApi.lock();
  try {
    const result = await createStaffServerAccessoryOperation(serviceNo.value, {
      action: "traffic_add", packageNo: selected.value.packageNo, packageVersion: selected.value.version,
      parameters: {}, confirmed: true, requestNo: requestNo.value,
    });
    message.success(tr(result.paymentRequired ? "paymentRequired" : "created"));
    confirmApi.close(); drawerApi.close(); emit("success");
  } finally { confirmApi.unlock(); }
}
</script>

<template>
  <Drawer :title="tr('title')" content-class="h-full min-h-0">
    <div class="flex h-full min-h-0 flex-col gap-3" data-testid="staff-traffic-package-purchase">
      <Alert :message="tr('expiryWarning')" :description="tr('executionWarning')" type="info" show-icon />
      <div class="min-h-0 flex-1"><Grid>
        <template #name="{ row }">{{ packageName(row) }}</template>
        <template #capacity="{ row }">{{ row.trafficGb }} GB</template>
        <template #price="{ row }">{{ amount(row) }}</template>
        <template #available="{ row }"><Tag :color="row.available ? 'green' : 'default'">{{ tr(row.available ? 'available' : 'soldOut') }}</Tag></template>
        <template #actions="{ row }"><Button type="link" :disabled="!row.available" @click="selectPackage(row)">{{ tr('buy') }}</Button></template>
      </Grid></div>
    </div>
  </Drawer>
  <Confirm :title="tr('confirm')" :confirm-text="tr('createOrder')">
    <Alert class="mb-4" :message="tr('expiryWarning')" type="warning" show-icon />
    <Descriptions v-if="selected" :column="1" bordered size="small">
      <DescriptionsItem :label="tr('name')">{{ packageName(selected) }}</DescriptionsItem>
      <DescriptionsItem :label="tr('capacity')">{{ selected.trafficGb }} GB</DescriptionsItem>
      <DescriptionsItem :label="tr('price')">{{ amount(selected) }}</DescriptionsItem>
    </Descriptions>
    <p class="mt-3 text-sm text-muted-foreground">{{ tr('executionWarning') }}</p>
  </Confirm>
</template>
