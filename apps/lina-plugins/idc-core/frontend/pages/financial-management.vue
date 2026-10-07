<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/finance",
  title: "Financial Management",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "#/adapter/form";
import type { VxeGridProps } from "#/adapter/vxe-table";

import type {
  FinancialAdjustment,
  FinancialAdjustmentDetail,
  FinancialInvoice,
  FinancialInvoiceDetail,
  FinancialOverview,
  FinancialTransaction,
  FinancialTransactionDetail,
} from "./financial-client";

import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";

import { useAccess } from "./plugin-access";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Form,
  FormItem,
  Input,
  InputNumber,
  message,
  Radio,
  RadioGroup,
  Select,
  Space,
  Statistic,
  Tabs,
  TabPane,
  Tag,
  Textarea,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { createNumberedPage } from "./numbered-page";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import VbenModalCompat from "./components/vben-modal-compat.vue";

import {
  configureFinancialAdjustmentPolicy,
  createFinancialInvoiceWaiver,
  createFinancialManualReceipt,
  financialAdjustmentDetail,
  financialAdjustments,
  financialInvoiceDetail,
  financialInvoices,
  financialOverview,
  financialTransactionDetail,
  financialTransactions,
  reviewFinancialAdjustment,
} from "./financial-client";

import FinancialOperationsPanel from "./components/financial-operations-panel.vue";
import RefundManagementPanel from "./components/refund-management-panel.vue";
import { useSourcePluginContext } from "./source-context";
import {
  customerDisplay,
  searchCustomerOptions,
  type CustomerSelectOption,
} from "./customer-selection";

type ActionKind = "manual_receipt" | "invoice_waiver" | "policy" | "review";
type DetailKind = "adjustment" | "invoice" | "transaction";

const { hasAccessByCodes } = useAccess();
const FinanceWorkspace = defineAsyncComponent(
  () => import("./components/credit-management-workspace.vue"),
);
const sourcePluginContext = useSourcePluginContext();
const canViewCredit = computed(() =>
  [
    "idc:admin:billing:credit:application:list",
    "idc:admin:billing:credit:account:list",
    "idc:admin:billing:credit:statement:list",
  ].some((code) => hasAccessByCodes([code])),
);
const canCreateReceipt = computed(() =>
  hasAccessByCodes(["idc:admin:billing:manual-receipt:create"]),
);
const canCreateWaiver = computed(() =>
  hasAccessByCodes(["idc:admin:billing:invoice-waiver:create"]),
);
const canReview = computed(() =>
  hasAccessByCodes(["idc:admin:billing:adjustment:review"]),
);
const canWritePolicy = computed(() =>
  hasAccessByCodes(["idc:admin:billing:adjustment-policy:write"]),
);

const overview = ref<FinancialOverview>();
const overviewLoading = ref(false);
const overviewError = ref("");
const activeTab = ref("invoices");
const operationsViewCode = computed<
  "adjustment" | "invoice" | "receivable" | "transaction"
>(() => {
  if (activeTab.value === "transactions") return "transaction";
  if (activeTab.value === "adjustments") return "adjustment";
  return "invoice";
});
const operationsFilters = reactive<Record<string, Record<string, unknown>>>({
  adjustment: {},
  invoice: {},
  transaction: {},
});
const operationsCurrentFilter = computed(
  () => operationsFilters[operationsViewCode.value] ?? {},
);

