<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/assisted-quotes",
  title: "Assisted Quotes",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "#/adapter/form";
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { createNumberedPage } from "./numbered-page";

import { useAccess } from "./plugin-access";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Divider,
  Modal as AntModal,
  Statistic,
  Tag,
  Textarea,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import VbenModalCompat from "./components/vben-modal-compat.vue";
import { customerDisplay, searchCustomerOptions, type CustomerSelectOption } from "./customer-selection";

import {
  assistedQuoteDetail,
  assistedQuotes,
  checkoutAssistedQuote,
  decideAssistedQuote,
  type AssistedPaymentTerm,
  type AssistedQuote,
} from "./assisted-quote-client";

const { hasAccessByCodes } = useAccess();
const permissions = {
  approve: computed(() =>
    hasAccessByCodes(["idc:admin:commerce:assisted-quote:approve"]),
  ),
  checkout: computed(() =>
    hasAccessByCodes(["idc:admin:commerce:assisted-quote:checkout"]),
  ),
  query: computed(() =>
    hasAccessByCodes(["idc:admin:commerce:assisted-quote:query"]),
  ),
  reject: computed(() =>
    hasAccessByCodes(["idc:admin:commerce:assisted-quote:reject"]),
  ),
};

const loading = ref(false);
const detailLoading = ref(false);
const submitting = ref(false);
const errorMessage = ref("");
const rows = ref<AssistedQuote[]>([]);
const detail = ref<AssistedQuote>();
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);
const currentPage = ref(1);
const pageSize = ref(10);
let loadSequence = 0;
const pagination = createNumberedPage<AssistedQuote>(
  (state) => {
    rows.value = state.items;
    currentPage.value = state.currentPage;
    pageSize.value = state.pageSize;
    gridApi.setGridOptions({ pagerConfig: { currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
  },
  () => { detailOpen.value = false; decisionOpen.value = false; detail.value = undefined; decisionTarget.value = undefined; },
);
const detailOpen = ref(false);
const decisionOpen = ref(false);
const decisionAction = ref<"approval" | "rejection">("approval");
const decisionReason = ref("");
const decisionTarget = ref<AssistedQuote>();


function columns(): VxeGridProps<AssistedQuote>["columns"] {
  return [
  {
    field: "requestNo",
    title: tr("table.request"),
    width: 218,
  },
  {
    field: "customerDisplayName",
    slots: { default: "customer" },
    title: tr("table.customer"),
    width: 190,
  },
  {
    field: "statusCode",
    slots: { default: "statusCode" },
    title: tr("table.status"),
    width: 150,
  },
  { field: "amount", slots: { default: "amount" }, title: tr("table.amount"), width: 170 },
  { field: "term", slots: { default: "term" }, title: tr("table.paymentTerm"), width: 160 },
  {
    field: "createdAt",
    slots: { default: "createdAt" },
    title: tr("table.created"),
    width: 180,
  },
  { field: "action", slots: { default: "action" }, title: tr("viewDetail"), width: 98, fixed: "right" },
  ];
}

function itemColumns(): VxeGridProps<AssistedQuote["items"][number]>["columns"] {
  return [
    { field: "lineNumber", title: "#", width: 56 },
    { field: "product", slots: { default: "product" }, title: tr("detail.product") },
    {
      field: "quantity",
      title: tr("detail.quantity"),
      width: 100,
    },
    {
      field: "purchaseCycles",
      title: tr("detail.cycles"),
      width: 100,
    },
  ];
}

function searchSchema(): VbenFormProps["schema"] { return [
  { component: "Select", componentProps: { allowClear: true, filterOption: false, loading: customerLoading.value, onSearch: searchCustomers, options: customerOptions.value, showSearch: true }, fieldName: "customerNo", label: tr("table.customer") },
  { component: "Select", componentProps: { allowClear: true, options: statusOptions.value }, fieldName: "statusCode", label: tr("table.status") },
]; }

const statusOptions = computed(() => [
  { label: tr("filter.all"), value: "" },
  ...["pending_approval", "approved", "rejected", "ordered"].map((value) => ({
    label: statusLabel(value),
    value,
  })),
]);

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => { void load(currentPage, pageSize); } },
  formOptions: { handleReset: search, handleSubmit: search, schema: searchSchema(), wrapperClass: "grid-cols-1 md:grid-cols-2" },
  gridOptions: {
    columns: columns(), data: [], emptyRender: { name: "Empty" }, height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 }, rowConfig: { keyField: "requestNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-assisted-quotes",
  },
});
const [ItemGrid, itemGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: itemColumns(), data: [], emptyRender: { name: "Empty" }, height: 300,
    pagerConfig: { enabled: false }, rowConfig: { keyField: "requestItemNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-assisted-quote-items",
  },
});
watch(rows, (items) => gridApi.setGridOptions({ data: items }), { deep: true });
watch(
  () => detail.value?.items || [],
  (items) => itemGridApi.setGridOptions({ data: items }),
  { deep: true },
);
watch(() => preferences.app.locale, () => {
  gridApi.setGridOptions({ columns: columns() });
  itemGridApi.setGridOptions({ columns: itemColumns() });
  gridApi.formApi.setState({ schema: searchSchema() });
});

