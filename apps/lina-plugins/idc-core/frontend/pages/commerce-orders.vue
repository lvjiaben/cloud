<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/orders",
  title: "Commerce Orders",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormProps } from "@vben/common-ui";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import OperationsDetailPanel from "./components/operations-detail-panel.vue";

import { useAccess } from "./plugin-access";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Space,
  Tag,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import VbenDrawerCompat from "./components/vben-drawer-compat.vue";

import {
  commerceOrderDetail,
  commerceOrders,
  type CommerceOrderDetail,
  type CommerceOrderSummary,
} from "./commerce-client";
import { deliveryStatusKey, serviceStatusKey } from "./commerce-delivery-state";
import { customerDisplay, searchCustomerOptions, type CustomerSelectOption } from "./customer-selection";
const loading = ref(false);
const route = useRoute();
const router = useRouter();
const operationsDetail = computed(() => ['orders', 'products'].includes(String(route.query.operationsMetric || '')));
const detailLoading = ref(false);
const detailError = ref("");
const detailReference = ref("");
const errorMessage = ref("");
const rows = ref<CommerceOrderSummary[]>([]);
const detail = ref<CommerceOrderDetail>();
const drawerOpen = ref(false);
let detailGeneration = 0;
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);
const currentPage = ref(1);
const pageSize = ref(10);
const total = ref(0);
const { hasAccessByCodes } = useAccess();
const canReadDeliveryTasks = computed(() =>
  ["idc:admin:product-fulfillment:task:list", "idc:admin:product-fulfillment:task:query"].every(code => hasAccessByCodes([code])),
);
const deliveryNeedsAttention = computed(() => detail.value?.deliveries.some(
  delivery => ["manual_processing", "not_completed"].includes(delivery.statusCode),
));

async function openDeliveryTasks() {
  const orderNo = detail.value?.summary.orderNo;
  if (!canReadDeliveryTasks.value || !orderNo) return;
  await router.push({path: "/idc/product-fulfillment/tasks", query: {orderNo}});
  drawerOpen.value = false;
}
const canApproveRisk = computed(() =>
  hasAccessByCodes(["idc:admin:commerce:risk-review:approve"]),
);
const canRejectRisk = computed(() =>
  hasAccessByCodes(["idc:admin:commerce:risk-review:reject"]),
);
function columns(): VxeGridProps<CommerceOrderSummary>["columns"] {
  return [
  {
    field: "orderNo",
    title: $t("plugin.idc-core.commerce.orders.table.order"),
    width: 210,
  },
  {
    field: "customerDisplayName",
    slots: { default: "customer" },
    title: $t("plugin.idc-core.commerce.orders.table.customer"),
    width: 240,
  },
  {
    field: "productSummary",
    slots: { default: "productSummary" },
    title: $t("plugin.idc-core.commerce.orders.table.product"),
    minWidth: 220,
  },
  {
    field: "ipAddress",
    slots: { default: "plainValue" },
    title: $t("plugin.idc-core.commerce.orders.table.ip"),
    width: 150,
  },
  {
    field: "createdAt",
    slots: { default: "createdAt" },
    title: $t("plugin.idc-core.commerce.orders.table.created"),
    width: 180,
  },
  {
    field: "paymentMethodCode",
    slots: { default: "paymentMethod" },
    title: $t("plugin.idc-core.commerce.orders.table.paymentMethod"),
    width: 140,
  },
  {
    field: "statusCode",
    slots: { default: "statusCode" },
    title: $t("plugin.idc-core.commerce.orders.table.status"),
    width: 130,
  },
  {
    field: "amount",
    slots: { default: "amount" },
    title: $t("plugin.idc-core.commerce.orders.table.amount"),
    width: 240,
  },
  { field: "remark", slots: { default: "plainValue" }, title: $t("plugin.idc-core.commerce.orders.table.remark"), minWidth: 160 },
  { field: "action", slots: { default: "action" }, title: $t("plugin.idc-core.commerce.orders.detail"), width: 100, fixed: "right" },
  ];
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    { component: "Select", componentProps: { allowClear: true, dropdownMatchSelectWidth: false, popupClassName: "plugin-customer-options", filterOption: false, loading: customerLoading.value, onSearch: searchCustomers, options: customerOptions.value, showSearch: true }, fieldName: "customerNo", label: $t("plugin.idc-core.commerce.orders.filter.customer") },
    { component: "Select", componentProps: { allowClear: true, options: statusOptions.value }, fieldName: "statusCode", label: $t("plugin.idc-core.commerce.orders.table.status") },
    { component: "Select", componentProps: { allowClear: true, options: riskOptions.value }, fieldName: "riskDecisionCode", label: $t("plugin.idc-core.commerce.orders.table.risk") },
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange({ currentPage: page, pageSize: size }: { currentPage: number; pageSize: number }) {
      void load(size === pageSize.value ? page : 1, size);
    },
  },
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    handleReset: search,
    handleSubmit: search,
    schema: [],
    wrapperClass: "plugin-admin-search",
    layout: "inline",
    actionLayout: "inline",
    actionPosition: "left",
    showCollapseButton: false,
  },
  gridOptions: {
    columns: columns(), data: [], emptyRender: { name: "Empty" }, height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 }, rowConfig: { keyField: "orderNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-commerce-orders",
  },
});
watch(rows, (items) => gridApi.setGridOptions({ data: items }), { deep: true });
watch(() => preferences.app.locale, () => { gridApi.formApi.setState({ schema: searchSchema() }); gridApi.setGridOptions({ columns: columns() }); });

