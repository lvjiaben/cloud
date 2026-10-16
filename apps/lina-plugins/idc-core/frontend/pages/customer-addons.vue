<script lang="ts">
export const pluginPageMeta = { routePath: "/account/plugins/idc-core/addons", title: "plugin.idc-core.addons.title" };
</script>
<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Page } from "@vben/common-ui";
import { Alert, Button, Card, Empty, Popconfirm, Select, Space, message } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import AssistedQuoteItem from "./components/assisted-quote-item.vue";
import { assistedProducts } from "./assisted-purchase-client";
import { minorToMajorAmount } from "./catalog-money";
type Offer = { publicSlug: string; nameZhCN: string; nameEnUS: string };
type Target = { entitlementNo: string; productNameZhCN: string; productNameEnUS: string; addons: Offer[] };
type Cart = { version: number; items: { cartItemNo: string; productNameZhCN: string; productNameEnUS: string; version: number }[] };
type Quote = { quoteNo: string; currencyCode: string; subtotalMinor: string; taxMinor: string; totalAmountMinor: string; expiresAt: number; items: { quoteItemNo: string }[] };
type Checkout = { invoiceNo: string; orderNo: string; statusCode: string; totalAmountMinor: string };
const targets = ref<Target[]>([]);
const target = ref("");
const currency = ref("");
const currencies = ref<string[]>([]);
const cart = ref<Cart>();
const quote = ref<Quote>();
const checkout = ref<Checkout>();
const busy = ref(false);
const loading = ref(true);
const failed = ref(false);
const paid = ref(false);
const added = ref(false);
const item = ref<InstanceType<typeof AssistedQuoteItem>>();
let checkoutKey = crypto.randomUUID();
let paymentKey = crypto.randomUUID();
let ownCartItemNo = "";
let pendingQuote: Quote | undefined;
let purchaseInput: Awaited<ReturnType<InstanceType<typeof AssistedQuoteItem>["getInput"]>> | undefined;
const api = (path: string) => pluginApiPath("idc-core", `account/${path}`);
const tr = (key: string) => $t(`plugin.idc-core.addons.${key}`);
const selected = computed(() => targets.value.find(row => row.entitlementNo === target.value));
const offers = computed(() => (selected.value?.addons || []).map(offer => ({ ...offer, currencies: [currency.value], availability: "available" })));
function name(value: { nameZhCN: string; nameEnUS: string }) { return preferences.app.locale === "zh-CN" ? value.nameZhCN || value.nameEnUS : value.nameEnUS || value.nameZhCN; }
function targetName(value: Target) { return `${name({ nameZhCN: value.productNameZhCN, nameEnUS: value.productNameEnUS })} · ${value.entitlementNo}`; }
function amount(value: string) { return `${currency.value} ${minorToMajorAmount(value, currency.value)}`; }
async function load() {
  loading.value = true; failed.value = false;
  try {
    const [result, catalog, current] = await Promise.all([
      requestClient.get<{ targets: Target[] }>(api("commerce/addon-targets")),
      assistedProducts({}), requestClient.get<{ cart: Cart | null }>(api("commerce/cart")),
    ]);
    targets.value = result.targets;
    currencies.value = catalog.availableCurrencies;
    currency.value ||= currencies.value[0] || "";
    target.value = targets.value.some(row => row.entitlementNo === target.value) ? target.value : targets.value[0]?.entitlementNo || "";
    cart.value = current.cart || undefined;
  } catch { failed.value = true; } finally { loading.value = false; }
}
async function remove(cartItem: Cart["items"][number]) {
  if (!cart.value || busy.value) return;
  busy.value = true;
  try {
    const result = await requestClient.delete<{ cart: Cart }>(api(`commerce/cart-items/${cartItem.cartItemNo}`), { data: { expectedCartVersion: cart.value.version, expectedItemVersion: cartItem.version } });
    cart.value = result.cart;
  } finally { busy.value = false; }
}
async function createQuote() {
  if (busy.value || !selected.value || !currency.value) return;
  busy.value = true; failed.value = false;
  try {
    if (!added.value) {
      const input = await item.value!.getInput();
      const opened = await requestClient.post<{ cart: Cart }>(api("commerce/carts"), { currencyCode: currency.value });
      cart.value = opened.cart;
      if (cart.value.items.length) { message.warning(tr("cartHint")); return; }
      const result = await requestClient.post<{ cart: Cart }>(api("commerce/cart-items"), { ...input, currencyCode: currency.value, expectedCartVersion: cart.value.version });
      cart.value = result.cart; added.value = true;
      ownCartItemNo = result.cart.items[0]!.cartItemNo;
      purchaseInput = input;
    }
    if (!pendingQuote) {
      const result = await requestClient.post<{ quote: Quote }>(api("commerce/quotes"), { expectedCartVersion: cart.value!.version, targetEntitlementNo: target.value });
      pendingQuote = result.quote;
    }
    // Freeze the same customer choices before checkout; retry the exact quote
    // if the configuration response is lost instead of creating another intent.
    await requestClient.post(api(`commerce/quote-items/${encodeURIComponent(pendingQuote.items[0]!.quoteItemNo)}/purchase-configuration`), {
      publicSlug: purchaseInput!.publicSlug, configurationRevision: purchaseInput!.configurationRevision, values: purchaseInput!.values,
    });
    quote.value = pendingQuote;
  } catch { failed.value = true; } finally { busy.value = false; }
}
async function pay() {
  if (!quote.value || busy.value) return;
  busy.value = true; failed.value = false;
  try {
    if (!checkout.value) {
      const result = await requestClient.post<{ checkout: Checkout }>(api("commerce/orders"), { quoteNo: quote.value.quoteNo, idempotencyKey: checkoutKey });
      checkout.value = result.checkout;
    }
    if (checkout.value.statusCode !== "paid" && checkout.value.totalAmountMinor !== "0") {
      await requestClient.post(api(`billing/invoices/${checkout.value.invoiceNo}/balance-payments`), { idempotencyKey: paymentKey });
    }
    paid.value = true;
    message.success(tr("paid"));
  } catch { failed.value = true; } finally { busy.value = false; }
}
async function reset() {
  if (busy.value || (checkout.value && !paid.value)) return;
  if (added.value && !checkout.value && cart.value) {
    const current = await requestClient.get<{ cart: Cart | null }>(api("commerce/cart"));
    cart.value = current.cart || undefined;
    const own = cart.value?.items.find(row => row.cartItemNo === ownCartItemNo);
    if (own) await remove(own);
  }
  quote.value = undefined; checkout.value = undefined; added.value = false; paid.value = false;
  ownCartItemNo = ""; pendingQuote = undefined; purchaseInput = undefined;
  checkoutKey = crypto.randomUUID(); paymentKey = crypto.randomUUID();
  await load();
}
onMounted(load);
</script>
<template>
  <Page :title="tr('title')" :loading="loading">
    <Alert class="mb-4" type="info" show-icon :message="tr('hint')" />
    <Alert v-if="failed" class="mb-4" type="error" show-icon :message="tr('failed')" />
    <Empty v-if="!loading && !targets.length" :description="tr('empty')"><Button @click="load">{{ tr('refresh') }}</Button></Empty>
    <div v-else class="mx-auto grid max-w-5xl gap-4">
      <Card :title="tr('chooseService')">
        <div class="grid gap-4 md:grid-cols-2">
          <label>{{ tr('service') }}<Select v-model:value="target" class="w-full" :disabled="busy || added" :options="targets.map(row => ({ value: row.entitlementNo, label: targetName(row) }))" data-testid="addon-target" /></label>
          <label>{{ tr('currency') }}<Select v-model:value="currency" class="w-full" :disabled="busy || added" :options="currencies.map(value => ({ value, label: value }))" data-testid="addon-currency" /></label>
        </div>
      </Card>
      <Card v-if="cart?.items.length && !added" :title="tr('cartTitle')">
        <Alert class="mb-3" type="warning" :message="tr('cartHint')" />
        <div v-for="row in cart.items" :key="row.cartItemNo" class="flex items-center justify-between gap-4 p-2">
          <span>{{ name({ nameZhCN: row.productNameZhCN, nameEnUS: row.productNameEnUS }) }}</span>
          <Popconfirm :title="tr('removeConfirm')" @confirm="remove(row)"><Button danger :disabled="busy">{{ tr('remove') }}</Button></Popconfirm>
        </div>
      </Card>
      <AssistedQuoteItem v-if="selected && currency && !quote" :key="`${target}:${currency}`" ref="item" :index="0" :currency="currency" :busy="busy || added" :allowed-products="offers" :removable="false" />
      <Space v-if="!quote"><Button type="primary" :loading="busy" :disabled="!selected || !currency || (!added && !!cart?.items.length)" data-testid="addon-quote" @click="createQuote">{{ tr('quote') }}</Button><Button v-if="added" :disabled="busy" @click="reset">{{ tr('reset') }}</Button></Space>
      <Card v-if="quote" :title="tr('quoteTitle')" data-testid="addon-quote-summary">
        <p>{{ tr('service') }}：{{ selected ? targetName(selected) : target }}</p>
        <p class="text-xl font-semibold">{{ tr('total') }}：{{ amount(quote.totalAmountMinor) }}</p>
        <Alert class="mb-4" type="info" :message="tr('paymentHint')" show-icon />
        <p v-if="checkout">{{ tr('order') }}：{{ checkout.orderNo }} · {{ tr('invoice') }}：{{ checkout.invoiceNo }}</p>
        <Alert v-if="paid" type="success" show-icon :message="tr('paid')" data-testid="addon-paid" />
        <Space v-else><Button type="primary" :loading="busy" data-testid="addon-pay" @click="pay">{{ tr('pay') }}</Button><Button v-if="!checkout" :disabled="busy" @click="reset">{{ tr('reset') }}</Button></Space>
        <Button v-if="paid" class="mt-4" @click="reset">{{ tr('another') }}</Button>
      </Card>
    </div>
  </Page>
</template>