const invoiceRows = ref<FinancialInvoice[]>([]);
const invoiceLoading = ref(false);
const invoiceError = ref("");
const invoicePage = ref(1);
const invoiceSize = ref(10);
let invoiceSequence = 0;
const invoicePagination = createNumberedPage<FinancialInvoice>(
  (state) => {
    invoiceRows.value = state.items;
    invoicePage.value = state.currentPage;
    invoiceSize.value = state.pageSize;
    invoiceGridApi.setGridOptions({
      pagerConfig: {
        enabled: true,
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    });
  },
  () => {
    detailOpen.value = false;
    actionOpen.value = false;
    invoiceDetail.value = undefined;
    actionTarget.value = undefined;
  },
);

const transactionRows = ref<FinancialTransaction[]>([]);
const transactionLoading = ref(false);
const transactionError = ref("");
const transactionPage = ref(1);
const transactionSize = ref(10);
let transactionSequence = 0;
const transactionPagination = createNumberedPage<FinancialTransaction>(
  (state) => {
    transactionRows.value = state.items;
    transactionPage.value = state.currentPage;
    transactionSize.value = state.pageSize;
    transactionGridApi.setGridOptions({
      pagerConfig: {
        enabled: true,
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    });
  },
  () => {
    detailOpen.value = false;
    actionOpen.value = false;
    transactionDetail.value = undefined;
    actionTarget.value = undefined;
  },
);

const adjustmentRows = ref<FinancialAdjustment[]>([]);
const adjustmentLoading = ref(false);
const adjustmentError = ref("");
const adjustmentPage = ref(1);
const adjustmentSize = ref(10);
let adjustmentSequence = 0;
const adjustmentPagination = createNumberedPage<FinancialAdjustment>(
  (state) => {
    adjustmentRows.value = state.items;
    adjustmentPage.value = state.currentPage;
    adjustmentSize.value = state.pageSize;
    adjustmentGridApi.setGridOptions({
      pagerConfig: {
        enabled: true,
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    });
  },
  () => {
    detailOpen.value = false;
    actionOpen.value = false;
    adjustmentDetail.value = undefined;
    actionTarget.value = undefined;
  },
);

function savedFilterValue(filter: Record<string, unknown>, key: string) {
  const value = filter[key];
  return typeof value === "string" ? value : "";
}

async function applyOperationsFilter(filter: Record<string, unknown>) {
  if (operationsViewCode.value === "transaction") {
    await transactionGridApi.formApi.setValues({
      customerNo: savedFilterValue(filter, "customerNo"),
      currencyCode: savedFilterValue(filter, "currencyCode"),
      transactionTypeCode: savedFilterValue(filter, "typeCode"),
    });
    await loadTransactions(1);
    return;
  }
  if (operationsViewCode.value === "adjustment") {
    await adjustmentGridApi.formApi.setValues({
      customerNo: savedFilterValue(filter, "customerNo"),
      currencyCode: savedFilterValue(filter, "currencyCode"),
      statusCode: savedFilterValue(filter, "statusCode"),
      adjustmentTypeCode: savedFilterValue(filter, "typeCode"),
    });
    await loadAdjustments(1);
    return;
  }
  await invoiceGridApi.formApi.setValues({
    customerNo: savedFilterValue(filter, "customerNo"),
    currencyCode: savedFilterValue(filter, "currencyCode"),
    statusCode: savedFilterValue(filter, "statusCode"),
  });
  await loadInvoices(1);
}

const detailOpen = ref(false);
const detailLoading = ref(false);
const detailError = ref("");
const detailKind = ref<DetailKind>("invoice");
const invoiceDetail = ref<FinancialInvoiceDetail>();
const transactionDetail = ref<FinancialTransactionDetail>();
const adjustmentDetail = ref<FinancialAdjustmentDetail>();

const actionOpen = ref(false);
const actionLoading = ref(false);
const actionError = ref("");
const actionKind = ref<ActionKind>("manual_receipt");
const actionTarget = ref<FinancialAdjustment | FinancialInvoice>();
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);
const form = reactive({
  amountMinor: "",
  businessPurposeCode: "invoice_payment",
  comment: "",
  currencyCode: "CNY",
  customerNo: "",
  decisionCode: "approved" as "approved" | "rejected",
  evidence: "",
  invoiceNo: "",
  proofReference: "",
  reasonCode: "offline_transfer",
  receivingAccountReference: "",
  requestNo: "",
  reviewThresholdMinor: "",
  targetMode: "invoice" as "balance" | "invoice",
});

function invoiceColumns(): VxeGridProps<FinancialInvoice>["columns"] {
  return [
    {
      field: "invoiceNo",
      slots: { default: "invoiceNo" },
      title: t("table.invoice"),
      width: 230,
    },
    {
      field: "customerDisplayName",
      slots: { default: "customer" },
      title: t("table.customer"),
      width: 210,
    },
    {
      field: "statusCode",
      slots: { default: "status" },
      title: t("table.status"),
      width: 138,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: t("table.receivable"),
      width: 176,
    },
    {
      field: "paymentDueAt",
      slots: { default: "dueAt" },
      title: t("table.dueAt"),
      width: 176,
    },
    {
      field: "action",
      slots: { default: "action" },
      title: t("detail.button"),
      width: 176,
      fixed: "right",
    },
  ];
}

function transactionColumns(): VxeGridProps<FinancialTransaction>["columns"] {
  return [
    {
      field: "transactionNo",
      slots: { default: "transactionNo" },
      title: t("table.transaction"),
      width: 230,
    },
    {
      field: "transactionTypeCode",
      slots: { default: "type" },
      title: t("table.type"),
      width: 170,
    },
    {
      field: "customerDisplayName",
      slots: { default: "customer" },
      title: t("table.customer"),
      width: 210,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: t("table.amount"),
      width: 176,
    },
    {
      field: "postedAt",
      slots: { default: "postedAt" },
      title: t("table.postedAt"),
      width: 176,
    },
    {
      field: "action",
      slots: { default: "action" },
      title: t("detail.button"),
      width: 92,
      fixed: "right",
    },
  ];
}

function adjustmentColumns(): VxeGridProps<FinancialAdjustment>["columns"] {
  return [
    {
      field: "adjustmentNo",
      slots: { default: "adjustmentNo" },
      title: t("table.adjustment"),
      width: 230,
    },
    {
      field: "adjustmentTypeCode",
      slots: { default: "type" },
      title: t("table.type"),
      width: 165,
    },
    {
      field: "customerDisplayName",
      slots: { default: "customer" },
      title: t("table.customer"),
      width: 210,
    },
    {
      field: "statusCode",
      slots: { default: "status" },
      title: t("table.status"),
      width: 140,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: t("table.amount"),
      width: 176,
    },
    {
      field: "createdAt",
      slots: { default: "createdAt" },
      title: t("table.createdAt"),
      width: 176,
    },
    {
      field: "action",
      slots: { default: "action" },
      title: t("detail.button"),
      width: 150,
      fixed: "right",
    },
  ];
}

function customerField() {
  return {
    component: "Select",
    componentProps: {
      allowClear: true,
      filterOption: false,
      loading: customerLoading.value,
      onSearch: searchCustomers,
      options: customerOptions.value,
      showSearch: true,
    },
    fieldName: "customerNo",
    label: t("table.customer"),
  };
}
const invoiceStatusOptions = computed(() =>
  optionList([
    "draft",
    "pending_payment",
    "partially_paid",
    "paid",
    "overdue",
    "cancelled",
    "bad_debt",
  ]),
);
const adjustmentStatusOptions = computed(() =>
  optionList(["pending_review", "posted", "rejected"]),
);
const adjustmentTypeOptions = computed(() =>
  optionList(["manual_receipt", "invoice_waiver"]),
);
const transactionTypeOptions = computed(() =>
  optionList([
    "invoice_issuance",
    "recharge",
    "payment",
    "balance_payment",
    "refund",
    "reversal",
    "manual_receipt",
    "writeoff",
    "gift",
    "credit",
    "migration",
  ]),
);
function invoiceSearchSchema(): VbenFormProps["schema"] {
  return [
    customerField(),
    {
      component: "Select",
      componentProps: { options: invoiceStatusOptions.value },
      fieldName: "statusCode",
      label: t("table.status"),
    },
    {
      component: "Input",
      fieldName: "currencyCode",
      label: t("filter.currency"),
    },
  ];
}
function transactionSearchSchema(): VbenFormProps["schema"] {
  return [
    customerField(),
    {
      component: "Select",
      componentProps: { options: transactionTypeOptions.value },
      fieldName: "transactionTypeCode",
      label: t("filter.transactionType"),
    },
    {
      component: "Input",
      fieldName: "currencyCode",
      label: t("filter.currency"),
    },
  ];
}
function adjustmentSearchSchema(): VbenFormProps["schema"] {
  return [
    customerField(),
    {
      component: "Select",
      componentProps: { options: adjustmentTypeOptions.value },
      fieldName: "adjustmentTypeCode",
      label: t("table.type"),
    },
    {
      component: "Select",
      componentProps: { options: adjustmentStatusOptions.value },
      fieldName: "statusCode",
      label: t("table.status"),
    },
    {
      component: "Input",
      fieldName: "currencyCode",
      label: t("filter.currency"),
    },
  ];
}

const [InvoiceGrid, invoiceGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize,
    }: {
      currentPage: number;
      pageSize: number;
    }) => void loadInvoices(currentPage, pageSize),
  },
  formOptions: {
    handleReset: searchInvoices,
    handleSubmit: searchInvoices,
    schema: invoiceSearchSchema(),
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    columns: invoiceColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 420,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "invoiceNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-financial-invoices",
  },
});
const [TransactionGrid, transactionGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize,
    }: {
      currentPage: number;
      pageSize: number;
    }) => void loadTransactions(currentPage, pageSize),
  },
  formOptions: {
    handleReset: searchTransactions,
    handleSubmit: searchTransactions,
    schema: transactionSearchSchema(),
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    columns: transactionColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 420,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "transactionNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-financial-transactions",
  },
});
const [AdjustmentGrid, adjustmentGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize,
    }: {
      currentPage: number;
      pageSize: number;
    }) => void loadAdjustments(currentPage, pageSize),
  },
  formOptions: {
    handleReset: searchAdjustments,
    handleSubmit: searchAdjustments,
    schema: adjustmentSearchSchema(),
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-4",
  },
  gridOptions: {
    columns: adjustmentColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 420,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "adjustmentNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-financial-adjustments",
  },
});
watch(invoiceRows, (rows) => invoiceGridApi.setGridOptions({ data: rows }), {
  deep: true,
});
watch(
  transactionRows,
  (rows) => transactionGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  adjustmentRows,
  (rows) => adjustmentGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => preferences.app.locale,
  () => {
    invoiceGridApi.setGridOptions({ columns: invoiceColumns() });
    transactionGridApi.setGridOptions({ columns: transactionColumns() });
    adjustmentGridApi.setGridOptions({ columns: adjustmentColumns() });
    invoiceGridApi.setState({ formOptions: { schema: invoiceSearchSchema() } });
    transactionGridApi.setState({
      formOptions: { schema: transactionSearchSchema() },
    });
    adjustmentGridApi.setState({
      formOptions: { schema: adjustmentSearchSchema() },
    });
  },
);