const statusOptions = computed(() => [
  { label: $t("plugin.idc-core.commerce.orders.filter.all"), value: "" },
  ...["pending_payment", "payment_processing", "paid", "fulfilling", "partially_fulfilled", "fulfilled", "cancelled", "partially_refunded", "refunded", "manual_review"].map(
    (value) => ({ label: statusLabel(value), value }),
  ),
]);

const riskOptions = computed(() => [
  { label: $t("plugin.idc-core.commerce.orders.filter.all"), value: "" },
  ...["approved", "manual_review", "rejected"].map((value) => ({
    label: statusLabel(value),
    value,
  })),
]);

function statusLabel(value: string) {
  const key = `plugin.idc-core.commerce.orders.status.${value}`;
  const translated = $t(key);
  return translated === key
    ? $t("plugin.idc-core.commerce.orders.status.unavailable")
    : translated;
}

function paymentTermLabel(value: string) {
  const key = `plugin.idc-core.commerce.orders.paymentTermValue.${value}`;
  const translated = $t(key);
  return translated === key
    ? $t("plugin.idc-core.commerce.orders.paymentTermValue.unavailable")
    : translated;
}

function paymentMethodLabel(value?: string, statusCode?: string) {
  if (!value) {
    return statusCode === "pending_payment"
      ? $t("plugin.idc-core.commerce.orders.paymentMethod.awaitingPayment")
      : $t("plugin.idc-core.commerce.orders.paymentMethod.notRecorded");
  }
  const key = `plugin.idc-core.commerce.orders.paymentMethod.${value}`;
  const translated = $t(key);
  return translated === key
    ? $t("plugin.idc-core.commerce.orders.paymentMethod.unavailable")
    : translated;
}

function productSummary(row: CommerceOrderSummary) {
  return row.productSummary || $t("plugin.idc-core.commerce.orders.productCount", { count: row.itemCount });
}

function plainOrderValue(row: CommerceOrderSummary, field: string) {
  const value = row[field as keyof CommerceOrderSummary];
  if (value) return value;
  return field === "ipAddress"
    ? $t("plugin.idc-core.commerce.orders.ipNotAssigned")
    : $t("plugin.idc-core.commerce.orders.noRemark");
}

function reviewTypeLabel(value: string) {
  const key = `plugin.idc-core.commerce.orders.reviewType.${value}`;
  const translated = $t(key);
  return translated === key
    ? $t("plugin.idc-core.commerce.orders.reviewType.unavailable")
    : translated;
}

function statusColor(value: string) {
  if (["active", "approved", "completed", "ready", "succeeded", "consumed"].includes(value)) return "success";
  if (["cancelled", "expired", "rejected", "released", "terminated"].includes(value)) return "default";
  if (value === "not_completed") return "error";
  if (["manual_review", "manual_processing", "awaiting_identity", "suspended"].includes(value)) return "warning";
  return "processing";
}

