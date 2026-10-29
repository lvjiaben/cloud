<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  CustomerOverview,
  CustomerReferrer,
  CustomerSegment,
} from "../customer-client";
import type { CommerceOrderSummary } from "../commerce-client";
import type {
  FinancialInvoice,
  FinancialTransaction,
} from "../financial-client";

import { computed, nextTick, onBeforeUnmount, ref } from "vue";
import { useRouter } from "vue-router";
import { createNumberedPage } from "../numbered-page";

import { useAccess } from "../plugin-access";
import { useVbenModal, useVbenDrawer } from "@vben/common-ui";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import {
  Alert,
  Button,
  Descriptions,
  Empty,
  TabPane,
  Tag,
  Tabs,
  Modal,
  Select,
  message,
} from "ant-design-vue";

import { useVbenForm } from "#/adapter/form";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  customerAccountTransition,
  customerVerificationUpdate,
  customerSegmentAdd,
  customerSegmentRemove,
  customerSegments,
  customerAuditRecords,
  customerAssignments,
  customerAssistanceEnd,
  customerAssistanceStart,
  customerOverview,
  customerReferrer,
} from "../customer-client";
import { commerceOrders } from "../commerce-client";
import { financialInvoices, financialTransactions } from "../financial-client";
import type { CustomerAuditRecord, CustomerAssistanceSession } from "../customer-client";
import CustomerStatisticsPanel from "./customer-statistics-panel.vue";
import CustomerProductsPanel from "./customer-products-panel.vue";
import CustomerApiPanel from "./customer-api-panel.vue";
import { requestClient } from "#/api/request";
import { getPluginStateMap } from "#/plugins/slot-registry";
import { AssistanceLaunchError, assistanceLaunchURL } from "../../assistance-launch";
import CustomerActivityPanel from "./customer-activity-panel.vue";
import VbenModalCompat from "./vben-modal-compat.vue";
import { preferences } from "@vben/preferences";

import CustomerBalanceModal from './customer-balance-modal.vue';
import { minorToMajorAmount } from '../catalog-money';

const [BalanceModal, balanceModalApi] = useVbenModal({ connectedComponent: CustomerBalanceModal });
const canRecharge = computed(() => hasAccessByCodes(['idc:admin:billing:manual-receipt:create']));
const canAdjustBalance = computed(() => hasAccessByCodes(['idc:admin:billing:balance-adjustment:create']));
function openBalanceAction(operation: 'recharge' | 'gift' | 'deduct') {
 if (!overview.value) return;
 balanceModalApi.setData({operation, customerNo: overview.value.customer.customerNo,
 displayName: overview.value.customer.displayName, balances: overview.value.balances || []}).open();
}
async function refreshBalanceOverview() {
 if (!overview.value) return;
 const customerNo = overview.value.customer.customerNo;
 const sequence = ++overviewSequence;
 const result = await customerOverview(customerNo);
 if (sequence === overviewSequence && overview.value?.customer.customerNo === customerNo) overview.value = result;
 emit('updated');
}
const overview = ref<CustomerOverview>();
const apiAvailable=ref(false);
const emit = defineEmits<{ updated: [] }>();
const segmentKind = ref<"group" | "tag" | null>(null);
const segmentSaving = ref(false);
const segmentOptions = ref<{ label: string; value: string }[]>([]);
const tagOptions = ref<{ label: string; value: string }[]>([]);
const tagInput = ref<string[]>([]);
let tagSearchSequence = 0;
let segmentKey = "";
let segmentSearchSequence = 0;
const auditRows = ref<CustomerAuditRecord[]>([]);
const auditLoading = ref(false);
const auditFailed = ref(false);

