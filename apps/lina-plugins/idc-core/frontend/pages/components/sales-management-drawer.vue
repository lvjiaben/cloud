<script setup lang="ts">
import { ref } from 'vue';
import type { VbenFormProps } from '@vben/common-ui';
import { useVbenDrawer } from '@vben/common-ui';
import { Alert, message } from 'ant-design-vue';
import { useVbenForm } from '#/adapter/form';
import { $t } from '#/locales';
import { searchCustomerOptions, type CustomerSelectOption } from '../customer-selection';
import { majorToMinorAmount, minorToMajorAmount } from '../catalog-money';
import { salesList, salesPriceLevels, salesPriceProducts, salesProducts, salesSave, salesStaff, type SalesProduct, type SalesRow, type SalesSection, type SalesTerm } from '../sales-management-client';
const emit = defineEmits<{ saved: [] }>();
const tr = (key: string) => $t(`plugin.idc-core.salesManagement.${key}`);
const section = ref<SalesSection>('levels');
const original = ref<SalesRow>();
const error = ref('');
const busy = ref(false);
let opening = 0;
const searches: Record<string, number> = {};
let customers: CustomerSelectOption[] = [];
let referrers: CustomerSelectOption[] = [];
let levels: SalesRow[] = [];
let products: SalesProduct[] = [];
let staff: { userId: number; name: string; selectable: boolean }[] = [];
let currencies: string[] = [];
let terms: SalesTerm[] = [];
let target: 'customer' | 'level' = 'level';
let priceTarget: 'customer' | 'level' | undefined;
let priceProduct: { productNo: string; name: string } | undefined;
let pricingType = 'discount_basis_points';
let commissionType = 'rate';
let fingerprint = '';
let requestKey = '';
const [Form, formApi] = useVbenForm({ commonConfig: { labelWidth: 135, componentProps: { class: 'w-full' } }, schema: [], showDefaultActions: false });
function field(fieldName: string, component: string, required = true, props: Record<string, unknown> = {}) {
  return { fieldName, component, label: tr(`fields.${fieldName}`), ...(required ? { rules: 'required' } : {}), componentProps: { 'data-testid': `sales-field-${fieldName}`, ...props } };
}
function choices(values: string[]) { return values.map(value => ({ value, label: tr(`values.${value}`) })); }
function termLabel(term: SalesTerm) { return `${term.currencyCode} · ${term.periodCount} ${tr(`periods.${term.periodUnitCode}`)} · ${$t(`plugin.idc-core.catalog.detail.code.charge.${term.chargeTypeCode}`)}`; }
async function search(kind: 'customers' | 'referrers' | 'levels' | 'products' | 'staff', value: string) {
  const sequence = opening;
 const request = (searches[kind] || 0) + 1; searches[kind] = request;
 const current = () => sequence === opening && searches[kind] === request;
  try {
    if (kind === 'customers' || kind === 'referrers') { const result = await searchCustomerOptions(value); if (!current()) return; const previous = kind === 'customers' ? customers : referrers; const selected = (await formApi.getValues())[kind === 'customers' ? 'customerNo' : 'referrerCustomerNo']; if (!current()) return; const kept = previous.filter(item => item.value === selected && !result.some(next => next.value === item.value)); if (kind === 'customers') customers = [...kept, ...result]; else referrers = [...kept, ...result]; }
    if (kind === 'levels') { const result = section.value === 'prices' ? await salesPriceLevels(value) : await salesList('levels', { page: 1, pageSize: 100, search: value }); if (!current()) return; const options = result.items.filter(row => row.status === 'active'); const selected = (await formApi.getValues()).levelNo; if (!current()) return; levels = [...levels.filter(item => item.levelNo === selected && !options.some(next => next.levelNo === item.levelNo)), ...options]; }
    if (kind === 'products') { const result = section.value === 'prices' ? await salesPriceProducts(value, original.value?.productNo || priceProduct?.productNo || '') : await salesProducts(value); if (!current()) return; products = result.items; currencies = result.currencies || []; }
    if (kind === 'staff') { const result = await salesStaff(value); if (!current()) return; const selected = (await formApi.getValues()).staffUserId; if (!current()) return; staff = [...staff.filter(item => item.userId === selected && !result.items.some(next => next.userId === item.userId)), ...result.items]; }
    await refreshSchema();
  } catch { if (current()) error.value = tr('loadFailed'); }
}
function select(kind: 'customers' | 'referrers' | 'levels' | 'products' | 'staff', fieldName: string, options: { label: string; value: string | number }[], required = true, extra = {}) {
  return field(fieldName, 'Select', required, { showSearch: true, filterOption: false, allowClear: !required, options, onSearch: (value: string) => search(kind, value), ...extra });
}
function schema(): VbenFormProps['schema'] {
  const editing = !!original.value;
  const customerField = () => select('customers', 'customerNo', customers, true, { disabled: editing });
  const levelField = () => select('levels', 'levelNo', levels.map(row => ({ value: row.levelNo!, label: row.name! })), true, { disabled: editing && section.value === 'prices' });
  const status = () => field('status', 'RadioGroup', true, { optionType: 'button', buttonStyle: 'solid', options: choices(section.value === 'agents' ? ['active', 'suspended'] : ['active', 'inactive']) });
  if (section.value === 'levels') return [field('name', 'Input'), field('englishName', 'Input', false), status()];
  if (section.value === 'agents') return [customerField(), levelField(), field('expiresAt', 'DatePicker', false, { showTime: true, valueFormat: 'x', format: 'YYYY-MM-DD HH:mm', allowClear: true }), status()];
  if (section.value === 'ownership') return [customerField(), select('staff', 'staffUserId', staff.map(row => ({ value: row.userId, label: row.selectable ? row.name : `${row.name} · ${tr('staffUnavailable')}`, disabled: !row.selectable })), false), select('referrers', 'referrerCustomerNo', referrers, false)];
  if (section.value === 'prices') return [
    ...(!priceTarget ? [field('target', 'RadioGroup', true, { disabled: editing, optionType: 'button', buttonStyle: 'solid', options: choices(['level', 'customer']), onChange: (event: { target: { value: 'customer' | 'level' } }) => { target = event.target.value; void refreshSchema(); } })] : []),
    target === 'customer' ? customerField() : levelField(),
    select('products', 'productNo', products.map(row => ({ value: row.productNo, label: row.name })), true, { disabled: editing || !!priceProduct, onChange: async (value: string) => { terms = products.find(row => row.productNo === value)?.terms || []; await formApi.setFieldValue('term', undefined); await refreshSchema(); } }),
    field('term', 'Select', true, { disabled: editing, options: terms.map((term, index) => ({ value: String(index), label: termLabel(term) })) }),
    field('adjustmentType', 'RadioGroup', true, { optionType: 'button', buttonStyle: 'solid', options: choices(['discount_basis_points', 'fixed_price']), onChange: (event: { target: { value: string } }) => { pricingType = event.target.value; void refreshSchema(); } }),
    pricingType === 'fixed_price' ? field('amount', 'Input', true, { placeholder: '0.00' }) : field('percentage', 'Input', true, { suffix: '%' }), status(),
  ];
  if (section.value === 'commission-rules') return [
    field('beneficiaryType', 'Select', true, { disabled: editing, options: choices(['sales', 'affiliate']) }),
    field('operationType', 'Select', true, { disabled: editing, options: choices(['new_purchase', 'renewal']) }),
    select('products', 'productNo', products.map(row => ({ value: row.productNo, label: row.name })), false, { disabled: editing, placeholder: tr('allProducts') }),
    field('currencyCode', 'Select', true, { disabled: editing, options: currencies.map(value => ({ value, label: value })) }),
    field('calculationType', 'RadioGroup', true, { optionType: 'button', buttonStyle: 'solid', options: choices(['rate', 'fixed']), onChange: (event: { target: { value: string } }) => { commissionType = event.target.value; void refreshSchema(); } }),
    commissionType === 'fixed' ? field('amount', 'Input') : field('percentage', 'Input', true, { suffix: '%' }),
    field('maturityDays', 'InputNumber', true, { min: 0, max: 3650, precision: 0 }), status(),
  ];
  return [field('amount', 'Input'), field('paidAt', 'DatePicker', true, { showTime: true, valueFormat: 'x', format: 'YYYY-MM-DD HH:mm' }), field('evidenceReference', 'Textarea', true, { rows: 4, maxlength: 512 })];
}
async function refreshSchema() { await formApi.setState({ schema: schema() }); }
function percentage(value: unknown) { const text = String(value || '').trim(); if (!/^\d{1,3}(?:\.\d{1,2})?$/.test(text)) throw new Error('invalid'); const [integer, fraction = ''] = text.split('.'); const points = Number(integer) * 100 + Number(fraction.padEnd(2, '0')); if (points > 10000) throw new Error('invalid'); return points; }
async function submit() {
  if (busy.value || !(await formApi.validate()).valid) return;
  busy.value = true; drawer.lock(true); error.value = '';
  try {
    const values = await formApi.getValues<Record<string, any>>();
    const row = original.value;
    const body: Record<string, unknown> = { expectedVersion: row?.version || 0 };
    if (section.value === 'levels') Object.assign(body, { levelNo: row?.levelNo || '', name: values.name, englishName: values.englishName || '', status: values.status });
    if (section.value === 'agents') Object.assign(body, { customerNo: values.customerNo, levelNo: values.levelNo, status: values.status, expiresAt: values.expiresAt ? Number(values.expiresAt) : null });
    if (section.value === 'ownership') Object.assign(body, { customerNo: values.customerNo, staffUserId: values.staffUserId || 0, referrerCustomerNo: values.referrerCustomerNo || '' });
    if (section.value === 'prices') {
      const term = terms[Number(values.term)]; if (!term) throw new Error('invalid');
      Object.assign(body, { priceNo: row?.priceNo || '', customerNo: target === 'customer' ? values.customerNo : '', levelNo: target === 'level' ? values.levelNo : '', productNo: values.productNo, ...term, adjustmentType: values.adjustmentType, status: values.status });
      if (values.adjustmentType === 'fixed_price') body.fixedPriceMinor = majorToMinorAmount(values.amount, term.currencyCode);
      else body.discountBasisPoints = 10000 - percentage(values.percentage);
    }
    if (section.value === 'commission-rules') {
      Object.assign(body, { ruleNo: row?.ruleNo || '', beneficiaryType: values.beneficiaryType, operationType: values.operationType, productNo: values.productNo || '', currencyCode: values.currencyCode, calculationType: values.calculationType, maturityDays: values.maturityDays, status: values.status });
      if (values.calculationType === 'fixed') body.fixedAmountMinor = majorToMinorAmount(values.amount, values.currencyCode); else body.rateBasisPoints = percentage(values.percentage);
    }
    if (section.value === 'commission/accounts') {
      Object.assign(body, { accountNo: row!.accountNo, amountMinor: majorToMinorAmount(values.amount, row!.currencyCode!), paidAt: Number(values.paidAt), evidenceReference: values.evidenceReference });
      if (String(values.evidenceReference).trim().length < 6) throw new Error('invalid');
      const next = JSON.stringify(body); if (next !== fingerprint) { fingerprint = next; requestKey = `sales-payout-${crypto.randomUUID()}`; } body.idempotencyKey = requestKey;
    }
    await salesSave(section.value === 'commission/accounts' ? 'commission/payouts' : section.value, body, !!row && section.value !== 'commission/accounts');
    message.success(tr('saved')); emit('saved'); drawer.close();
  } catch { error.value = tr('saveFailed'); } finally { busy.value = false; drawer.lock(false); }
}
const [Drawer, drawer] = useVbenDrawer({
  onConfirm: submit,
  async onOpenChange(open) {
    if (!open) { opening++; return; }
    const sequence = ++opening;
    const data = drawer.getData<{ section: SalesSection; row?: SalesRow; priceTarget?: 'customer' | 'level'; priceProduct?: { productNo: string; name: string } }>();
    priceTarget = data.priceTarget; priceProduct = data.priceProduct;
    section.value = data.section; original.value = data.row; error.value = ''; fingerprint = ''; requestKey = '';
    customers = []; referrers = []; levels = []; products = []; staff = []; terms = []; currencies = [];
    const row = data.row;
    if (row?.customerNo) customers = [{ value: row.customerNo, label: `${row.customerName || ''} #${row.customerCode || ''}` }];
    if (row?.referrerCustomerNo) referrers = [{ value: row.referrerCustomerNo, label: `${row.referrerName || ''} #${row.referrerCode || ''}` }];
    if (row?.levelNo) levels = [{ levelNo: row.levelNo, name: row.levelName || '', status: 'active' }];
    if (row?.staffUserId) staff = [{ userId: row.staffUserId, name: row.staffName || '', selectable: true }];
    target = priceTarget || (row?.customerNo ? 'customer' : 'level'); pricingType = row?.adjustmentType || (priceTarget ? 'fixed_price' : 'discount_basis_points'); commissionType = row?.calculationType || 'rate';
    await formApi.resetForm(); await refreshSchema();
    await formApi.setValues({ ...row, productNo: row?.productNo || priceProduct?.productNo, target, status: row?.status || 'active', expiresAt: row?.expiresAt ? String(row.expiresAt) : undefined, adjustmentType: pricingType, calculationType: commissionType, beneficiaryType: row?.beneficiaryType || 'sales', operationType: row?.operationType || 'new_purchase', maturityDays: row?.maturityDays || 0,
      percentage: row?.discountBasisPoints != null ? String((10000 - row.discountBasisPoints) / 100) : row?.rateBasisPoints != null ? String(row.rateBasisPoints / 100) : '100',
      amount: row?.currencyCode ? minorToMajorAmount(row.fixedPriceMinor || row.fixedAmountMinor || row.availableMinor || '0', row.currencyCode) : '0.00', paidAt: String(Date.now()),
    }, false);
    const needed: Promise<void>[] = [];
    if (section.value === 'agents' || (section.value === 'prices' && target === 'level')) needed.push(search('levels', ''));
    if ((['agents', 'ownership'].includes(section.value) || (section.value === 'prices' && target === 'customer')) && !row?.customerNo) needed.push(search('customers', ''));
    if (section.value === 'ownership') { needed.push(search('staff', '')); needed.push(search('referrers', '')); }
    if (section.value === 'prices' || section.value === 'commission-rules') needed.push(search('products', row?.productName || ''));
    await Promise.all(needed); if (sequence !== opening) return;
    if (section.value === 'prices' && (row?.productNo || priceProduct)) {
      const productNo = row?.productNo || priceProduct!.productNo;
      terms = products.find(item => item.productNo === productNo)?.terms || [];
      if (row) {
        const frozen = { currencyCode: row.currencyCode!, chargeTypeCode: row.chargeTypeCode!, periodUnitCode: row.periodUnitCode!, periodCount: row.periodCount! };
        terms = [frozen];
        if (!products.some(item => item.productNo === productNo)) products.push({ productNo, name: row.productName || priceProduct?.name || '', englishName: '', terms });
      }
      await refreshSchema();
      if (row || terms.length === 1) await formApi.setFieldValue('term', '0');
    }
    if (!row?.currencyCode && currencies.length) await formApi.setFieldValue('currencyCode', currencies[0]);
  },
});
</script>
<template>
  <Drawer :title="section === 'commission/accounts' ? tr('recordPayout') : tr(original ? 'edit' : 'add')" class="w-[580px]">
    <div data-testid="sales-management-drawer">
    <Alert class="mb-4" type="info" show-icon :message="section === 'prices' && priceTarget ? tr(priceTarget === 'customer' ? 'customPriceHelp' : 'levelPriceHelp') : tr(`help.${section.replace('commission/', '')}`)" />
    <Form />
    <Alert v-if="error" class="mt-4" type="error" show-icon :message="error" data-testid="sales-form-error" />
    </div>
  </Drawer>
</template>
