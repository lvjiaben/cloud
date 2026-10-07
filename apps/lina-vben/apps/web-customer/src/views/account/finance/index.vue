<script setup lang="ts">
import type {
  FinancialInvoice,
  FinancialInvoiceDetail,
  FinancialOverview,
  FinancialRenewalEntitlement,
  FinancialTransaction,
} from '#/api/idc/customer-finance';
import type { CustomerPaymentChannel } from '#/api/idc/customer-payments';

import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';
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
  Input,
  InputNumber,
  message,
  Modal as AntModal,
  Select,
  Skeleton,
  Switch,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';
import QRCode from 'qrcode';

import { persistentCommerceActionKey } from '#/api/idc/customer-commerce';
import VbenDrawerCompat from '#/components/vben-drawer-compat.vue';
import VbenModalCompat from '#/components/vben-modal-compat.vue';
import {
  getCreditOverview,
  settleInvoiceFromCredit,
} from '#/api/idc/customer-credit';
import {
  getFinancialInvoice,
  getFinancialOverview,
  listFinancialInvoices,
  listFinancialRenewalEntitlements,
  listFinancialTransactions,
  mergeFinancialInvoices,
  payFinancialInvoiceFromBalance,
  updateFinancialRenewalPolicy,
} from '#/api/idc/customer-finance';
import {
  createCustomerBalanceRecharge,
  listCustomerPaymentChannels,
} from '#/api/idc/customer-payments';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const busy = ref(false);
const failed = ref(false);
const overview = ref<FinancialOverview>();
const invoices = ref<FinancialInvoice[]>([]);
const detail = ref<FinancialInvoiceDetail>();
const transactions = ref<FinancialTransaction[]>([]);
const renewals = ref<FinancialRenewalEntitlement[]>([]);
const nextCursor = ref('');
const cursor = ref('');
const cursorHistory = ref<string[]>([]);
const statusCode = ref('');
const currencyCode = ref('');
const accountType = ref('customer_balance');
const selectedInvoiceNos = ref<string[]>([]);
const renewalOpen = ref(false);
const rechargeOpen = ref(false);
const paymentChannels = ref<CustomerPaymentChannel[]>([]);
const rechargeChannelsLoading = ref(false);
const rechargeSubmitting = ref(false);
const paymentQrOpen = ref(false);
const paymentQrDataUrl = ref('');
const paymentAttemptNo = ref('');
const creditAccounts = ref<
  Array<{
    availableLimitMinor: string;
    currencyCode: string;
    statusCode: string;
  }>
>([]);
const rechargeForm = reactive({
  amountMajor: '',
  channelNo: '',
  currencyCode: 'CNY',
  modeCode: '',
});
const editingRenewal = ref<FinancialRenewalEntitlement>();
const renewalForm = reactive({
  autoRenewEnabled: true,
  failureNotificationEnabled: true,
  leadDays: 7,
  maximumAttempts: 3,
  renewalCycles: 1,
  retryHours: 6,
});

const invoiceNo = computed(() =>
  typeof route.params.invoiceNo === 'string' ? route.params.invoiceNo : '',
);
const view = computed(() => {
  if (invoiceNo.value) return 'invoice-detail';
  if (route.name === 'CustomerInvoices') return 'invoices';
  if (route.name === 'CustomerTransactions') return 'transactions';
  if (route.name === 'CustomerRenewals') return 'renewals';
  return 'overview';
});
const title = computed(() => {
  if (view.value === 'invoice-detail') {
    return $t('finance.invoice.detailTitle', { invoiceNo: invoiceNo.value });
  }
  return $t(`finance.views.${view.value}`);
});
const isEnglish = computed(
  () => document.documentElement.lang.toLowerCase() === 'en-us',
);