const groupCursor = ref("");
const tagCursor = ref("");
const assignmentLoading = ref<"group" | "tag" | "">("");
const assistanceSaving = ref(false);
const assistanceSession = ref<CustomerAssistanceSession>();
const referrer = ref<CustomerReferrer>();
const statusSaving = ref(false);
const verificationSaving = ref(false);
const activeTab = ref("summary");
const loadedTabs = ref(new Set<string>());
const orderLoading = ref(false);
const invoiceLoading = ref(false);
const transactionLoading = ref(false);
const orderRows = ref<CommerceOrderSummary[]>([]);
const invoiceRows = ref<FinancialInvoice[]>([]);
const transactionRows = ref<FinancialTransaction[]>([]);
const orderPage = ref(1);
const orderSize = ref(10);
let orderSequence = 0;
const orderPagination = createNumberedPage<CommerceOrderSummary>((state) => {
  orderRows.value = state.items;
  orderPage.value = state.currentPage;
  orderSize.value = state.pageSize;
  orderGridApi.setGridOptions({
    data: state.items,
    pagerConfig: {
      enabled: true,
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    },
  });
});
const invoicePage = ref(1);
const invoiceSize = ref(10);
let invoiceSequence = 0;
const invoicePagination = createNumberedPage<FinancialInvoice>((state) => {
  invoiceRows.value = state.items;
  invoicePage.value = state.currentPage;
  invoiceSize.value = state.pageSize;
  invoiceGridApi.setGridOptions({
    data: state.items,
    pagerConfig: {
      enabled: true,
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    },
  });
});
const transactionPage = ref(1);
const transactionSize = ref(10);
let transactionSequence = 0;
const transactionPagination = createNumberedPage<FinancialTransaction>(
  (state) => {
    transactionRows.value = state.items;
    transactionPage.value = state.currentPage;
    transactionSize.value = state.pageSize;
    transactionGridApi.setGridOptions({
      data: state.items,
      pagerConfig: {
        enabled: true,
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    });
  },
);
let overviewSequence = 0;
const { hasAccessByCodes } = useAccess();
const router = useRouter();
const canOpenOrder = computed(() => ["idc:admin:commerce:order:list", "idc:admin:commerce:order:query"].every(code => hasAccessByCodes([code])));
const canOpenInvoice = computed(() => ["idc:admin:billing:invoice:list", "idc:admin:billing:invoice:query"].every(code => hasAccessByCodes([code])));

async function openBusinessDetail(kind: "invoice" | "order", reference: string) {
  if (!(kind === "order" ? canOpenOrder.value : canOpenInvoice.value)) return;
  await router.push({ path: kind === "order" ? "/idc/orders" : "/idc/finance", query: { [kind === "order" ? "orderNo" : "invoiceNo"]: reference } });
  drawerApi.close();
}
const canAssist = computed(() =>
  hasAccessByCodes(["idc:admin:customer:assist"]),
);
const canManageStatus = computed(() =>
  hasAccessByCodes(["idc:admin:customer:update"]),
);

function tr(key: string) {
  return $t(`plugin.idc-core.customer.overview.${key}`);
}

function orderColumns(): VxeGridProps<CommerceOrderSummary>["columns"] {
  return [
    { field: "orderNo", slots: { default: "copyNo" }, title: tr("table.orderNo"), width: 290 },
    {
      field: "productSummary",
      slots: { default: "product" },
      title: tr("table.product"),
      minWidth: 220,
    },
    {
      field: "ipAddress",
      slots: { default: "plain" },
      title: tr("table.ip"),
      width: 150,
    },
    {
      field: "createdAt",
      slots: { default: "time" },
      title: tr("table.time"),
      width: 175,
    },
    {
      field: "paymentMethodCode",
      slots: { default: "payment" },
      title: tr("table.payment"),
      width: 150,
    },
    {
      field: "statusCode",
      slots: { default: "orderStatus" },
      title: tr("table.status"),
      width: 130,
    },
    {
      field: "totalAmountMinor",
      slots: { default: "orderAmount" },
      title: tr("table.amount"),
      width: 150,
    },
    {
      field: "remark",
      slots: { default: "plain" },
      title: tr("table.remark"),
      minWidth: 160,
    },
  ];
}

function invoiceColumns(): VxeGridProps<FinancialInvoice>["columns"] {
  return [
    { field: "invoiceNo", slots: { default: "copyNo" }, title: tr("table.billNo"), width: 290 },
    {
      field: "orderNo",
      slots: { default: "plain" },
      title: tr("table.orderNo"),
      width: 210,
    },
    {
      field: "createdAt",
      slots: { default: "time" },
      title: tr("table.time"),
      width: 175,
    },
    {
      field: "statusCode",
      slots: { default: "financeStatus" },
      title: tr("table.status"),
      width: 130,
    },
    {
      field: "totalAmountMinor",
      slots: { default: "invoiceAmount" },
      title: tr("table.amount"),
      width: 150,
    },
    {
      field: "outstandingAmountMinor",
      slots: { default: "outstandingAmount" },
      title: tr("table.outstanding"),
      minWidth: 150,
    },
  ];
}

function transactionColumns(): VxeGridProps<FinancialTransaction>["columns"] {
  return [
    { field: "transactionNo", slots: { default: "copyNo" }, title: tr("table.transactionNo"), width: 230 },
    {
      field: "transactionTypeCode",
      slots: { default: "transactionType" },
      title: tr("table.type"),
      width: 150,
    },
    {
      field: "relatedInvoiceNo",
      slots: { default: "plain" },
      title: tr("table.billNo"),
      minWidth: 210,
    },
    {
      field: "postedAt",
      slots: { default: "time" },
      title: tr("table.time"),
      width: 175,
    },
    {
      field: "amountMinor",
      slots: { default: "transactionAmount" },
      title: tr("table.amount"),
      width: 160,
    },
    { field: "entryCount", title: tr("table.entries"), width: 100 },
  ];
}

const [OrderGrid, orderGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize,
    }: {
      currentPage: number;
      pageSize: number;
    }) => void loadOrders(currentPage, pageSize),
  },
  gridOptions: {
    columns: orderColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 430,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "orderNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-customer-360-orders",
  },
});
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
  gridOptions: {
    columns: invoiceColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 430,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "invoiceNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-customer-360-invoices",
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
  gridOptions: {
    columns: transactionColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 430,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "transactionNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-customer-360-transactions",
  },
});

function localizedStatus(
  prefix: "commerce.orders.status" | "finance.status",
  value?: string,
) {
  if (!value) return "—";
  const key = `plugin.idc-core.${prefix}.${value}`;
  const translated = $t(key);
  return translated === key ? tr("unknownStatus") : translated;
}

function paymentLabel(value?: string, statusCode?: string) {
  if (!value) {
    return statusCode === "pending_payment"
      ? $t("plugin.idc-core.commerce.orders.paymentMethod.awaitingPayment")
      : $t("plugin.idc-core.commerce.orders.paymentMethod.notRecorded");
  }
  const key = `plugin.idc-core.commerce.orders.paymentMethod.${value}`;
  const translated = $t(key);
  return translated === key ? tr("unknownValue") : translated;
}

function orderProductLabel(row: CommerceOrderSummary) {
  return (
    row.productSummary ||
    $t("plugin.idc-core.commerce.orders.productCount", { count: row.itemCount })
  );
}

function orderPlainLabel(row: CommerceOrderSummary, field: string) {
  const value = row[field as keyof CommerceOrderSummary];
  if (value) return value;
  return field === "ipAddress"
    ? $t("plugin.idc-core.commerce.orders.ipNotAssigned")
    : $t("plugin.idc-core.commerce.orders.noRemark");
}

function formatMinor(value: string | undefined, currency = "CNY") {
  if (!value) return "—";
  const negative = value.startsWith('-');
  return `${currency} ${negative ? '−' : ''}${minorToMajorAmount(negative ? value.slice(1) : value, currency)}`;
}

