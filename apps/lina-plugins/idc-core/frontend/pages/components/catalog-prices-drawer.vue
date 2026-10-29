<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { Alert, Button, Popconfirm, Tag, message } from 'ant-design-vue';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useAccess } from '../plugin-access';
import { createNumberedPage } from '../numbered-page';
import { minorToMajorAmount } from '../catalog-money';
import { salesDeletePrice, salesList, type SalesRow } from '../sales-management-client';
import SalesManagementDrawer from './sales-management-drawer.vue';

const props = defineProps<{inline?: boolean; target?: 'customer' | 'level'; product?: {productNo:string; name:string}}>();
const target = ref<'customer' | 'level'>(props.target || 'customer');
const product = ref<{ productNo: string; name: string }>();
const error = ref('');
const deleting = ref('');
const pagination = createNumberedPage<SalesRow>(
  ({ currentPage, pageSize, total }) => grid.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }),
  () => { editor.close(); drawer?.close(); },
);
const { hasAccessByCodes } = useAccess();
const canManage = computed(() => hasAccessByCodes(['idc:admin:sales-channel:pricing:manage']));
const canAdd = computed(() => canManage.value && (target.value === 'level' || hasAccessByCodes(['idc:admin:customer:query'])));
const tr = (key: string) => $t(`plugin.idc-core.salesManagement.${key}`);
const [EditDrawer, editor] = props.inline ? useVbenModal({ connectedComponent: SalesManagementDrawer }) : useVbenDrawer({ connectedComponent: SalesManagementDrawer });
function open(row?: SalesRow) {
  editor.setData({ section: 'prices', row, priceTarget: target.value, priceProduct: product.value });
  editor.open();
}
function columns() {
  return [
    { field: target.value === 'customer' ? 'customerName' : 'levelName', title: tr(`fields.${target.value === 'customer' ? 'customerName' : 'levelName'}`), minWidth: 180, slots: { default: 'target' } },
    { field: 'productName', title: tr('fields.productName'), minWidth: 190 },
    { field: 'currencyCode', title: tr('fields.currencyCode'), width: 90 },
    { field: 'periodUnitCode', title: tr('fields.periodUnitCode'), width: 130, slots: { default: 'period' } },
    { field: 'fixedPriceMinor', title: tr('fields.price'), width: 155, slots: { default: 'price' } },
    { field: 'status', title: tr('fields.status'), width: 90, slots: { default: 'status' } },
    { field: 'actions', title: tr('actions'), width: 140, fixed: 'right' as const, slots: { default: 'actions' } },
  ];
}
const [Grid, grid] = useVbenVxeGrid({
  gridOptions: {
    columns: [], height: '100%', minHeight: 220,
    rowConfig: { keyField: 'priceNo' },
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100] },
    toolbarConfig: { refresh: true },
    proxyConfig: { autoLoad: false, ajax: { query: async ({ page }: { page: { currentPage: number; pageSize: number } }, values: { search?: string } = {}) => pagination.query(page, async (currentPage, pageSize) => {
      const result = await salesList('prices', { page: currentPage, pageSize, search: String(values.search || ''), target: target.value, productNo: product.value?.productNo });
      return { items: result.items, total: result.total };
    }) } },
  },
});
function translate() {
  grid.setGridOptions({ columns: columns() });
  if (!props.inline) grid.setState({ formOptions: { submitOnEnter: true, schema: [{ fieldName: 'search', component: 'Input', label: tr('search'), componentProps: { allowClear: true, placeholder: tr(target.value === 'customer' ? 'customPriceSearch' : 'levelPriceSearch') } }] } });
}
watch(() => tr('search'), translate);
async function remove(row: SalesRow) {
  if (deleting.value || !canManage.value) return;
  deleting.value = row.priceNo!; error.value = '';
  try {
    await salesDeletePrice(row.priceNo!, row.version!);
    message.success(tr('deleted'));
    await grid.reload();
  } catch { error.value = tr('deleteFailed'); }
  finally { deleting.value = ''; }
}
const [Drawer, drawer] = props.inline ? [undefined,undefined] as const : useVbenDrawer({
  onOpenChange(isOpen) {
    if (!isOpen) { pagination.invalidate(); editor.close(); return; }
    const data = drawer!.getData<{ target: 'customer' | 'level'; product?: { productNo: string; name: string } }>();
    target.value = data.target; product.value = data.product; error.value = '';
    pagination.reset(); translate();
  },
  async onOpened() {
    await grid.formApi.resetForm();
    await grid.reload();
  },
});
onMounted(async () => {
  if (!props.inline) return;
  product.value = props.product; translate();
  await grid.reload();
});
</script>

<template>
  <component :is="inline ? 'div' : Drawer" :style="inline ? {width:'100%'} : undefined" :title="target === 'customer' ? tr('customPrices') : `${tr('levelPrices')} · ${product?.name || ''}`" class="w-[1100px] max-w-[95vw]" :show-confirm-button="false">
    <div class="flex min-h-0 flex-col gap-3" :style="inline ? {height:'560px'} : {height:'100%'}" data-testid="prices-workspace">
      <Alert v-if="!inline" type="info" show-icon :message="tr(target === 'customer' ? 'customPriceHelp' : 'levelPriceHelp')" />
      <Alert v-if="error" type="error" show-icon :message="error" data-testid="price-delete-error" />
      <div v-if="inline && canAdd" class="flex justify-end">
        <Button type="primary" data-testid="price-add" @click="open()">{{ tr('addAgentPrice') }}</Button>
      </div>
      <Grid class="min-h-0 flex-1">
        <template v-if="!inline" #toolbar-tools><Button v-if="canAdd" type="primary" data-testid="price-add" @click="open()">{{ tr(target === 'customer' ? 'addCustomPrice' : 'addLevelPrice') }}</Button></template>
        <template #target="{ row }"><div>{{ target === 'customer' ? row.customerName : row.levelName }}</div><div v-if="target === 'customer'" class="text-muted-foreground">#{{ row.customerCode }}</div></template>
        <template #period="{ row }">{{ row.periodCount }} {{ tr(`periods.${row.periodUnitCode}`) }} · {{ $t(`plugin.idc-core.catalog.detail.code.charge.${row.chargeTypeCode}`) }}</template>
        <template #price="{ row }">{{ row.adjustmentType === 'fixed_price' ? `${row.currencyCode} ${minorToMajorAmount(row.fixedPriceMinor, row.currencyCode)}` : `${((10000 - row.discountBasisPoints) / 100).toFixed(2)}% ${tr('ofPrice')}` }}</template>
        <template #status="{ row }"><Tag :color="row.status === 'active' ? 'success' : 'default'">{{ tr(`values.${row.status}`) }}</Tag></template>
        <template #actions="{ row }"><div v-if="canManage" class="flex gap-2"><ghost-button :data-testid="`price-edit-${row.priceNo}`" @click="open(row)">{{ tr('edit') }}</ghost-button><Popconfirm :title="tr('deleteConfirm')" @confirm="remove(row)"><ghost-button :disabled="!!deleting" :data-testid="`price-delete-${row.priceNo}`">{{ tr('delete') }}</ghost-button></Popconfirm></div></template>
      </Grid>
      <EditDrawer :modal="inline" @saved="grid.query()" />
    </div>
  </component>
</template>