const navigation = [
  { label: 'finance.views.overview', path: '/account/billing' },
  { label: 'credit.menu', path: '/account/billing/credit' },
  { label: 'finance.views.invoices', path: '/account/billing/invoices' },
  {
    label: 'finance.views.transactions',
    path: '/account/billing/transactions',
  },
  { label: 'finance.views.renewals', path: '/account/billing/renewals' },
];
const statusOptions = computed(() => [
  { label: $t('finance.status.all'), value: '' },
  ...[
    'pending_payment',
    'partially_paid',
    'overdue',
    'paid',
    'cancelled',
    'refunded',
  ].map((value) => ({ label: statusLabel(value), value })),
]);
const accountOptions = computed(() =>
  [
    'customer_balance',
    'customer_credit',
    'customer_gift',
    'customer_commission',
  ].map((value) => ({ label: $t(`finance.account.${value}`), value })),
);
const rechargeAvailable = computed(() => paymentChannels.value.length > 0);
const rechargeCurrencyOptions = computed(() => {
  const currencies = new Set<string>();
  for (const channel of paymentChannels.value) {
    for (const currency of channel.currencies) currencies.add(currency);
  }
  return [...currencies].sort().map((value) => ({ label: value, value }));
});
const rechargeChannelOptions = computed(() =>
  paymentChannels.value
    .filter((channel) => channel.currencies.includes(rechargeForm.currencyCode))
    .map((channel) => ({
      label: channel.displayName,
      value: channel.channelNo,
    })),
);
const selectedRechargeChannel = computed(() =>
  paymentChannels.value.find(
    (channel) => channel.channelNo === rechargeForm.channelNo,
  ),
);
const rechargeModeOptions = computed(() =>
  (selectedRechargeChannel.value?.modes || []).map((value) => ({
    label: $t(`finance.recharge.modes.${value}`),
    value,
  })),
);
const creditPaymentAccount = computed(() => {
  const invoice = detail.value?.invoice;
  if (!invoice || !['pending_payment', 'overdue'].includes(invoice.statusCode))
    return undefined;
  return creditAccounts.value.find(
    (item) =>
      item.currencyCode === invoice.currencyCode &&
      item.statusCode === 'active' &&
      BigInt(item.availableLimitMinor) >=
        BigInt(invoice.outstandingAmountMinor),
  );
});

function statusLabel(value: unknown) {
  const code = String(value || 'pending');
  const key = `finance.status.${code}`;
  const translated = $t(key);
  return translated === key ? code : translated;
}

function statusColor(value: unknown) {
  const code = String(value || '');
  if (['active', 'paid', 'succeeded'].includes(code)) return 'success';
  if (['cancelled', 'expired', 'refunded'].includes(code)) return 'default';
  if (['exhausted', 'failed', 'overdue'].includes(code)) return 'error';
  return 'processing';
}

function currencyDigits(code: string) {
  try {
    return (
      new Intl.NumberFormat('en', {
        currency: code,
        style: 'currency',
      }).resolvedOptions().maximumFractionDigits ?? 2
    );
  } catch {
    return 2;
  }
}

function amount(value: unknown, code: string) {
  const raw = String(value ?? '');
  if (!/^-?\d+$/u.test(raw)) return '—';
  const negative = raw.startsWith('-');
  const digits = raw.replace(/^-/, '').replace(/^0+(?=\d)/, '') || '0';
  const scale = currencyDigits(code);
  if (!scale) return `${code} ${negative ? '-' : ''}${digits}`;
  const padded = digits.padStart(scale + 1, '0');
  return `${code} ${negative ? '-' : ''}${padded.slice(0, -scale)}.${padded.slice(-scale)}`;
}

function majorToMinor(value: string, code: string) {
  const normalized = value.trim();
  if (!/^\d+(?:\.\d+)?$/u.test(normalized)) return '';
  const scale = currencyDigits(code);
  const [integerPart = '0', decimalPart = ''] = normalized.split('.');
  if (decimalPart.length > scale) return '';
  const minor =
    `${integerPart.replace(/^0+(?=\d)/u, '') || '0'}${decimalPart.padEnd(scale, '0')}`.replace(
      /^0+(?=\d)/u,
      '',
    );
  return minor && minor !== '0' ? minor : '';
}

function formatTime(value: unknown) {
  const timestamp = Number(value || 0);
  return timestamp > 0 ? dayjs(timestamp).format('YYYY-MM-DD HH:mm') : '—';
}

function localized(item: {
  descriptionEnUS?: string;
  descriptionZhCN?: string;
  productNameEnUS?: string;
  productNameZhCN?: string;
}) {
  return isEnglish.value
    ? item.descriptionEnUS || item.productNameEnUS || '—'
    : item.descriptionZhCN || item.productNameZhCN || '—';
}

function resetPaging() {
  cursor.value = '';
  cursorHistory.value = [];
  nextCursor.value = '';
}