const currentPageMetrics = computed(() => ({
  approved: rows.value.filter((row) => row.statusCode === "approved").length,
  ordered: rows.value.filter((row) => row.statusCode === "ordered").length,
  pending: rows.value.filter((row) => row.statusCode === "pending_approval")
    .length,
}));

function tr(suffix: string, params?: Record<string, number>) {
  return params
    ? $t(`plugin.idc-core.commerce.assisted.${suffix}`, params)
    : $t(`plugin.idc-core.commerce.assisted.${suffix}`);
}

function statusLabel(value: string) {
  const key = `plugin.idc-core.commerce.assisted.status.${value}`;
  const translated = $t(key);
  return translated === key ? tr("status.unknown") : translated;
}

function orderStatusLabel(value?: string) {
  if (!value) return "—";
  const key = `plugin.idc-core.commerce.orders.status.${value}`;
  const translated = $t(key);
  return translated === key ? tr("status.unknown") : translated;
}

function statusColor(value: string) {
  if (["approved", "ordered"].includes(value)) return "success";
  if (value === "rejected") return "error";
  return "warning";
}

function localized<T extends object>(
  item: T,
  prefix: "categoryName" | "name" | "productName",
) {
  const zh = document.documentElement.lang.toLowerCase().startsWith("zh");
  const value = (item as Record<string, unknown>)[
    `${prefix}${zh ? "ZhCN" : "EnUS"}`
  ];
  return typeof value === "string" && value ? value : "—";
}

function currencyDigits(currency: string) {
  try {
    return (
      new Intl.NumberFormat(undefined, {
        currency,
        style: "currency",
      }).resolvedOptions().maximumFractionDigits ?? 2
    );
  } catch {
    return 2;
  }
}

function amount(value: string, currency: string) {
  if (!/^[-]?\d+$/.test(value)) return "—";
  const digits = currencyDigits(currency);
  const raw = BigInt(value);
  const absolute = raw < 0n ? -raw : raw;
  const divisor = 10n ** BigInt(digits);
  const fraction = (absolute % divisor).toString().padStart(digits, "0");
  return `${raw < 0n ? "-" : ""}${currency} ${absolute / divisor}${
    digits ? `.${fraction}` : ""
  }`;
}

function paymentTermLabel(value: AssistedPaymentTerm) {
  const key = `plugin.idc-core.commerce.assisted.paymentTerm.${value}`;
  const translated = $t(key);
  return translated === key ? tr("paymentTerm.unknown") : translated;
}

function paymentDueLabel(seconds: number) {
  if (!Number.isInteger(seconds) || seconds < 1) return "—";
  if (seconds % 86_400 === 0) return tr("duration.days", { count: seconds / 86_400 });
  if (seconds % 3_600 === 0) return tr("duration.hours", { count: seconds / 3_600 });
  return tr("duration.minutes", { count: Math.ceil(seconds / 60) });
}

async function load(targetPage = currentPage.value, targetSize = pageSize.value) {
  const sequence = ++loadSequence;
  loading.value = true;
  errorMessage.value = "";
  await pagination.query({ currentPage: targetPage, pageSize: targetSize }, async (pageNumber, limit) => {
    const values = await gridApi.formApi.getValues();
    try {
    const page = await assistedQuotes({
      page: pageNumber,
      customerNo: String(values.customerNo || "") || undefined,
      limit,
      statusCode: String(values.statusCode || "") || undefined,
    });
    return { items: page.requests, total: page.total };
    } catch (error) {
      if (sequence === loadSequence) errorMessage.value = tr("loadFailed");
      throw error;
    }
  });
  if (sequence === loadSequence) loading.value = false;
}

function search() {
  void load(1);
}

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try { customerOptions.value = await searchCustomerOptions(keyword); gridApi.formApi.setState({ schema: searchSchema() }); }
  finally { customerLoading.value = false; }
}

onBeforeUnmount(() => { loadSequence += 1; pagination.invalidate(); });

async function openDetail(requestNo: string) {
  if (!permissions.query.value) return;
  detailOpen.value = true;
  detailLoading.value = true;
  detail.value = undefined;
  try {
    detail.value = await assistedQuoteDetail(requestNo);
  } catch {
    errorMessage.value = tr("detailLoadFailed");
  } finally {
    detailLoading.value = false;
  }
}