function amount(value: unknown, currency: string) {
  if (typeof value !== "string" && typeof value !== "number") return "—";
  const digits =
    new Intl.NumberFormat(undefined, { currency, style: "currency" })
      .resolvedOptions().maximumFractionDigits ?? 2;
  const raw = BigInt(`${value}`);
  const divisor = BigInt(`1${"0".repeat(digits)}`);
  const absolute = raw < 0n ? -raw : raw;
  const fraction = (absolute % divisor).toString().padStart(digits, "0");
  return `${raw < 0n ? "-" : ""}${currency} ${absolute / divisor}${digits ? `.${fraction}` : ""}`;
}

function localized(item: Record<string, unknown> | { productNameZhCN: string; productNameEnUS: string }) {
  const locale = document.documentElement.lang;
  const value = locale.toLowerCase().startsWith("zh")
    ? item.productNameZhCN
    : item.productNameEnUS;
  return typeof value === "string" ? value : "—";
}

function deliveryTime(delivery: { completedAt?: number; startedAt?: number; updatedAt: number }) {
  return delivery.completedAt || delivery.startedAt || delivery.updatedAt;
}

async function load(targetPage = currentPage.value, size = pageSize.value) {
  if (loading.value) return;
  loading.value = true;
  errorMessage.value = "";
  try {
    const values = await gridApi.formApi.getValues();
    const page = await commerceOrders({
      page: targetPage,
      customerNo: String(values.customerNo || "").trim() || undefined,
      limit: size,
      riskDecisionCode: String(values.riskDecisionCode || "") || undefined,
      statusCode: String(values.statusCode || "") || undefined,
    });
    rows.value = page.orders;
    currentPage.value = targetPage;
    pageSize.value = size;
    total.value = page.total;
    gridApi.setGridOptions({ pagerConfig: { currentPage: targetPage, pageSize: size, total: page.total } });
  } catch (error) {
    const status = (error as { response?: { status?: number } })?.response?.status;
    if (status === 401 || status === 403) {
      ++detailGeneration;
      rows.value = [];
      total.value = 0;
      currentPage.value = 1;
      drawerOpen.value = false;
      detailLoading.value = false;
      detail.value = undefined;
    }
    errorMessage.value = $t("plugin.idc-core.commerce.orders.loadFailed");
  } finally {
    gridApi.setGridOptions({ pagerConfig: { currentPage: currentPage.value, pageSize: pageSize.value, total: total.value } });
    loading.value = false;
  }
}

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try { customerOptions.value = await searchCustomerOptions(keyword); }
  finally {
    customerLoading.value = false;
    gridApi.formApi.setState({ schema: searchSchema() });
  }
}

function search() {
  void load(1);
}

async function openDetail(orderNo: string) {
  const generation = ++detailGeneration;
  detailReference.value = orderNo;
  detailError.value = "";
  drawerOpen.value = true;
  detailLoading.value = true;
  detail.value = undefined;
  errorMessage.value = "";
  try {
    const result = await commerceOrderDetail(orderNo);
    if (generation !== detailGeneration) return;
    detail.value = result.order;
  } catch {
    if (generation !== detailGeneration) return;
    detailError.value = $t("plugin.idc-core.commerce.orders.detailLoadFailed");
  } finally {
    if (generation === detailGeneration) detailLoading.value = false;
  }
}

watch(drawerOpen, open => { if (!open) detailGeneration += 1; });
onBeforeUnmount(() => { detailGeneration += 1; });

function openRequestedOrder() {
  const reference = route.query.orderNo;
  if (route.path.endsWith("/idc/orders") && typeof reference === "string" && reference.length > 0 && reference.length <= 128 &&
      ["idc:admin:commerce:order:list", "idc:admin:commerce:order:query"].every(code => hasAccessByCodes([code]))) void openDetail(reference);
}

