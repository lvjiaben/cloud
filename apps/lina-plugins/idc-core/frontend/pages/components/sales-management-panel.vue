<script setup lang="ts">
import { useAdminGridLayout } from "../../../../vben-adapter/frontend/use-admin-grid-layout";
import { computed, watch } from 'vue';
import { useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { Alert, Button, Tag } from 'ant-design-vue';
import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { formatTimestamp } from '#/utils/time';
import { useAccess } from '../plugin-access';
import { minorToMajorAmount } from '../catalog-money';
import { salesList, type SalesRow, type SalesSection } from '../sales-management-client';
import SalesManagementDrawer from './sales-management-drawer.vue';
import SalesLevelPricingModal from './sales-level-pricing-modal.vue';
const props = defineProps<{ section: SalesSection }>();
const { hasAccessByCodes } = useAccess();
const tr = (key: string) => $t(`plugin.idc-core.salesManagement.${key}`);
const permissions: Partial<Record<SalesSection, string>> = { levels: 'qualification:manage', agents: 'qualification:manage', prices: 'pricing:manage', ownership: 'assignment:manage', 'commission-rules': 'commission:manage', 'commission/accounts': 'commission:payout' };
const permission = computed(() => permissions[props.section] || '');
const canEdit = computed(() => !!permission.value && hasAccessByCodes([`idc:admin:sales-channel:${permission.value}`]));
const canPriceLevel = computed(() => props.section === 'levels' && hasAccessByCodes(['idc:admin:sales-channel:pricing:manage']));
const [Drawer, drawer] = useVbenDrawer({ connectedComponent: SalesManagementDrawer });
const [PricingModal, pricingModal] = useVbenModal({ connectedComponent: SalesLevelPricingModal });
function openPricing(row: SalesRow) { pricingModal.setData({ levelNo: row.levelNo, name: row.name }); pricingModal.open(); }
function open(row?: SalesRow) { drawer.setData({ section: props.section, row }); drawer.open(); }
function money(row: SalesRow, field: keyof SalesRow) { const raw = row[field]; return typeof raw === 'string' && raw !== '' ? `${row.currencyCode} ${minorToMajorAmount(raw, row.currencyCode || 'CNY')}` : '—'; }
function columns() {
  const base = (field: keyof SalesRow, title: string = field, width?: number) => ({ field, title: tr(`fields.${title}`), ...(width ? { width } : { minWidth: 170 }) });
  let fields: ReturnType<typeof base>[] = [];
  if (props.section === 'levels') fields = [base('name'), base('englishName'), ...(canPriceLevel.value ? [base('percentageBasisPoints', 'percentageBasisPoints', 160)] : []), base('status', 'status', 110)];
  if (props.section === 'agents') fields = [base('customerName'), base('customerCode', 'customerCode', 130), base('levelName'), base('status', 'status', 110), base('expiresAt', 'expiresAt', 180)];
  if (props.section === 'prices') fields = [base('productName'), base('customerName'), base('levelName'), base('currencyCode', 'currencyCode', 100), base('periodUnitCode', 'periodUnitCode', 140), base('fixedPriceMinor', 'price', 160), base('status', 'status', 100)];
  if (props.section === 'ownership') fields = [base('customerName'), base('customerCode', 'customerCode', 130), base('staffName'), base('referrerName')];
  if (props.section === 'commission-rules') fields = [base('beneficiaryType'), base('operationType'), base('productName'), base('currencyCode', 'currencyCode', 100), base('fixedAmountMinor', 'commission', 160), base('maturityDays', 'maturityDays', 130), base('status', 'status', 100)];
  if (props.section === 'commission/accounts') fields = [base('beneficiaryName'), base('currencyCode', 'currencyCode', 100), ...(['creditedMinor','pendingMinor','availableMinor','paidMinor','reversedMinor','recoveryDueMinor'] as const).map(f => base(f, f, 150))];
  if (props.section === 'commission/records') fields = [base('orderNo'), base('customerName'), base('beneficiaryName'), base('currencyCode', 'currencyCode', 100), ...(['basisMinor','originalMinor','pendingMinor','maturedMinor','reversedMinor'] as const).map(f => base(f, f, 145)), base('maturesAt', 'maturesAt', 180), base('status', 'status', 120)];
  if (props.section === 'commission/payouts') fields = [base('beneficiaryName'), base('amountMinor', 'amountMinor', 150), base('evidenceReference'), base('paidAt', 'paidAt', 180), base('createdAt', 'createdAt', 180)];
  return [...fields.map(field => ({ ...field, slots: { default: 'value' } })), ...(canEdit.value || canPriceLevel.value ? [{ title: tr('actions'), field: 'actions', width: props.section === 'commission/accounts' ? 175 : props.section === 'levels' ? 185 : 140, showOverflow: false, fixed: 'right' as const, slots: { default: 'actions' } }] : [])];
}
function display(row: SalesRow, field: keyof SalesRow) {
  if (field === 'percentageBasisPoints') return row.percentageBasisPoints == null ? tr('globalPercentageUnset') : `${row.percentageBasisPoints / 100}%`;
  if (field === 'orderNo' && !row.orderNo) return `${tr('renewalInvoice')} ${row.invoiceNo || ''}`;
  if (field === 'fixedPriceMinor') return row.adjustmentType === 'fixed_price' ? money(row, field) : `${((10000 - (row.discountBasisPoints || 0)) / 100).toFixed(2)}% ${tr('ofPrice')}`;
  if (field === 'fixedAmountMinor') return row.calculationType === 'fixed' ? money(row, field) : `${((row.rateBasisPoints || 0) / 100).toFixed(2)}%`;
  if (field.endsWith('Minor')) return money(row, field);
  if (field.endsWith('At')) return row[field] ? formatTimestamp(Number(row[field])) : tr('noExpiry');
  if (field === 'periodUnitCode') return `${row.periodCount} ${tr(`periods.${row.periodUnitCode}`)}`;
  if (['status', 'beneficiaryType', 'operationType'].includes(field)) return tr(`values.${row[field]}`);
  return row[field] || (field === 'productName' ? tr('allProducts') : '—');
}
const [Grid, grid] = useVbenVxeGrid({
  formOptions: { schema: [], submitOnChange: false, submitOnEnter: true },
  gridOptions: {
    columns: [], height: '100%', minHeight: 180, keepSource: true,
    pagerConfig: { pageSize: 20, pageSizes: [10, 20, 50, 100] },
    toolbarConfig: { refresh: true, zoom: true },
    proxyConfig: { ajax: { query: async ({ page }: { page: { currentPage: number; pageSize: number } }, formValues: { search?: string }) => {
      const result = await salesList(props.section, { page: page.currentPage, pageSize: page.pageSize, search: formValues.search || '' });
      return { items: result.items, total: result.total };
    } } },
  },
});
watch([() => tr('search'), () => props.section, canEdit, canPriceLevel], () => {
  grid.setGridOptions({ columns: columns() });
  grid.setState({ formOptions: { schema: [{ fieldName: 'search', component: 'Input', label: tr('search'), componentProps: { placeholder: tr(`searchHints.${props.section.replace('commission/', '')}`) } }] } });
}, { immediate: true });
function payoutDisabled(row: SalesRow) { return !row.availableMinor || BigInt(row.availableMinor) <= 0n || BigInt(row.recoveryDueMinor || '0') > 0n; }
const layoutRoot = useAdminGridLayout(grid);
</script>
<template>
  <div :ref="layoutRoot" class="flex h-full min-h-0 flex-col gap-3" :data-testid="`sales-panel-${section}`">
    <Alert type="info" show-icon :message="tr(`help.${section.replace('commission/', '')}`)" />
    <Grid class="min-h-0 flex-1">
      <template #toolbar-tools>
        <Button v-if="canEdit && !section.startsWith('commission/')" type="primary" data-testid="sales-add" @click="open()">{{ tr(`create.${section}`) }}</Button>
      </template>
      <template #value="{ row, column }"><Tag v-if="column.field === 'status'" :color="row.status === 'active' ? 'success' : 'default'">{{ display(row, column.field) }}</Tag><span v-else>{{ display(row, column.field) }}</span></template>
      <template #actions="{ row }"><div class="flex items-center gap-2"><span v-if="section === 'commission/accounts' && BigInt(row.recoveryDueMinor || '0') > 0n" class="whitespace-normal text-xs text-orange-600">{{ tr('recoveryBlocksPayout') }}</span><ghost-button v-if="canEdit" :disabled="section === 'commission/accounts' && payoutDisabled(row)" @click="open(row)">{{ section === 'commission/accounts' ? tr('recordPayout') : tr('edit') }}</ghost-button><ghost-button v-if="canPriceLevel" :data-testid="`sales-level-pricing-${row.levelNo}`" @click="openPricing(row)">{{ tr('globalPercentage') }}</ghost-button></div></template>
    </Grid>
    <Drawer @saved="grid.query()" />
    <PricingModal @saved="grid.query()" />
  </div>
</template>
