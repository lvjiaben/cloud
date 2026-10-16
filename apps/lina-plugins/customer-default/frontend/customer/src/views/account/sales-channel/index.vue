<script setup lang="ts">
import type {
  CustomerCommissionAccount,
  CustomerCommissionEntry,
  CustomerCommissionSettlement,
  CustomerReferralOverview,
} from '#/api/idc/customer-sales-channel';

import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Skeleton,
  Space,
  Tag,
  message,
} from 'ant-design-vue';

import {
  getCustomerCommissionOverview,
  getCustomerReferralOverview,
  listCustomerCommissionSettlements,
} from '#/api/idc/customer-sales-channel';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const failed = ref(false);
const referral = ref<CustomerReferralOverview>();
const accounts = ref<CustomerCommissionAccount[]>([]);
const entries = ref<CustomerCommissionEntry[]>([]);
const settlements = ref<CustomerCommissionSettlement[]>([]);

const view = computed(() =>
  route.name === 'CustomerCommissions' || route.query.tab === 'commission'
    ? 'commission'
    : 'referral',
);
const title = computed(() =>
  view.value === 'commission'
    ? $t('salesChannel.customer.commission.title')
    : $t('salesChannel.customer.referral.title'),
);
function amount(value: string | undefined, currencyCode: string) {
  const minor = Number(value || 0);
  if (!Number.isFinite(minor)) return '—';
  return new Intl.NumberFormat(undefined, {
    style: 'currency', currency: currencyCode,
  }).format(minor / 100);
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '—';
}