watch(activeTab, async (tab) => {
  // Ant Tabs only mounts an inactive pane after it becomes active. Waiting for
  // that mount keeps the Vben search-form API available before the first query.
  await nextTick();
  if (tab === "transactions") void loadTransactions();
  if (tab === "adjustments") void loadAdjustments();
});

const reasonOptions = computed(() =>
  ["offline_transfer", "service_compensation"].map((value) => ({
    label: t(`reason.${value}`),
    value,
  })),
);

function t(key: string) {
  return $t(`plugin.idc-core.finance.${key}`);
}

function optionList(values: string[]) {
  return [
    { label: t("filter.all"), value: "" },
    ...values.map((value) => ({
      label: statusLabel(value),
      value,
    })),
  ];
}

function statusLabel(value: string) {
  const key = `plugin.idc-core.finance.status.${value}`;
  const translated = $t(key);
  return translated === key ? t("status.unknown") : translated;
}

function directionLabel(value: string) {
  const key = `plugin.idc-core.finance.direction.${value}`;
  const translated = $t(key);
  return translated === key ? t("status.unknown") : translated;
}

function statusColor(value: string) {
  if (["paid", "posted", "approved", "succeeded"].includes(value))
    return "success";
  if (["overdue", "rejected", "bad_debt"].includes(value)) return "error";
  if (["pending_review", "partially_paid"].includes(value)) return "warning";
  if (["cancelled", "draft"].includes(value)) return "default";
  return "processing";
}

function formatMinor(value: string, currency: string) {
  try {
    const digits =
      new Intl.NumberFormat(undefined, {
        currency,
        style: "currency",
      }).resolvedOptions().maximumFractionDigits ?? 2;
    const raw = BigInt(value);
    const divisor = 10n ** BigInt(digits);
    const absolute = raw < 0n ? -raw : raw;
    const fraction = (absolute % divisor).toString().padStart(digits, "0");
    return `${raw < 0n ? "−" : ""}${currency} ${absolute / divisor}${digits ? `.${fraction}` : ""}`;
  } catch {
    return `${currency} —`;
  }
}

