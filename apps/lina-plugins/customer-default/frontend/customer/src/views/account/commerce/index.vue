<script setup lang="ts">
import type {
  Cart,
  FinalQuote,
  OrderDetail,
  OrderSummary,
  PaymentPreparation,
} from "#/api/idc/customer-commerce";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import { Page } from "@vben/common-ui";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  message,
  Modal,
  Select,
  Skeleton,
  Space,
  Tag,
} from "ant-design-vue";
import dayjs from "dayjs";

import {
  cancelOrder,
  checkoutQuote,
  createFinalQuote,
  getCart,
  getOrder,
  listOrders,
  mergeGuestCart,
  persistentCommerceActionKey,
  preparePayment,
  removeCartItem,
  resumePaymentPreparation,
} from "#/api/idc/customer-commerce";
import { $t } from "#/locales";

import {
  hasPendingQuoteConfiguration,
  quoteConfigurationAction,
} from "./customer-commerce-configuration-state";
import {
  customerDeliveryRows,
  customerFulfillmentColor,
  customerFulfillmentLabelKey,
  customerServiceLabelKey,
  managementPreparationLabelKey,
} from "./customer-fulfillment-state";

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const busy = ref(false);
const failed = ref(false);
const cart = ref<Cart>();
const quote = ref<FinalQuote>();
const quoteFingerprint = ref("");
const preparation = ref<PaymentPreparation>();
const createdOrderNo = ref("");
const orders = ref<OrderSummary[]>([]);
const detail = ref<OrderDetail>();
const detailRefreshFailed = ref(false);
let detailSequence = 0;
let detailRefreshCount = 0;
let detailRefreshTimer: ReturnType<typeof setTimeout> | undefined;
const deliveryRows = computed(() => customerDeliveryRows(detail.value));
const nextCursor = ref("");
const cursor = ref("");
const cursorHistory = ref<string[]>([]);
const statusCode = ref("");
const isEnglish = computed(
  () => document.documentElement.lang.toLowerCase() === "en-us",
);

const isCart = computed(() => route.name === "CustomerCart");
const orderNo = computed(() =>
  typeof route.params.orderNo === "string" ? route.params.orderNo : "",
);
const isDetail = computed(() => Boolean(orderNo.value));
const pageTitle = computed(() =>
  isCart.value
    ? $t("commerce.cart.title")
    : isDetail.value
      ? $t("commerce.order.detailTitle", { orderNo: orderNo.value })
      : $t("commerce.orders.title"),
);
const paymentState = computed(
  () => preparation.value ?? detail.value?.paymentPreparation,
);
const paymentReady = computed(
  () =>
    preparation.value?.ready ??
    detail.value?.paymentPreparation?.statusCode === "ready",
);
const quoteChanged = computed(() => {
  if (!quote.value || !quoteFingerprint.value) return false;
  return quoteFingerprint.value !== fingerprint(quote.value);
});
const configurationPending = computed(() =>
  hasPendingQuoteConfiguration(quote.value?.items ?? []),
);

async function configureQuoteItem(item: FinalQuote["items"][number]) {
  if (!quote.value || !item.publicSlug) return;
  let target = item;
  if (quoteConfigurationAction(item) === "reconfigure") {
    await refreshQuote();
    const replacement = quote.value?.items.find(
      (candidate) => candidate.cartItemNo === item.cartItemNo,
    );
    if (!replacement || replacement.configurationFrozen) return;
    target = replacement;
  }
  await router.push({
    path: `/account/products/${encodeURIComponent(target.publicSlug)}/configure`,
    query: {
      currency: quote.value.currencyCode,
      quoteItemNo: target.quoteItemNo,
    },
  });
}

const statusOptions = computed(() => [
  { label: $t("commerce.status.all"), value: "" },
  ...[
    "pending_payment",
    "manual_review",
    "provisioning",
    "completed",
    "cancelled",
  ].map((value) => ({ label: statusLabel(value), value })),
]);