function statusLabel(status: string) {
  const key = `salesChannel.customer.status.${status}`;
  const translated = $t(key);
  return translated === key ? status : translated;
}

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const [referralResult, commissionResult, settlementResult] = await Promise.all([
      getCustomerReferralOverview(),
      getCustomerCommissionOverview(),
      listCustomerCommissionSettlements(),
    ]);
    referral.value = referralResult;
    accounts.value = commissionResult.accounts || [];
    entries.value = commissionResult.entries || [];
    settlements.value = settlementResult.items || [];
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function copyReferralLink() {
  if (!referral.value?.referralUrl) return;
  try {
    await navigator.clipboard.writeText(referral.value.referralUrl);
    message.success($t('salesChannel.customer.referral.copied'));
  } catch {
    message.error($t('salesChannel.customer.referral.copyFailed'));
  }
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="title">
    <div class="sales-customer-page" data-testid="customer-sales-channel-page">
      <Card :bordered="false" class="sales-toolbar">
        <div>
          <p>{{ $t('salesChannel.customer.lead') }}</p>
        </div>
        <Space>
          <Button :type="view === 'referral' ? 'primary' : 'text'" @click="router.push('/account/referrals')">{{ $t('salesChannel.customer.referral.tab') }}</Button>
          <Button :type="view === 'commission' ? 'primary' : 'text'" @click="router.push({ path: '/account/referrals', query: { tab: 'commission' } })">{{ $t('salesChannel.customer.commission.tab') }}</Button>
          <Button :loading="loading" data-testid="customer-sales-refresh" @click="load">{{ $t('salesChannel.customer.refresh') }}</Button>
        </Space>
      </Card>

      <Alert v-if="failed" :message="$t('salesChannel.customer.loadFailed')" show-icon type="error" />
      <Skeleton v-else-if="loading" active :paragraph="{ rows: 8 }" />

      <template v-else-if="view === 'referral' && referral">
        <Card :title="$t('salesChannel.customer.referral.linkTitle')">
          <p class="lead">{{ $t('salesChannel.customer.referral.linkLead') }}</p>
          <Space.Compact block>
            <Input :value="referral.referralUrl" readonly data-testid="customer-referral-url" />
            <Button type="primary" data-testid="customer-referral-copy" @click="copyReferralLink">{{ $t('salesChannel.customer.referral.copy') }}</Button>
          </Space.Compact>
          <div class="facts">
            <div><span>{{ $t('salesChannel.customer.referral.code') }}</span><strong>{{ referral.referralCode }}</strong></div>
            <div><span>{{ $t('salesChannel.customer.referral.policy') }}</span><strong>{{ statusLabel(referral.attributionPolicyCode) }}</strong></div>
            <div><span>{{ $t('salesChannel.customer.referral.expires') }}</span><strong>{{ formatTime(referral.attributionExpiresAt) }}</strong></div>
          </div>
        </Card>
        <div class="metric-grid">
          <Card><span>{{ $t('salesChannel.customer.referral.referred') }}</span><strong>{{ referral.referredCustomerCount }}</strong></Card>
          <Card><span>{{ $t('salesChannel.customer.referral.attributed') }}</span><strong>{{ referral.attributedOrderCount }}</strong></Card>
          <Card><span>{{ $t('salesChannel.customer.referral.qualified') }}</span><strong>{{ referral.qualifiedOrderCount }}</strong></Card>
          <Card><span>{{ $t('salesChannel.customer.referral.rejected') }}</span><strong>{{ referral.rejectedAttributionCount }}</strong></Card>
        </div>
      </template>

      <template v-else-if="view === 'commission'">
        <div class="metric-grid">
          <Card v-for="account in accounts" :key="account.currencyCode" :data-testid="`customer-commission-${account.currencyCode}`">
            <div class="row-heading"><span>{{ $t('salesChannel.customer.commission.available') }}</span><Tag>{{ account.currencyCode }}</Tag></div>
            <strong class="large-amount">{{ amount(account.availableAmountMinor, account.currencyCode) }}</strong>
            <div class="facts"><div><span>{{ $t('salesChannel.customer.commission.pending') }}</span><strong>{{ amount(account.pendingAmountMinor, account.currencyCode) }}</strong></div><div><span>{{ $t('salesChannel.customer.commission.matured') }}</span><strong>{{ amount(account.maturedAmountMinor, account.currencyCode) }}</strong></div><div><span>{{ $t('salesChannel.customer.commission.frozen') }}</span><strong>{{ amount(account.frozenAmountMinor, account.currencyCode) }}</strong></div></div>
          </Card>
        </div>
        <Card :title="$t('salesChannel.customer.commission.settlementsTitle')">
          <Empty v-if="!settlements.length" :description="$t('salesChannel.customer.commission.emptySettlements')" />
          <div v-else class="settlement-list"><article v-for="item in settlements" :key="item.settlementNo" class="settlement-item"><div><strong>{{ item.settlementNo }}</strong><small>{{ formatTime(item.submittedAt) }}</small></div><div>{{ amount(item.amountMinor, item.currencyCode) }}</div><Tag>{{ statusLabel(item.statusCode) }}</Tag></article></div>
        </Card>
        <Card :title="$t('salesChannel.customer.commission.entriesTitle')">
          <Empty v-if="!entries.length" :description="$t('salesChannel.customer.commission.emptyEntries')" />
          <div v-else class="settlement-list"><article v-for="item in entries" :key="item.entryNo" class="settlement-item"><div><strong>{{ item.entryNo }}</strong><small>{{ formatTime(item.createdAt) }}</small></div><div>{{ amount(item.amountMinor, item.currencyCode) }}</div><Tag>{{ statusLabel(item.statusCode) }}</Tag></article></div>
        </Card>
      </template>
    </div>
  </Page>
</template>

<style scoped>
.sales-customer-page { display: grid; gap: 16px; max-width: 100%; margin: 0 auto; }
.sales-toolbar :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.sales-toolbar strong { display: block; margin-bottom: 6px; font-size: 18px; }
.sales-toolbar p, .lead { margin: 0; color: hsl(var(--muted-foreground)); }
.metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; }
.metric-grid :deep(.ant-card-body) { display: grid; gap: 8px; }
.metric-grid strong, .large-amount { font-size: 28px; }
.row-heading, .facts > div, .settlement-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.facts { display: grid; gap: 10px; margin-top: 16px; }
.facts strong { text-align: right; }
.settlement-list { display: grid; gap: 10px; }
.settlement-item { padding: 12px; border: 1px solid hsl(var(--border)); border-radius: 10px; }
.settlement-item > div:first-child { display: grid; gap: 4px; }
.settlement-item small { color: hsl(var(--muted-foreground)); }
@media (max-width: 720px) { .sales-toolbar { align-items: stretch; flex-direction: column; } .settlement-item { align-items: flex-start; flex-direction: column; } }
</style>