function shortNo(value?: string) {
  if (!value) return "—";
  return value.length > 28 ? `${value.slice(0, 14)}…${value.slice(-8)}` : value;
}

async function loadOverview() {
  overviewLoading.value = true;
  overviewError.value = "";
  try {
    overview.value = await financialOverview();
  } catch {
    overviewError.value = t("overview.loadFailed");
  } finally {
    overviewLoading.value = false;
  }
}

async function loadInvoices(
  targetPage = invoicePage.value,
  targetSize = invoiceSize.value,
) {
  const sequence = ++invoiceSequence;
  invoiceLoading.value = true;
  invoiceError.value = "";
  await invoicePagination.query(
    { currentPage: targetPage, pageSize: targetSize },
    async (pageNumber, limit) => {
      try {
        const values = await invoiceGridApi.formApi.getValues();
        operationsFilters.invoice = {
          currencyCode: String(values.currencyCode || "")
            .trim()
            .toUpperCase(),
          customerNo: String(values.customerNo || ""),
          statusCode: String(values.statusCode || ""),
        };
        const page = await financialInvoices({
          currencyCode:
            String(values.currencyCode || "")
              .trim()
              .toUpperCase() || undefined,
          page: pageNumber,
          customerNo: String(values.customerNo || "") || undefined,
          limit,
          statusCode: String(values.statusCode || "") || undefined,
        });
        return { items: page.invoices, total: page.total };
      } catch (failure) {
        if (sequence === invoiceSequence)
          invoiceError.value = t("invoices.loadFailed");
        throw failure;
      } finally {
        if (sequence === invoiceSequence) invoiceLoading.value = false;
      }
    },
  );
}
async function loadTransactions(
  targetPage = transactionPage.value,
  targetSize = transactionSize.value,
) {
  const sequence = ++transactionSequence;
  transactionLoading.value = true;
  transactionError.value = "";
  await transactionPagination.query(
    { currentPage: targetPage, pageSize: targetSize },
    async (pageNumber, limit) => {
      try {
        const values = await transactionGridApi.formApi.getValues();
        operationsFilters.transaction = {
          currencyCode: String(values.currencyCode || "")
            .trim()
            .toUpperCase(),
          customerNo: String(values.customerNo || ""),
          typeCode: String(values.transactionTypeCode || ""),
        };
        const page = await financialTransactions({
          currencyCode:
            String(values.currencyCode || "")
              .trim()
              .toUpperCase() || undefined,
          page: pageNumber,
          customerNo: String(values.customerNo || "") || undefined,
          limit,
          transactionTypeCode:
            String(values.transactionTypeCode || "").trim() || undefined,
        });
        return { items: page.transactions, total: page.total };
      } catch (failure) {
        if (sequence === transactionSequence)
          transactionError.value = t("transactions.loadFailed");
        throw failure;
      } finally {
        if (sequence === transactionSequence) transactionLoading.value = false;
      }
    },
  );
}
async function loadAdjustments(
  targetPage = adjustmentPage.value,
  targetSize = adjustmentSize.value,
) {
  const sequence = ++adjustmentSequence;
  adjustmentLoading.value = true;
  adjustmentError.value = "";
  await adjustmentPagination.query(
    { currentPage: targetPage, pageSize: targetSize },
    async (pageNumber, limit) => {
      try {
        const values = await adjustmentGridApi.formApi.getValues();
        operationsFilters.adjustment = {
          currencyCode: String(values.currencyCode || "")
            .trim()
            .toUpperCase(),
          customerNo: String(values.customerNo || ""),
          statusCode: String(values.statusCode || ""),
          typeCode: String(values.adjustmentTypeCode || ""),
        };
        const page = await financialAdjustments({
          adjustmentTypeCode:
            String(values.adjustmentTypeCode || "") || undefined,
          currencyCode:
            String(values.currencyCode || "")
              .trim()
              .toUpperCase() || undefined,
          page: pageNumber,
          customerNo: String(values.customerNo || "") || undefined,
          limit,
          statusCode: String(values.statusCode || "") || undefined,
        });
        return { items: page.adjustments, total: page.total };
      } catch (failure) {
        if (sequence === adjustmentSequence)
          adjustmentError.value = t("adjustments.loadFailed");
        throw failure;
      } finally {
        if (sequence === adjustmentSequence) adjustmentLoading.value = false;
      }
    },
  );
}
function searchInvoices() {
  void loadInvoices(1);
}
function searchTransactions() {
  void loadTransactions(1);
}
function searchAdjustments() {
  void loadAdjustments(1);
}

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try {
    customerOptions.value = await searchCustomerOptions(keyword);
    // Inactive tabs have no mounted formApi yet; the grid store is available
    // before mount and applies this schema when that tab is first opened.
    invoiceGridApi.setState({ formOptions: { schema: invoiceSearchSchema() } });
    transactionGridApi.setState({
      formOptions: { schema: transactionSearchSchema() },
    });
    adjustmentGridApi.setState({
      formOptions: { schema: adjustmentSearchSchema() },
    });
  } finally {
    customerLoading.value = false;
  }
}

async function openDetail(kind: DetailKind, reference: string) {
  detailKind.value = kind;
  detailOpen.value = true;
  detailLoading.value = true;
  detailError.value = "";
  invoiceDetail.value = undefined;
  transactionDetail.value = undefined;
  adjustmentDetail.value = undefined;
  try {
    if (kind === "invoice")
      invoiceDetail.value = await financialInvoiceDetail(reference);
    if (kind === "transaction")
      transactionDetail.value = await financialTransactionDetail(reference);
    if (kind === "adjustment")
      adjustmentDetail.value = await financialAdjustmentDetail(reference);
  } catch {
    detailError.value = t("detail.loadFailed");
  } finally {
    detailLoading.value = false;
  }
}