function statusLabel(value: unknown) {
  const code = String(value || "pending");
  const key = `commerce.status.${code}`;
  const translated = $t(key);
  return translated === key ? $t("commerce.status.unknown") : translated;
}

function statusColor(value: unknown) {
  const code = String(value || "");
  if (
    [
      "paid",
      "active",
      "completed",
      "consumed",
      "fulfilled",
      "ready",
      "succeeded",
    ].includes(code)
  ) {
    return "success";
  }
  if (
    ["cancelled", "expired", "rejected", "released", "terminated"].includes(
      code,
    )
  )
    return "default";
  if (["awaiting_identity", "manual_review", "suspended"].includes(code))
    return "warning";
  return "processing";
}

function localized(item: {
  productNameEnUS?: unknown;
  productNameZhCN?: unknown;
}) {
  const english = String(item.productNameEnUS || "");
  const chinese = String(item.productNameZhCN || "");
  return (isEnglish.value ? english : chinese) || chinese || english || "—";
}

function currencyDigits(currencyCode: string) {
  try {
    return (
      new Intl.NumberFormat("en", {
        currency: currencyCode,
        style: "currency",
      }).resolvedOptions().maximumFractionDigits ?? 2
    );
  } catch {
    return 2;
  }
}

function amount(value: unknown, currencyCode: string) {
  const raw = String(value ?? "");
  if (!/^-?\d+$/u.test(raw)) return "—";
  const negative = raw.startsWith("-");
  const digits = raw.replace(/^-/, "").replace(/^0+(?=\d)/, "") || "0";
  const scale = currencyDigits(currencyCode);
  if (scale === 0) return `${currencyCode} ${negative ? "-" : ""}${digits}`;
  const padded = digits.padStart(scale + 1, "0");
  return `${currencyCode} ${negative ? "-" : ""}${padded.slice(0, -scale)}.${padded.slice(-scale)}`;
}

function formatTime(value: unknown) {
  const timestamp = Number(value || 0);
  return timestamp > 0 ? dayjs(timestamp).format("YYYY-MM-DD HH:mm") : "—";
}

function fingerprint(value: FinalQuote) {
  return JSON.stringify({
    discountMinor: value.discountMinor,
    items: value.items.map((item) => ({
      adjustments: item.adjustments,
      cartItemNo: item.cartItemNo,
      subtotalMinor: item.subtotalMinor,
      taxAmountMinor: item.taxAmountMinor,
      totalAmountMinor: item.totalAmountMinor,
    })),
    subtotalMinor: value.subtotalMinor,
    taxAmountMinor: value.taxAmountMinor,
    totalAmountMinor: value.totalAmountMinor,
  });
}

function identityLocation(targetOrderNo: string, contextNo = "") {
  return {
    path: "/account/identity",
    query: {
      payment_context: contextNo || undefined,
      return_to: `/account/orders/${encodeURIComponent(targetOrderNo)}`,
    },
  };
}

