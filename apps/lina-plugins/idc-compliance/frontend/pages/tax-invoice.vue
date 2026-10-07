<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/tax-invoices",
  title: "Tax Invoices",
};
</script>

<script setup lang="ts">
import type {
  RedLetterCandidate,
  RedLetterDetail,
  RedLetterTask,
  TaxInvoice,
  TaxInvoiceDetail,
} from "./tax-invoice-management-client";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormSchema } from "#/adapter/form";
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
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
  Empty,
  Modal as AntModal,
  Radio,
  RadioGroup,
  Select,
  SelectOption,
  Space,
  TabPane,
  Tabs,
  Tag,
  message,
} from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import VbenModalCompat from "./components/vben-modal-compat.vue";
import {
  redLetterComplete,
  redLetterGet,
  redLetterList,
  redLetterResolve,
  taxInvoiceApprove,
  taxInvoiceCancel,
  taxInvoiceGet,
  taxInvoiceList,
  taxInvoiceReject,
  taxInvoiceSubmit,
} from "./tax-invoice-management-client";

const { hasAccessByCodes } = useAccess();
const canInvoiceQuery = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:tax-invoice:query"]),
);
const canInvoiceReview = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:tax-invoice:review"]),
);
const canInvoiceSubmit = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:tax-invoice:submit"]),
);
const canInvoiceCancel = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:tax-invoice:cancel"]),
);
const canRedQuery = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:red-letter:query"]),
);
const canRedHandle = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:red-letter:handle"]),
);
const redLetterRoute = globalThis.location?.pathname.includes("/red-letters");
const activeTab = ref(
  redLetterRoute
    ? "redLetters"
    : canInvoiceQuery.value
      ? "invoices"
      : "redLetters",
);
const invoiceLoading = ref(false),
  redLoading = ref(false),
  detailLoading = ref(false),
  acting = ref(false);
const invoices = ref<TaxInvoice[]>([]),
  redTasks = ref<RedLetterTask[]>([]);
