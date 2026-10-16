<script setup lang="ts">
import { Alert, Card, Descriptions, DescriptionsItem } from "ant-design-vue";
import { channelText as t } from "../channel-i18n";
const endpoints = [
 ["GET", "/channel/capabilities", "capability.read"],
 ["GET", "/channel/catalog?limit=24", "catalog.read"],
 ["GET", "/channel/catalog/products?page=1&pageSize=24", "catalog.read"],
 ["GET", "/channel/catalog/products/{publicSlug}/snapshot", "catalog.read"],
 ["GET", "/channel/catalog/products/{publicSlug}/configuration", "catalog.read"],
 ["GET", "/channel/catalog/products/{publicSlug}/configuration/evaluate", "catalog.read"],
 ["GET", "/channel/catalog/products/{publicSlug}/billing-terms", "catalog.read"],
 ["POST", "/channel/quotes", "catalog.read"],
 ["POST", "/channel/orders", "order.create"],
 ["GET", "/channel/orders/{id}", "order.read"],
 ["GET", "/channel/orders/by-request/{requestId}", "order.read"],
 ["GET", "/channel/services?orderNo=ORDER_NO&limit=100", "service.read"],
 ["GET", "/channel/services/{serviceNo}/operations/{operationNo}", "service.read"],
 ["GET", "/channel/finance", "settlement.read"],
 ["POST", "/channel/services/{serviceNo}/operations/renew", "service.renew"],
];
const example = `curl --user "$APP_ID:$APP_SECRET" \\
  "$SITE_URL/x/idc-channel/api/v1/channel/catalog?limit=24"`;
const purchase = JSON.stringify({ requestId: "shop-quote-20261010-0001", purchase: { publicSlug: "PRODUCT_SLUG", configurationRevision: "FROM_CONFIGURATION", billingTermToken: "FROM_QUOTE", currencyCode: "CNY", purchaseCycles: 1, quantity: 1, values: [] } }, null, 2);
const order = JSON.stringify({ requestId: "shop-order-20261010-0001", quoteNo: "QUOTE_NO_FROM_RESPONSE" }, null, 2);
const renewal = JSON.stringify({ requestId: "shop-renewal-20261010-0001", parameters: { renewalCycles: 1 }, confirmed: true }, null, 2);
</script>
<template>
 <div class="grid gap-4">
  <Alert type="info" show-icon :message="t('open.guideIntro')" :description="t('open.https')" />
  <Card :title="t('open.connectTitle')"><p>{{ t('open.connectText') }}</p><pre class="mt-3 overflow-auto rounded bg-muted p-4 text-sm">{{ example }}</pre><p class="mt-3">{{ t('open.response') }}</p></Card>
  <Card :title="t('open.purchaseTitle')"><p>{{ t('open.purchaseText') }}</p><p class="mt-3">{{ t('open.configuration') }}</p><code class="mt-2 block break-all">/x/idc-channel/api/v1/channel/catalog/products/{publicSlug}/configuration</code><code class="mt-2 block break-all">GET /x/idc-channel/api/v1/channel/catalog/products/{publicSlug}/billing-terms</code><p class="mt-3">{{ t('open.quoteNote') }}</p><pre class="mt-3 overflow-auto rounded bg-muted p-4 text-sm">{{ purchase }}</pre><code>POST /channel/orders</code><pre class="mt-3 overflow-auto rounded bg-muted p-4 text-sm">{{ order }}</pre></Card>
  <Card :title="t('open.renewTitle')"><p>{{ t('open.renewText') }}</p><pre class="mt-3 overflow-auto rounded bg-muted p-4 text-sm">{{ renewal }}</pre></Card>
  <Card :title="t('open.endpoints')"><p class="mb-3">{{ t('open.pagination') }}</p><p class="mb-3"><code>/x/idc-channel/api/v1</code></p><Descriptions bordered :column="1"><DescriptionsItem v-for="item in endpoints" :key="item[1]" :label="item[0]"><code class="break-all">{{ item[1] }}</code><p>{{ t('open.permission') }}: <code>{{ item[2] }}</code></p></DescriptionsItem></Descriptions></Card>
  <Alert type="warning" show-icon :message="t('open.retryTitle')" :description="t('open.retryText')" />
 </div>
</template>