async function loadCart() {
  loading.value = true;
  failed.value = false;
  try {
    try {
      await mergeGuestCart();
    } catch {
      message.warning($t("commerce.messages.mergeFailed"));
    }
    cart.value = (await getCart()).cart || undefined;
    quote.value = cart.value?.latestQuote;
    quoteFingerprint.value = quote.value ? fingerprint(quote.value) : "";
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadOrders(targetCursor = "") {
  loading.value = true;
  failed.value = false;
  try {
    const result = await listOrders({
      cursor: targetCursor || undefined,
      statusCode: statusCode.value || undefined,
    });
    orders.value = result.orders || [];
    nextCursor.value = result.nextCursor || "";
    cursor.value = targetCursor;
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadDetail(quiet = false) {
  if (!orderNo.value) return;
  if (detailRefreshTimer) clearTimeout(detailRefreshTimer);
  const sequence = ++detailSequence;
  const targetOrder = orderNo.value;
  if (!quiet) {
    loading.value = true;
    failed.value = false;
  }
  try {
    const response = (await getOrder(targetOrder)).order;
    if (sequence !== detailSequence || targetOrder !== orderNo.value) return;
    detail.value = response;
    detailRefreshFailed.value = false;
    const processing = response.deliveries.some(
      (item) =>
        item.statusCode === "processing" ||
        (item.statusCode === "awaiting_payment" &&
          response.invoice?.statusCode === "paid"),
    );
    if (processing && detailRefreshCount < 24 && !document.hidden) {
      detailRefreshTimer = setTimeout(() => {
        detailRefreshCount += 1;
        void loadDetail(true);
      }, 5000);
    }
  } catch {
    if (sequence !== detailSequence) return;
    if (quiet) detailRefreshFailed.value = true;
    else failed.value = true;
  } finally {
    if (sequence === detailSequence) loading.value = false;
  }
}

function loadCurrent() {
  if (detailRefreshTimer) clearTimeout(detailRefreshTimer);
  detailSequence += 1;
  detailRefreshCount = 0;
  detailRefreshFailed.value = false;
  preparation.value = undefined;
  failed.value = false;
  if (isCart.value) return loadCart();
  if (isDetail.value) return loadDetail();
  return loadOrders();
}

async function removeItem(itemNo: string, itemVersion: number) {
  if (!cart.value || busy.value) return;
  busy.value = true;
  try {
    cart.value = (
      await removeCartItem(itemNo, cart.value.version, itemVersion)
    ).cart;
    quote.value = undefined;
  } catch {
    message.error($t("commerce.messages.actionFailed"));
    await loadCart();
  } finally {
    busy.value = false;
  }
}

async function refreshQuote() {
  if (!cart.value?.items.length || busy.value) return;
  busy.value = true;
  if (quote.value) quoteFingerprint.value = fingerprint(quote.value);
  try {
    quote.value = (await createFinalQuote(cart.value.version)).quote;
    cart.value.version = quote.value.cartVersion;
  } catch {
    message.error($t("commerce.messages.quoteFailed"));
    await loadCart();
  } finally {
    busy.value = false;
  }
}

async function submitCheckout() {
  if (!quote.value || busy.value || configurationPending.value) return;
  busy.value = true;
  try {
    const result = await checkoutQuote(
      quote.value.quoteNo,
      persistentCommerceActionKey(`checkout.${quote.value.quoteNo}`),
    );
    createdOrderNo.value = result.checkout.orderNo;
    preparation.value = (
      await preparePayment(result.checkout.orderNo)
    ).preparation;
    message.success($t("commerce.messages.orderCreated"));
  } catch {
    message.error($t("commerce.messages.checkoutFailed"));
  } finally {
    busy.value = false;
  }
}

async function beginPayment() {
  if (!detail.value || busy.value) return;
  busy.value = true;
  try {
    preparation.value = (
      await preparePayment(detail.value.summary.orderNo)
    ).preparation;
    await loadDetail();
  } catch {
    message.error($t("commerce.messages.actionFailed"));
  } finally {
    busy.value = false;
  }
}

async function recheckPayment() {
  const contextNo = paymentState.value?.contextNo;
  if (!contextNo || busy.value) return;
  busy.value = true;
  try {
    preparation.value = (await resumePaymentPreparation(contextNo)).preparation;
    if (isDetail.value) await loadDetail();
  } catch {
    message.error($t("commerce.messages.actionFailed"));
  } finally {
    busy.value = false;
  }
}

function requestCancellation() {
  if (!detail.value || busy.value) return;
  Modal.confirm({
    cancelText: $t("commerce.actions.back"),
    content: $t("commerce.cancel.description"),
    okText: $t("commerce.cancel.confirm"),
    okType: "danger",
    title: $t("commerce.cancel.title"),
    async onOk() {
      if (!detail.value) return;
      busy.value = true;
      try {
        await cancelOrder(
          detail.value.summary.orderNo,
          persistentCommerceActionKey(`cancel.${detail.value.summary.orderNo}`),
        );
        message.success($t("commerce.messages.cancelled"));
        await loadDetail();
      } finally {
        busy.value = false;
      }
    },
  });
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(cursor.value);
  void loadOrders(nextCursor.value);
}

function previousPage() {
  void loadOrders(cursorHistory.value.pop() || "");
}

onMounted(async () => {
  await loadCurrent();
});
watch(() => route.fullPath, loadCurrent);
onBeforeUnmount(() => {
  detailSequence += 1;
  if (detailRefreshTimer) clearTimeout(detailRefreshTimer);
});
</script>

<template>
  <Page
    class="customer-inner-page"
    header-class="customer-inner-heading"
    content-class="customer-inner-content"
    :title="pageTitle"
  >
    <div class="commerce-page" data-testid="customer-commerce-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <p>
            {{ isCart ? $t("commerce.cart.lead") : $t("commerce.orders.lead") }}
          </p>
        </div>
        <Space wrap>
          <Button
            v-if="isDetail"
            :loading="loading"
            data-testid="commerce-detail-refresh"
            @click="loadCurrent"
            >{{ $t("commerce.order.refresh") }}</Button
          >
          <Button
            data-testid="commerce-cart-nav"
            :type="isCart ? 'primary' : 'default'"
            @click="router.push('/account/cart')"
            >{{ $t("commerce.cart.menu") }}</Button
          >
          <Button
            data-testid="commerce-orders-nav"
            :type="!isCart ? 'primary' : 'default'"
            @click="router.push('/account/orders')"
            >{{ $t("commerce.orders.menu") }}</Button
          >
        </Space>
      </Card>

      <Alert
        v-if="failed"
        class="section-gap"
        :message="$t('commerce.messages.loadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="loadCurrent">{{
            $t("commerce.actions.retry")
          }}</Button>
        </template>
      </Alert>
      <Skeleton v-if="loading" active :paragraph="{ rows: 8 }" />

      <template v-else-if="isCart && !failed">
        <Empty
          v-if="!cart?.items.length"
          :description="$t('commerce.cart.empty')"
        >
          <Button
            type="primary"
            @click="router.push('/account/products/catalog')"
            >{{ $t("commerce.cart.browse") }}</Button
          >
        </Empty>
        <div v-else class="cart-layout">
          <div class="stack">
            <Card
              v-for="item in cart.items"
              :key="item.cartItemNo"
              :data-testid="`commerce-cart-item-${item.cartItemNo}`"
            >
              <div class="row-heading">
                <div>
                  <h2>{{ localized(item) }}</h2>
                  <p>
                    {{ item.quantity }}
                    {{ $t("commerce.units.items") }} · {{ item.purchaseCycles }}
                    {{ $t("commerce.units.cycles") }}
                  </p>
                </div>
                <Tag :color="statusColor(item.statusCode)">{{
                  statusLabel(item.statusCode)
                }}</Tag>
              </div>
              <Button
                danger
                :disabled="busy"
                size="small"
                @click="removeItem(item.cartItemNo, item.version)"
                >{{ $t("commerce.actions.remove") }}</Button
              >
            </Card>
          </div>

          <Card class="quote-card">
            <h2>{{ $t("commerce.quote.title") }}</h2>
            <Alert
              v-if="quoteChanged"
              :message="$t('commerce.quote.changed')"
              show-icon
              type="warning"
            />
            <dl v-if="quote" class="amounts" data-testid="commerce-final-quote">
              <div>
                <dt>{{ $t("commerce.amount.subtotal") }}</dt>
                <dd>{{ amount(quote.subtotalMinor, quote.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t("commerce.amount.discount") }}</dt>
                <dd>-{{ amount(quote.discountMinor, quote.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t("commerce.amount.tax") }}</dt>
                <dd>{{ amount(quote.taxAmountMinor, quote.currencyCode) }}</dd>
              </div>
              <div class="total">
                <dt>{{ $t("commerce.amount.total") }}</dt>
                <dd>
                  {{ amount(quote.totalAmountMinor, quote.currencyCode) }}
                </dd>
              </div>
            </dl>
            <p v-if="quote" class="muted">
              {{ $t("commerce.quote.expires") }}
              {{ formatTime(quote.expiresAt) }}
            </p>
            <div
              v-if="quote?.items.some((item) => item.configurationRequired)"
              class="mb-3 space-y-2"
            >
              <div
                v-for="item in quote.items.filter(
                  (entry) => entry.configurationRequired,
                )"
                :key="item.quoteItemNo"
                class="flex items-center justify-between gap-2 rounded border p-2"
              >
                <span>{{ localized(item) }}</span>
                <Tag v-if="item.configurationFrozen" color="success">{{
                  $t("commerce.configuration.ready")
                }}</Tag>
                <Button
                  size="small"
                  :type="item.configurationFrozen ? 'default' : 'primary'"
                  @click="configureQuoteItem(item)"
                  >{{
                    item.configurationFrozen
                      ? $t("commerce.configuration.reconfigure")
                      : $t("commerce.configuration.configure")
                  }}</Button
                >
              </div>
              <Alert
                v-if="configurationPending"
                :message="$t('commerce.configuration.required')"
                show-icon
                type="warning"
              />
            </div>
            <Space direction="vertical" style="width: 100%">
              <Button
                block
                data-testid="commerce-primary-action"
                :disabled="Boolean(createdOrderNo) || configurationPending"
                :loading="busy"
                type="primary"
                @click="quote ? submitCheckout() : refreshQuote()"
                >{{
                  quote
                    ? $t("commerce.actions.checkout")
                    : $t("commerce.actions.quote")
                }}</Button
              >
              <Button
                v-if="quote && !createdOrderNo"
                block
                :disabled="busy"
                @click="refreshQuote"
                >{{ $t("commerce.actions.quoteAgain") }}</Button
              >
            </Space>
          </Card>
        </div>

        <Card
          v-if="createdOrderNo"
          class="section-gap"
          data-testid="commerce-created-order"
        >
          <h2>{{ $t("commerce.messages.orderCreated") }}</h2>
          <Button
            type="link"
            @click="router.push(`/account/orders/${createdOrderNo}`)"
            >{{ createdOrderNo }}</Button
          >
          <Alert
            class="section-gap"
            :description="
              paymentReady
                ? $t('commerce.payment.readyDescription')
                : $t('commerce.payment.identityDescription')
            "
            :message="
              paymentReady
                ? $t('commerce.payment.ready')
                : statusLabel(paymentState?.statusCode)
            "
            show-icon
            :type="paymentReady ? 'success' : 'warning'"
          />
          <Space wrap>
            <Button
              v-if="paymentState && !paymentReady"
              type="primary"
              @click="
                router.push(
                  identityLocation(createdOrderNo, paymentState.contextNo),
                )
              "
              >{{ $t("commerce.actions.verify") }}</Button
            >
            <Button
              v-if="paymentState"
              :loading="busy"
              @click="recheckPayment"
              >{{ $t("commerce.actions.recheck") }}</Button
            >
          </Space>
        </Card>
      </template>

      <template v-else-if="!isDetail && !failed">
        <Select
          v-model:value="statusCode"
          class="status-filter"
          :options="statusOptions"
          @change="
            cursorHistory = [];
            loadOrders();
          "
        />
        <Empty
          v-if="!orders.length"
          :description="$t('commerce.orders.empty')"
        />
        <div v-else class="order-grid">
          <Card
            v-for="item in orders"
            :key="item.orderNo"
            :data-testid="`commerce-order-${item.orderNo}`"
          >
            <div class="row-heading">
              <div>
                <small>{{ $t("commerce.order.label") }}</small>
                <h2>{{ item.orderNo }}</h2>
              </div>
              <Tag :color="statusColor(item.statusCode)">{{
                statusLabel(item.statusCode)
              }}</Tag>
            </div>
            <dl class="order-facts">
              <div>
                <dt>{{ $t("commerce.amount.total") }}</dt>
                <dd>{{ amount(item.totalAmountMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t("commerce.order.created") }}</dt>
                <dd>{{ formatTime(item.createdAt) }}</dd>
              </div>
              <div>
                <dt>{{ $t("commerce.order.due") }}</dt>
                <dd>{{ formatTime(item.paymentDueAt) }}</dd>
              </div>
            </dl>
            <Button
              type="primary"
              @click="router.push(`/account/orders/${item.orderNo}`)"
              >{{ $t("commerce.actions.detail") }}</Button
            >
          </Card>
        </div>
        <div class="pager">
          <Button :disabled="!cursorHistory.length" @click="previousPage">{{
            $t("commerce.actions.previous")
          }}</Button>
          <Button :disabled="!nextCursor" type="primary" @click="nextPage">{{
            $t("commerce.actions.next")
          }}</Button>
        </div>
      </template>

      <template v-else-if="detail && !failed">
        <Button
          class="back-button"
          type="link"
          @click="router.push('/account/orders')"
          >← {{ $t("commerce.actions.backToOrders") }}</Button
        >
        <div
          class="order-overview"
          :class="{
            'order-overview-payable': detail.summary.canPreparePayment,
          }"
        >
          <section
            v-if="deliveryRows.length"
            class="order-progress"
            data-testid="commerce-deliveries"
          >
            <h2>{{ $t("commerce.order.delivery") }}</h2>
            <Alert
              v-if="detailRefreshFailed"
              class="mb-3"
              type="warning"
              show-icon
              :message="$t('commerce.fulfillment.refreshPaused')"
            />
            <div class="fact-grid">
              <Card
                v-for="(delivery, index) in deliveryRows"
                :key="`${delivery.productNameZhCN}-${index}`"
              >
                <div class="row-heading">
                  <strong>{{ localized(delivery) }}</strong>
                  <Tag :color="customerFulfillmentColor(delivery.statusCode)">{{
                    $t(customerFulfillmentLabelKey(delivery.statusCode))
                  }}</Tag>
                </div>
                <p>
                  {{ delivery.quantity }} {{ $t("commerce.units.items") }} ·
                  {{
                    $t(
                      delivery.ongoingService
                        ? "commerce.fulfillment.ongoing"
                        : "commerce.fulfillment.once",
                    )
                  }}
                </p>
                <p v-if="delivery.statusCode === 'processing'">
                  {{ $t("commerce.fulfillment.processingHint") }}
                </p>
                <p v-if="delivery.statusCode === 'manual_processing'">
                  {{ $t("commerce.fulfillment.manualHint") }}
                </p>
                <p v-if="delivery.statusCode === 'not_completed'">
                  {{ $t("commerce.fulfillment.incompleteHint") }}
                </p>
                <small>{{
                  formatTime(
                    delivery.completedAt ||
                      delivery.startedAt ||
                      delivery.updatedAt,
                  )
                }}</small>
              </Card>
            </div>
          </section>
          <div
            class="detail-grid"
            :class="{ 'detail-grid-single': !detail.invoice }"
          >
            <Card data-testid="commerce-order-facts">
              <div class="row-heading">
                <div>
                  <h2>{{ detail.summary.orderNo }}</h2>
                  <p>{{ formatTime(detail.summary.createdAt) }}</p>
                </div>
                <Tag :color="statusColor(detail.summary.statusCode)">{{
                  statusLabel(detail.summary.statusCode)
                }}</Tag>
              </div>
              <Descriptions :column="1" bordered size="small">
                <DescriptionsItem :label="$t('commerce.amount.total')">{{
                  amount(
                    detail.summary.totalAmountMinor,
                    detail.summary.currencyCode,
                  )
                }}</DescriptionsItem>
                <DescriptionsItem :label="$t('commerce.order.due')">{{
                  formatTime(detail.summary.paymentDueAt)
                }}</DescriptionsItem>
              </Descriptions>
              <Space class="section-gap" wrap>
                <Button
                  v-if="detail.summary.canPreparePayment"
                  :loading="busy"
                  type="primary"
                  @click="beginPayment"
                  >{{ $t("commerce.actions.preparePayment") }}</Button
                >
                <Button
                  v-if="detail.summary.canPreparePayment && paymentState"
                  :loading="busy"
                  @click="recheckPayment"
                  >{{ $t("commerce.actions.recheck") }}</Button
                >
                <Button
                  v-if="
                    detail.summary.canPreparePayment &&
                    paymentState &&
                    !paymentReady
                  "
                  type="primary"
                  @click="
                    router.push(
                      identityLocation(
                        detail.summary.orderNo,
                        paymentState.contextNo,
                      ),
                    )
                  "
                  >{{ $t("commerce.actions.verify") }}</Button
                >
                <Button
                  v-if="detail.summary.canCancel"
                  danger
                  :loading="busy"
                  @click="requestCancellation"
                  >{{ $t("commerce.actions.cancel") }}</Button
                >
              </Space>
              <Alert
                v-if="detail.summary.canPreparePayment && paymentState"
                class="section-gap"
                :message="
                  paymentReady
                    ? $t('commerce.payment.ready')
                    : statusLabel(paymentState.statusCode)
                "
                show-icon
                :type="paymentReady ? 'success' : 'warning'"
              />
            </Card>
            <Card
              v-if="detail.invoice"
              data-testid="commerce-order-invoice"
              :title="$t('commerce.order.invoice')"
            >
              <Descriptions :column="1" bordered size="small">
                <DescriptionsItem label="#">{{
                  detail.invoice.invoiceNo
                }}</DescriptionsItem>
                <DescriptionsItem :label="$t('commerce.amount.total')">{{
                  amount(
                    detail.invoice.totalAmountMinor,
                    detail.summary.currencyCode,
                  )
                }}</DescriptionsItem>
                <DescriptionsItem :label="$t('commerce.order.due')">{{
                  formatTime(detail.invoice.paymentDueAt)
                }}</DescriptionsItem>
              </Descriptions>
              <Button
                v-if="Number(detail.invoice.refundableAmountMinor || 0) > 0"
                class="section-gap"
                data-testid="commerce-order-refund"
                type="primary"
                @click="
                  router.push({
                    path: '/account/refunds',
                    query: { orderNo: detail.summary.orderNo },
                  })
                "
              >
                {{ $t("commerce.order.requestRefund") }}
              </Button>
            </Card>
          </div>
        </div>
        <section class="section-gap">
          <h2>{{ $t("commerce.order.items") }}</h2>
          <div class="order-grid">
            <Card v-for="item in detail.items" :key="String(item.orderItemNo)">
              <div class="row-heading">
                <h3>{{ localized(item) }}</h3>
                <strong>{{
                  amount(item.totalAmountMinor, detail.summary.currencyCode)
                }}</strong>
              </div>
              <p>
                {{ item.quantity }} {{ $t("commerce.units.items") }} ·
                {{ item.purchaseCycles }} {{ $t("commerce.units.cycles") }}
              </p>
            </Card>
          </div>
        </section>
        <section
          v-if="detail.serviceEntitlements.length"
          class="section-gap"
          data-testid="commerce-service-entitlements"
        >
          <h2>{{ $t("commerce.order.services") }}</h2>
          <div class="fact-grid">
            <Card
              v-for="(service, index) in detail.serviceEntitlements"
              :key="`${service.productNameZhCN}-${index}`"
            >
              <div class="row-heading">
                <strong>{{ localized(service) }}</strong>
                <Tag :color="statusColor(service.statusCode)">{{
                  $t(customerServiceLabelKey(service.statusCode))
                }}</Tag>
              </div>
              <p>
                {{ $t("commerce.fulfillment.serviceStarted") }}：{{
                  formatTime(service.startedAt)
                }}
              </p>
              <small
                >{{ $t("commerce.fulfillment.serviceExpires") }}：{{
                  service.expiresAt
                    ? formatTime(service.expiresAt)
                    : $t("commerce.fulfillment.noExpiry")
                }}</small
              >
              <Alert
                v-if="
                  managementPreparationLabelKey(service.managementPreparation)
                "
                class="mt-3"
                show-icon
                :type="
                  service.managementPreparation === 'preparing'
                    ? 'info'
                    : 'warning'
                "
                :message="
                  $t(
                    managementPreparationLabelKey(
                      service.managementPreparation,
                    ),
                  )
                "
                data-testid="service-management-preparation"
              />
              <div
                v-if="
                  service.entitlementOwnerId === 'idc-resource' &&
                  !managementPreparationLabelKey(service.managementPreparation)
                "
                class="mt-3"
              >
                <Button
                  type="primary"
                  :aria-label="$t('nativeServers.manage')"
                  @click="
                    router.push(
                      `/account/native-servers/by-entitlement/${encodeURIComponent(service.entitlementNo)}`,
                    )
                  "
                  >{{ $t("nativeServers.manage") }}</Button
                >
              </div>
            </Card>
          </div>
        </section>
      </template>
    </div>
  </Page>
</template>

<style scoped>
.commerce-page {
  margin: 0 auto;
  max-width: 100%;
}
.page-toolbar :deep(.ant-card-body) {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
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
  max-width: 760px;
  margin: 0;
  color: hsl(var(--muted-foreground));
}
.cart-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 20px;
  align-items: start;
}
.stack,
.order-grid {
  display: grid;
  gap: 16px;
}
.quote-card {
  position: sticky;
  top: 18px;
  border-top: 2px solid hsl(var(--primary));
}
.row-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.row-heading h2,
.row-heading h3 {
  margin: 0;
  overflow-wrap: anywhere;
}
.row-heading p {
  margin: 6px 0 0;
  color: hsl(var(--muted-foreground));
}
.fact-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  margin: 18px 0;
}
.muted {
  color: hsl(var(--muted-foreground));
}
.amounts,
.order-facts {
  display: grid;
  gap: 10px;
  margin: 18px 0;
}
.amounts div,
.order-facts div {
  display: flex;
  justify-content: space-between;
  gap: 18px;
}
.amounts dd,
.order-facts dd {
  margin: 0;
  font-weight: 700;
}
.amounts .total {
  padding-top: 12px;
  font-size: 18px;
  border-top: 1px solid hsl(var(--border));
}
.status-filter {
  width: min(100%, 280px);
  margin-bottom: 18px;
}
.order-grid {
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
}
.pager {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
}
.order-overview {
  display: flex;
  flex-direction: column;
}
.order-overview-payable .detail-grid {
  order: -1;
  margin-bottom: 18px;
}
.detail-grid {
  display: grid;
  align-items: start;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.detail-grid-single {
  grid-template-columns: minmax(0, 1fr);
}
.order-progress > h2 {
  margin: 0 0 10px;
  font-size: 16px;
  font-weight: 600;
}
.order-progress .fact-grid {
  margin: 0 0 18px;
}
.detail-grid :deep(.ant-descriptions-item-label) {
  width: 128px;
  white-space: nowrap;
}
.detail-grid :deep(.ant-descriptions-item-content) {
  overflow-wrap: anywhere;
}
.section-gap {
  margin-top: 18px;
}
.back-button {
  margin: 0 0 12px -14px;
}
@media (max-width: 900px) {
  .cart-layout,
  .detail-grid {
    grid-template-columns: 1fr;
  }
  .quote-card {
    position: static;
  }
}
@media (max-width: 640px) {
  .page-toolbar :deep(.ant-card-body) {
    align-items: stretch;
    flex-direction: column;
  }
  .page-toolbar :deep(.ant-space) {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .order-grid {
    grid-template-columns: 1fr;
  }
  .row-heading {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