const invoicePage = ref({ currentPage: 1, pageSize: 10, total: 0 });
const redPage = ref({ currentPage: 1, pageSize: 10, total: 0 });
let invoiceLoadSequence = 0;
let redLoadSequence = 0;
let detailSequence = 0;
const invoicePager = createNumberedPage<TaxInvoice>((state) => {
  invoices.value = state.items;
  invoicePage.value = state;
  invoiceGridApi.setGridOptions({ pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
}, () => {
  detailSequence++;
  invoiceDetail.value = undefined;
  invoiceDrawer.value = decisionModal.value = false;
  detailLoading.value = false;
});
const redPager = createNumberedPage<RedLetterTask>((state) => {
  redTasks.value = state.items;
  redPage.value = state;
  redGridApi.setGridOptions({ pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
}, () => {
  detailSequence++;
  redDetail.value = undefined;
  redDrawer.value = allocationModal.value = evidenceModal.value = false;
  detailLoading.value = false;
  Object.keys(selectionByRefund).forEach((key) => delete selectionByRefund[key]);
});
onBeforeUnmount(() => {
  invoiceLoadSequence++; redLoadSequence++; detailSequence++;
  invoicePager.invalidate(); redPager.invalidate();
});
const invoiceCustomer = ref(""),
  invoiceStatus = ref(""),
  redCustomer = ref(""),
  redStatus = ref("");
const invoiceDetail = ref<TaxInvoiceDetail>(),
  redDetail = ref<RedLetterDetail>();
const invoiceDrawer = ref(false),
  redDrawer = ref(false),
  decisionModal = ref(false),
  allocationModal = ref(false),
  evidenceModal = ref(false);
const decisionAction = ref<"cancel" | "reject">("reject");
const selectionByRefund = reactive<Record<string, string>>({});

const invoiceStatuses = [
  "pending_review",
  "approved_pending_submission",
  "rejected",
  "queued",
  "submitting",
  "issued",
  "delivered",
  "failed",
  "result_unknown",
  "red_letter_pending",
  "partially_red_lettered",
  "red_lettered",
  "canceled",
];
const redStatuses = [
  "pending_review",
  "awaiting_confirmation",
  "queued",
  "submitting",
  "processing",
  "completed",
  "failed",
  "result_unknown",
  "manual_required",
  "canceled",
];
const reasonCodes = [
  "buyer_or_tax_information_incorrect",
  "amount_or_billing_invoice_adjustment_required",
  "duplicate_request",
  "business_not_invoiceable",
  "customer_requested_cancellation",
  "order_or_refund_changed",
  "other",
];

function decisionFormSchema(): VbenFormSchema[] {
  return [
    {
      component: "Select" as const,
      componentProps: {
        options: reasonCodes.map((code) => ({
          label: label("reason", code),
          value: code,
        })),
      },
      fieldName: "reasonCode",
      label: t("decision.reason"),
      rules: z.string().min(1),
    },
    {
      component: "Textarea" as const,
      componentProps: { maxlength: 2000, rows: 4 },
      fieldName: "internalNote",
      label: t("decision.note"),
    },
  ];
}

function evidenceFormSchema(): VbenFormSchema[] {
  return [
    {
      component: "Input" as const,
      componentProps: { maxlength: 128 },
      fieldName: "evidenceReference",
      label: t("red.evidenceReference"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "Input" as const,
      componentProps: { maxlength: 64 },
      fieldName: "evidenceSha256",
      label: t("red.evidenceSha"),
      rules: z.string().regex(/^[0-9a-f]{64}$/),
    },
    {
      component: "Input" as const,
      componentProps: { type: "datetime-local" },
      fieldName: "occurredAt",
      label: t("red.occurredAt"),
      rules: z.string().min(1),
    },
  ];
}

const [DecisionForm, decisionFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});
const [EvidenceForm, evidenceFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});
function invoiceColumns(): VxeGridProps<TaxInvoice>["columns"] {
  return [
    { title: t("invoice.no"), field: "taxInvoiceNo", minWidth: 180 },
    { title: t("common.customer"), field: "customerNo", minWidth: 150 },
    {
      title: t("common.status"),
      field: "statusCode",
      slots: { default: "status" },
      width: 150,
    },
    {
      title: t("common.amount"),
      field: "grossAmountMinor",
      slots: { default: "amount" },
      width: 140,
    },
    {
      title: t("invoice.type"),
      field: "invoiceTypeCode",
      slots: { default: "type" },
      width: 140,
    },
    {
      title: t("common.submittedAt"),
      field: "submittedAt",
      slots: { default: "time" },
      width: 180,
    },
    {
      title: t("common.actions"),
      field: "actions",
      slots: { default: "actions" },
      fixed: "right",
      width: 100,
    },
  ];
}
function redColumns(): VxeGridProps<RedLetterTask>["columns"] {
  return [
    { title: t("red.no"), field: "taskNo", minWidth: 180 },
    { title: t("common.customer"), field: "customerNo", minWidth: 150 },
    {
      title: t("common.status"),
      field: "statusCode",
      slots: { default: "status" },
      width: 150,
    },
    {
      title: t("common.amount"),
      field: "grossAmountMinor",
      slots: { default: "amount" },
      width: 140,
    },
    {
      title: t("red.confirmation"),
      field: "customerConfirmationRequired",
      slots: { default: "confirmation" },
      width: 140,
    },
    {
      title: t("common.createdAt"),
      field: "createdAt",
      slots: { default: "time" },
      width: 180,
    },
    {
      title: t("common.actions"),
      field: "actions",
      slots: { default: "actions" },
      fixed: "right",
      width: 100,
    },
  ];
}
const [InvoiceGrid, invoiceGridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    handleSubmit: async (values) => {
      invoiceCustomer.value = String(values.customerNo || "").trim();
      invoiceStatus.value = String(values.statusCode || "");
      await loadInvoices(true);
    },
    handleReset: async () => {
      await invoiceGridApi.formApi.resetForm();
      invoiceCustomer.value = invoiceStatus.value = "";
      await loadInvoices(true);
    },
    showCollapseButton: false,
    wrapperClass: "grid-cols-1 md:grid-cols-3",
  },
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => void loadInvoices(false, currentPage, pageSize) },
  gridOptions: {
    columns: invoiceColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: "taxInvoiceNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-tax-invoices",
  },
});
const [RedGrid, redGridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    handleSubmit: async (values) => {
      redCustomer.value = String(values.customerNo || "").trim();
      redStatus.value = String(values.statusCode || "");
      await loadRed(true);
    },
    handleReset: async () => {
      await redGridApi.formApi.resetForm();
      redCustomer.value = redStatus.value = "";
      await loadRed(true);
    },
    showCollapseButton: false,
    wrapperClass: "grid-cols-1 md:grid-cols-3",
  },
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => void loadRed(false, currentPage, pageSize) },
  gridOptions: {
    columns: redColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: "taskNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-red-letter-tasks",
  },
});
watch(invoices, (rows) => invoiceGridApi.setGridOptions({ data: rows }), {
  deep: true,
});
watch(redTasks, (rows) => redGridApi.setGridOptions({ data: rows }), {
  deep: true,
});
function searchSchema(redLetter: boolean): VbenFormSchema[] {
  return [
    { component: "Input", fieldName: "customerNo", label: t("common.customer"), componentProps: { allowClear: true, placeholder: t("filter.customer") } },
    { component: "Select", fieldName: "statusCode", label: t("common.status"), componentProps: { allowClear: true, placeholder: t("filter.status"), options: (redLetter ? redStatuses : invoiceStatuses).map((value) => ({ value, label: label(redLetter ? "redStatus" : "status", value) })) } },
  ];
}
watch(
  () => preferences.app.locale,
  () => {
    invoiceGridApi.setGridOptions({ columns: invoiceColumns() });
    invoiceGridApi.setState({ formOptions: { schema: searchSchema(false) } });
    redGridApi.setState({ formOptions: { schema: searchSchema(true) } });
    redGridApi.setGridOptions({ columns: redColumns() });
  },
);
function invoiceAllocationColumns(): VxeGridProps<any>["columns"] {
  return [
    {
      title: t("invoice.billingInvoice"),
      field: "billingInvoiceNo",
    },
    { title: t("invoice.order"), field: "orderNo" },
    {
      title: t("common.amount"),
      field: "grossAmountMinor",
      slots: { default: "amount" },
    },
  ];
}
function invoiceLineColumns(): VxeGridProps<any>["columns"] {
  return [
    { title: t("invoice.line"), field: "lineNo" },
    {
      title: t("invoice.classification"),
      field: "taxClassificationCode",
    },
    {
      title: t("invoice.rate"),
      field: "taxRatePpm",
      slots: { default: "rate" },
    },
    {
      title: t("common.amount"),
      field: "grossAmountMinor",
      slots: { default: "amount" },
    },
  ];
}
function redAllocationColumns(): VxeGridProps<any>["columns"] {
  return [
    { title: t("invoice.no"), field: "taxInvoiceNo" },
    {
      title: t("red.refundAllocation"),
      field: "refundAllocationNo",
    },
    {
      title: t("common.amount"),
      field: "grossAmountMinor",
      slots: { default: "amount" },
    },
  ];
}
const [InvoiceAllocationGrid, invoiceAllocationGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: invoiceAllocationColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 280,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "allocationNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-tax-invoice-allocations",
  },
});
const [InvoiceLineGrid, invoiceLineGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: invoiceLineColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 280,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "itemNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-tax-invoice-lines",
  },
});
const [RedAllocationGrid, redAllocationGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: redAllocationColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 280,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "taxInvoiceItemNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-red-letter-allocations",
  },
});
watch(
  () => invoiceDetail.value?.allocations || [],
  (rows) => invoiceAllocationGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => invoiceDetail.value?.items || [],
  (rows) => invoiceLineGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => redDetail.value?.allocations || [],
  (rows) => redAllocationGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
const candidateGroups = computed(() => {
  const groups = new Map<string, RedLetterCandidate[]>();
  for (const item of redDetail.value?.candidates ?? [])
    groups.set(item.refundAllocationNo, [
      ...(groups.get(item.refundAllocationNo) ?? []),
      item,
    ]);
  return [...groups.entries()];
});

function t(key: string) {
  return $t(`plugin.idc-compliance.taxInvoiceManagement.${key}`);
}
function label(prefix: string, code?: string) {
  if (!code) return "—";
  return t(`${prefix}.${code}`);
}
function money(value: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(value / 100);
}
function date(value?: number) {
  return value ? new Date(value).toLocaleString() : "—";
}
function tone(code: string) {
  if (["issued", "delivered", "completed"].includes(code)) return "green";
  if (["failed", "rejected"].includes(code)) return "red";
  if (
    [
      "pending_review",
      "result_unknown",
      "manual_required",
      "awaiting_confirmation",
    ].includes(code)
  )
    return "orange";
  return "blue";
}

async function loadInvoices(reset = false, currentPage = invoicePage.value.currentPage, pageSize = invoicePage.value.pageSize) {
  if (!canInvoiceQuery.value) return;
  const sequence = ++invoiceLoadSequence;
  const filters = { customerNo: invoiceCustomer.value || undefined, statusCode: invoiceStatus.value || undefined };
  invoiceLoading.value = true;
  try {
    await invoicePager.query({ currentPage: reset ? 1 : currentPage, pageSize }, (page, size) => taxInvoiceList({ ...filters, page, pageSize: size }));
  } finally {
    if (sequence === invoiceLoadSequence) invoiceLoading.value = false;
  }
}
async function loadRed(reset = false, currentPage = redPage.value.currentPage, pageSize = redPage.value.pageSize) {
  if (!canRedQuery.value) return;
  const sequence = ++redLoadSequence;
  const filters = { customerNo: redCustomer.value || undefined, statusCode: redStatus.value || undefined };
  redLoading.value = true;
  try {
    await redPager.query({ currentPage: reset ? 1 : currentPage, pageSize }, (page, size) => redLetterList({ ...filters, page, pageSize: size }));
  } finally {
    if (sequence === redLoadSequence) redLoading.value = false;
  }
}
async function openInvoice(no: string) {
  const sequence = ++detailSequence;
  invoiceDrawer.value = true;
  detailLoading.value = true;
  try {
    const result = await taxInvoiceGet(no);
    if (sequence === detailSequence) invoiceDetail.value = result;
  } finally {
    if (sequence === detailSequence) detailLoading.value = false;
  }
}
async function openRed(no: string) {
  const sequence = ++detailSequence;
  redDrawer.value = true;
  detailLoading.value = true;
  try {
    const result = await redLetterGet(no);
    if (sequence === detailSequence) redDetail.value = result;
  } finally {
    if (sequence === detailSequence) detailLoading.value = false;
  }
}
async function refreshInvoice() {
  if (invoiceDetail.value)
    await openInvoice(invoiceDetail.value.item.taxInvoiceNo);
  await loadInvoices();
}
async function refreshRed() {
  if (redDetail.value) await openRed(redDetail.value.item.taskNo);
  await loadRed();
}

async function approve(selected: unknown) {
  const type = selected as string;
  const item = invoiceDetail.value?.item;
  if (!item) return;
  acting.value = true;
  try {
    const res = await taxInvoiceApprove(item.taxInvoiceNo, type, item.version);
    if (!res.result.applied)
      message.warning(
        label("capability", res.result.item.capabilityReasonCode),
      );
    else message.success(t("message.approved"));
    await refreshInvoice();
  } finally {
    acting.value = false;
  }
}
async function submitInvoice() {
  const item = invoiceDetail.value?.item;
  if (!item) return;
  AntModal.confirm({
    title: t("confirm.submitTitle"),
    content: t("confirm.submitBody"),
    okType: "primary",
    async onOk() {
      acting.value = true;
      try {
        const res = await taxInvoiceSubmit(item.taxInvoiceNo, item.version);
        if (!res.result.applied)
          message.warning(
            label("capability", res.result.item.capabilityReasonCode),
          );
        else message.success(t("message.submitted"));
        await refreshInvoice();
      } finally {
        acting.value = false;
      }
    },
  });
}
async function openDecision(action: "cancel" | "reject") {
  decisionAction.value = action;
  decisionModal.value = true;
  await nextTick();
  try {
    await decisionFormApi.setState({ schema: decisionFormSchema() });
    await decisionFormApi.resetForm();
  } catch (error) {
    decisionModal.value = false;
    throw error;
  }
}
async function saveDecision() {
  const item = invoiceDetail.value?.item;
  if (!item) return;
  const { valid } = await decisionFormApi.validate();
  if (!valid) return;
  const values = await decisionFormApi.getValues<{
    internalNote?: string;
    reasonCode: string;
  }>();
  acting.value = true;
  try {
    if (decisionAction.value === "reject")
      await taxInvoiceReject(
        item.taxInvoiceNo,
        values.reasonCode,
        values.internalNote || "",
        item.version,
      );
    else
      await taxInvoiceCancel(
        item.taxInvoiceNo,
        values.reasonCode,
        values.internalNote || "",
        item.version,
      );
    decisionModal.value = false;
    message.success(t(`message.${decisionAction.value}ed`));
    await refreshInvoice();
  } finally {
    acting.value = false;
  }
}
function openAllocation() {
  Object.keys(selectionByRefund).forEach(
    (key) => delete selectionByRefund[key],
  );
  allocationModal.value = true;
}
async function saveAllocation() {
  const item = redDetail.value?.item;
  if (!item || candidateGroups.value.some(([key]) => !selectionByRefund[key]))
    return;
  const selections = candidateGroups.value.map(([refundNo, candidates]) => {
    const candidate = candidates.find(
      (value) => value.taxInvoiceAllocationNo === selectionByRefund[refundNo],
    )!;
    return {
      refundAllocationNo: refundNo,
      taxInvoiceAllocationNo: candidate.taxInvoiceAllocationNo,
      grossAmountMinor: candidate.requestedGrossMinor,
    };
  });
  acting.value = true;
  try {
    await redLetterResolve(item.taskNo, item.version, selections);
    allocationModal.value = false;
    message.success(t("message.allocated"));
    await refreshRed();
  } finally {
    acting.value = false;
  }
}
async function openEvidence() {
  evidenceModal.value = true;
  await nextTick();
  try {
    await evidenceFormApi.setState({ schema: evidenceFormSchema() });
    await evidenceFormApi.resetForm();
    await evidenceFormApi.setValues({
      occurredAt: new Date().toISOString().slice(0, 16),
    });
  } catch (error) {
    evidenceModal.value = false;
    throw error;
  }
}
async function saveEvidence() {
  const item = redDetail.value?.item;
  if (!item) return;
  const { valid } = await evidenceFormApi.validate();
  if (!valid) return;
  const values = await evidenceFormApi.getValues<{
    evidenceReference: string;
    evidenceSha256: string;
    occurredAt: string;
  }>();
  acting.value = true;
  try {
    await redLetterComplete(item.taskNo, {
      expectedVersion: item.version,
      evidenceReference: values.evidenceReference,
      evidenceSha256: values.evidenceSha256,
      occurredAt: new Date(values.occurredAt).getTime(),
    });
    evidenceModal.value = false;
    message.success(t("message.completed"));
    await refreshRed();
  } finally {
    acting.value = false;
  }
}

onMounted(() => {
  invoiceGridApi.setState({ formOptions: { schema: searchSchema(false) } });
  redGridApi.setState({ formOptions: { schema: searchSchema(true) } });
  loadInvoices(true);
  loadRed(true);
});
</script>

<template>
  <Page :auto-content-height="true" class="min-w-0 w-full" content-class="min-h-0 min-w-0" data-testid="tax-invoice-management-page">
    <div class="invoice-shell">
      <header class="page-heading">
        <span>{{ t("eyebrow") }}</span>
        <h1>{{ t("title") }}</h1>
        <p>{{ t("subtitle") }}</p>
      </header>
      <Card class="invoice-directory" :title="redLetterRoute ? t('tabs.redLetters') : t('tabs.invoices')">
        <template #extra
          ><Tag color="orange">{{ t("providerClosed") }}</Tag></template
        >
        <Alert
          :message="t('gateTitle')"
          :description="t('gateDescription')"
          type="warning"
          show-icon
        />
        <Tabs v-model:active-key="activeTab" class="invoice-tabs">
          <TabPane
            v-if="canInvoiceQuery"
            key="invoices"
            :tab="t('tabs.invoices')"
          >
            <InvoiceGrid :loading="invoiceLoading" class="min-h-0 min-w-0">
              <template #status="{ row }"
                ><Tag :color="tone(row.statusCode)">{{
                  label("status", row.statusCode)
                }}</Tag></template
              >
              <template #amount="{ row }">{{
                money(row.grossAmountMinor, row.currencyCode)
              }}</template>
              <template #type="{ row }">{{
                label("type", row.invoiceTypeCode)
              }}</template>
              <template #time="{ row }">{{ date(row.submittedAt) }}</template>
              <template #actions="{ row }"
                ><Button
                  :data-testid="`tax-invoice-view-${row.taxInvoiceNo}`"
                  type="link"
                  @click="openInvoice(row.taxInvoiceNo)"
                  >{{ t("actions.view") }}</Button
                ></template
              >
            </InvoiceGrid>
          </TabPane>
          <TabPane
            v-if="canRedQuery"
            key="redLetters"
            :tab="t('tabs.redLetters')"
          >
            <RedGrid :loading="redLoading" class="min-h-0 min-w-0">
              <template #status="{ row }"
                ><Tag :color="tone(row.statusCode)">{{
                  label("redStatus", row.statusCode)
                }}</Tag></template
              >
              <template #amount="{ row }">{{
                money(row.grossAmountMinor, row.currencyCode)
              }}</template>
              <template #confirmation="{ row }">{{
                row.customerConfirmationRequired
                  ? t("common.required")
                  : t("common.notRequired")
              }}</template>
              <template #time="{ row }">{{ date(row.createdAt) }}</template>
              <template #actions="{ row }"
                ><Button
                  :data-testid="`red-letter-view-${row.taskNo}`"
                  type="link"
                  @click="openRed(row.taskNo)"
                  >{{ t("actions.view") }}</Button
                ></template
              >
            </RedGrid>
          </TabPane>
        </Tabs>
      </Card>
    </div>

    <VbenDrawerCompat
      v-model:open="invoiceDrawer"
      :title="t('invoice.detailTitle')"
      width="min(920px, 96vw)"
      :loading="detailLoading"
    >
      <template v-if="invoiceDetail"
        ><div data-testid="tax-invoice-detail">
          <Alert
            v-if="!invoiceDetail.item.capabilityReady"
            :message="t('gateTitle')"
            :description="
              label('capability', invoiceDetail.item.capabilityReasonCode)
            "
            type="warning"
            show-icon
          />
          <Descriptions bordered :column="2" size="small"
            ><DescriptionsItem :label="t('invoice.no')">{{
              invoiceDetail.item.taxInvoiceNo
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.status')"
              ><Tag :color="tone(invoiceDetail.item.statusCode)">{{
                label("status", invoiceDetail.item.statusCode)
              }}</Tag></DescriptionsItem
            ><DescriptionsItem :label="t('common.customer')">{{
              invoiceDetail.item.customerNo
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.amount')">{{
              money(
                invoiceDetail.item.grossAmountMinor,
                invoiceDetail.item.currencyCode,
              )
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.type')">{{
              label("type", invoiceDetail.item.invoiceTypeCode)
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.maskedNumber')">{{
              invoiceDetail.item.maskedInvoiceNumber || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.netTax')"
              >{{
                money(
                  invoiceDetail.item.netAmountMinor,
                  invoiceDetail.item.currencyCode,
                )
              }}
              /
              {{
                money(
                  invoiceDetail.item.taxAmountMinor,
                  invoiceDetail.item.currencyCode,
                )
              }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.submittedAt')">{{
              date(invoiceDetail.item.submittedAt)
            }}</DescriptionsItem></Descriptions
          >
          <Divider>{{ t("invoice.buyerProfile") }}</Divider
          ><Descriptions
            bordered
            :column="2"
            size="small"
            data-testid="tax-invoice-buyer-profile"
            ><DescriptionsItem :label="t('invoice.titleType')">{{
              label("titleType", invoiceDetail.buyer.titleTypeCode)
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.profileNo')">{{
              invoiceDetail.buyer.profileNo
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.invoiceTitle')">{{
              invoiceDetail.buyer.invoiceTitle
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.taxRegistrationNo')">{{
              invoiceDetail.buyer.taxRegistrationNo || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.registeredAddress')">{{
              invoiceDetail.buyer.registeredAddress || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.registeredPhone')">{{
              invoiceDetail.buyer.registeredPhone || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.bankName')">{{
              invoiceDetail.buyer.bankName || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.bankAccountMasked')">{{
              invoiceDetail.buyer.bankAccountMasked || "—"
            }}</DescriptionsItem></Descriptions
          >
          <Divider>{{ t("invoice.allocations") }}</Divider
          ><InvoiceAllocationGrid
            ><template #amount="{ row }">{{
              money(row.grossAmountMinor, invoiceDetail.item.currencyCode)
            }}</template></InvoiceAllocationGrid
          >
          <Divider>{{ t("invoice.taxLines") }}</Divider
          ><InvoiceLineGrid
            ><template #rate="{ row }"
              >{{ (row.taxRatePpm / 10000).toFixed(2) }}%</template
            ><template #amount="{ row }">{{
              money(row.grossAmountMinor, invoiceDetail.item.currencyCode)
            }}</template></InvoiceLineGrid
          >
          <Divider>{{ t("invoice.files") }}</Divider
          ><Empty
            v-if="!invoiceDetail.files.length"
            :description="t('invoice.noFiles')"
          />
          <div
            v-for="file in invoiceDetail.files"
            :key="file.fileNo"
            class="evidence-card"
          >
            <strong>{{ label("fileType", file.fileTypeCode) }}</strong
            ><span>{{ file.mediaType }} · {{ file.byteSize }} B</span
            ><code>{{ file.contentSha256 }}</code>
          </div>
          <Alert
            v-if="invoiceDetail.historyTruncated"
            :message="t('common.truncated')"
            type="warning"
            show-icon
          />
          <Divider /><Space wrap
            ><Select
              v-if="
                canInvoiceReview &&
                invoiceDetail.item.statusCode === 'pending_review'
              "
              data-testid="tax-invoice-approve-select"
              :placeholder="t('actions.approve')"
              :disabled="!invoiceDetail.item.capabilityReady"
              style="width: 240px"
              @change="approve"
              ><SelectOption
                v-for="type in invoiceDetail.item.supportedInvoiceTypes"
                :key="type"
                :value="type"
                >{{ label("type", type) }}</SelectOption
              ></Select
            ><Button
              v-if="
                canInvoiceReview &&
                invoiceDetail.item.statusCode === 'pending_review'
              "
              data-testid="tax-invoice-reject"
              danger
              @click="openDecision('reject')"
              >{{ t("actions.reject") }}</Button
            ><Button
              v-if="
                canInvoiceSubmit &&
                invoiceDetail.item.statusCode === 'approved_pending_submission'
              "
              data-testid="tax-invoice-submit"
              type="primary"
              :disabled="!invoiceDetail.item.capabilityReady"
              @click="submitInvoice"
              >{{ t("actions.submit") }}</Button
            ><Button
              v-if="
                canInvoiceCancel &&
                (['pending_review', 'approved_pending_submission'].includes(
                  invoiceDetail.item.statusCode,
                ) ||
                  (invoiceDetail.item.statusCode === 'failed' &&
                    invoiceDetail.item.providerNotIssuedAt > 0))
              "
              data-testid="tax-invoice-cancel"
              danger
              @click="openDecision('cancel')"
              >{{ t("actions.cancel") }}</Button
            ></Space
          >
        </div>
      </template>
    </VbenDrawerCompat>

    <VbenDrawerCompat
      v-model:open="redDrawer"
      :title="t('red.detailTitle')"
      width="min(920px, 96vw)"
      :loading="detailLoading"
      ><template v-if="redDetail"
        ><div data-testid="red-letter-detail">
          <Descriptions bordered :column="2" size="small"
            ><DescriptionsItem :label="t('red.no')">{{
              redDetail.item.taskNo
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.status')"
              ><Tag :color="tone(redDetail.item.statusCode)">{{
                label("redStatus", redDetail.item.statusCode)
              }}</Tag></DescriptionsItem
            ><DescriptionsItem :label="t('red.refund')">{{
              redDetail.item.refundOperationNo
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('invoice.no')">{{
              redDetail.item.taxInvoiceNo || "—"
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.amount')">{{
              money(
                redDetail.item.grossAmountMinor,
                redDetail.item.currencyCode,
              )
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('red.confirmation')">{{
              redDetail.item.customerConfirmationRequired
                ? t("red.customerMustConfirm")
                : t("common.notRequired")
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('red.manualReason')">{{
              label("manualReason", redDetail.item.manualReasonCode)
            }}</DescriptionsItem
            ><DescriptionsItem :label="t('common.createdAt')">{{
              date(redDetail.item.createdAt)
            }}</DescriptionsItem></Descriptions
          >
          <Alert
            v-if="redDetail.item.customerConfirmationRequired"
            :message="t('red.customerConfirmationTitle')"
            :description="t('red.customerConfirmationBody')"
            type="warning"
            show-icon
          />
          <Divider>{{ t("red.allocations") }}</Divider
          ><RedAllocationGrid
            ><template #amount="{ row }">{{
              money(row.grossAmountMinor, redDetail.item.currencyCode)
            }}</template></RedAllocationGrid
          >
          <template v-if="redDetail.evidence"
            ><Divider>{{ t("red.evidence") }}</Divider>
            <div class="evidence-card">
              <strong>{{ redDetail.evidence.evidenceReference }}</strong
              ><span>{{ date(redDetail.evidence.occurredAt) }}</span
              ><code>{{ redDetail.evidence.evidenceSha256 }}</code>
            </div></template
          >
          <Divider /><Space
            ><Button
              v-if="canRedHandle && !redDetail.item.allocationResolved"
              data-testid="red-letter-resolve"
              type="primary"
              @click="openAllocation"
              >{{ t("actions.resolveAllocation") }}</Button
            ><Button
              v-if="
                canRedHandle &&
                redDetail.item.statusCode === 'manual_required' &&
                redDetail.item.allocationResolved
              "
              data-testid="red-letter-complete"
              type="primary"
              @click="openEvidence"
              >{{ t("actions.completeWithEvidence") }}</Button
            ></Space
          >
        </div>
      </template></VbenDrawerCompat
    >

    <VbenModalCompat
      v-model:open="decisionModal"
      :title="t(`decision.${decisionAction}Title`)"
      :confirm-loading="acting"
      :ok-button-props="{ danger: true }"
      @ok="saveDecision"
    >
      <DecisionForm />
      <Alert
        :message="t('decision.customerVisibility')"
        type="info"
        show-icon
      />
      ></VbenModalCompat
    >
    <VbenModalCompat
      v-model:open="allocationModal"
      :title="t('red.allocateTitle')"
      :confirm-loading="acting"
      @ok="saveAllocation"
      ><Alert :message="t('red.allocateWarning')" type="warning" show-icon />
      <div
        v-for="[refundNo, candidates] in candidateGroups"
        :key="refundNo"
        class="candidate-group"
      >
        <strong>{{ refundNo }}</strong
        ><RadioGroup v-model:value="selectionByRefund[refundNo]"
          ><Space direction="vertical"
            ><Radio
              v-for="candidate in candidates"
              :key="candidate.taxInvoiceAllocationNo"
              :value="candidate.taxInvoiceAllocationNo"
              >{{ candidate.taxInvoiceNo }} ·
              {{
                money(
                  candidate.requestedGrossMinor,
                  redDetail!.item.currencyCode,
                )
              }}</Radio
            ></Space
          ></RadioGroup
        >
      </div></VbenModalCompat
    >
    <VbenModalCompat
      v-model:open="evidenceModal"
      :title="t('red.evidenceTitle')"
      :confirm-loading="acting"
      @ok="saveEvidence"
    >
      <Alert :message="t('red.evidenceWarning')" type="warning" show-icon />
      <EvidenceForm />
      ></VbenModalCompat
    >
  </Page>