onMounted(async () => {
  gridApi.formApi.setState({ schema: searchSchema() });
  if (!operationsDetail.value) { void searchCustomers(); await load(); openRequestedOrder(); }
});
watch(() => route.query.orderNo, () => { if (!operationsDetail.value) openRequestedOrder(); });
watch(operationsDetail, (active) => {
  if (!active) { void load(); void searchCustomers(); }
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" auto-content-height>
    <div class="commerce-order-page" data-testid="commerce-order-page">
      <OperationsDetailPanel v-if="operationsDetail" :can-open="hasAccessByCodes(['idc:admin:commerce:order:query'])" @open="openDetail($event.id)" />
      <div v-show="!operationsDetail" class="order-directory">
        <Alert v-if="errorMessage" class="page-alert" :message="errorMessage" show-icon type="error" />
        <Grid :loading="loading" class="min-h-0 flex-1">
          <template #customer="{ row }"><span class="order-customer"><span>{{ row.customerDisplayName }}</span><span v-if="row.customerPrimaryPhone"> · {{ row.customerPrimaryPhone }}</span><span v-if="row.customerCode" class="whitespace-nowrap"> · #{{ row.customerCode }}</span></span></template>
          <template #productSummary="{ row }">{{ productSummary(row) }}</template>
          <template #plainValue="{ row, column }">{{ plainOrderValue(row, column.field) }}</template>
          <template #paymentMethod="{ row }">{{ paymentMethodLabel(row.paymentMethodCode, row.statusCode) }}</template>
          <template #statusCode="{ row }"><Tag :color="statusColor(row.statusCode)">{{ statusLabel(row.statusCode) }}</Tag></template>
          <template #amount="{ row }"><strong class="amount-value">{{ amount(row.totalAmountMinor, row.currencyCode) }}</strong></template>
          <template #createdAt="{ row }">{{ formatTimestamp(row.createdAt) }}</template>
          <template #action="{ row }"><Button size="small" type="link" @click="openDetail(row.orderNo)">{{ $t("plugin.idc-core.commerce.orders.detail") }}</Button></template>
        </Grid>
      </div>

      <VbenDrawerCompat v-model:open="drawerOpen" :title="$t('plugin.idc-core.commerce.orders.detailTitle')" :width="720">
        <Alert v-if="detailError" :message="detailError" show-icon type="error"><template #action><Button @click="openDetail(detailReference)">{{ $t('plugin.idc-core.fulfillment.common.retry') }}</Button></template></Alert>
        <div v-else-if="detailLoading" class="drawer-loading">{{ $t("plugin.idc-core.commerce.orders.loading") }}</div>
        <template v-else-if="detail">
          <Descriptions :column="2" bordered size="small">
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.table.order')">{{ detail.summary.orderNo }}</DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.table.customer')">{{ customerDisplay(detail.summary) }}</DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.table.status')"><Tag :color="statusColor(detail.summary.statusCode)">{{ statusLabel(detail.summary.statusCode) }}</Tag></DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.table.amount')">{{ amount(detail.summary.totalAmountMinor, detail.summary.currencyCode) }}</DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.paymentDue')">{{ formatTimestamp(detail.summary.paymentDueAt || 0) }}</DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.table.risk')"><Tag :color="statusColor(detail.summary.riskDecisionCode)">{{ statusLabel(detail.summary.riskDecisionCode) }}</Tag></DescriptionsItem>
            <DescriptionsItem v-if="detail.summary.assistedRequestNo" :label="$t('plugin.idc-core.commerce.orders.assistedRequest')">{{ detail.summary.assistedRequestNo }}</DescriptionsItem>
            <DescriptionsItem :label="$t('plugin.idc-core.commerce.orders.paymentTerm')">{{ paymentTermLabel(detail.summary.paymentTermCode) }}</DescriptionsItem>
          </Descriptions>

          <Alert v-if="deliveryNeedsAttention" class="drawer-section" type="warning" show-icon :message="$t('plugin.idc-core.commerce.orders.deliveryAttention')" data-testid="order-delivery-attention">
            <template v-if="canReadDeliveryTasks" #action>
              <Button data-testid="order-open-delivery-tasks" @click="openDeliveryTasks">{{ $t('plugin.idc-core.commerce.orders.openDeliveryTasks') }}</Button>
            </template>
          </Alert>

          <Alert
            v-if="detail.canApproveRisk || detail.canRejectRisk"
            class="page-alert"
            :description="$t('plugin.idc-core.commerce.orders.riskEvidenceNote')"
            :message="$t('plugin.idc-core.commerce.orders.riskPending')"
            show-icon
            type="warning"
          />
          <Space v-if="detail.canApproveRisk || detail.canRejectRisk" wrap>
            <Button v-if="canApproveRisk" disabled>{{ $t("plugin.idc-core.commerce.orders.approveRisk") }}</Button>
            <Button v-if="canRejectRisk" danger disabled>{{ $t("plugin.idc-core.commerce.orders.rejectRisk") }}</Button>
          </Space>

          <section class="drawer-section" data-testid="commerce-order-items">
            <h3>{{ $t("plugin.idc-core.commerce.orders.items") }}</h3>
            <Card v-for="item in detail.items" :key="String(item.orderItemNo)" class="detail-card" size="small">
              <div class="detail-row"><strong>{{ localized(item) }}</strong><span>{{ amount(item.totalAmountMinor, detail.summary.currencyCode) }}</span></div>
              <small>{{ item.quantity }} × {{ item.purchaseCycles }}</small>
            </Card>
          </section>

          <section class="drawer-section">
            <h3>{{ $t("plugin.idc-core.commerce.orders.fulfillment") }}</h3>
            <div class="fact-grid">
              <Card v-for="(delivery, index) in detail.deliveries" :key="`${delivery.productNameZhCN}-${index}`" class="detail-card" size="small">
                <div class="detail-row"><strong>{{ localized(delivery) }}</strong><Tag :color="statusColor(delivery.statusCode)">{{ $t(deliveryStatusKey(delivery.statusCode)) }}</Tag></div>
                <small>{{ delivery.quantity }} {{ $t("plugin.idc-core.commerce.orders.delivery.quantity") }} · {{ $t(delivery.ongoingService ? "plugin.idc-core.commerce.orders.delivery.ongoing" : "plugin.idc-core.commerce.orders.delivery.once") }}</small>
                <small class="detail-time">{{ formatTimestamp(deliveryTime(delivery)) }}</small>
              </Card>
            </div>
          </section>

          <section v-if="detail.serviceEntitlements.length" class="drawer-section">
            <h3>{{ $t("plugin.idc-core.commerce.orders.services") }}</h3>
            <div class="fact-grid">
              <Card v-for="(service, index) in detail.serviceEntitlements" :key="`${service.productNameZhCN}-${index}`" class="detail-card" size="small">
                <div class="detail-row"><strong>{{ localized(service) }}</strong><Tag :color="statusColor(service.statusCode)">{{ $t(serviceStatusKey(service.statusCode)) }}</Tag></div>
                <small>{{ $t("plugin.idc-core.commerce.orders.service.started") }}：{{ formatTimestamp(service.startedAt) }}</small>
                <small class="detail-time">{{ $t("plugin.idc-core.commerce.orders.service.expires") }}：{{ service.expiresAt ? formatTimestamp(service.expiresAt) : $t("plugin.idc-core.commerce.orders.service.noExpiry") }}</small>
              </Card>
            </div>
          </section>

          <section v-if="detail.reviews.length" class="drawer-section">
            <h3>{{ $t("plugin.idc-core.commerce.orders.reviews") }}</h3>
            <Card v-for="review in detail.reviews" :key="String(review.reviewNo)" class="detail-card" size="small">
              <div class="detail-row"><strong>{{ reviewTypeLabel(String(review.reviewTypeCode)) }}</strong><Tag :color="statusColor(String(review.statusCode))">{{ statusLabel(String(review.statusCode)) }}</Tag></div>
            </Card>
          </section>
        </template>
      </VbenDrawerCompat>
    </div>
  </Page>
