<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/currencies', title: 'Merchant Currencies' };
</script>
<script setup lang="ts">
import '../../../vben-adapter/frontend/admin.css';
import { useAdminGridLayout } from '../../../vben-adapter/frontend/use-admin-grid-layout';
import { computed, ref, watch } from 'vue';
import { Page, useVbenModal } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { Alert, Button, Popconfirm, Tag, message } from 'ant-design-vue';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useAccess } from './plugin-access';
import { currencyList, currencyUpdate, currencyDelete, currencyLabel, type MerchantCurrency } from './currency-client';
import CurrencyCreateModal from './components/currency-create-modal.vue';
const tr = (key: string) => $t(`plugin.idc-core.currency.${key}`);
const { hasAccessByCodes } = useAccess();
const canWrite = computed(() => hasAccessByCodes(['idc:admin:currency:write']));
const busy = ref(''); const error = ref('');
const [CreateModal, createModal] = useVbenModal({ connectedComponent: CurrencyCreateModal });
function columns() { return [
  { field: 'currencyCode', title: tr('name'), minWidth: 200, slots: { default: 'name' } },
  { field: 'enabled', title: tr('status'), width: 100, slots: { default: 'status' } },
  { field: 'isDefault', title: tr('default'), width: 100, slots: { default: 'default' } },
  { field: 'actions', title: tr('actions'), width: 260, fixed: 'right' as const, slots: { default: 'actions' } },
]; }
const [Grid, grid] = useVbenVxeGrid({ gridOptions: {
  columns: [], height: '100%', minHeight: 300, rowConfig: { keyField: 'currencyCode' },
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100] }, toolbarConfig: { refresh: true },
  proxyConfig: { ajax: { query: async ({ page }: { page: { currentPage: number; pageSize: number } }) => {
    error.value = ''; try { return await currencyList(page.currentPage, page.pageSize); }
    catch (err) { error.value = tr('loadFailed'); throw err; }
  } } },
} });
const setLayoutRoot = useAdminGridLayout(grid);
watch(() => tr('name'), () => grid.setGridOptions({ columns: columns() }), { immediate: true });
async function update(row: MerchantCurrency, makeDefault = false) {
  if (busy.value || !canWrite.value) return;
  busy.value = row.currencyCode; error.value = '';
  try { await currencyUpdate(row, makeDefault || !row.enabled, makeDefault); message.success(tr('saved')); await grid.reload(); }
  catch { await grid.reload(); error.value = tr('updateFailed'); }
  finally { busy.value = ''; }
}
async function remove(row: MerchantCurrency) {
  if (busy.value || !canWrite.value) return;
  busy.value = row.currencyCode; error.value = '';
  try { await currencyDelete(row); message.success(tr('deleted')); await grid.reload(); }
  catch { await grid.reload(); error.value = tr('deleteFailed'); }
  finally { busy.value = ''; }
}
</script>
<template>
  <Page auto-content-height class="plugin-admin-page">
    <div :ref="setLayoutRoot" class="flex h-full min-h-0 flex-col gap-3" data-testid="currency-management">
      <Alert :message="tr('help')" type="info" show-icon />
      <Alert v-if="error" :message="error" type="error" show-icon />
      <Grid class="min-h-0 flex-1">
        <template #toolbar-tools><div class="flex gap-2">
          <Button v-if="canWrite" type="primary" data-testid="currency-add" @click="createModal.open()">{{ tr('add') }}</Button>
        </div></template>
        <template #name="{ row }">{{ currencyLabel(row.currencyCode, preferences.app.locale) }}</template>
        <template #status="{ row }"><Tag :color="row.enabled ? 'success' : 'default'">{{ tr(row.enabled ? 'enabled' : 'disabled') }}</Tag></template>
        <template #default="{ row }"><Tag v-if="row.isDefault" color="blue">{{ tr('default') }}</Tag><span v-else>—</span></template>
        <template #actions="{ row }"><div v-if="canWrite" class="flex gap-2">
          <Popconfirm v-if="!row.isDefault" :title="tr(row.enabled ? 'disableConfirm' : 'enableConfirm')" @confirm="update(row)"><ghost-button :disabled="!!busy" :data-testid="`currency-toggle-${row.currencyCode}`">{{ tr(row.enabled ? 'disable' : 'enable') }}</ghost-button></Popconfirm>
          <Popconfirm v-if="row.enabled && !row.isDefault" :title="tr('defaultConfirm')" @confirm="update(row, true)"><ghost-button :disabled="!!busy" :data-testid="`currency-default-${row.currencyCode}`">{{ tr('setDefault') }}</ghost-button></Popconfirm>
          <Popconfirm v-if="!row.isDefault" :title="tr('deleteConfirm')" @confirm="remove(row)"><ghost-button danger :disabled="!!busy" :data-testid="`currency-delete-${row.currencyCode}`">{{ tr('delete') }}</ghost-button></Popconfirm>
        </div></template>
      </Grid>
      <CreateModal @saved="grid.reload()" />
    </div>
  </Page>
</template>