function resetActionForm() {
  Object.assign(form, {
    amountMinor: "",
    businessPurposeCode: "invoice_payment",
    comment: "",
    currencyCode: "CNY",
    customerNo: "",
    decisionCode: "approved",
    evidence: "",
    invoiceNo: "",
    proofReference: "",
    reasonCode: "offline_transfer",
    receivingAccountReference: "",
    requestNo: actionRequestNo(),
    reviewThresholdMinor: "",
    targetMode: "invoice",
  });
}

function openAction(
  kind: ActionKind,
  target?: FinancialAdjustment | FinancialInvoice,
) {
  resetActionForm();
  actionKind.value = kind;
  actionTarget.value = target;
  actionError.value = "";
  if (target && "adjustmentNo" in target) {
    form.amountMinor = target.amountMinor;
    form.currencyCode = target.currencyCode;
    form.invoiceNo = target.invoiceNo ?? "";
  }
  if (target && "invoiceNo" in target && !("adjustmentNo" in target)) {
    form.invoiceNo = target.invoiceNo;
    form.amountMinor = target.outstandingAmountMinor;
    form.currencyCode = target.currencyCode;
  }
  actionOpen.value = true;
}

function actionRequestNo() {
  const random =
    typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `finance-${random}`;
}

async function digestEvidence(value: string) {
  const canonical = value.trim();
  if (!canonical) throw new Error("evidence-required");
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  return [...new Uint8Array(digest)]
    .map((item) => item.toString(16).padStart(2, "0"))
    .join("");
}

async function submitAction() {
  actionLoading.value = true;
  actionError.value = "";
  try {
    if (
      !/^[1-9][0-9]{0,15}$/.test(
        actionKind.value === "policy"
          ? form.reviewThresholdMinor
          : form.amountMinor,
      )
    )
      throw new Error("amount-invalid");
    if (actionKind.value === "manual_receipt") {
      const balanceTarget = form.targetMode === "balance";
      await createFinancialManualReceipt({
        amountMinor: form.amountMinor,
        businessPurposeCode: form.businessPurposeCode.trim(),
        currencyCode: balanceTarget
          ? form.currencyCode.trim().toUpperCase()
          : undefined,
        customerNo: balanceTarget ? form.customerNo.trim() : undefined,
        evidenceDigest: await digestEvidence(form.evidence),
        invoiceNo: balanceTarget ? undefined : form.invoiceNo.trim(),
        proofReference: form.proofReference.trim(),
        reasonCode: form.reasonCode.trim(),
        receivingAccountReference: form.receivingAccountReference.trim(),
        requestNo: form.requestNo,
      });
    } else if (actionKind.value === "invoice_waiver") {
      await createFinancialInvoiceWaiver({
        amountMinor: form.amountMinor,
        evidenceDigest: await digestEvidence(form.evidence),
        invoiceNo: form.invoiceNo.trim(),
        reasonCode: form.reasonCode.trim(),
        requestNo: form.requestNo,
      });
    } else if (actionKind.value === "review") {
      const target = actionTarget.value as FinancialAdjustment;
      await reviewFinancialAdjustment(target.adjustmentNo, {
        commentDigest: await digestEvidence(form.comment),
        decisionCode: form.decisionCode,
        evidenceDigest: await digestEvidence(form.evidence),
        requestNo: form.requestNo,
      });
    } else {
      await configureFinancialAdjustmentPolicy({
        currencyCode: form.currencyCode.trim().toUpperCase(),
        requestNo: form.requestNo,
        reviewThresholdMinor: form.reviewThresholdMinor,
      });
    }
    message.success(t("action.succeeded"));
    actionOpen.value = false;
    await Promise.all([
      loadOverview(),
      loadInvoices(),
      loadAdjustments(),
      loadTransactions(),
    ]);
  } catch {
    actionError.value = t("action.failed");
  } finally {
    actionLoading.value = false;
  }
}

const actionTitle = computed(() => t(`action.${actionKind.value}.title`));

onBeforeUnmount(() => {
  invoiceSequence += 1;
  transactionSequence += 1;
  adjustmentSequence += 1;
  invoicePagination.invalidate();
  transactionPagination.invalidate();
  adjustmentPagination.invalidate();
});

onMounted(() => {
  void searchCustomers();
  void Promise.all([loadOverview(), loadInvoices()]);
});
</script>