async function loadOrders(
  currentPage = orderPage.value,
  pageSize = orderSize.value,
) {
  if (!overview.value) return;
  const customerNo = overview.value.customer.customerNo;
  const sequence = ++orderSequence;
  orderLoading.value = true;
  await orderPagination.query(
    { currentPage, pageSize },
    async (pageNumber, limit) => {
      try {
        const page = await commerceOrders({
          customerNo,
          page: pageNumber,
          limit,
        });
        if (sequence === orderSequence) loadedTabs.value.add("orders");
        return { items: page.orders, total: page.total };
      } finally {
        if (sequence === orderSequence) orderLoading.value = false;
      }
    },
  );
}

async function loadInvoices(
  currentPage = invoicePage.value,
  pageSize = invoiceSize.value,
) {
  if (!overview.value) return;
  const customerNo = overview.value.customer.customerNo;
  const sequence = ++invoiceSequence;
  invoiceLoading.value = true;
  await invoicePagination.query(
    { currentPage, pageSize },
    async (pageNumber, limit) => {
      try {
        const page = await financialInvoices({
          customerNo,
          page: pageNumber,
          limit,
        });
        if (sequence === invoiceSequence) loadedTabs.value.add("invoices");
        return { items: page.invoices, total: page.total };
      } finally {
        if (sequence === invoiceSequence) invoiceLoading.value = false;
      }
    },
  );
}

async function loadTransactions(
  currentPage = transactionPage.value,
  pageSize = transactionSize.value,
) {
  if (!overview.value) return;
  const customerNo = overview.value.customer.customerNo;
  const sequence = ++transactionSequence;
  transactionLoading.value = true;
  await transactionPagination.query(
    { currentPage, pageSize },
    async (pageNumber, limit) => {
      try {
        const page = await financialTransactions({
          customerNo,
          page: pageNumber,
          limit,
        });
        if (sequence === transactionSequence)
          loadedTabs.value.add("transactions");
        return { items: page.transactions, total: page.total };
      } finally {
        if (sequence === transactionSequence) transactionLoading.value = false;
      }
    },
  );
}

function changeTab(key: string | number) {
  const target = String(key);
  if (target === "records") void loadAudit();
  if (loadedTabs.value.has(target)) return;
  if (target === "orders") void loadOrders();
  if (target === "invoices") void loadInvoices();
  if (target === "transactions") void loadTransactions();
}

onBeforeUnmount(() => {
  overviewSequence += 1;
  orderSequence += 1;
  invoiceSequence += 1;
  transactionSequence += 1;
  orderPagination.invalidate();
  invoicePagination.invalidate();
  transactionPagination.invalidate();
});

async function startAssistance() {
  if (!overview.value || assistanceSaving.value) return;
  const targetWindow = window.open("about:blank", "_blank");
  if (!targetWindow) {
    message.warning($t("plugin.idc-core.customer.assistance.popupBlocked"));
    return;
  }
  assistanceSaving.value = true;
  try {
    const plugin = (await getPluginStateMap()).get("idc-core");
    if (!plugin?.version) throw new AssistanceLaunchError("Customer assistance plugin version is unavailable");
    const result = await customerAssistanceStart(
      overview.value.customer.customerNo,
      {
        idempotencyKey: `customer-assistance-${crypto.randomUUID()}`,
        reason: "Administrator started customer assistance from customer details.",
      },
    );
    assistanceSession.value = result;
    if (result.launchUrl) {
      targetWindow.location.replace(assistanceLaunchURL(result.launchUrl, result.sessionNo, plugin.version));
    } else {
      throw new AssistanceLaunchError("Customer assistance launch URL is unavailable");
    }
    message.success($t("plugin.idc-core.customer.assistance.started"));
  } catch (error) {
    targetWindow.close();
    if (error instanceof AssistanceLaunchError) {
      message.error($t("plugin.idc-core.customer.assistance.unavailable"));
    } else {
      throw error;
    }
  } finally {
    assistanceSaving.value = false;
  }
}

async function endAssistance() {
  if (!assistanceSession.value || assistanceSaving.value) return;
  assistanceSaving.value = true;
  try {
    const ended = await customerAssistanceEnd(
      assistanceSession.value.sessionNo,
    );
    assistanceSession.value = { ...assistanceSession.value, ...ended };
    message.success($t("plugin.idc-core.customer.assistance.ended"));
  } finally {
    assistanceSaving.value = false;
  }
}

const title = computed(() =>
  overview.value?.customer.displayName
    ? `${$t("plugin.idc-core.customer.overview.title")} · ${overview.value.customer.displayName}`
    : $t("plugin.idc-core.customer.overview.title"),
);

const customerBusinessSurfaces = computed(() => {
  const business = overview.value?.business;
  if (!business) return [];
  return [
    {
      count: business.tickets,
      key: "tickets",
      label: tr("businessTickets"),
      path: "/admin/idc/support",
    },
    {
      count: business.taxInvoices,
      key: "taxInvoices",
      label: tr("businessTaxInvoices"),
      path: "/admin/idc/compliance/tax-invoices",
    },
    {
      count: business.notifications,
      key: "notifications",
      label: tr("businessNotifications"),
      path: "/admin/idc/notifications/deliveries",
    },
    {
      count: business.audits,
      key: "audits",
      label: tr("businessAudits"),
      path: "",
    },
  ];
});

function customerBusinessSurface(keys: string[]) {
  return customerBusinessSurfaces.value.filter((item) =>
    keys.includes(item.key),
  );
}

function localizedSegmentName(segment: CustomerSegment) {
  return segment.nameZhCn || segment.nameEnUs || segment.code;
}

async function loadAssignments(kind: "group" | "tag") {
  if (!overview.value || assignmentLoading.value) return;
  assignmentLoading.value = kind;
  try {
    const cursor = kind === "group" ? groupCursor.value : tagCursor.value;
    const result = await customerAssignments(
      overview.value.customer.customerNo,
      {
        cursor: cursor || undefined,
        kind,
        limit: 25,
        locale:
          overview.value.customer.preferredLocale === "en-US"
            ? "en-US"
            : "zh-CN",
      },
    );
    const target =
      kind === "group" ? overview.value.groups : overview.value.tags;
    target.items = cursor ? [...target.items, ...result.items] : result.items;
    target.hasMore = Boolean(result.nextCursor);
    if (kind === "group") groupCursor.value = result.nextCursor;
    else tagCursor.value = result.nextCursor;
  } finally {
    assignmentLoading.value = "";
  }
}

