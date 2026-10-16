<script setup lang="ts">
import type { FinancialInvoiceDetail } from '#/api/idc/customer-finance';
import type { OrderDetail } from '#/api/idc/customer-commerce';
import type {
  CustomerRefund,
  CustomerRefundPreview,
  CustomerRefundSelection,
} from '#/api/idc/customer-refunds';

import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Empty,
  InputNumber,
  message,
  Modal as AntModal,
  Select,
  Skeleton,
  Space,
  Tag,
  Textarea,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import { getOrder } from '#/api/idc/customer-commerce';
import { getFinancialInvoice } from '#/api/idc/customer-finance';
import {
  clearCustomerRefundActionKey,
  customerRefundActionKey,
  listCustomerRefunds,
  previewCustomerRefund,
  respondCustomerRefundBalance,
  submitCustomerRefund,
} from '#/api/idc/customer-refunds';
import { $t } from '#/locales';
import VbenDrawerCompat from '#/components/vben-drawer-compat.vue';

type SelectionState = {
  amountMinor: string;
  entitlementNos: string[];
  quantity: number;
  selected: boolean;
};

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const contextLoading = ref(false);
const previewLoading = ref(false);
const submitLoading = ref(false);
const historyFailed = ref(false);
const orderFailed = ref(false);
const refunds = ref<CustomerRefund[]>([]);
const nextCursor = ref('');
const cursor = ref('');
const cursorHistory = ref<string[]>([]);
const order = ref<OrderDetail>();
const invoice = ref<FinancialInvoiceDetail>();
const preview = ref<CustomerRefundPreview>();
const previewOpen = ref(false);
const impactConfirmed = ref(false);
const reasonCode = ref('');
const reasonDetail = ref('');
const selections = reactive<Record<string, SelectionState>>({});

const orderNo = computed(() =>
  typeof route.query.orderNo === 'string' ? route.query.orderNo.trim() : '',
);
const isEnglish = computed(
  () => document.documentElement.lang.toLowerCase() === 'en-us',
);
const reasonOptions = computed(() =>
  [
    'duplicate_purchase',
    'wrong_configuration',
    'service_issue',
    'no_longer_needed',
    'other',
  ].map((value) => ({ label: $t(`refunds.reason.${value}`), value })),
);

function selectionKey(invoiceItemNo: string) {
  return invoiceItemNo;
}

function selectionState(key: string) {
  selections[key] ??= {
    amountMinor: '',
    entitlementNos: [],
    quantity: 1,
    selected: false,
  };
  return selections[key];
}

function entitlementsFor(orderItemNo: string) {
  return (order.value?.serviceEntitlements || []).filter(
    (item) => item.orderItemNo === orderItemNo && item.entitlementNo !== '',
  );
}

function orderItemQuantity(orderItemNo: string) {
  const item = (order.value?.items || []).find(
    (candidate) => String(candidate.orderItemNo || '') === orderItemNo,
  );
  return Math.max(1, Number(item?.quantity || 1));
}

function chooseWhole(invoiceItemNo: string, checked: boolean) {
	const state = selectionState(selectionKey(invoiceItemNo));
	state.selected = checked;
	if (!checked) state.entitlementNos = [];
}

function chooseEntitlement(
  invoiceItemNo: string,
  entitlementNo: string,
  checked: boolean,
) {
	const state = selectionState(selectionKey(invoiceItemNo));
	const values = new Set(state.entitlementNos);
	if (checked) values.add(entitlementNo);
	else values.delete(entitlementNo);
	state.entitlementNos = [...values].sort();
	state.selected = true;
}

function selectedInputs(): CustomerRefundSelection[] {
  const result: CustomerRefundSelection[] = [];
  for (const item of invoice.value?.items || []) {
		const whole = selectionState(selectionKey(item.invoiceItemNo));
		if (whole.selected) {
			result.push({
				amountMinor: whole.amountMinor.trim(),
				entitlementNos: [...whole.entitlementNos],
				invoiceItemNo: item.invoiceItemNo,
				quantity: whole.quantity,
			});
		}
  }
  return result;
}

function validSelections(values: CustomerRefundSelection[]) {
  return (
    values.length > 0 &&
    values.every(
      (item) =>
        /^[1-9][0-9]{0,18}$/u.test(item.amountMinor) && item.quantity > 0,
    )
  );
}