<template>
  <Page auto-content-height>
    <div class="finance-page" data-testid="financial-management-page">
      <Card size="small" :title="t('title')">
        <template #extra>
          <Space>
            <Button
              v-if="canCreateReceipt"
              data-testid="finance-manual-receipt"
              @click="openAction('manual_receipt')"
              >{{ t("action.manual_receipt.button") }}</Button
            >
            <Button
              v-if="canWritePolicy"
              class="policy-button"
              data-testid="finance-policy"
              @click="openAction('policy')"
              >{{ t("action.policy.button") }}</Button
            >
          </Space>
        </template>
      </Card>

      <Alert
        v-if="overviewError"
        :message="overviewError"
        show-icon
        type="error"
      />
      <section class="metric-grid" :class="{ 'is-loading': overviewLoading }">
        <Card :bordered="false" class="metric-card metric-primary">
          <span>{{ t("overview.openInvoices") }}</span>
          <Statistic :value="overview?.openInvoiceCount ?? 0" />
          <small>{{ t("overview.openInvoicesNote") }}</small>
        </Card>
        <Card :bordered="false" class="metric-card">
          <span>{{ t("overview.overdue") }}</span>
          <Statistic :value="overview?.overdueInvoiceCount ?? 0" />
          <small>{{ t("overview.overdueNote") }}</small>
        </Card>
        <Card :bordered="false" class="metric-card">
          <span>{{ t("overview.pendingReview") }}</span>
          <Statistic :value="overview?.pendingAdjustmentCount ?? 0" />
          <small>{{ t("overview.pendingReviewNote") }}</small>
        </Card>
        <Card :bordered="false" class="metric-card receivable-card">
          <span>{{ t("overview.receivables") }}</span>
          <div v-if="overview?.receivables.length" class="receivable-list">
            <strong
              v-for="item in overview.receivables"
              :key="item.currencyCode"
            >
              {{ formatMinor(item.outstandingMinor, item.currencyCode) }}
            </strong>
          </div>
          <small v-else>{{ t("overview.noReceivables") }}</small>
        </Card>
      </section>

      <FinancialOperationsPanel
        v-if="!['credit', 'refunds'].includes(activeTab)"
        :current-filter="operationsCurrentFilter"
        :view-code="operationsViewCode"
        @apply-filter="applyOperationsFilter"
      />

      <Card :bordered="false" class="workspace-card">
        <Tabs v-model:active-key="activeTab" class="finance-tabs">
          <TabPane key="invoices" :tab="t('tabs.invoices')">
            <Alert
              v-if="invoiceError"
              class="table-alert"
              :message="invoiceError"
              show-icon
              type="error"
            />
            <InvoiceGrid :loading="invoiceLoading">
              <template #customer="{ row }">{{
                customerDisplay(row)
              }}</template>
              <template #invoiceNo="{ row }"
                ><span class="mono">{{
                  shortNo(row.invoiceNo)
                }}</span></template
              >
              <template #status="{ row }"
                ><Tag :color="statusColor(row.statusCode)">{{
                  statusLabel(row.statusCode)
                }}</Tag></template
              >
              <template #amount="{ row }"
                ><strong>{{
                  formatMinor(row.outstandingAmountMinor, row.currencyCode)
                }}</strong
                ><small class="subvalue"
                  >{{ t("table.ofTotal") }}
                  {{
                    formatMinor(row.totalAmountMinor, row.currencyCode)
                  }}</small
                ></template
              >
              <template #dueAt="{ row }">{{
                formatTimestamp(row.paymentDueAt)
              }}</template>
              <template #action="{ row }"
                ><Space
                  ><Button
                    type="link"
                    size="small"
                    @click="openDetail('invoice', row.invoiceNo)"
                    >{{ t("detail.button") }}</Button
                  ><Button
                    v-if="canCreateWaiver && row.outstandingAmountMinor !== '0'"
                    data-testid="finance-invoice-waiver"
                    type="link"
                    size="small"
                    danger
                    @click="openAction('invoice_waiver', row)"
                    >{{ t("action.invoice_waiver.button") }}</Button
                  ></Space
                ></template
              >
            </InvoiceGrid>
          </TabPane>

          <TabPane key="transactions" :tab="t('tabs.transactions')">
            <Alert
              v-if="transactionError"
              class="table-alert"
              :message="transactionError"
              show-icon
              type="error"
            />
            <TransactionGrid :loading="transactionLoading">
              <template #customer="{ row }">{{
                row.customerDisplayName
                  ? customerDisplay(row)
                  : t("table.merchant")
              }}</template>
              <template #transactionNo="{ row }"
                ><span class="mono">{{
                  shortNo(row.transactionNo)
                }}</span></template
              >
              <template #type="{ row }"
                ><Tag>{{ statusLabel(row.transactionTypeCode) }}</Tag></template
              >
              <template #amount="{ row }"
                ><strong>{{
                  formatMinor(row.amountMinor, row.currencyCode)
                }}</strong></template
              >
              <template #postedAt="{ row }">{{
                formatTimestamp(row.postedAt)
              }}</template>
              <template #action="{ row }"
                ><Button
                  :data-testid="`finance-transaction-detail-${row.transactionNo}`"
                  type="link"
                  size="small"
                  @click="openDetail('transaction', row.transactionNo)"
                  >{{ t("detail.button") }}</Button
                ></template
              >
            </TransactionGrid>
          </TabPane>

          <TabPane key="adjustments" :tab="t('tabs.adjustments')">
            <Alert
              v-if="adjustmentError"
              class="table-alert"
              :message="adjustmentError"
              show-icon
              type="error"
            />
            <AdjustmentGrid :loading="adjustmentLoading">
              <template #customer="{ row }">{{
                customerDisplay(row)
              }}</template>
              <template #adjustmentNo="{ row }"
                ><span class="mono">{{
                  shortNo(row.adjustmentNo)
                }}</span></template
              >
              <template #type="{ row }"
                ><Tag>{{ statusLabel(row.adjustmentTypeCode) }}</Tag></template
              >
              <template #status="{ row }"
                ><Tag :color="statusColor(row.statusCode)">{{
                  statusLabel(row.statusCode)
                }}</Tag></template
              >
              <template #amount="{ row }"
                ><strong>{{
                  formatMinor(row.amountMinor, row.currencyCode)
                }}</strong></template
              >
              <template #createdAt="{ row }">{{
                formatTimestamp(row.createdAt)
              }}</template>
              <template #action="{ row }"
                ><Space
                  ><Button
                    :data-testid="`finance-adjustment-detail-${row.adjustmentNo}`"
                    type="link"
                    size="small"
                    @click="openDetail('adjustment', row.adjustmentNo)"
                    >{{ t("detail.button") }}</Button
                  ><Button
                    v-if="canReview && row.statusCode === 'pending_review'"
                    data-testid="finance-adjustment-review"
                    type="link"
                    size="small"
                    @click="openAction('review', row)"
                    >{{ t("action.review.button") }}</Button
                  ></Space
                ></template
              >
            </AdjustmentGrid>
          </TabPane>
          <TabPane key="refunds">
            <template #tab>
              <span data-testid="refund-management-tab">{{
                t("tabs.refunds")
              }}</span>
            </template>
            <RefundManagementPanel />
          </TabPane>
          <TabPane v-if="canViewCredit" key="credit">
            <template #tab>
              <span data-testid="credit-management-tab">{{
                t("tabs.credit")
              }}</span>
            </template>
            <FinanceWorkspace :context="sourcePluginContext" />
          </TabPane>
        </Tabs>
      </Card>

      <VbenDrawerCompat
        v-model:open="detailOpen"
        :title="t(`detail.${detailKind}`)"
        :width="760"
      >
        <Alert
          v-if="detailError"
          :message="detailError"
          show-icon
          type="error"
        />
        <div v-else-if="detailLoading" class="detail-loading">
          {{ t("detail.loading") }}
        </div>
        <template v-else-if="invoiceDetail">
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem :label="t('table.invoice')">{{
              invoiceDetail.invoice.invoiceNo
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.customer')">{{
              customerDisplay(invoiceDetail.invoice)
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.status')"
              ><Tag :color="statusColor(invoiceDetail.invoice.statusCode)">{{
                statusLabel(invoiceDetail.invoice.statusCode)
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem :label="t('table.receivable')">{{
              formatMinor(
                invoiceDetail.invoice.outstandingAmountMinor,
                invoiceDetail.invoice.currencyCode,
              )
            }}</DescriptionsItem>
          </Descriptions>
          <h3 class="detail-section-title">{{ t("detail.invoiceItems") }}</h3>
          <div
            v-for="item in invoiceDetail.items"
            :key="item.invoiceItemNo"
            class="ledger-line"
          >
            <span
              >#{{ item.lineNumber }} ·
              {{ item.descriptionZhCN || item.descriptionEnUS }}</span
            >
            <strong>{{
              formatMinor(
                item.totalAmountMinor,
                invoiceDetail.invoice.currencyCode,
              )
            }}</strong>
          </div>
        </template>
        <template v-else-if="transactionDetail">
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem :label="t('table.transaction')">{{
              transactionDetail.transactionNo
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.type')">{{
              statusLabel(transactionDetail.transactionTypeCode)
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.amount')">{{
              formatMinor(
                transactionDetail.amountMinor,
                transactionDetail.currencyCode,
              )
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.postedAt')">{{
              formatTimestamp(transactionDetail.postedAt)
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('detail.source')" :span="2">{{
              statusLabel(transactionDetail.sourceTypeCode)
            }}</DescriptionsItem>
          </Descriptions>
          <h3 class="detail-section-title">{{ t("detail.entries") }}</h3>
          <div
            v-for="entry in transactionDetail.entries"
            :key="entry.entryNo"
            class="ledger-line"
          >
            <span
              >#{{ entry.lineNumber }} ·
              {{ statusLabel(entry.accountTypeCode) }} ·
              {{ directionLabel(entry.directionCode) }}</span
            >
            <strong>{{
              formatMinor(entry.amountMinor, transactionDetail.currencyCode)
            }}</strong>
          </div>
        </template>
        <template v-else-if="adjustmentDetail">
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem :label="t('table.adjustment')">{{
              adjustmentDetail.adjustment.adjustmentNo
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.status')"
              ><Tag
                :color="statusColor(adjustmentDetail.adjustment.statusCode)"
                >{{ statusLabel(adjustmentDetail.adjustment.statusCode) }}</Tag
              ></DescriptionsItem
            >
            <DescriptionsItem :label="t('table.customer')">{{
              customerDisplay(adjustmentDetail.adjustment)
            }}</DescriptionsItem>
            <DescriptionsItem :label="t('table.amount')">{{
              formatMinor(
                adjustmentDetail.adjustment.amountMinor,
                adjustmentDetail.adjustment.currencyCode,
              )
            }}</DescriptionsItem>
          </Descriptions>
        </template>
      </VbenDrawerCompat>

      <VbenModalCompat
        v-model:open="actionOpen"
        :confirm-loading="actionLoading"
        :title="actionTitle"
        :width="640"
        @ok="submitAction"
      >
        <div data-testid="finance-action-modal"></div>
        <Alert
          class="reauth-alert"
          :message="t('action.reauth')"
          show-icon
          type="warning"
        />
        <Alert
          v-if="actionError"
          class="reauth-alert"
          :message="actionError"
          show-icon
          type="error"
        />
        <Form layout="vertical">
          <template v-if="actionKind === 'manual_receipt'">
            <FormItem :label="t('action.targetMode')"
              ><RadioGroup v-model:value="form.targetMode"
                ><Radio value="invoice">{{ t("action.invoiceTarget") }}</Radio
                ><Radio value="balance">{{
                  t("action.balanceTarget")
                }}</Radio></RadioGroup
              ></FormItem
            >
            <FormItem
              v-if="form.targetMode === 'invoice'"
              :label="t('table.invoice')"
              ><Input v-model:value="form.invoiceNo"
            /></FormItem>
            <template v-else
              ><FormItem :label="t('table.customer')"
                ><Select
                  v-model:value="form.customerNo"
                  show-search
                  :filter-option="false"
                  :loading="customerLoading"
                  :options="customerOptions"
                  @search="searchCustomers"
                  data-testid="finance-customer-no" /></FormItem
              ><FormItem :label="t('filter.currency')"
                ><Input
                  v-model:value="form.currencyCode"
                  :maxlength="3" /></FormItem
            ></template>
            <FormItem :label="t('action.amountMinor')"
              ><Input
                v-model:value="form.amountMinor"
                data-testid="finance-amount-minor"
            /></FormItem>
            <FormItem :label="t('action.reasonCode')"
              ><Select v-model:value="form.reasonCode" :options="reasonOptions"
            /></FormItem>
            <FormItem :label="t('action.proofReference')"
              ><Input
                v-model:value="form.proofReference"
                data-testid="finance-proof-reference"
            /></FormItem>
            <FormItem :label="t('action.receivingAccount')"
              ><Input
                v-model:value="form.receivingAccountReference"
                data-testid="finance-receiving-account"
            /></FormItem>
            <FormItem :label="t('action.businessPurpose')"
              ><Input v-model:value="form.businessPurposeCode"
            /></FormItem>
            <FormItem :label="t('action.evidence')"
              ><Textarea
                v-model:value="form.evidence"
                data-testid="finance-evidence"
                :rows="3"
            /></FormItem>
          </template>
          <template v-else-if="actionKind === 'invoice_waiver'">
            <FormItem :label="t('table.invoice')"
              ><Input v-model:value="form.invoiceNo"
            /></FormItem>
            <FormItem :label="t('action.amountMinor')"
              ><Input v-model:value="form.amountMinor"
            /></FormItem>
            <FormItem :label="t('action.reasonCode')"
              ><Input v-model:value="form.reasonCode"
            /></FormItem>
            <FormItem :label="t('action.evidence')"
              ><Textarea v-model:value="form.evidence" :rows="4"
            /></FormItem>
          </template>
          <template v-else-if="actionKind === 'review'">
            <FormItem :label="t('action.decision')"
              ><RadioGroup v-model:value="form.decisionCode"
                ><Radio value="approved">{{ t("action.approve") }}</Radio
                ><Radio value="rejected">{{
                  t("action.reject")
                }}</Radio></RadioGroup
              ></FormItem
            >
            <FormItem :label="t('action.comment')"
              ><Textarea v-model:value="form.comment" :rows="3"
            /></FormItem>
            <FormItem :label="t('action.evidence')"
              ><Textarea v-model:value="form.evidence" :rows="3"
            /></FormItem>
          </template>
          <template v-else>
            <FormItem :label="t('filter.currency')"
              ><Input v-model:value="form.currencyCode" :maxlength="3"
            /></FormItem>
            <FormItem :label="t('action.thresholdMinor')"
              ><InputNumber
                v-model:value="form.reviewThresholdMinor"
                class="full-width"
                :controls="false"
                string-mode
            /></FormItem>
          </template>
        </Form>
      </VbenModalCompat>
    </div>
  </Page>
</template>

<style scoped>
.finance-page {
  --ink: #102a2a;
  --mint: #25a77a;
  --line: #dce8e4;
  color: var(--ink);
  display: grid;
  gap: 18px;
}
.policy-button {
  background: #7ee5c1;
  border-color: #7ee5c1;
  color: #103c38;
  font-weight: 700;
}
.metric-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  transition: opacity 0.2s ease;
}
.metric-grid.is-loading {
  opacity: 0.58;
}
.metric-card {
  border: 1px solid var(--line);
  border-radius: 0;
  box-shadow: none;
  min-height: 150px;
}
.metric-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: space-between;
}
.metric-card span {
  color: #5f7471;
  font-size: 13px;
  font-weight: 650;
}
.metric-card small {
  color: #84928f;
}
.metric-primary {
  background: hsl(var(--card));
  border-color: #bde9d8;
}
.receivable-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 58px;
  overflow: auto;
}
.receivable-list strong {
  color: #116d54;
  font-size: 17px;
}
.workspace-card {
  border: 1px solid var(--line);
  border-radius: 0;
  box-shadow: none;
  overflow: hidden;
}
.finance-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 22px;
}
.finance-tabs :deep(.ant-tabs-tab-btn) {
  font-weight: 650;
}
.toolbar {
  display: grid;
  gap: 10px;
  grid-template-columns:
    minmax(180px, 1.2fr) minmax(150px, 0.8fr) minmax(110px, 0.45fr)
    auto;
  margin-bottom: 16px;
}
.toolbar-wide {
  grid-template-columns:
    minmax(170px, 1.2fr) minmax(150px, 0.7fr) minmax(145px, 0.7fr)
    minmax(100px, 0.4fr) auto;
}
.table-alert,
.reauth-alert {
  margin-bottom: 14px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.subvalue {
  color: #8a9997;
  display: block;
  margin-top: 3px;
}
.pager {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 18px;
}
.detail-loading {
  color: #6c7f7b;
  padding: 40px;
  text-align: center;
}
.detail-section-title {
  font-size: 14px;
  margin: 26px 0 12px;
}
.ledger-line {
  align-items: center;
  border-bottom: 1px solid #edf2f0;
  display: flex;
  gap: 14px;
  justify-content: space-between;
  padding: 12px 4px;
}
.ledger-line span {
  color: #526865;
}
.digest {
  font-family: ui-monospace, monospace;
  overflow-wrap: anywhere;
}
.full-width {
  width: 100%;
}
@media (max-width: 1100px) {
  .metric-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .toolbar,
  .toolbar-wide {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }
  .toolbar,
  .toolbar-wide {
    grid-template-columns: 1fr;
  }
  .pager {
    justify-content: stretch;
  }
  .pager button {
    flex: 1;
  }
}
</style>
