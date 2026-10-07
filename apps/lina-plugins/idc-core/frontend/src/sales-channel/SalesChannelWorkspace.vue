<script setup lang="ts">
import { onMounted, ref } from 'vue';
import type { PluginHostMountContext } from '../host-context';
import { createTranslator } from '../runtime/i18n';
import { createSalesChannelApi, type SalesChannelOverview } from './api';
import { salesChannelMessages } from './messages';

const props = defineProps<{ context: PluginHostMountContext }>();
const tr = createTranslator(() => props.context, salesChannelMessages);
const api = createSalesChannelApi(props.context);
const data = ref<SalesChannelOverview>();
const loading = ref(false);
const failed = ref(false);

async function load() {
  loading.value = true; failed.value = false;
  try { data.value = await api.overview(); } catch { failed.value = true; } finally { loading.value = false; }
}
function amount(value: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency, minimumFractionDigits: 2 }).format(value / 100);
}
onMounted(load);
</script>

<template>
  <main class="sales-page" data-testid="sales-channel-workspace">
    <header class="sales-header">
      <div><p class="sales-eyebrow">{{ tr('sales.eyebrow') }}</p><h1>{{ tr('sales.title') }}</h1><p class="sales-subtitle">{{ tr('sales.subtitle') }}</p></div>
      <button class="sales-refresh" :disabled="loading" data-testid="sales-refresh" @click="load">{{ loading ? '…' : tr('sales.refresh') }}</button>
    </header>
    <div v-if="failed" class="sales-notice error">{{ tr('sales.failed') }}</div>
    <template v-else-if="data">
      <section class="sales-grid">
        <article class="sales-card"><span>{{ tr('sales.qualifications') }}</span><strong>{{ data.qualifications }}</strong></article>
        <article class="sales-card"><span>{{ tr('sales.links') }}</span><strong>{{ data.affiliateLinks }}</strong></article>
        <article class="sales-card"><span>{{ tr('sales.attributions') }}</span><strong>{{ data.attributions }}</strong></article>
        <article class="sales-card"><span>{{ tr('sales.maturity') }}</span><strong>{{ data.pendingMaturity }}</strong></article>
        <article class="sales-card"><span>{{ tr('sales.settlement') }}</span><strong>{{ data.pendingSettlement }}</strong></article>
      </section>
      <section class="sales-panel"><h2>{{ tr('sales.currency') }}</h2><div v-if="data.currencies.length" class="currency-grid"><article v-for="item in data.currencies" :key="item.currencyCode" class="currency-card"><div class="currency-name">{{ item.currencyCode }}</div><dl><div><dt>{{ tr('sales.accrued') }}</dt><dd>{{ amount(item.accruedMinor, item.currencyCode) }}</dd></div><div><dt>{{ tr('sales.matured') }}</dt><dd>{{ amount(item.maturedMinor, item.currencyCode) }}</dd></div></dl></article></div><p v-else class="sales-empty">{{ tr('sales.empty') }}</p></section>
    </template>
  </main>
</template>