</template>

<style scoped>
.invoice-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.invoice-directory { display: flex; flex: 0 0 526px; flex-direction: column; min-height: 0; min-width: 0; overflow: hidden; }
.invoice-directory :deep(.ant-card-head) { flex: none; }
.invoice-directory :deep(.ant-card-body) { display: flex; flex: 1; flex-direction: column; min-height: 0; min-width: 0; padding: 8px; }
.invoice-directory :deep(.ant-alert) { flex: none; }
.invoice-directory :deep(.ant-alert-with-description) { padding-block: 12px; }
.invoice-tabs { flex: 1; min-height: 0; min-width: 0; }
.invoice-tabs :deep(.ant-tabs-nav) { flex: none; margin-bottom: 8px; }
.invoice-tabs :deep(.ant-tabs-content-holder),
.invoice-tabs :deep(.ant-tabs-content),
.invoice-tabs :deep(.ant-tabs-tabpane-active) { height: 100%; min-height: 0; min-width: 0; }
.invoice-tabs :deep(.ant-tabs-content-holder) { overflow: hidden; }
.page-heading {
  flex: none;
  border-left: 3px solid #1677ff;
  min-width: 0;
  padding: 2px 0 2px 14px;
}
.page-heading span {
  color: #1677ff;
  display: block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  margin-bottom: 4px;
}
.page-heading h1 {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.35;
  margin: 0;
}
.page-heading p {
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
  margin: 6px 0 0;
  max-width: 880px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
}
.evidence-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px;
  margin-bottom: 10px;
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  background: hsl(var(--muted) / 0.35);
}
.evidence-card code {
  word-break: break-all;
  font-size: 12px;
}
.candidate-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 16px;
  padding: 12px;
  border: 1px solid hsl(var(--border));
  border-radius: 10px;
}
@media (max-width: 720px) {
}
</style>
