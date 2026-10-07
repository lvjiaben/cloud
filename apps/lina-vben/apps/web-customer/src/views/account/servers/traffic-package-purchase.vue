<script setup lang="ts">
import type { TrafficPackage } from '#/api/idc/traffic-packages';
import type { ServerAccessoryQuote } from '#/api/idc/customer-servers';
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { Alert, Button, Descriptions, DescriptionsItem, message, Tag } from 'ant-design-vue';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listTrafficPackages } from '#/api/idc/traffic-packages';
import { createServerAccessoryQuote, checkoutServerAccessoryQuote } from '#/api/idc/customer-servers';
import { $t } from '#/locales';

const emit = defineEmits<{ success: [] }>();
const router = useRouter();
const serviceNo = ref('');
const busy = ref(false);
const quote = ref<ServerAccessoryQuote>();
const checkoutKey = ref('');
let requestVersion = 0;
const tr = (key: string) => $t(`trafficPackages.${key}`);
const name = (item: TrafficPackage) => preferences.app.locale === 'zh-CN' ? item.nameZhCN : item.nameEnUS;
function amount(minor: number, currency: string) {
  const formatter = new Intl.NumberFormat(preferences.app.locale, { style: 'currency', currency });
  return formatter.format(minor / 10 ** (formatter.resolvedOptions().maximumFractionDigits ?? 2));
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: { collapsed: false, showCollapseButton: false, schema: [
    { fieldName: 'keyword', component: 'Input', label: tr('name'), componentProps: { allowClear: true, maxlength: 120 } },
  ] },
  gridOptions: {
    id: 'customer-traffic-packages', height: '100%', border: true, showOverflow: true,
    columns: [
      { field: 'name', title: tr('name'), minWidth: 180, slots: { default: 'name' } },
      { field: 'trafficGb', title: tr('capacity'), minWidth: 110, align: 'center', slots: { default: 'capacity' } },
      { field: 'amountMinor', title: tr('price'), minWidth: 150, align: 'right', slots: { default: 'price' } },
      { field: 'available', title: tr('availability'), minWidth: 100, align: 'center', slots: { default: 'available' } },
      { field: 'actions', title: tr('actions'), width: 100, fixed: 'right', align: 'center', slots: { default: 'actions' } },
    ],
    rowConfig: { keyField: 'packageNo' },
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10,20,50,100], layouts: ['Total','Sizes','PrevJump','PrevPage','Number','NextPage','NextJump','FullJump'] },
    proxyConfig: { autoLoad: false, ajax: { query: ({ page }: { page: { currentPage: number; pageSize: number } }, values: Record<string, unknown>) => listTrafficPackages(serviceNo.value, { page: page.currentPage, pageSize: page.pageSize, keyword: String(values.keyword || '') }) } },
  },
});

const [Drawer, drawerApi] = useVbenDrawer({
  class: 'w-[min(94vw,1000px)]', footer: false,
  onOpenChange(open) {
    if (open) { serviceNo.value = drawerApi.getData<{ serviceNo: string }>().serviceNo; }
    else { requestVersion++; quote.value = undefined; confirmApi.close(); }
  },
  onOpened: () => gridApi.reload(),
});
const [ConfirmModal, confirmApi] = useVbenModal({ onConfirm: checkout, closeOnClickModal: false });

async function selectPackage(item: TrafficPackage) {
  if (!item.available || busy.value) return;
  busy.value = true;
  const version = ++requestVersion;
  try {
    const key = `traffic-${crypto.randomUUID()}`;
    const result = await createServerAccessoryQuote(serviceNo.value, 'traffic_add', {}, key, item.packageNo);
    if (version !== requestVersion) return;
    quote.value = result.quote;
    checkoutKey.value = `${key}-checkout`;
    confirmApi.open();
  } finally { busy.value = false; }
}

async function checkout() {
  if (!quote.value) return false;
  confirmApi.lock();
  try {
    const { checkout } = await checkoutServerAccessoryQuote(quote.value.quoteNo, checkoutKey.value);
    confirmApi.close(); drawerApi.close(); emit('success');
    if (checkout.paymentRequired && checkout.invoiceNo) {
      message.info(tr('paymentRequired'));
      await router.push(`/account/billing/invoices/${encodeURIComponent(checkout.invoiceNo)}`);
    } else { message.success(tr('created')); }
  } finally { confirmApi.unlock(); }
}
</script>

<template>
  <Drawer :title="tr('title')" content-class="h-full min-h-0">
    <div class="flex h-full min-h-0 flex-col gap-3" data-testid="traffic-package-purchase">
      <Alert :message="tr('expiryWarning')" :description="tr('executionWarning')" type="info" show-icon />
      <div class="min-h-0 flex-1"><Grid>
        <template #name="{ row }">{{ name(row) }}</template>
        <template #capacity="{ row }">{{ row.trafficGb }} GB</template>
        <template #price="{ row }">{{ amount(row.amountMinor, row.currencyCode) }}</template>
        <template #available="{ row }"><Tag :color="row.available ? 'green' : 'default'">{{ tr(row.available ? 'available' : 'soldOut') }}</Tag></template>
        <template #actions="{ row }"><Button type="link" :disabled="!row.available || busy" @click="selectPackage(row)">{{ tr('buy') }}</Button></template>
      </Grid></div>
    </div>
  </Drawer>
  <ConfirmModal :title="tr('confirm')" :confirm-text="tr('createOrder')">
    <Alert class="mb-4" :message="tr('expiryWarning')" type="warning" show-icon />
    <Descriptions v-if="quote" :column="1" bordered size="small">
      <DescriptionsItem :label="tr('name')">{{ preferences.app.locale === 'zh-CN' ? quote.packageNameZhCN : quote.packageNameEnUS }}</DescriptionsItem>
      <DescriptionsItem :label="tr('capacity')">{{ quote.trafficGb }} GB</DescriptionsItem>
      <DescriptionsItem :label="tr('price')">{{ amount(quote.totalAmountMinor, quote.currencyCode) }}</DescriptionsItem>
    </Descriptions>
    <p class="mt-3 text-sm text-muted-foreground">{{ tr('executionWarning') }}</p>
  </ConfirmModal>
</template>