async function refreshOverview() {
  if (!overview.value) return;
  overview.value = await customerOverview(overview.value.customer.customerNo);
}

function referrerLabel() {
  if (!referrer.value?.customerNo) return tr("noReferrer");
  return [
    referrer.value.displayName,
    referrer.value.customerCode ? `#${referrer.value.customerCode}` : "",
  ]
    .filter(Boolean)
    .join(" · ");
}

const canReadAudit = computed(() => hasAccessByCodes(["idc:admin:audit:list"]));
const [AuditGrid, auditGridApi] = useVbenVxeGrid({ gridOptions: { columns: [], data: [], height: 430, pagerConfig: { enabled: false }, toolbarConfig: { enabled: false }, rowConfig: { keyField: "auditNo" } } });
async function loadAudit() {
  if (!overview.value || auditLoading.value) return;
  const customerNo = overview.value.customer.customerNo;
  auditLoading.value = true; auditFailed.value = false;
  try {
    const result = await customerAuditRecords(customerNo);
    if (overview.value?.customer.customerNo !== customerNo) return;
    auditRows.value = result.items;
    auditGridApi.setGridOptions({ data: result.items, columns: [
      { field: "auditNo", title: tr("recordId"), minWidth: 230, slots: { default: "auditNo" } },
      { field: "operationCode", title: tr("operation"), minWidth: 180, slots: { default: "operation" } },
      { field: "result", title: tr("table.status"), width: 110, slots: { default: "auditResult" } },
      { field: "occurredAt", title: tr("table.time"), width: 175, formatter: ({ cellValue }: { cellValue: number }) => formatTimestamp(cellValue) },
    ] });
  } catch { if (overview.value?.customer.customerNo === customerNo) auditFailed.value = true; }
  finally { auditLoading.value = false; }
}
function operationLabel(code: string) {
  const known: Record<string, string> = { "customer.direct.create": "accountCreated", "customer.password.reset": "passwordReset", "customer.segment.assign": "segmentAdded", "customer.segment.revoke": "segmentRemoved", "customer.note.create": "noteAdded", "customer.verification.update": "verificationUpdated" };
  if (known[code]) return tr(known[code]);
  if (code.includes("account") && code.includes("status")) return tr("statusChanged");
  return tr("customerOperation");
}
async function copyNumber(value: string) {
  try { await navigator.clipboard.writeText(value); message.success(tr("copied")); }
  catch { message.warning(tr("copyFailed")); }
}
function accountColor(status: string) { return status === "active" ? "success" : status === "disabled" ? "error" : status === "restricted" ? "warning" : "default"; }
function confirmVerification(action: "approve" | "clear") {
 const target = overview.value?.customer;
 if (!target || verificationSaving.value) return;
 const key = crypto.randomUUID();
 Modal.confirm({ title: tr(action === "approve" ? "verifyConfirm" : "clearVerifyConfirm"), content: tr(action === "approve" ? "verifyHint" : "clearVerifyHint"), async onOk() {
  verificationSaving.value = true;
  try {
   await customerVerificationUpdate(target.customerNo, {action,expectedVersion:target.version,idempotencyKey:key});
   if (overview.value?.customer.customerNo === target.customerNo) await refreshOverview();
   emit("updated"); message.success(tr(action === "approve" ? "verifyDone" : "clearVerifyDone"));
  } finally { verificationSaving.value = false; }
 } });
}
function confirmStatusToggle() {
  const target = overview.value?.customer;
  if (!target || statusSaving.value) return;
  const unban = target.status === "disabled";
  const key = `customer-status-${crypto.randomUUID()}`;
  Modal.confirm({ title: tr(unban ? "unbanConfirm" : "banConfirm"), content: tr(unban ? "unbanHint" : "banHint"), async onOk() {
    statusSaving.value = true;
    try {
      await customerAccountTransition(target.customerNo, { expectedVersion: target.version, idempotencyKey: key, reason: unban ? "account_reactivated" : "manual_disable", toStatus: unban ? "active" : "disabled" });
      await refreshOverview(); emit("updated"); message.success(tr("statusUpdated"));
    } finally { statusSaving.value = false; }
  } });
}
const [SegmentForm, segmentFormApi] = useVbenForm({ commonConfig: { componentProps: { class: "w-full" }, labelWidth: 90 }, schema: [], showDefaultActions: false });
async function searchSegments(namePrefix = "") {
  const current = ++segmentSearchSequence;
  if (!segmentKind.value) return;
  const result = await customerSegments({ kind: segmentKind.value, locale: "zh-CN", namePrefix, limit: 25 });
  if (current !== segmentSearchSequence) return;
  segmentOptions.value = result.items.map(item => ({ value: item.number, label: localizedSegmentName(item) }));
  await segmentFormApi.updateSchema([{ fieldName: "segmentNo", componentProps: { options: segmentOptions.value } }]);
}
async function openSegment(kind: "group" | "tag") {
  segmentKind.value = kind; segmentKey = crypto.randomUUID();
  await nextTick();
  await segmentFormApi.setState({ schema: [
    { fieldName: "segmentNo", component: "Select", label: tr("existingSegment"), componentProps: { "data-testid": "customer-segment-select", allowClear: true, showSearch: true, filterOption: false, onSearch: searchSegments, options: [] } },
    { fieldName: "name", component: "Input", label: tr("newSegment"), componentProps: { "data-testid": "customer-segment-name", maxlength: 128 }, dependencies: { triggerFields: ["segmentNo"], show: values => !values.segmentNo } },
  ] });
  await segmentFormApi.resetForm(); await searchSegments();
}
async function searchTags(namePrefix = "") {
  const sequence = ++tagSearchSequence;
  const result = await customerSegments({kind:"tag",locale:preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US",namePrefix,limit:25});
  if (sequence !== tagSearchSequence) return;
  tagOptions.value = result.items.map(item => ({value:item.number,label:localizedSegmentName(item)}));
}
async function addEnteredTag(selection: unknown) {
  const values = Array.isArray(selection) ? selection.filter((value): value is string => typeof value === "string") : [];
  if (!overview.value || segmentSaving.value || values.length === 0) return;
  const value = values[0]!.trim();
  if (!value || value.length > 128) { tagInput.value = []; return; }
  const existing = tagOptions.value.find(item => item.value === value || item.label === value);
  segmentSaving.value = true;
  try {
    await customerSegmentAdd(overview.value.customer.customerNo,{kind:"tag",segmentNo:existing?.value,name:existing ? undefined : value,idempotencyKey:crypto.randomUUID()});
    tagInput.value = [];await refreshOverview();await searchTags();message.success(tr("segmentSaved"));
  } finally { segmentSaving.value = false; }
}
async function saveSegment() {
  if (!overview.value || !segmentKind.value || segmentSaving.value) return;
  const values = await segmentFormApi.getValues<{ segmentNo?: string; name?: string }>();
  if (!values.segmentNo && !values.name?.trim()) { message.warning(tr("segmentRequired")); return; }
  segmentSaving.value = true;
  try {
    await customerSegmentAdd(overview.value.customer.customerNo, { kind: segmentKind.value, segmentNo: values.segmentNo || undefined, name: values.segmentNo ? undefined : values.name!.trim(), idempotencyKey: segmentKey });
    segmentKind.value = null; await refreshOverview(); message.success(tr("segmentSaved"));
  } finally { segmentSaving.value = false; }
}
function removeSegment(kind: "group" | "tag", item: CustomerSegment) {
  const customerNo = overview.value?.customer.customerNo;
  if (!customerNo) return;
  const key = crypto.randomUUID();
  Modal.confirm({ title: tr("removeSegment"), content: localizedSegmentName(item), async onOk() {
    await customerSegmentRemove(customerNo, item.number, { kind, idempotencyKey: key }); await refreshOverview(); message.success(tr("segmentSaved"));
  } });
}

const [Drawer, drawerApi] = useVbenDrawer({
  footer: false,
  async onOpenChange(open) {
    const sequence = ++overviewSequence;
    if (!open) {
      balanceModalApi.close();
      overview.value = undefined;
      referrer.value = undefined;
      assistanceSession.value = undefined;
      groupCursor.value = "";
      tagCursor.value = "";
      auditRows.value = []; auditFailed.value = false;
      activeTab.value = "summary";
      loadedTabs.value = new Set();
      orderRows.value = [];
      invoiceRows.value = [];
      transactionRows.value = [];
      orderSequence += 1;
      orderPagination.reset();
      orderLoading.value = false;
      invoiceSequence += 1;
      invoicePagination.reset();
      invoiceLoading.value = false;
      transactionSequence += 1;
      transactionPagination.reset();
      transactionLoading.value = false;
      return;
    }
    const data = drawerApi.getData<{ customerNo?: string; initialTab?: "services" }>();
    if (!data?.customerNo) return;
    drawerApi.setState({ loading: true });
    try {
      const result = await customerOverview(data.customerNo);
      if (sequence !== overviewSequence) return;
      overview.value = result;
      activeTab.value = data.initialTab === "services" ? "services" : "summary";
      apiAvailable.value=false;
      void requestClient.get<{list:{id:string;installed:number;enabled:number}[]}>("/plugins/dynamic").then(plugins=>{if(sequence===overviewSequence)apiAvailable.value=plugins.list.some(item=>item.id==="idc-channel"&&item.installed===1&&item.enabled===1);}).catch(()=>{if(sequence===overviewSequence)apiAvailable.value=false;});
      tagInput.value = [];
      if (canManageStatus.value) void searchTags();
      try {
        const ownerReferrer = await customerReferrer(data.customerNo);
        if (sequence === overviewSequence) referrer.value = ownerReferrer;
      } catch {
        if (sequence === overviewSequence) referrer.value = undefined;
      }
    } finally {
      if (sequence === overviewSequence) drawerApi.setState({ loading: false });
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-[min(1180px,calc(100vw-24px))] max-w-[calc(100vw-24px)]">
    <div
      v-if="overview"
      class="customer-overview"
      data-testid="customer-overview-drawer"
    >
      <header class="detail-header">
        <div><h2>{{ overview.customer.displayName }}</h2>
          <button class="copy-number" data-testid="customer-id-copy" @click="copyNumber(overview.customer.customerCode || overview.customer.customerNo)">{{ overview.customer.customerCode || overview.customer.customerNo }}</button>
        </div>
        <div class="detail-actions">
          <Button v-if="canManageStatus && ['active','restricted','disabled'].includes(overview.customer.status)" :danger="overview.customer.status !== 'disabled'" :loading="statusSaving" data-testid="customer-ban-toggle" @click="confirmStatusToggle">{{ tr(overview.customer.status === 'disabled' ? 'unban' : 'ban') }}</Button>
          <Button v-if="canAssist" data-testid="customer-assistance-open"  :loading="assistanceSaving" @click="startAssistance">{{ $t('plugin.idc-core.customer.assistance.open') }}</Button>
        </div>
      </header>

      <Descriptions bordered :column="2" size="small">
        <Descriptions.Item :label="tr('customerId')"><button class="copy-number" @click="copyNumber(overview.customer.customerCode || overview.customer.customerNo)">{{ overview.customer.customerCode || overview.customer.customerNo }}</button></Descriptions.Item>
        <Descriptions.Item :label="$t('plugin.idc-core.customer.table.phone')"><div class="account-lines"><span v-if="overview.customer.primaryPhone">{{ overview.customer.primaryPhone }}</span><span v-if="overview.customer.primaryEmail">{{ overview.customer.primaryEmail }}</span><span v-if="!overview.customer.primaryPhone && !overview.customer.primaryEmail">—</span></div></Descriptions.Item>
        <Descriptions.Item :label="$t('plugin.idc-core.customer.table.type')">{{
          $t(`plugin.idc-core.customer.type.${overview.customer.type}`)
        }}</Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.customer.table.level')"
          >{{
            overview.customer.level?.nameZhCn ||
            overview.customer.level?.nameEnUs ||
            "—"
          }}</Descriptions.Item
        >
        <Descriptions.Item :label="tr('accountStatus')"><Tag :color="accountColor(overview.customer.status)">{{ $t(`plugin.idc-core.customer.status.${overview.customer.status}`) }}</Tag></Descriptions.Item>
        <Descriptions.Item :label="tr('verificationStatus')"><div class="verification-actions">
          <Tag :color="overview.customer.verification === 'verified' ? 'success' : 'default'">{{ $t(`plugin.idc-core.customer.verification.${overview.customer.verification}`) }}</Tag>
          <template v-if="canManageStatus">
            <Button size="small" data-testid="customer-verification-approve" :loading="verificationSaving" :disabled="overview.customer.verification === 'verified'" @click="confirmVerification('approve')">{{ tr('verify') }}</Button>
            <Button size="small" data-testid="customer-verification-clear" :loading="verificationSaving" @click="confirmVerification('clear')">{{ tr('clearVerify') }}</Button>
          </template>
        </div></Descriptions.Item>
        <Descriptions.Item :label="tr('referrer')">{{
          referrerLabel()
        }}</Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.customer.table.createdAt')"
          >{{ formatTimestamp(overview.customer.createdAt) }}</Descriptions.Item
        >
        <Descriptions.Item :label="tr('updatedAt')">{{
          formatTimestamp(overview.customer.updatedAt)
        }}</Descriptions.Item>
        <Descriptions.Item :label="tr('localeRegion')" :span="2"
          >{{ overview.customer.preferredLocale }} ·
          {{ overview.customer.countryCode }}</Descriptions.Item
        >
      </Descriptions>

      <Alert
        v-if="assistanceSession"
        :message="$t('plugin.idc-core.customer.assistance.activeTitle')"
        :description="
          $t('plugin.idc-core.customer.assistance.activeDescription', {
            time: formatTimestamp(assistanceSession.expiresAt),
          })
        "
        :type="assistanceSession.status === 'active' ? 'warning' : 'success'"
        show-icon
      >
        <template v-if="assistanceSession.status === 'active'" #action>
          <Button
            :loading="assistanceSaving"
            danger
            size="small"
            @click="endAssistance"
            >{{ $t("plugin.idc-core.customer.assistance.end") }}</Button
          >
        </template>
      </Alert>

      <Tabs v-model:active-key="activeTab" @change="changeTab">
        <TabPane key="summary" :tab="tr('tabs.summary')">
          <div class="tab-panel">
            <CustomerStatisticsPanel v-if="overview.statistics" :statistics="overview.statistics" @view-products="activeTab = 'services'; changeTab('services')" />


            <section class="overview-section">
              <div class="section-title flex items-center justify-between gap-3">
               <span>{{ tr("balances") }}</span>
               <div class="flex flex-wrap gap-2">
                <Button v-if="canRecharge" data-testid="customer-balance-recharge" @click="openBalanceAction('recharge')">{{ $t('plugin.idc-core.customer.balanceAction.recharge') }}</Button>
                <Button v-if="canAdjustBalance" data-testid="customer-balance-gift" @click="openBalanceAction('gift')">{{ $t('plugin.idc-core.customer.balanceAction.gift') }}</Button>
                <Button v-if="canAdjustBalance" data-testid="customer-balance-deduct" @click="openBalanceAction('deduct')">{{ $t('plugin.idc-core.customer.balanceAction.deduct') }}</Button>
               </div>
              </div>
              <div v-if="overview.balances?.length" class="balance-grid">
                <article
                  v-for="balance in overview.balances"
                  :key="`${balance.accountTypeCode}-${balance.currencyCode}`"
                  class="balance-card"
                >
                  <div class="balance-heading">
                    <span>{{
                      localizedStatus("finance.status", balance.accountTypeCode)
                    }}</span>
                    <Tag
                      :color="
                        balance.statusCode === 'active' ? 'success' : 'default'
                      "
                      >{{
                        localizedStatus("finance.status", balance.statusCode)
                      }}</Tag
                    >
                  </div>
                  <strong>{{
                    formatMinor(
                      balance.availableBalanceMinor,
                      balance.currencyCode,
                    )
                  }}</strong>
                  <small>{{ tr("availableBalance") }}</small>
                  <div class="balance-detail">
                    <span
                      >{{ tr("bookBalance") }}
                      {{
                        formatMinor(balance.balanceMinor, balance.currencyCode)
                      }}</span
                    >
                    <span
                      >{{ tr("heldBalance") }}
                      {{
                        formatMinor(
                          balance.heldBalanceMinor,
                          balance.currencyCode,
                        )
                      }}</span
                    >
                  </div>
                </article>
              </div>
              <Empty
                v-else
                :description="tr('noBalance')"
                :image="Empty.PRESENTED_IMAGE_SIMPLE"
              />
            </section>

            <section class="overview-section segment-grid">
              <div>
                <div class="segment-heading"><span class="section-title">{{ tr("groups") }}</span><Button v-if="canManageStatus" size="small" data-testid="customer-group-add" @click="openSegment('group')">{{ tr('add') }}</Button></div>
                <div v-if="overview.groups.items.length" class="segment-list">
                  <Tag
                    v-for="item in overview.groups.items"
                    :key="item.number" :closable="canManageStatus" @close.prevent="removeSegment('group', item)"
                    >{{ localizedSegmentName(item) }}</Tag
                  >
                </div>
                <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />
                <small v-if="overview.groups.hasMore">{{ tr("more") }}</small>
                <Button
                  v-if="overview.groups.hasMore"
                  :loading="assignmentLoading === 'group'"
                  size="small"
                  type="link"
                  @click="loadAssignments('group')"
                  >{{ tr("loadMore") }}</Button
                >
              </div>
              <div>
                <div class="segment-heading"><span class="section-title">{{ tr("tags") }}</span></div>
                <Select v-if="canManageStatus" v-model:value="tagInput" data-testid="customer-tag-input" class="mb-3 w-full" mode="tags" :options="tagOptions" :filter-option="false" :disabled="segmentSaving" :loading="segmentSaving" :placeholder="$t('plugin.idc-core.customer.tagBatch.tagsPlaceholder')" @search="searchTags" @change="addEnteredTag" />
                <div v-if="overview.tags.items.length" class="segment-list">
                  <Tag
                    v-for="item in overview.tags.items"
                    :key="item.number"
                    :color="item.colorToken || 'blue'" :closable="canManageStatus" @close.prevent="removeSegment('tag', item)"
                    >{{ localizedSegmentName(item) }}</Tag
                  >
                </div>
                <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />
                <small v-if="overview.tags.hasMore">{{ tr("more") }}</small>
                <Button
                  v-if="overview.tags.hasMore"
                  :loading="assignmentLoading === 'tag'"
                  size="small"
                  type="link"
                  @click="loadAssignments('tag')"
                  >{{ tr("loadMore") }}</Button
                >
              </div>
            </section>
          </div>
        </TabPane>

        <TabPane
          key="orders"
          :tab="`${tr('tabs.orders')} (${overview.business.orders})`"
        >
          <div class="business-table">
            <OrderGrid>
              <template #copyNo="{ row, column }"><div class="flex items-center gap-2"><button class="copy-number" @click="copyNumber(row[column.field])">{{ row[column.field] }}</button><Button v-if="canOpenOrder" size="small" type="link" :data-testid="`customer-order-detail-${row.orderNo}`" @click="openBusinessDetail('order', row.orderNo)">{{ $t('plugin.idc-core.commerce.orders.detail') }}</Button></div></template>
              <template #product="{ row }">{{
                orderProductLabel(row)
              }}</template>
              <template #plain="{ row, column }">{{
                orderPlainLabel(row, column.field)
              }}</template>
              <template #time="{ row }">{{
                formatTimestamp(row.createdAt)
              }}</template>
              <template #payment="{ row }">{{
                paymentLabel(row.paymentMethodCode, row.statusCode)
              }}</template>
              <template #orderStatus="{ row }"
                ><Tag>{{
                  localizedStatus("commerce.orders.status", row.statusCode)
                }}</Tag></template
              >
              <template #orderAmount="{ row }">{{
                formatMinor(row.totalAmountMinor, row.currencyCode)
              }}</template>
            </OrderGrid>
          </div>
        </TabPane>

        <TabPane
          key="invoices"
          :tab="`${tr('tabs.invoices')} (${overview.business.bills})`"
        >
          <div class="business-table">
            <InvoiceGrid>
              <template #copyNo="{ row, column }"><div class="flex items-center gap-2"><button class="copy-number" @click="copyNumber(row[column.field])">{{ row[column.field] }}</button><Button v-if="canOpenInvoice" size="small" type="link" :data-testid="`customer-invoice-detail-${row.invoiceNo}`" @click="openBusinessDetail('invoice', row.invoiceNo)">{{ $t('plugin.idc-core.commerce.orders.detail') }}</Button></div></template>
              <template #plain="{ row, column }">{{
                row[column.field] || "—"
              }}</template>
              <template #time="{ row }">{{
                formatTimestamp(row.createdAt)
              }}</template>
              <template #financeStatus="{ row }"
                ><Tag>{{
                  localizedStatus("finance.status", row.statusCode)
                }}</Tag></template
              >
              <template #invoiceAmount="{ row }">{{
                formatMinor(row.totalAmountMinor, row.currencyCode)
              }}</template>
              <template #outstandingAmount="{ row }">{{
                formatMinor(row.outstandingAmountMinor, row.currencyCode)
              }}</template>
            </InvoiceGrid>
          </div>
        </TabPane>

        <TabPane
          key="transactions"
          :tab="`${tr('tabs.transactions')} (${overview.business.transactions})`"
        >
          <div class="business-table">
            <TransactionGrid>
              <template #copyNo="{ row, column }"><button class="copy-number" @click="copyNumber(row[column.field])">{{ row[column.field] }}</button></template>
              <template #plain="{ row, column }">{{
                row[column.field] || "—"
              }}</template>
              <template #time="{ row }">{{
                formatTimestamp(row.postedAt)
              }}</template>
              <template #transactionType="{ row }">{{
                localizedStatus("finance.status", row.transactionTypeCode)
              }}</template>
              <template #transactionAmount="{ row }">{{
                formatMinor(row.amountMinor, row.currencyCode)
              }}</template>
            </TransactionGrid>
          </div>
        </TabPane>

        <TabPane key="services" :tab="$t('plugin.idc-core.customer.workspace.products')">
          <CustomerProductsPanel v-if="activeTab === 'services'" :customer-no="overview.customer.customerNo" />
        </TabPane>
        <TabPane v-if="apiAvailable && hasAccessByCodes(['idc:admin:channel:token'])" key="api" :tab="$t('plugin.idc-core.customer.workspace.apiOverview')">
          <CustomerApiPanel v-if="activeTab === 'api'" :customer-no="overview.customer.customerNo" />
        </TabPane>

        <TabPane key="tickets">
          <template #tab
            ><span data-testid="customer-360-tab-tickets"
              >{{ tr("businessTickets") }} ({{
                overview.business.tickets
              }})</span
            ></template
          >
          <div class="business-surface" data-testid="customer-360-tickets">
            <div class="business-surface-grid">
              <article
                v-for="item in customerBusinessSurface(['tickets'])"
                :key="item.key"
              >
                <div>
                  <strong>{{ item.count }}</strong
                  ><span>{{ item.label }}</span>
                </div>
                <Button
                  :href="`${item.path}?customerNo=${encodeURIComponent(overview.customer.customerNo)}`"
                  target="_blank"
                  type="primary"
                  >{{ tr("manage") }}</Button
                >
              </article>
            </div>
          </div>
        </TabPane>

        <TabPane key="taxInvoices">
          <template #tab
            ><span data-testid="customer-360-tab-tax-invoices"
              >{{ tr("businessTaxInvoices") }} ({{
                overview.business.taxInvoices
              }})</span
            ></template
          >
          <div class="business-surface" data-testid="customer-360-tax-invoices">
            <div class="business-surface-grid">
              <article
                v-for="item in customerBusinessSurface([
                  'taxInvoices',
                ])"
                :key="item.key"
              >
                <div>
                  <strong>{{ item.count }}</strong
                  ><span>{{ item.label }}</span>
                </div>
                <Button
                  :href="`${item.path}?customerNo=${encodeURIComponent(overview.customer.customerNo)}`"
                  target="_blank"
                  type="primary"
                  >{{ tr("manage") }}</Button
                >
              </article>
            </div>
          </div>
        </TabPane>

        <TabPane key="notes" :tab="tr('notes')"><CustomerActivityPanel notes-only :customer-no="overview.customer.customerNo" @changed="refreshOverview" /></TabPane>
        <TabPane v-if="canReadAudit" key="records" :tab="tr('records')">
          <div class="audit-panel">
            <div class="segment-heading"><span class="text-muted-foreground">{{ tr('recentRecords') }}</span><Button :loading="auditLoading" @click="loadAudit">{{ tr('refresh') }}</Button></div>
            <Alert v-if="auditFailed" type="error" :message="tr('recordsFailed')" show-icon />
            <AuditGrid>
              <template #auditNo="{ row }"><button class="copy-number" @click="copyNumber(row.auditNo)">{{ row.auditNo }}</button></template>
              <template #operation="{ row }">{{ operationLabel(row.operationCode) }}</template>
              <template #auditResult="{ row }"><Tag :color="row.result === 'succeeded' ? 'success' : 'error'">{{ tr(row.result === 'succeeded' ? 'done' : 'notDone') }}</Tag></template>
            </AuditGrid>
          </div>
        </TabPane>
      </Tabs>
    </div>

    <VbenModalCompat :open="!!segmentKind" :confirm-loading="segmentSaving" :title="tr(segmentKind === 'group' ? 'addGroup' : 'addTag')" @update:open="value => { if (!value) { segmentKind = null; ++segmentSearchSequence; } }" @ok="saveSegment"><SegmentForm /></VbenModalCompat>

    <BalanceModal @success="refreshBalanceOverview" />
  </Drawer>
