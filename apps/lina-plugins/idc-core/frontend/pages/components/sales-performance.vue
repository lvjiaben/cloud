<!-- Monthly sales performance reads payment facts even when no commission rule exists. -->
<script setup lang="ts">
import { useAdminGridLayout } from "../../../../vben-adapter/frontend/use-admin-grid-layout";
import { ref, watch } from 'vue';
import dayjs from 'dayjs';
import { Alert } from 'ant-design-vue';
import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { formatTimestamp } from '#/utils/time';
import { minorToMajorAmount } from '../catalog-money';
import { salesPerformance } from '../sales-management-client';
const tr = (key: string) => $t(`plugin.idc-core.salesManagement.performance.${key}`);
const context = ref<{ timezone: string; asOf: number }>();
function columns() {
  return [
    { field: 'beneficiaryName', title: tr('owner'), minWidth: 180 },
    { field: 'beneficiaryType', title: tr('type'), width: 125, slots: { default: 'type' } },
    { field: 'currencyCode', title: tr('currency'), width: 90 },
    { field: 'paidOrderCount', title: tr('orders'), width: 145 },
    { field: 'paidRenewalCount', title: tr('renewals'), width: 145 },
    ...['receivedMinor', 'refundedMinor', 'netReceivedMinor'].map(field => ({ field, title: tr(field), width: 175, slots: { default: 'money' } })),
  ];
}
function schema() {
  return [
    { fieldName: 'month', component: 'DatePicker', label: tr('month'), defaultValue: dayjs().format('YYYY-MM'), componentProps: { picker: 'month', valueFormat: 'YYYY-MM', allowClear: false, placeholder: tr('month') }, rules: 'required' },
    { fieldName: 'search', component: 'Input', label: tr('owner'), componentProps: { placeholder: tr('searchHint'), allowClear: true } },
    { fieldName: 'beneficiaryType', component: 'Select', label: tr('type'), componentProps: { allowClear: true, placeholder: tr('allTypes'), options: [{ value: 'sales', label: tr('sales') }, { value: 'affiliate', label: tr('affiliate') }] } },
    { fieldName: 'currencyCode', component: 'Input', label: tr('currency'), componentProps: { placeholder: tr('currencyHint'), allowClear: true, maxlength: 3 } },
  ];
}
const [Grid, grid] = useVbenVxeGrid({
  formOptions: { schema: [], submitOnChange: false, submitOnEnter: true },
  gridOptions: {
    columns: [], height: '100%', minHeight: 180,
    pagerConfig: { pageSize: 20, pageSizes: [10, 20, 50, 100] },
    toolbarConfig: { refresh: true, zoom: true },
    proxyConfig: { ajax: { query: async ({ page }, values) => {
      const result = await salesPerformance({ ...values, month: values.month || dayjs().format('YYYY-MM'), currencyCode: values.currencyCode?.trim().toUpperCase() || '', page: page.currentPage, pageSize: page.pageSize });
      context.value = { timezone: result.timezone, asOf: result.asOf };
      return { items: result.items, total: result.total };
    } } },
  },
});
watch(() => tr('month'), () => { grid.setGridOptions({ columns: columns() }); grid.setState({ formOptions: { schema: schema() } }); }, { immediate: true });
const layoutRoot = useAdminGridLayout(grid);
</script>
<template>
  <div :ref="layoutRoot" class="flex h-full min-h-0 flex-col gap-3" data-testid="sales-performance">
    <Alert type="info" show-icon :message="tr('help')" />
    <p v-if="context" class="px-1 text-xs text-muted-foreground" data-testid="sales-performance-cutoff">{{ tr('timezone') }}: {{ context.timezone }} · {{ tr('asOf') }}: {{ formatTimestamp(context.asOf) }}</p>
    <Grid class="min-h-0 flex-1">
      <template #type="{ row }">{{ tr(row.beneficiaryType) }}</template>
      <template #money="{ row, column }">{{ row.currencyCode }} {{ minorToMajorAmount(row[column.field], row.currencyCode) }}</template>
    </Grid>
  </div>
</template>
