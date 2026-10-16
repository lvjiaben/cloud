<script lang="ts">
export const pluginPageMeta = { routePath: "/account/plugins/idc-core/entitlement-upgrades", title: "plugin.idc-core.upgrades.title" };
</script>
<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Card, Empty, Space, Tag, message } from "ant-design-vue";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import AssistedQuoteItem from "./components/assisted-quote-item.vue";
import { minorToMajorAmount } from "./catalog-money";

type Target = { entitlementNo: string; publicSlug: string; nameZhCN: string; nameEnUS: string; currencyCode: string };
type Upgrade = { upgradeNo: string; entitlementNo: string; currencyCode: string; statusCode: string; invoiceNo: string; sourcePeriodMinor: string; targetPeriodMinor: string; netMinor: string; taxMinor: string; totalMinor: string; expiresAt: number };
type Cart = { version: number; items: { cartItemNo: string; version: number }[] };
const targets = ref<Target[]>([]);
const items = ref<Upgrade[]>([]);
const selected = ref<Target>();
const upgrade = ref<Upgrade>();
const item = ref<InstanceType<typeof AssistedQuoteItem>>();
const loading = ref(false);
const busy = ref(false);
const failed = ref(false);
const actionFailed = ref(false);
const cartBlocked = ref(false);
let cartItemNo = "";
let quoteNo = "";
let frozenInput: Awaited<ReturnType<InstanceType<typeof AssistedQuoteItem>["getInput"]>> | undefined;
let frozenItemNo = "";
let configurationFrozen = false;
let quoteKey = "";
const api = (path: string) => pluginApiPath("idc-core", `account/${path}`);
const tr = (key: string) => $t(`plugin.idc-core.upgrades.${key}`);
const offers = computed(() => selected.value ? [{ ...selected.value, currencies: [selected.value.currencyCode], availability: "available" }] : []);
const awaitingPayment = computed(() => upgrade.value?.statusCode === "pending_payment");
function name(target: Target) { return preferences.app.locale === "zh-CN" ? target.nameZhCN || target.nameEnUS : target.nameEnUS || target.nameZhCN; }
function amount(value: string, currency: string) { return `${currency} ${minorToMajorAmount(value, currency)}`; }
async function load() {
  loading.value = true; failed.value = false;
  try {
    const result = await requestClient.get<{ targets: Target[]; items: Upgrade[] }>(api("entitlement-upgrades"));
    targets.value = result.targets; items.value = result.items;
  } catch { failed.value = true; } finally { loading.value = false; }
}
async function open(target: Target) {
  selected.value = target; upgrade.value = undefined;
  cartItemNo = ""; quoteNo = ""; frozenItemNo = ""; frozenInput = undefined; configurationFrozen = false; quoteKey = crypto.randomUUID();
  actionFailed.value = false; cartBlocked.value = false;
  modalApi.open();
}
// Remove only this dialog's exact cart item with fresh optimistic versions.
// Existing customer purchase intents are never removed or replaced.
async function removeOwnCartItem() {
  if (!cartItemNo) return;
  const result = await requestClient.get<{ cart: Cart | null }>(api("commerce/cart"));
  const row = result.cart?.items.find(value => value.cartItemNo === cartItemNo);
  if (row && result.cart) await requestClient.delete(api(`commerce/cart-items/${encodeURIComponent(row.cartItemNo)}`), { data: { expectedCartVersion: result.cart.version, expectedItemVersion: row.version } });
  cartItemNo = "";
}
async function preview() {
  if (busy.value || !selected.value) return;
  busy.value = true; actionFailed.value = false; cartBlocked.value = false;
  try {
    if (!quoteNo) {
      const input = frozenInput || await item.value!.getInput();
      // One service is upgraded, and the backend verifies the original billing period.
      if (input.quantity !== 1) { message.warning(tr("oneService")); return; }
      const opened = await requestClient.post<{ cart: Cart }>(api("commerce/carts"), { currencyCode: selected.value.currencyCode });
      let cart = opened.cart;
      if (cart.items.some(row => row.cartItemNo !== cartItemNo)) { cartBlocked.value = true; return; }
      frozenInput = input;
      if (!cartItemNo) {
        const added = await requestClient.post<{ cart: Cart }>(api("commerce/cart-items"), { ...input, currencyCode: selected.value.currencyCode, expectedCartVersion: cart.version });
        cart = added.cart;
        cartItemNo = cart.items[0]!.cartItemNo;
      }
      const frozen = await requestClient.post<{ quote: { quoteNo: string; items: { quoteItemNo: string }[] } }>(api("commerce/quotes"), { expectedCartVersion: cart.version });
      quoteNo = frozen.quote.quoteNo;
      frozenItemNo = frozen.quote.items[0]!.quoteItemNo;
    }
    if (!configurationFrozen) {
      await requestClient.post(api(`commerce/quote-items/${encodeURIComponent(frozenItemNo)}/purchase-configuration`), {
        publicSlug: frozenInput!.publicSlug, configurationRevision: frozenInput!.configurationRevision, values: frozenInput!.values,
      });
      configurationFrozen = true;
    }
    const result = await requestClient.post<{ upgrade: Upgrade }>(api(`entitlements/${encodeURIComponent(selected.value.entitlementNo)}/upgrades`), { quoteNo, idempotencyKey: quoteKey });
    upgrade.value = result.upgrade;
    await removeOwnCartItem();
  } catch { actionFailed.value = true; } finally { busy.value = false; }
}
async function pay() {
  if (!upgrade.value || busy.value) return;
  busy.value = true; actionFailed.value = false;
  try {
    if (upgrade.value.statusCode === "quoted") {
      const confirmed = await requestClient.post<{ upgrade: Upgrade }>(api(`entitlement-upgrades/${encodeURIComponent(upgrade.value.upgradeNo)}/confirmations`));
      upgrade.value = confirmed.upgrade;
    }
    if (upgrade.value.statusCode === "pending_payment") {
      const paid = await requestClient.post<{ upgrade: Upgrade }>(api(`entitlement-upgrades/${encodeURIComponent(upgrade.value.upgradeNo)}/balance-payments`));
      upgrade.value = paid.upgrade;
    }
    message.success(tr("submitted"));
    busy.value = false;
    modalApi.close(); await load();
  } catch { actionFailed.value = true; } finally { busy.value = false; }
}
async function retryConfiguration() {
  if (busy.value) return;
  busy.value = true;
  try { await removeOwnCartItem(); quoteNo = ""; frozenItemNo = ""; frozenInput = undefined; configurationFrozen = false; quoteKey = crypto.randomUUID(); upgrade.value = undefined; actionFailed.value = false; }
  catch { actionFailed.value = true; } finally { busy.value = false; }
}
const [Modal, modalApi] = useVbenModal({
  showConfirmButton: false,
  async onBeforeClose() {
    if (busy.value) return false;
    try { await removeOwnCartItem(); return true; } catch { actionFailed.value = true; return false; }
  },
});
onMounted(load);
</script>
<template>
  <Page :title="tr('title')" :loading="loading">
    <Alert class="mb-4" type="info" show-icon :message="tr('hint')" />
    <Alert v-if="failed" class="mb-4" type="error" show-icon :message="tr('failed')" />
    <Button class="mb-4" :loading="loading" @click="load">{{ tr('refresh') }}</Button>
    <Empty v-if="!loading && !targets.length" :description="tr('empty')" />
    <div class="grid max-w-5xl gap-4 md:grid-cols-2">
      <Card v-for="target in targets" :key="target.entitlementNo" :title="name(target)">
        <p>{{ target.entitlementNo }} · {{ target.currencyCode }}</p>
        <Button type="primary" data-testid="upgrade-open" @click="open(target)">{{ tr('start') }}</Button>
      </Card>
    </div>
    <Card v-if="items.length" class="mt-4 max-w-5xl" :title="tr('history')">
      <div v-for="row in items" :key="row.upgradeNo" class="flex flex-wrap items-center gap-3 border-b py-3">
        <span>{{ row.entitlementNo }}</span><Tag>{{ tr(`status.${row.statusCode}`) }}</Tag>
        <span>{{ amount(row.totalMinor, row.currencyCode) }}</span>
        <Button v-if="row.statusCode === 'pending_payment'" @click="upgrade = row; selected = undefined; modalApi.open()">{{ tr('continuePay') }}</Button>
      </div>
    </Card>
    <Modal :title="tr('title')" class="w-[720px] max-w-full">
      <Alert v-if="actionFailed" class="mb-4" type="error" show-icon :message="tr('failed')" />
      <Alert v-if="cartBlocked" class="mb-4" type="warning" show-icon :message="tr('cartHint')" />
      <template v-if="!upgrade && selected">
        <AssistedQuoteItem :key="selected.entitlementNo" ref="item" :index="0" :currency="selected.currencyCode" :busy="busy || !!cartItemNo" :allowed-products="offers" :removable="false" />
        <Space class="mt-4"><Button type="primary" :loading="busy" data-testid="upgrade-preview" @click="preview">{{ tr('viewFee') }}</Button><Button v-if="cartItemNo" :disabled="busy" @click="retryConfiguration">{{ tr('back') }}</Button></Space>
      </template>
      <div v-if="upgrade" class="space-y-2" data-testid="upgrade-summary">
        <p>{{ tr('currentPrice') }}：{{ amount(upgrade.sourcePeriodMinor, upgrade.currencyCode) }}</p>
        <p>{{ tr('nextPrice') }}：{{ amount(upgrade.targetPeriodMinor, upgrade.currencyCode) }}</p>
        <p>{{ tr('tax') }}：{{ amount(upgrade.taxMinor, upgrade.currencyCode) }}</p>
        <p class="text-xl font-semibold">{{ tr('changeFee') }}：{{ amount(upgrade.totalMinor, upgrade.currencyCode) }}</p>
        <p v-if="upgrade.invoiceNo">{{ tr('invoice') }}：{{ upgrade.invoiceNo }}</p>
        <Alert class="mb-4" type="info" show-icon :message="tr('confirmHint')" />
        <Space><Button type="primary" :loading="busy" data-testid="upgrade-pay" @click="pay">{{ tr('confirmChange') }}</Button><Button v-if="!awaitingPayment" :disabled="busy" @click="retryConfiguration">{{ tr('back') }}</Button></Space>
      </div>
    </Modal>
  </Page>
</template>