function openDecision(target: AssistedQuote, action: "approval" | "rejection") {
  decisionTarget.value = target;
  decisionAction.value = action;
  decisionReason.value = "";
  decisionOpen.value = true;
}

async function submitDecision() {
  const target = decisionTarget.value;
  if (!target || decisionReason.value.trim().length < 2) {
    errorMessage.value = tr("invalidDecisionReason");
    return;
  }
  submitting.value = true;
  try {
    const updated = await decideAssistedQuote(
      target.requestNo,
      decisionAction.value,
      {
        decisionReason: decisionReason.value.trim(),
        idempotencyKey: crypto.randomUUID(),
      },
    );
    decisionOpen.value = false;
    detail.value = updated;
    message.success(
      decisionAction.value === "approval" ? tr("approved") : tr("rejected"),
    );
    await load();
  } catch {
    errorMessage.value = tr("decisionFailed");
  } finally {
    submitting.value = false;
  }
}

function confirmCheckout(target: AssistedQuote) {
  AntModal.confirm({
    content: tr("checkoutWarning"),
    okText: tr("checkout"),
    onOk: async () => {
      const updated = await checkoutAssistedQuote(target.requestNo);
      detail.value = updated;
      message.success(tr("checkedOut"));
      await load();
    },
    title: tr("checkoutTitle"),
  });
}

onMounted(() => { void searchCustomers(); void load(); });
</script>

