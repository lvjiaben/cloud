<script setup lang="ts">
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { OperationsRecord } from '../operations-details-client';

import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { preferences } from '@vben/preferences';
import { Alert, Button, Card, Space } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { formatTimestamp } from '#/utils/time';

import { minorToMajorAmount } from '../catalog-money';
import { operationsRecords } from '../operations-details-client';

defineProps<{ canOpen?: boolean }>();
const emit = defineEmits<{ open: [record: OperationsRecord] }>();
const route = useRoute();
const router = useRouter();
const error = ref(false);
const metric = computed(() => String(route.query.operationsMetric || ''));
const monetary = computed(() => ['income', 'orders', 'products'].includes(metric.value));
const title = computed(() => tr(`title.${metric.value}`));
const period = computed(() => {
  if (metric.value === 'servers') return $t('plugin.idc-core.operationsData.nextSevenDays');
  const start = Number(route.query.startAt);
  const end = Number(route.query.endAt);
  return start > 0 && end > start
    ? `${formatTimestamp(start)} — ${formatTimestamp(end - 1)}`
    : tr('allTime');
});
const filterDescription = computed(() => {
  const status = route.query.status;
  return typeof status === 'string' && ['online', 'pending', 'processed'].includes(status)
    ? tr(`filter.${status}`)
    : '';
});

function tr(key: string) { return $t(`plugin.idc-core.operationsDetails.${key}`); }

function columns(): VxeGridProps<OperationsRecord>['columns'] {
  const result: NonNullable<VxeGridProps<OperationsRecord>['columns']> = [
    { field: 'id', title: tr('reference'), minWidth: 210 },
    { field: 'label', title: tr('description'), minWidth: 220, slots: { default: 'description' } },
    { field: 'customerName', title: tr('customer'), minWidth: 180, slots: { default: 'customer' } },
  ];
  if (monetary.value) result.push({ field: 'amountMinor', title: tr('amount'), width: 230, slots: { default: 'amount' } });
  if (metric.value === 'products') result.push({ field: 'quantity', title: tr('quantity'), width: 100 });
  result.push({ field: 'occurredAt', title: tr(`time.${metric.value}`), width: 185, slots: { default: 'occurredAt' } });
  result.push({ field: 'actions', title: tr('actions'), width: 100, fixed: 'right', slots: { default: 'actions' } });
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    height: 420,
    pagerConfig: {
      enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100],
      layouts: ['Total', 'Sizes', 'PrevJump', 'PrevPage', 'Number', 'NextPage', 'NextJump', 'FullJump'],
    },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: { page: { currentPage: number; pageSize: number } }) => {
          error.value = false;
          try {
            return await operationsRecords(route.query, page.currentPage, page.pageSize);
          } catch (failure) {
            error.value = true;
            gridApi.setGridOptions({ data: [], pagerConfig: { total: 0 } });
            throw failure;
          }
        },
      },
    },
    toolbarConfig: { refresh: false },
  },
});

function amount(record: OperationsRecord) {
  if (!record.currencyCode) return '—';
  return `${record.currencyCode} ${minorToMajorAmount(record.amountMinor || '0', record.currencyCode)}`;
}

async function clearFilter() {
  const query = { ...route.query };
  for (const key of ['operationsMetric', 'startAt', 'endAt', 'asOf', 'currencyCode', 'status', 'productNo']) delete query[key];
  await router.replace({ path: route.path, query });
}

onMounted(() => {
  gridApi.setGridOptions({ columns: columns() });
  void gridApi.query();
});
watch(() => route.fullPath, () => {
  if (!route.query.operationsMetric) return;
  gridApi.setGridOptions({ columns: columns() });
  void gridApi.reload();
});
watch(() => preferences.app.locale, () => gridApi.setGridOptions({ columns: columns() }));
</script>

<template>
  <Card :bordered="false" class="operations-detail-panel" data-testid="operations-detail-panel" :title="title">
    <template #extra>
      <Space wrap>
        <Button @click="router.push('/idc/operations/workbench')">{{ tr('back') }}</Button>
        <Button @click="clearFilter">{{ tr('allRecords') }}</Button>
        <Button @click="gridApi.query()">{{ tr('refresh') }}</Button>
      </Space>
    </template>
    <p class="operations-detail-filter">{{ period }}<span v-if="monetary && route.query.currencyCode"> · {{ route.query.currencyCode }}</span><span v-if="filterDescription"> · {{ filterDescription }}</span></p>
    <Alert v-if="error" class="mb-4" type="error" show-icon :message="tr('loadFailed')" />
    <Grid>
      <template #description="{ row }">{{ row.label || title }}</template>
      <template #customer="{ row }">{{ row.customerName || row.customerNo || '—' }}</template>
      <template #amount="{ row }">{{ amount(row) }}</template>
      <template #occurredAt="{ row }">{{ row.occurredAt ? formatTimestamp(row.occurredAt) : '—' }}</template>
      <template #actions="{ row }"><ghost-button v-if="canOpen" @click="emit('open', row)">{{ tr('view') }}</ghost-button><span v-else>—</span></template>
    </Grid>
  </Card>
</template>

<style scoped>
.operations-detail-filter { margin: 0 0 16px; color: hsl(var(--muted-foreground)); }
.operations-detail-panel { min-width: 0; }
</style>