async function loadCurrent() {
  loading.value = true;
  failed.value = false;
  selectedInvoiceNos.value = [];
  try {
    if (view.value === 'overview') {
      overview.value = (await getFinancialOverview()).overview;
      await loadRechargeChannels();
    } else if (view.value === 'invoices') {
      const result = await listFinancialInvoices({
        currencyCode: currencyCode.value.trim().toUpperCase() || undefined,
        cursor: cursor.value || undefined,
        statusCode: statusCode.value || undefined,
      });
      invoices.value = result.invoices || [];
      nextCursor.value = result.nextCursor || '';
    } else if (view.value === 'invoice-detail') {
      detail.value = (await getFinancialInvoice(invoiceNo.value)).detail;
      try {
        creditAccounts.value =
          (await getCreditOverview()).overview.accounts || [];
      } catch {
        // The invoice remains usable when the optional credit account lookup fails.
        creditAccounts.value = [];
      }
    } else if (view.value === 'transactions') {
      const result = await listFinancialTransactions({
        accountType: accountType.value,
        currencyCode: currencyCode.value.trim().toUpperCase() || undefined,
        cursor: cursor.value || undefined,
      });
      transactions.value = result.transactions || [];
      nextCursor.value = result.nextCursor || '';
    } else {
      const result = await listFinancialRenewalEntitlements({
        cursor: cursor.value || undefined,
        statusCode: statusCode.value || undefined,
      });
      renewals.value = result.entitlements || [];
      nextCursor.value = result.nextCursor || '';
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadRechargeChannels() {
  rechargeChannelsLoading.value = true;
  try {
    const result = await listCustomerPaymentChannels();
    paymentChannels.value = result.channels || [];
    const currencies = new Set(
      paymentChannels.value.flatMap((channel) => channel.currencies),
    );
    if (!currencies.has(rechargeForm.currencyCode)) {
      rechargeForm.currencyCode =
        [...currencies].sort()[0] || rechargeForm.currencyCode;
    }
    syncRechargeSelections();
  } catch {
    paymentChannels.value = [];
  } finally {
    rechargeChannelsLoading.value = false;
  }
}

function syncRechargeSelections() {
  const channels = paymentChannels.value.filter((channel) =>
    channel.currencies.includes(rechargeForm.currencyCode),
  );
  if (
    !channels.some((channel) => channel.channelNo === rechargeForm.channelNo)
  ) {
    rechargeForm.channelNo = channels[0]?.channelNo || '';
  }
  const modes = selectedRechargeChannel.value?.modes || [];
  if (!modes.includes(rechargeForm.modeCode)) {
    rechargeForm.modeCode = modes[0] || '';
  }
}

function openRecharge() {
  syncRechargeSelections();
  rechargeOpen.value = true;
}

function submitPostRedirect(url: string, fields: Record<string, string>) {
  const form = document.createElement('form');
  form.method = 'POST';
  form.action = url;
  for (const [name, value] of Object.entries(fields)) {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    form.append(input);
  }
  form.hidden = true;
  document.body.append(form);
  form.submit();
}

function invokeWechatJSAPI(fields: Record<string, string>, attemptNo: string) {
  const bridge = (
    window as typeof window & {
      WeixinJSBridge?: {
        invoke: (
          name: string,
          input: Record<string, string>,
          callback: () => void,
        ) => void;
      };
    }
  ).WeixinJSBridge;
  if (!bridge) {
    throw new Error('wechat bridge unavailable');
  }
  bridge.invoke('getBrandWCPayRequest', fields, () => {
    void router.push(`/payment/return/${encodeURIComponent(attemptNo)}`);
  });
}

async function submitRecharge() {
  const amountMinor = majorToMinor(
    rechargeForm.amountMajor,
    rechargeForm.currencyCode,
  );
  if (
    !amountMinor ||
    !rechargeForm.channelNo ||
    !rechargeForm.modeCode ||
    rechargeSubmitting.value
  ) {
    message.warning($t('finance.recharge.invalid'));
    return;
  }
  rechargeSubmitting.value = true;
  try {
    const created = await createCustomerBalanceRecharge({
      amountMinor,
      channelNo: rechargeForm.channelNo,
      currencyCode: rechargeForm.currencyCode,
      modeCode: rechargeForm.modeCode,
    });
    paymentAttemptNo.value = created.attemptNo;
    if (created.redirectMethod === 'POST') {
      rechargeOpen.value = false;
      submitPostRedirect(created.redirectUrl, created.redirectFields);
      return;
    }
    if (created.redirectMethod === 'GET') {
      rechargeOpen.value = false;
      window.location.assign(created.redirectUrl);
      return;
    }
    if (created.redirectMethod === 'QR') {
      paymentQrDataUrl.value = await QRCode.toDataURL(created.redirectUrl, {
        errorCorrectionLevel: 'M',
        margin: 2,
        width: 280,
      });
      rechargeOpen.value = false;
      await nextTick();
      await new Promise((resolve) => setTimeout(resolve, 300));
      paymentQrOpen.value = true;
      return;
    }
    if (created.redirectMethod === 'JSAPI') {
      rechargeOpen.value = false;
      invokeWechatJSAPI(created.redirectFields, created.attemptNo);
      return;
    }
    throw new Error('unsupported payment redirect');
  } catch {
    message.error($t('finance.recharge.failed'));
  } finally {
    rechargeSubmitting.value = false;
  }
}

function applyFilters() {
  resetPaging();
  void loadCurrent();
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(cursor.value);
  cursor.value = nextCursor.value;
  void loadCurrent();
}

function previousPage() {
  cursor.value = cursorHistory.value.pop() || '';
  void loadCurrent();
}

function toggleInvoice(item: FinancialInvoice, checked: boolean) {
  if (!item.canMerge) return;
  const values = new Set(selectedInvoiceNos.value);
  if (checked) values.add(item.invoiceNo);
  else values.delete(item.invoiceNo);
  selectedInvoiceNos.value = [...values];
}

function confirmMerge() {
  if (selectedInvoiceNos.value.length < 2 || busy.value) return;
  const sources = [...selectedInvoiceNos.value].sort();
  AntModal.confirm({
    content: $t('finance.merge.description'),
    okText: $t('finance.merge.confirm'),
    title: $t('finance.merge.title'),
    async onOk() {
      busy.value = true;
      try {
        await mergeFinancialInvoices(
          sources,
          persistentCommerceActionKey(`billing.merge.${sources.join('.')}`),
        );
        resetPaging();
        await loadCurrent();
        message.success($t('finance.messages.merged'));
      } finally {
        busy.value = false;
      }
    },
  });
}

function confirmBalancePayment() {
  if (!detail.value?.invoice.canPayFromBalance || busy.value) return;
  AntModal.confirm({
    content: $t('finance.balancePayment.description'),
    okText: $t('finance.balancePayment.confirm'),
    title: $t('finance.balancePayment.title'),
    async onOk() {
      busy.value = true;
      try {
        await payFinancialInvoiceFromBalance(
          detail.value!.invoice.invoiceNo,
          persistentCommerceActionKey(
            `billing.balance.${detail.value!.invoice.invoiceNo}`,
          ),
        );
        detail.value = (
          await getFinancialInvoice(detail.value!.invoice.invoiceNo)
        ).detail;
        message.success($t('finance.messages.paid'));
      } finally {
        busy.value = false;
      }
    },
  });
}

function confirmCreditPayment() {
  const invoice = detail.value?.invoice;
  const account = creditPaymentAccount.value;
  if (!invoice || !account || busy.value) return;
  const remaining = (
    BigInt(account.availableLimitMinor) - BigInt(invoice.outstandingAmountMinor)
  ).toString();
  AntModal.confirm({
    content: $t('finance.creditPayment.description', {
      amount: amount(invoice.outstandingAmountMinor, invoice.currencyCode),
      remaining: amount(remaining, invoice.currencyCode),
    }),
    okText: $t('finance.creditPayment.confirm'),
    title: $t('finance.creditPayment.title'),
    async onOk() {
      busy.value = true;
      try {
        await settleInvoiceFromCredit(invoice.invoiceNo);
        detail.value = (await getFinancialInvoice(invoice.invoiceNo)).detail;
        message.success($t('finance.creditPayment.success'));
      } catch {
        message.error($t('finance.creditPayment.failed'));
      } finally {
        busy.value = false;
      }
    },
  });
}

function editRenewal(service: FinancialRenewalEntitlement) {
  editingRenewal.value = service;
  renewalForm.autoRenewEnabled = service.policyNo
    ? service.autoRenewEnabled
    : true;
  renewalForm.failureNotificationEnabled = service.policyNo
    ? service.failureNotificationEnabled
    : true;
  renewalForm.leadDays = service.policyNo
    ? Math.max(1, Math.round(service.leadSeconds / 86_400))
    : 7;
  renewalForm.maximumAttempts = service.policyNo ? service.maximumAttempts : 3;
  renewalForm.renewalCycles = service.policyNo ? service.renewalCycles : 1;
  renewalForm.retryHours = service.policyNo
    ? Math.max(1, Math.round(service.retryIntervalSeconds / 3_600))
    : 6;
  renewalOpen.value = true;
}

async function saveRenewal() {
  if (!editingRenewal.value || busy.value) return;
  busy.value = true;
  try {
    await updateFinancialRenewalPolicy(editingRenewal.value.entitlementNo, {
      autoRenewEnabled: renewalForm.autoRenewEnabled,
      expectedEntitlementVersion: editingRenewal.value.entitlementVersion,
      failureNotificationEnabled: renewalForm.failureNotificationEnabled,
      leadSeconds: renewalForm.leadDays * 86_400,
      maximumAttempts: renewalForm.maximumAttempts,
      renewalCycles: renewalForm.renewalCycles,
      retryIntervalSeconds: renewalForm.retryHours * 3_600,
    });
    renewalOpen.value = false;
    await loadCurrent();
    message.success($t('finance.messages.renewalSaved'));
  } finally {
    busy.value = false;
  }
}

onMounted(loadCurrent);
watch(
  () => route.fullPath,
  () => {
    resetPaging();
    void loadCurrent();
  },
);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="title">
    <div class="finance-page" data-testid="customer-finance-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <p>{{ $t('finance.lead') }}</p>
        </div>
      </Card>

      <nav class="finance-nav" :aria-label="$t('finance.eyebrow')">
        <Button
          v-for="item in navigation"
          :key="item.path"
          :type="
            route.path === item.path ||
            (item.path.endsWith('/invoices') && view === 'invoice-detail')
              ? 'primary'
              : 'text'
          "
          @click="router.push(item.path)"
          >{{ $t(item.label) }}</Button
        >
      </nav>

      <Alert
        v-if="failed"
        :message="$t('finance.messages.loadFailed')"
        show-icon
        type="error"
      >
        <template #action
          ><Button size="small" @click="loadCurrent">{{
            $t('finance.actions.retry')
          }}</Button></template
        >
      </Alert>
      <Skeleton v-if="loading" active :paragraph="{ rows: 8 }" />

      <template v-else-if="!failed && view === 'overview' && overview">
        <div class="metric-grid">
          <Card
            ><span>{{ $t('finance.overview.openInvoices') }}</span
            ><strong>{{ overview.openInvoiceCount }}</strong></Card
          >
          <Card
            ><span>{{ $t('finance.overview.autoRenew') }}</span
            ><strong>{{ overview.automaticRenewalEnabled }}</strong></Card
          >
          <Card
            v-for="item in overview.openReceivables"
            :key="item.currencyCode"
            class="metric-emphasis"
          >
            <span
              >{{ $t('finance.amount.outstanding') }} ·
              {{ item.currencyCode }}</span
            >
            <strong>{{
              amount(item.outstandingMinor, item.currencyCode)
            }}</strong>
          </Card>
        </div>
        <div v-if="overview.balances.length" class="card-grid">
          <Card
            v-for="item in overview.balances"
            :key="`${item.accountTypeCode}-${item.currencyCode}`"
            :data-testid="`finance-balance-${item.currencyCode}`"
          >
            <div class="row-heading">
              <span>{{ $t(`finance.account.${item.accountTypeCode}`) }}</span
              ><Tag>{{ item.currencyCode }}</Tag>
            </div>
            <strong class="large-amount">{{
              amount(item.availableBalanceMinor, item.currencyCode)
            }}</strong>
            <dl class="facts">
              <div>
                <dt>{{ $t('finance.amount.balance') }}</dt>
                <dd>{{ amount(item.balanceMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.amount.available') }}</dt>
                <dd>
                  {{ amount(item.availableBalanceMinor, item.currencyCode) }}
                </dd>
              </div>
            </dl>
          </Card>
        </div>
        <Empty v-else :description="$t('finance.empty.balances')" />
        <Alert
          v-if="!rechargeAvailable"
          class="section-gap"
          :message="$t('finance.recharge.unavailable')"
          show-icon
          type="info"
        >
          <template #action
            ><Button :loading="rechargeChannelsLoading" disabled>{{
              $t('finance.recharge.action')
            }}</Button></template
          >
        </Alert>
        <Card v-else class="recharge-card section-gap">
          <div class="row-heading">
            <div>
              <h2>{{ $t('finance.recharge.title') }}</h2>
              <p>{{ $t('finance.recharge.description') }}</p>
            </div>
            <Button
              data-testid="finance-recharge-open"
              type="primary"
              @click="openRecharge"
            >
              {{ $t('finance.recharge.action') }}
            </Button>
          </div>
        </Card>
      </template>

      <template v-else-if="!failed && view === 'invoices'">
        <div class="filters">
          <Select
            v-model:value="statusCode"
            :options="statusOptions"
            @change="applyFilters"
          /><Input
            v-model:value="currencyCode"
            :maxlength="3"
            :placeholder="$t('finance.filters.currency')"
            @press-enter="applyFilters"
          /><Button @click="applyFilters">{{
            $t('finance.actions.apply')
          }}</Button>
        </div>
        <div class="selection">
          <span>{{
            $t('finance.merge.selected', { count: selectedInvoiceNos.length })
          }}</span
          ><Button
            :disabled="selectedInvoiceNos.length < 2"
            type="primary"
            @click="confirmMerge"
            >{{ $t('finance.merge.action') }}</Button
          >
        </div>
        <Empty
          v-if="!invoices.length"
          :description="$t('finance.empty.invoices')"
        />
        <div v-else class="card-grid">
          <Card
            v-for="item in invoices"
            :key="item.invoiceNo"
            :data-testid="`finance-invoice-${item.invoiceNo}`"
          >
            <div class="row-heading">
              <div>
                <Checkbox
                  :checked="selectedInvoiceNos.includes(item.invoiceNo)"
                  :disabled="!item.canMerge"
                  @update:checked="toggleInvoice(item, $event)"
                /><strong>{{ item.invoiceNo }}</strong>
              </div>
              <Tag :color="statusColor(item.statusCode)">{{
                statusLabel(item.statusCode)
              }}</Tag>
            </div>
            <dl class="facts">
              <div>
                <dt>{{ $t('finance.amount.total') }}</dt>
                <dd>{{ amount(item.totalAmountMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.amount.outstanding') }}</dt>
                <dd>
                  {{ amount(item.outstandingAmountMinor, item.currencyCode) }}
                </dd>
              </div>
              <div>
                <dt>{{ $t('finance.invoice.due') }}</dt>
                <dd>{{ formatTime(item.paymentDueAt) }}</dd>
              </div>
            </dl>
            <Button
              type="primary"
              @click="
                router.push(`/account/billing/invoices/${item.invoiceNo}`)
              "
              >{{ $t('finance.actions.detail') }}</Button
            >
          </Card>
        </div>
      </template>

      <template v-else-if="!failed && view === 'invoice-detail' && detail">
        <Button
          class="back"
          type="link"
          @click="router.push('/account/billing/invoices')"
          >← {{ $t('finance.actions.backToInvoices') }}</Button
        >
        <Card>
          <div class="row-heading">
            <h2>{{ detail.invoice.invoiceNo }}</h2>
            <Tag :color="statusColor(detail.invoice.statusCode)">{{
              statusLabel(detail.invoice.statusCode)
            }}</Tag>
          </div>
          <Descriptions :column="1" bordered size="small"
            ><DescriptionsItem :label="$t('finance.amount.total')">{{
              amount(
                detail.invoice.totalAmountMinor,
                detail.invoice.currencyCode,
              )
            }}</DescriptionsItem
            ><DescriptionsItem :label="$t('finance.amount.outstanding')">{{
              amount(
                detail.invoice.outstandingAmountMinor,
                detail.invoice.currencyCode,
              )
            }}</DescriptionsItem
            ><DescriptionsItem :label="$t('finance.invoice.due')">{{
              formatTime(detail.invoice.paymentDueAt)
            }}</DescriptionsItem></Descriptions
          >
          <Button
            v-if="detail.invoice.canPayFromBalance"
            class="section-gap"
            data-testid="finance-balance-pay"
            type="primary"
            @click="confirmBalancePayment"
            >{{ $t('finance.balancePayment.action') }}</Button
          >
          <Button
            v-if="creditPaymentAccount"
            class="section-gap credit-pay"
            data-testid="finance-credit-pay"
            @click="confirmCreditPayment"
            >{{ $t('finance.creditPayment.action') }}</Button
          >
        </Card>
        <h2 class="section-gap">{{ $t('finance.invoice.items') }}</h2>
        <div class="card-grid">
          <Card v-for="item in detail.items" :key="item.invoiceItemNo"
            ><div class="row-heading">
              <strong>{{ localized(item) }}</strong
              ><strong>{{
                amount(item.totalAmountMinor, detail.invoice.currencyCode)
              }}</strong>
            </div></Card
          >
        </div>
      </template>

      <template v-else-if="!failed && view === 'transactions'">
        <div class="filters">
          <Select
            v-model:value="accountType"
            :options="accountOptions"
            @change="applyFilters"
          /><Input
            v-model:value="currencyCode"
            :maxlength="3"
            :placeholder="$t('finance.filters.currency')"
            @press-enter="applyFilters"
          />
        </div>
        <Empty
          v-if="!transactions.length"
          :description="$t('finance.empty.transactions')"
        />
        <div v-else class="card-grid">
          <Card v-for="item in transactions" :key="item.transactionNo"
            ><div class="row-heading">
              <strong>{{ item.transactionNo }}</strong
              ><Tag>{{ item.currencyCode }}</Tag>
            </div>
            <dl class="facts">
              <div>
                <dt>{{ $t('finance.transaction.businessAmount') }}</dt>
                <dd>{{ amount(item.amountMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.transaction.change') }}</dt>
                <dd>{{ amount(item.changeMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.transaction.posted') }}</dt>
                <dd>{{ formatTime(item.postedAt) }}</dd>
              </div>
            </dl></Card
          >
        </div>
      </template>

      <template v-else-if="!failed && view === 'renewals'">
        <Alert
          class="renewal-note"
          :message="$t('finance.renewal.note')"
          show-icon
          type="info"
        />
        <Empty
          v-if="!renewals.length"
          :description="$t('finance.empty.renewals')"
        />
        <div v-else class="card-grid">
          <Card
            v-for="item in renewals"
            :key="item.entitlementNo"
            :data-testid="`finance-renewal-${item.entitlementNo}`"
            ><div class="row-heading">
              <div>
                <h2>{{ localized(item) }}</h2>
              </div>
              <Tag :color="item.autoRenewEnabled ? 'success' : 'default'">{{
                item.autoRenewEnabled
                  ? $t('finance.renewal.enabled')
                  : $t('finance.renewal.disabled')
              }}</Tag>
            </div>
            <dl class="facts">
              <div>
                <dt>{{ $t('finance.renewal.entitlementStatus') }}</dt>
                <dd>{{ statusLabel(item.entitlementStatusCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.renewal.expiry') }}</dt>
                <dd>{{ formatTime(item.entitlementExpiresAt) }}</dd>
              </div>
              <div>
                <dt>{{ $t('finance.renewal.nextAttempt') }}</dt>
                <dd>{{ formatTime(item.nextAttemptAt) }}</dd>
              </div>
            </dl>
            <Alert
              v-if="item.latestAttemptOutcomeCode === 'insufficient_balance'"
              :data-testid="`renewal-attempt-${item.entitlementNo}`"
              :message="$t('finance.renewal.insufficient')"
              show-icon
              type="warning"
            /><Button
              class="section-gap"
              :disabled="!item.canConfigureAutomaticRenewal"
              @click="editRenewal(item)"
              >{{ $t('finance.renewal.configure') }}</Button
            ></Card
          >
        </div>
      </template>

      <div
        v-if="['invoices', 'renewals', 'transactions'].includes(view)"
        class="pager"
      >
        <Button :disabled="!cursorHistory.length" @click="previousPage">{{
          $t('finance.actions.previous')
        }}</Button
        ><Button :disabled="!nextCursor" type="primary" @click="nextPage">{{
          $t('finance.actions.next')
        }}</Button>
      </div>

      <VbenDrawerCompat
        v-model:open="renewalOpen"
        :title="$t('finance.renewal.configure')"
        width="min(92vw, 460px)"
      >
        <div class="renewal-form">
          <label
            ><span>{{ $t('finance.renewal.enabled') }}</span
            ><Switch v-model:checked="renewalForm.autoRenewEnabled" /></label
          ><label
            ><span>{{ $t('finance.renewal.failureNotify') }}</span
            ><Switch
              v-model:checked="renewalForm.failureNotificationEnabled" /></label
          ><label
            ><span>{{ $t('finance.renewal.cycles') }}</span
            ><InputNumber
              v-model:value="renewalForm.renewalCycles"
              :min="1"
              :max="120" /></label
          ><label
            ><span>{{ $t('finance.renewal.leadDays') }}</span
            ><InputNumber
              v-model:value="renewalForm.leadDays"
              :min="1"
              :max="90" /></label
          ><label
            ><span>{{ $t('finance.renewal.attempts') }}</span
            ><InputNumber
              v-model:value="renewalForm.maximumAttempts"
              :min="1"
              :max="20" /></label
          ><label
            ><span>{{ $t('finance.renewal.retryHours') }}</span
            ><InputNumber
              v-model:value="renewalForm.retryHours"
              :min="1"
              :max="168"
          /></label>
        </div>
        <template #extra
          ><Button :loading="busy" type="primary" @click="saveRenewal">{{
            $t('finance.actions.save')
          }}</Button></template
        >
      </VbenDrawerCompat>
      <VbenModalCompat
        v-model:open="rechargeOpen"
        :cancel-text="$t('finance.recharge.cancel')"
        :confirm-loading="rechargeSubmitting"
        :ok-text="$t('finance.recharge.continue')"
        :title="$t('finance.recharge.title')"
        @ok="submitRecharge"
      >
        <div class="recharge-form">
          <Alert
            :message="$t('finance.recharge.confirmationNote')"
            show-icon
            type="info"
          />
          <label>
            <span>{{ $t('finance.recharge.currency') }}</span>
            <Select
              v-model:value="rechargeForm.currencyCode"
              :options="rechargeCurrencyOptions"
              @change="syncRechargeSelections"
            />
          </label>
          <label>
            <span>{{ $t('finance.recharge.amount') }}</span>
            <InputNumber
              v-model:value="rechargeForm.amountMajor"
              :min="0"
              :precision="currencyDigits(rechargeForm.currencyCode)"
              string-mode
            />
          </label>
          <label>
            <span>{{ $t('finance.recharge.channel') }}</span>
            <Select
              v-model:value="rechargeForm.channelNo"
              :options="rechargeChannelOptions"
              @change="syncRechargeSelections"
            />
          </label>
          <label>
            <span>{{ $t('finance.recharge.mode') }}</span>
            <Select
              v-model:value="rechargeForm.modeCode"
              :options="rechargeModeOptions"
            />
          </label>
        </div>
      </VbenModalCompat>
      <VbenModalCompat
        v-model:open="paymentQrOpen"
        :cancel-text="$t('finance.recharge.close')"
        :ok-text="$t('finance.recharge.checkStatus')"
        :title="$t('finance.recharge.scanTitle')"
        @ok="
          router.push(`/payment/return/${encodeURIComponent(paymentAttemptNo)}`)
        "
      >
        <div class="payment-qr">
          <img
            v-if="paymentQrDataUrl"
            :alt="$t('finance.recharge.scanTitle')"
            :src="paymentQrDataUrl"
          />
          <p>{{ $t('finance.recharge.scanDescription') }}</p>
        </div>
      </VbenModalCompat>
    </div>
  </Page>
</template>

<style scoped>
.finance-page {
  max-width: 100%;
  margin: 0 auto;
}
.page-toolbar {
  margin-bottom: 16px;
}
.page-toolbar strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar p {
  max-width: 820px;
  margin: 0;
  color: hsl(var(--muted-foreground));
}
.finance-nav {
  display: flex;
  gap: 6px;
  margin-bottom: 20px;
  padding: 6px;
  overflow-x: auto;
  border: 1px solid hsl(var(--border));
  border-radius: 12px;
  background: hsl(var(--card));
}
.metric-grid,
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}
.metric-grid :deep(.ant-card-body) {
  display: grid;
  gap: 8px;
}
.metric-grid strong,
.large-amount {
  font-size: 28px;
}
.metric-emphasis {
  border-top: 2px solid hsl(var(--primary));
}
.row-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}
.row-heading h2 {
  margin: 0;
}
.facts {
  display: grid;
  gap: 10px;
  margin: 18px 0;
}
.facts div {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}
.facts dd {
  margin: 0;
  font-weight: 700;
  text-align: right;
}
.filters,
.selection,
.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 16px;
}
.filters :deep(.ant-select),
.filters :deep(.ant-input) {
  width: min(100%, 260px);
}
.pager {
  justify-content: flex-end;
  margin-top: 18px;
}
.section-gap,
.renewal-note {
  margin-top: 18px;
}
.back {
  margin: 0 0 12px -14px;
}
.renewal-form {
  display: grid;
  gap: 18px;
}
.recharge-form {
  display: grid;
  gap: 18px;
}
.recharge-form label,
.renewal-form label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.recharge-form :deep(.ant-select),
.recharge-form :deep(.ant-input-number) {
  width: min(100%, 280px);
}
.recharge-card h2,
.recharge-card p {
  margin: 0;
}
.recharge-card p {
  margin-top: 8px;
  color: hsl(var(--muted-foreground));
}
.payment-qr {
  display: grid;
  justify-items: center;
  gap: 14px;
  text-align: center;
}
.payment-qr img {
  width: min(280px, 80vw);
  border: 10px solid #fff;
  border-radius: 14px;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.12);
}
@media (max-width: 640px) {
  .metric-grid,
  .card-grid {
    grid-template-columns: 1fr;
  }
  .filters,
  .selection {
    align-items: stretch;
    flex-direction: column;
  }
  .filters :deep(.ant-select),
  .filters :deep(.ant-input),
  .filters :deep(.ant-btn),
  .selection :deep(.ant-btn) {
    width: 100%;
  }
  .row-heading {
    flex-direction: column;
  }
}
</style>