<template>
  <Page auto-content-height>
    <div class="assisted-page" data-testid="assisted-quote-page">
      <Card :bordered="false" class="page-toolbar" size="small">
        <template #title>{{ tr("title") }}</template>
      </Card>

      <div class="metrics" data-testid="assisted-quote-metrics">
        <Card :bordered="false"
          ><Statistic
            :title="tr('metric.pending')"
            :value="currentPageMetrics.pending"
        /></Card>
        <Card :bordered="false"
          ><Statistic
            :title="tr('metric.approved')"
            :value="currentPageMetrics.approved"
        /></Card>
        <Card :bordered="false"
          ><Statistic
            :title="tr('metric.ordered')"
            :value="currentPageMetrics.ordered"
        /></Card>
        <Card :bordered="false" class="guard-card">
          <span>{{ tr("metric.guard") }}</span>
          <strong>{{ tr("metric.guardValue") }}</strong>
        </Card>
      </div>

      <Card :bordered="false" class="directory-card">
        <Alert
          v-if="errorMessage"
          class="page-alert"
          :message="errorMessage"
          closable
          show-icon
          type="error"
          @close="errorMessage = ''"
        />
        <Grid :loading="loading">
          <template #customer="{ row }">{{ customerDisplay(row) }}</template>
          <template #statusCode="{ row }"><Tag :color="statusColor(row.statusCode)">{{ statusLabel(row.statusCode) }}</Tag></template>
          <template #amount="{ row }"><strong class="amount-value">{{ amount(row.totalAmountMinor, row.currencyCode) }}</strong></template>
          <template #term="{ row }">{{ paymentTermLabel(row.paymentTermCode) }}</template>
          <template #createdAt="{ row }">{{ formatTimestamp(row.createdAt) }}</template>
          <template #action="{ row }"><Button v-if="permissions.query.value" :data-testid="`assisted-quote-detail-${row.requestNo}`" size="small" type="link" @click="openDetail(row.requestNo)">{{ tr("viewDetail") }}</Button></template>
        </Grid>
      </Card>

      <VbenDrawerCompat
        v-model:open="detailOpen"
        :title="tr('detail.title')"
        :width="760"
      >
        <div v-if="detailLoading" class="drawer-loading">
          {{ tr("loading") }}
        </div>
        <template v-else-if="detail">
          <Alert
            :message="tr('detail.safety')"
            class="detail-alert"
            show-icon
            type="info"
          />
          <Descriptions :column="2" bordered size="small">
            <DescriptionsItem :label="tr('table.request')">{{
              detail.requestNo
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('table.status')"
              ><Tag :color="statusColor(detail.statusCode)">{{
                statusLabel(detail.statusCode)
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem :label="tr('table.customer')">{{
              customerDisplay(detail)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('detail.currentPrice')">{{
              amount(detail.subtotalMinor, detail.currencyCode)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('detail.discount')">{{
              amount(detail.manualDiscountMinor, detail.currencyCode)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('table.amount')">{{
              amount(detail.totalAmountMinor, detail.currencyCode)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('table.paymentTerm')"
              >{{ paymentTermLabel(detail.paymentTermCode) }} ·
              {{ paymentDueLabel(detail.paymentDueSeconds) }}</DescriptionsItem
            >
            <DescriptionsItem :label="tr('detail.expires')">{{
              formatTimestamp(detail.expiresAt)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('table.created')">{{
              formatTimestamp(detail.createdAt)
            }}</DescriptionsItem>
            <DescriptionsItem :label="tr('detail.reason')" :span="2">{{
              detail.reason
            }}</DescriptionsItem>
            <DescriptionsItem
              v-if="detail.internalNote"
              :label="tr('detail.note')"
              :span="2"
              >{{ detail.internalNote }}</DescriptionsItem
            >
            <DescriptionsItem
              v-if="detail.decisionCode"
              :label="tr('detail.decision')"
              >{{ statusLabel(detail.decisionCode) }}</DescriptionsItem
            >
            <DescriptionsItem
              v-if="detail.decisionAt"
              :label="tr('detail.decisionAt')"
              >{{ formatTimestamp(detail.decisionAt) }}</DescriptionsItem
            >
            <DescriptionsItem
              v-if="detail.decisionReason"
              :label="tr('detail.decisionReason')"
              :span="2"
              >{{ detail.decisionReason }}</DescriptionsItem
            >
            <DescriptionsItem
              v-if="detail.quoteNo"
              :label="tr('detail.quoteNo')"
              >{{ detail.quoteNo }}</DescriptionsItem
            >
            <DescriptionsItem
              v-if="detail.orderNo"
              :label="tr('detail.orderNo')"
              >{{ detail.orderNo }} ·
              {{ orderStatusLabel(detail.orderStatusCode) }}</DescriptionsItem
            >
          </Descriptions>
          <Divider>{{ tr("detail.items") }}</Divider>
          <ItemGrid><template #product="{ row }">{{ localized(row, "productName") }}</template></ItemGrid>
          <div class="drawer-actions">
            <Button
              v-if="permissions.approve.value && detail.canApprove"
              data-testid="assisted-quote-approve"
              type="primary"
              @click="openDecision(detail, 'approval')"
              >{{ tr("approve") }}</Button
            >
            <Button
              v-if="permissions.reject.value && detail.canReject"
              data-testid="assisted-quote-reject"
              danger
              @click="openDecision(detail, 'rejection')"
              >{{ tr("reject") }}</Button
            >
            <Button
              v-if="permissions.checkout.value && detail.canCheckout"
              data-testid="assisted-quote-checkout"
              type="primary"
              @click="confirmCheckout(detail)"
              >{{ tr("checkout") }}</Button
            >
          </div>
        </template>
      </VbenDrawerCompat>

      <VbenModalCompat
        v-model:open="decisionOpen"
        :confirm-loading="submitting"
        :ok-text="decisionAction === 'approval' ? tr('approve') : tr('reject')"
        :title="
          decisionAction === 'approval'
            ? tr('approvalTitle')
            : tr('rejectionTitle')
        "
        @ok="submitDecision"
      >
        <div data-testid="assisted-decision-modal">
          <Alert
            :message="tr('decisionSafety')"
            class="detail-alert"
            show-icon
            type="warning"
          />
          <label class="decision-field"
            ><span>{{ tr("field.decisionReason") }}</span
            ><Textarea
              v-model:value="decisionReason"
              :maxlength="1000"
              :rows="4"
              data-testid="assisted-decision-reason"
          /></label>
        </div>
      </VbenModalCompat>
    </div>
  </Page>
</template>

<style scoped>
.assisted-page {
  display: grid;
  gap: 16px;
  grid-template-rows: auto auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
  padding: 4px;
}
.page-toolbar :deep(.ant-card-head) {
  min-height: 48px;
}
.metrics {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.guard-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: center;
}
.guard-card span {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
}
.guard-card strong {
  color: #1677ff;
  font-size: 18px;
  margin-top: 4px;
}
.directory-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.directory-card :deep(.ant-card-body) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}
.directory-card :deep(.vxe-grid) {
  flex: 1;
  min-height: 0;
}
.filters {
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(220px, 1fr) 220px auto;
  margin-bottom: 16px;
}
.page-alert,
.detail-alert {
  margin-bottom: 16px;
}
.amount-value {
  color: #10243f;
  font-variant-numeric: tabular-nums;
}
.pagination-actions,
.drawer-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 16px;
}
.drawer-loading {
  color: hsl(var(--muted-foreground));
  padding: 48px 0;
  text-align: center;
}
label {
  display: grid;
  gap: 6px;
}
label > span {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  font-weight: 600;
}
.decision-field {
  margin-top: 16px;
}
@media (max-width: 900px) {
  .metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .filters {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 640px) {
  .metrics {
    grid-template-columns: 1fr;
  }
}
</style>