</template>

<style scoped>
.commerce-order-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
  min-width: 0;
  max-width: 100%;
}
.order-directory {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}
.order-customer { display: block; white-space: normal; overflow-wrap: anywhere; }
.commerce-order-page > .ant-card { min-width: 0; }
.page-heading { display: flex; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.page-heading h2 { margin: 0 0 6px; font-size: 20px; }
.page-heading p { margin: 0; color: hsl(var(--muted-foreground)); }
.filters { display: grid; grid-template-columns: minmax(220px, 1.3fr) minmax(150px, .8fr) minmax(150px, .8fr) auto; gap: 10px; margin-bottom: 16px; }
.page-alert { margin: 14px 0; }
.drawer-loading { padding: 48px 0; text-align: center; color: hsl(var(--muted-foreground)); }
.drawer-section { margin-top: 24px; }
.drawer-section h3 { margin-bottom: 10px; font-size: 15px; }
.detail-card + .detail-card { margin-top: 8px; }
.amount-value { white-space: nowrap; }
.detail-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.detail-card small { color: hsl(var(--muted-foreground)); }
.detail-time { display: block; margin-top: 4px; }
.fact-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.section-copy { margin: 0 0 12px; color: hsl(var(--muted-foreground)); }
@media (max-width: 760px) {
  .filters, .fact-grid { grid-template-columns: 1fr; }
  .filters .ant-btn { width: 100%; }
}
</style>