</template>

<style scoped>
.customer-overview {
  display: grid;
  gap: 12px;
}

.tab-panel {
  display: grid;
  gap: 16px;
}

.section-title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.customer-number {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.overview-state {
  align-items: flex-end;
  display: grid;
  gap: 10px;
  justify-items: end;
}

.overview-section {
  display: grid;
  gap: 12px;
}

.business-surface {
  display: grid;
  gap: 12px;
}

.business-surface-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.business-surface-grid article {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  display: flex;
  justify-content: space-between;
  min-height: 82px;
  padding: 16px;
}

.business-surface-grid article > div {
  display: grid;
  gap: 4px;
}

.business-surface-grid strong {
  color: hsl(var(--foreground));
  font-size: 24px;
  line-height: 1;
}

.business-surface-grid span {
  color: hsl(var(--muted-foreground));
}

.section-title {
  color: hsl(var(--foreground));
}

.count-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.balance-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.balance-card {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  display: grid;
  gap: 4px;
  padding: 14px;
}

.balance-card > strong {
  color: hsl(var(--foreground));
  font-size: 22px;
  font-variant-numeric: tabular-nums;
}

.balance-heading,
.balance-detail,
.table-footer {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.balance-detail {
  color: hsl(var(--muted-foreground));
  flex-wrap: wrap;
  font-size: 12px;
  gap: 8px;
  margin-top: 8px;
}

.business-table {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.table-footer {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.count-card {
  background: hsl(var(--muted) / 55%);
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: grid;
  gap: 3px;
  min-height: 72px;
  padding: 12px;
}

.count-card strong {
  color: hsl(var(--foreground));
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}

.count-card span,
.overview-section small {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.segment-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.segment-grid > div {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  min-height: 142px;
  padding: 14px;
}

.segment-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 14px 0 8px;
}

@media (width <= 720px) {
  .balance-grid,
  .count-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .segment-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style scoped>
.customer-overview { width: 100%; min-width: 0; gap: 20px; }
.customer-overview :deep(.ant-tabs), .customer-overview :deep(.ant-tabs-content-holder), .customer-overview :deep(.ant-tabs-content), .customer-overview :deep(.ant-tabs-tabpane) { width: 100%; min-width: 0; max-width: 100%; }
.customer-overview :deep(.ant-tabs-content-holder) { overflow: hidden; }
.business-table, .audit-panel, .tab-panel { width: 100%; min-width: 0; max-width: 100%; }
.detail-header, .detail-actions, .segment-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.detail-actions { flex-wrap: wrap; justify-content: flex-end; }
.detail-header h2 { margin: 0 0 6px; font-size: 18px; font-weight: 600; }
.copy-number { padding: 0; color: #1677ff; text-decoration: underline; text-underline-offset: 3px; overflow-wrap: anywhere; cursor: pointer; }
.account-lines, .audit-panel { display: grid; gap: 8px; }
.business-surface-grid { grid-template-columns: repeat(auto-fit, minmax(min(280px, 100%), 1fr)); }
@media (width <= 720px) { .detail-header { align-items: flex-start; flex-direction: column; } }
</style>

<style scoped>
.verification-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.verification-actions .ant-tag { margin: 0; }
</style>