function localized(item: {
  descriptionEnUS?: string;
  descriptionZhCN?: string;
  productNameEnUS?: string;
  productNameZhCN?: string;
}) {
  return isEnglish.value
    ? item.descriptionEnUS ||
        item.productNameEnUS ||
        item.descriptionZhCN ||
        item.productNameZhCN ||
        '—'
    : item.descriptionZhCN ||
        item.productNameZhCN ||
        item.descriptionEnUS ||
        item.productNameEnUS ||
        '—';
}

function formatMinor(value: string, currencyCode: string) {
  if (!/^\d+$/u.test(value)) return '—';
  let digits = 2;
  try {
    digits =
      new Intl.NumberFormat('en', {
        currency: currencyCode,
        style: 'currency',
      }).resolvedOptions().maximumFractionDigits ?? 2;
  } catch {
    digits = 2;
  }
  const padded = value.padStart(digits + 1, '0');
  return digits
    ? `${currencyCode} ${padded.slice(0, -digits)}.${padded.slice(-digits)}`
    : `${currencyCode} ${padded}`;
}

function statusColor(status: string) {
  if (['approved', 'succeeded'].includes(status)) return 'success';
  if (['failed', 'rejected'].includes(status)) return 'error';
  if (['result_unknown', 'awaiting_balance_consent'].includes(status))
    return 'warning';
  return 'processing';
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

async function loadHistory() {
  loading.value = true;
  historyFailed.value = false;
  try {
    const result = await listCustomerRefunds({
      cursor: cursor.value || undefined,
    });
    refunds.value = result.refunds || [];
    nextCursor.value = result.nextCursor || '';
  } catch {
    historyFailed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadOrderContext() {
  order.value = undefined;
  invoice.value = undefined;
  preview.value = undefined;
  orderFailed.value = false;
  for (const key of Object.keys(selections)) delete selections[key];
  if (!orderNo.value) return;
  contextLoading.value = true;
  try {
    const orderResult = await getOrder(orderNo.value);
    const invoiceNo = String(orderResult.order.invoice?.invoiceNo || '');
    if (!invoiceNo) throw new Error('invoice-missing');
    const invoiceResult = await getFinancialInvoice(invoiceNo);
    order.value = orderResult.order;
    invoice.value = invoiceResult.detail;
  } catch {
    orderFailed.value = true;
  } finally {
    contextLoading.value = false;
  }
}

async function createPreview() {
  const values = selectedInputs();
  if (!validSelections(values)) {
    message.warning($t('refunds.messages.selectionRequired'));
    return;
  }
  previewLoading.value = true;
  try {
    preview.value = (
      await previewCustomerRefund(orderNo.value, values)
    ).preview;
    reasonCode.value = '';
    reasonDetail.value = '';
    impactConfirmed.value = false;
    previewOpen.value = true;
  } catch {
    message.error($t('refunds.messages.previewFailed'));
  } finally {
    previewLoading.value = false;
  }
}

async function submitRefund() {
  if (!preview.value || !reasonCode.value || !impactConfirmed.value) {
    message.warning($t('refunds.messages.confirmRequired'));
    return;
  }
  if (reasonCode.value === 'other' && !reasonDetail.value.trim()) {
    message.warning($t('refunds.messages.otherDetailRequired'));
    return;
  }
  const scope = `${orderNo.value}:${preview.value.impactDigest}`;
  submitLoading.value = true;
  try {
    await submitCustomerRefund({
      idempotencyKey: customerRefundActionKey(scope),
      orderNo: orderNo.value,
      previewDigest: preview.value.impactDigest,
      reasonCode: reasonCode.value,
      reasonDetail: reasonDetail.value.trim(),
      selections: selectedInputs(),
    });
    clearCustomerRefundActionKey(scope);
    previewOpen.value = false;
    message.success($t('refunds.messages.submitted'));
    await Promise.all([loadHistory(), loadOrderContext()]);
    await router.replace({ path: '/account/refunds' });
  } catch {
    message.error($t('refunds.messages.submitFailed'));
  } finally {
    submitLoading.value = false;
  }
}

function respondBalance(refund: CustomerRefund, consent: boolean) {
  AntModal.confirm({
    cancelText: $t('refunds.actions.cancel'),
    content: consent
      ? $t('refunds.balance.acceptDetail', {
          amount: formatMinor(refund.amountMinor, refund.currencyCode),
        })
      : $t('refunds.balance.declineDetail'),
    okText: consent
      ? $t('refunds.balance.accept')
      : $t('refunds.balance.keepPending'),
    title: consent
      ? $t('refunds.balance.acceptTitle')
      : $t('refunds.balance.declineTitle'),
    async onOk() {
      const scope = `${refund.refundNo}:${refund.version}:${consent}`;
      try {
        await respondCustomerRefundBalance({
          consent,
          expectedVersion: refund.version,
          idempotencyKey: customerRefundActionKey(scope),
          refundNo: refund.refundNo,
        });
        clearCustomerRefundActionKey(scope);
        message.success($t('refunds.balance.saved'));
        await loadHistory();
      } catch {
        message.error($t('refunds.balance.failed'));
        throw new Error('balance-response-failed');
      }
    },
  });
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(cursor.value);
  cursor.value = nextCursor.value;
  void loadHistory();
}

function previousPage() {
  cursor.value = cursorHistory.value.pop() || '';
  void loadHistory();
}

watch(orderNo, () => void loadOrderContext());
onMounted(() => void Promise.all([loadHistory(), loadOrderContext()]));
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('refunds.title')">
    <div class="refund-page" data-testid="customer-refund-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <p>{{ $t('refunds.lead') }}</p>
        </div>
        <Button @click="router.push('/account/orders')">
          {{ $t('refunds.actions.chooseOrder') }}
        </Button>
      </Card>

      <Alert
        v-if="historyFailed"
        :message="$t('refunds.messages.historyFailed')"
        show-icon
        type="error"
      />
      <Skeleton v-if="loading" active :paragraph="{ rows: 4 }" />
      <section v-else class="history-section">
        <div class="section-heading">
          <div>
            <h2>{{ $t('refunds.history.title') }}</h2>
            <p>{{ $t('refunds.history.lead') }}</p>
          </div>
        </div>
        <Empty
          v-if="!refunds.length"
          :description="$t('refunds.history.empty')"
        />
        <div v-else class="refund-grid">
          <Card
            v-for="refund in refunds"
            :key="refund.refundNo"
            :data-testid="`customer-refund-${refund.refundNo}`"
          >
            <div class="card-heading">
              <div>
                <strong>{{ refund.refundNo }}</strong>
                <p>{{ refund.orderNo }}</p>
              </div>
              <Tag :color="statusColor(refund.statusCode)">
                {{ $t(`refunds.status.${refund.statusCode}`) }}
              </Tag>
            </div>
            <strong class="refund-amount">
              {{ formatMinor(refund.amountMinor, refund.currencyCode) }}
            </strong>
            <dl>
              <div>
                <dt>{{ $t('refunds.history.reason') }}</dt>
                <dd>{{ $t(`refunds.reason.${refund.reasonCode}`) }}</dd>
              </div>
              <div>
                <dt>{{ $t('refunds.history.destination') }}</dt>
                <dd>
                  {{ $t(`refunds.destination.${refund.destinationCode}`) }}
                </dd>
              </div>
              <div>
                <dt>{{ $t('refunds.history.submittedAt') }}</dt>
                <dd>{{ formatTime(refund.submittedAt) }}</dd>
              </div>
            </dl>
            <Alert
              v-if="refund.statusCode === 'awaiting_balance_consent'"
              :message="$t('refunds.balance.wholeHeld')"
              show-icon
              type="warning"
            />
            <Space
              v-if="refund.statusCode === 'awaiting_balance_consent'"
              class="card-actions"
              wrap
            >
              <Button
                data-testid="refund-balance-accept"
                type="primary"
                @click="respondBalance(refund, true)"
              >
                {{ $t('refunds.balance.accept') }}
              </Button>
              <Button
                data-testid="refund-balance-keep-pending"
                @click="respondBalance(refund, false)"
              >
                {{ $t('refunds.balance.keepPending') }}
              </Button>
            </Space>
          </Card>
        </div>
        <div class="pager">
          <Button :disabled="!cursorHistory.length" @click="previousPage">
            {{ $t('refunds.actions.previous') }}
          </Button>
          <Button :disabled="!nextCursor" type="primary" @click="nextPage">
            {{ $t('refunds.actions.next') }}
          </Button>
        </div>
      </section>

      <section v-if="orderNo" class="application-section">
        <div class="section-heading">
          <div>
            <h2>{{ $t('refunds.application.title') }}</h2>
            <p>{{ $t('refunds.application.order', { orderNo }) }}</p>
          </div>
        </div>
        <Skeleton v-if="contextLoading" active :paragraph="{ rows: 6 }" />
        <Alert
          v-else-if="orderFailed"
          :message="$t('refunds.messages.orderFailed')"
          show-icon
          type="error"
        />
        <template v-else-if="invoice && order">
          <Alert
            :message="$t('refunds.application.amountHelp')"
            show-icon
            type="info"
          />
          <div class="selection-list">
            <Card
              v-for="item in invoice.items"
              :key="item.invoiceItemNo"
              class="selection-card"
              :data-testid="`refund-selection-${item.invoiceItemNo}`"
            >
              <div class="card-heading">
                <div>
                  <h3>{{ localized(item) }}</h3>
                </div>
                <Tag>{{ invoice.invoice.currencyCode }}</Tag>
              </div>
              <p>
                {{ $t('refunds.application.lineTotal') }}:
                <strong>{{
                  formatMinor(
                    item.totalAmountMinor,
                    invoice.invoice.currencyCode,
                  )
                }}</strong>
              </p>
              <div class="target-row">
                <Checkbox
                  :data-testid="`refund-whole-${item.invoiceItemNo}`"
                  :checked="
                    selectionState(selectionKey(item.invoiceItemNo)).selected
                  "
                  @update:checked="chooseWhole(item.invoiceItemNo, $event)"
                >
                  {{ $t('refunds.application.wholeItem') }}
                </Checkbox>
                <InputNumber
                  :data-testid="`refund-quantity-${item.invoiceItemNo}`"
                  v-model:value="
                    selectionState(selectionKey(item.invoiceItemNo)).quantity
                  "
                  :disabled="
                    !selectionState(selectionKey(item.invoiceItemNo)).selected
                  "
                  :max="orderItemQuantity(item.sourceReference)"
                  :min="1"
                  :placeholder="$t('refunds.application.quantity')"
                />
                <InputNumber
                  :data-testid="`refund-amount-${item.invoiceItemNo}`"
                  v-model:value="
                    selectionState(selectionKey(item.invoiceItemNo)).amountMinor
                  "
                  :disabled="
                    !selectionState(selectionKey(item.invoiceItemNo)).selected
                  "
                  :min="1"
                  :placeholder="$t('refunds.application.amountMinor')"
                  string-mode
                />
              </div>
              <div
                v-for="entitlement in entitlementsFor(item.sourceReference)"
                :key="entitlement.entitlementNo"
                class="target-row service-row"
              >
                <Checkbox
                  :data-testid="`refund-entitlement-${entitlement.entitlementNo}`"
                  :checked="
                    selectionState(
                      selectionKey(item.invoiceItemNo),
                    ).entitlementNos.includes(entitlement.entitlementNo)
                  "
                  @update:checked="
                    chooseEntitlement(
                      item.invoiceItemNo,
                      entitlement.entitlementNo,
                      $event,
                    )
                  "
                >
                  {{ $t('refunds.application.entitlement') }} ·
                  {{ localized(entitlement) }}
                </Checkbox>
              </div>
            </Card>
          </div>
          <Button
            data-testid="refund-preview"
            :loading="previewLoading"
            type="primary"
            @click="createPreview"
          >
            {{ $t('refunds.actions.preview') }}
          </Button>
        </template>
      </section>

      <VbenDrawerCompat
        v-model:open="previewOpen"
        :title="$t('refunds.preview.title')"
        root-class-name="refund-preview-drawer"
      >
        <template v-if="preview">
          <Alert
            v-if="preview.requiresEntitlementTermination"
            :message="$t('refunds.preview.terminationWarning')"
            show-icon
            type="error"
          />
          <Alert
            v-if="preview.hasDangerousOperation"
            class="section-gap"
            :message="$t('refunds.preview.dangerousWarning')"
            show-icon
            type="warning"
          />
          <Descriptions class="section-gap" :column="1" bordered size="small">
            <DescriptionsItem :label="$t('refunds.preview.amount')">
              {{ formatMinor(preview.amountMinor, preview.currencyCode) }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('refunds.preview.approval')">
              {{ $t(`refunds.approval.${preview.approvalModeCode}`) }}
            </DescriptionsItem>
          </Descriptions>
          <Card
            v-for="item in preview.items"
            :key="item.invoiceItemNo"
            class="section-gap"
          >
            <div class="card-heading">
              <strong>{{ localized(item) }}</strong>
              <strong>{{
                formatMinor(item.amountMinor, preview.currencyCode)
              }}</strong>
            </div>
            <p>
              {{ $t('refunds.application.refundable') }}:
              {{
                formatMinor(item.refundableAmountMinor, preview.currencyCode)
              }}
            </p>
            <p>
              {{ $t('refunds.application.quantity') }}: {{ item.quantity }}
            </p>
            <Alert
              v-for="(impact, impactIndex) in item.impacts"
              :key="`${item.invoiceItemNo}-${impactIndex}`"
              class="section-gap"
              :message="
                impact.requiresTermination
                  ? $t('refunds.preview.entitlementTermination', {
                      productName: localized(impact),
                    })
                  : $t('refunds.preview.entitlementImpact', {
                      productName: localized(impact),
                    })
              "
              show-icon
              :type="impact.requiresTermination ? 'warning' : 'info'"
            />
          </Card>
          <div class="preview-form section-gap">
            <label>
              <span>{{ $t('refunds.preview.reason') }}</span>
              <Select
                v-model:value="reasonCode"
                data-testid="refund-reason"
                :options="reasonOptions"
              />
            </label>
            <label v-if="reasonCode === 'other'">
              <span>{{ $t('refunds.preview.reasonDetail') }}</span>
              <Textarea v-model:value="reasonDetail" :maxlength="1000" />
            </label>
            <Checkbox
              v-model:checked="impactConfirmed"
              data-testid="refund-impact-confirm"
            >
              {{ $t('refunds.preview.confirmImpact') }}
            </Checkbox>
          </div>
        </template>
        <template #extra>
          <Button
            data-testid="refund-submit"
            :loading="submitLoading"
            type="primary"
            @click="submitRefund"
          >
            {{ $t('refunds.actions.submit') }}
          </Button>
        </template>
      </VbenDrawerCompat>
    </div>
  </Page>
</template>

<style scoped>
.refund-page {
  max-width: 100%;
  margin: 0 auto;
}
:global(.refund-preview-drawer .ant-drawer-content-wrapper) {
  width: min(94vw, 720px) !important;
}
.page-toolbar :deep(.ant-card-body) {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 0;
}
.page-toolbar { margin-bottom: 16px; }
.page-toolbar strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar p,
.section-heading p,
.card-heading p {
  margin: 0;
  color: hsl(var(--muted-foreground));
}
.page-toolbar p {
  color: hsl(var(--muted-foreground));
}
.history-section,
.application-section {
  margin-top: 26px;
}
.section-heading,
.card-heading,
.target-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.section-heading {
  margin-bottom: 16px;
}
.section-heading h2,
.card-heading h3 {
  margin: 0 0 6px;
}
.refund-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}
.refund-amount {
  display: block;
  margin: 18px 0;
  font-size: 26px;
}
.refund-grid dl {
  display: grid;
  gap: 8px;
}
.refund-grid dl div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.refund-grid dd {
  margin: 0;
  text-align: right;
}
.card-actions,
.section-gap {
  margin-top: 16px;
}
.selection-list {
  display: grid;
  gap: 16px;
  margin: 16px 0;
}
.selection-card {
  border-left: 4px solid #c2410c;
}
.target-row {
  padding: 12px 0;
  border-top: 1px solid hsl(var(--border));
}
.target-row :deep(.ant-input-number) {
  width: min(100%, 230px);
}
.service-row {
  padding-left: 22px;
}
.preview-form {
  display: grid;
  gap: 16px;
}
.preview-form label {
  display: grid;
  gap: 8px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
}
@media (max-width: 640px) {
  .page-toolbar :deep(.ant-card-body),
  .section-heading,
  .card-heading,
  .target-row {
    align-items: stretch;
    flex-direction: column;
  }
  .refund-grid {
    grid-template-columns: 1fr;
  }
  .target-row :deep(.ant-input-number),
  .target-row :deep(.ant-checkbox-wrapper) {
    width: 100%;
  }
  .service-row {
    padding-left: 0;
  }
}
</style>
