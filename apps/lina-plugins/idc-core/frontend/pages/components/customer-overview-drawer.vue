<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  CustomerAccountTransitionReason,
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
import { createNumberedPage } from "../numbered-page";

import { useAccess } from "../plugin-access";
import { useVbenDrawer } from "@vben/common-ui";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Empty,
  TabPane,
  Tag,
  Tabs,
  message,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  customerAccountTransition,
  customerAssignments,
  customerAssistanceEnd,
  customerAssistanceStart,
  customerOverview,
  customerReferrer,
} from "../customer-client";
import { commerceOrders } from "../commerce-client";
import { financialInvoices, financialTransactions } from "../financial-client";
import type { CustomerAssistanceSession } from "../customer-client";
import type { ContractPolicyRule } from "../contract-policy-client";
import ContractPolicyEditor from "./contract-policy-editor.vue";
import CustomerActivityPanel from "./customer-activity-panel.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

const overview = ref<CustomerOverview>();
const groupCursor = ref("");
const tagCursor = ref("");
const assignmentLoading = ref<"group" | "tag" | "">("");
const assistanceOpen = ref(false);
const assistanceSaving = ref(false);
const assistanceConfirmed = ref(false);
const assistanceSession = ref<CustomerAssistanceSession>();
const referrer = ref<CustomerReferrer>();
const statusOpen = ref(false);
const statusSaving = ref(false);
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
const canEditContractPolicy = computed(() =>
  hasAccessByCodes(["idc:admin:customer:contract-policy:write"]),
);
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
    { field: "orderNo", title: tr("table.orderNo"), width: 210 },
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
    { field: "invoiceNo", title: tr("table.billNo"), width: 210 },
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
    { field: "transactionNo", title: tr("table.transactionNo"), width: 230 },
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

function assistanceFormSchema() {
  return [
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "customer-assistance-reason",
        maxlength: 512,
        rows: 4,
        showCount: true,
      },
      fieldName: "reason",
      label: $t("plugin.idc-core.customer.assistance.reason"),
      rules: z.string().trim().min(10).max(512),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-assistance-ticket",
        maxlength: 64,
      },
      fieldName: "ticketNo",
      label: $t("plugin.idc-core.customer.assistance.ticketNo"),
      rules: z.string().max(64).optional(),
    },
  ];
}

const [AssistanceForm, assistanceFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

const [StatusForm, statusFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

function statusTransitionOptions() {
  switch (overview.value?.customer.status) {
    case "pending_activation":
      return [
        {
          label: tr("statusAction.activate"),
          value: "active:activation_completed",
        },
        {
          label: tr("statusAction.requireVerification"),
          value: "restricted:verification_required",
        },
        { label: tr("statusAction.disable"), value: "disabled:manual_disable" },
      ];
    case "active":
      return [
        {
          label: tr("statusAction.restrictRisk"),
          value: "restricted:risk_restriction",
        },
        {
          label: tr("statusAction.requireVerification"),
          value: "restricted:verification_required",
        },
        { label: tr("statusAction.disable"), value: "disabled:manual_disable" },
      ];
    case "restricted":
      return [
        {
          label: tr("statusAction.resolveRestriction"),
          value: "active:restriction_resolved",
        },
        { label: tr("statusAction.disable"), value: "disabled:manual_disable" },
      ];
    case "disabled":
      return [
        {
          label: tr("statusAction.reactivate"),
          value: "active:account_reactivated",
        },
      ];
    default:
      return [];
  }
}

async function openStatusEditor() {
  if (!overview.value) return;
  statusOpen.value = true;
  await nextTick();
  await statusFormApi.setState({
    schema: [
      {
        component: "Select",
        componentProps: {
          "data-testid": "customer-status-transition",
          options: statusTransitionOptions(),
          placeholder: tr("statusSelectPlaceholder"),
        },
        fieldName: "transition",
        label: tr("statusActionLabel"),
        rules: "required",
      },
    ],
  });
  await statusFormApi.resetForm();
}

async function saveStatus() {
  if (!overview.value || statusSaving.value) return;
  const { valid } = await statusFormApi.validate();
  if (!valid) return;
  const values = await statusFormApi.getValues<{ transition: string }>();
  const [toStatus, reason] = values.transition.split(":") as [
    "active" | "disabled" | "restricted",
    CustomerAccountTransitionReason,
  ];
  statusSaving.value = true;
  try {
    await customerAccountTransition(overview.value.customer.customerNo, {
      expectedVersion: overview.value.customer.version,
      idempotencyKey: `customer-status-${crypto.randomUUID()}`,
      reason,
      toStatus,
    });
    statusOpen.value = false;
    await refreshOverview();
    message.success(tr("statusUpdated"));
  } finally {
    statusSaving.value = false;
  }
}

async function openAssistance() {
  assistanceConfirmed.value = false;
  assistanceOpen.value = true;
  await nextTick();
  await assistanceFormApi.setState({ schema: assistanceFormSchema() });
  await assistanceFormApi.resetForm();
}

async function startAssistance() {
  if (!overview.value || assistanceSaving.value) return;
  const { valid } = await assistanceFormApi.validate();
  if (!valid || !assistanceConfirmed.value) {
    message.warning($t("plugin.idc-core.customer.assistance.completeRequired"));
    return;
  }
  const values = await assistanceFormApi.getValues<{
    reason: string;
    ticketNo?: string;
  }>();
  const reason = values.reason.trim();
  const targetWindow = window.open("about:blank", "_blank");
  if (!targetWindow) {
    message.warning($t("plugin.idc-core.customer.assistance.popupBlocked"));
    return;
  }
  assistanceSaving.value = true;
  try {
    const result = await customerAssistanceStart(
      overview.value.customer.customerNo,
      {
        idempotencyKey: `customer-assistance-${crypto.randomUUID()}`,
        reason,
        ticketNo: values.ticketNo?.trim() || undefined,
      },
    );
    assistanceSession.value = result;
    assistanceOpen.value = false;
    if (result.launchUrl) {
      targetWindow.location.replace(result.launchUrl);
    } else {
      targetWindow.close();
      message.warning($t("plugin.idc-core.customer.assistance.popupBlocked"));
      return;
    }
    message.success($t("plugin.idc-core.customer.assistance.started"));
  } catch (error) {
    targetWindow.close();
    throw error;
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

function applyContractPolicy(rule: ContractPolicyRule) {
  if (!overview.value) return;
  overview.value.customer.enterpriseContractPolicyCode = rule.policyCode as
    | "none"
    | "required";
  overview.value.customer.enterpriseContractTemplateNo = rule.templateNo;
  overview.value.customer.version = rule.version;
}

const title = computed(() =>
  overview.value?.customer.displayName
    ? `${$t("plugin.idc-core.customer.overview.title")} · ${overview.value.customer.displayName}`
    : $t("plugin.idc-core.customer.overview.title"),
);

const countCards = computed(() => {
  const counts = overview.value?.counts;
  if (!counts) return [];
  return [
    ["activeContacts", counts.activeContacts],
    ["activeAddresses", counts.activeAddresses],
    ["activeCustomFields", counts.activeCustomFields],
    ["activeNotes", counts.activeNotes],
    ["plannedFollowUps", counts.plannedFollowUps],
    ["availableAttachments", counts.availableAttachments],
    ["activeBlacklist", counts.activeBlacklist],
    ["openRisks", counts.openRisks],
    ["activeGroups", counts.activeGroups],
    ["activeTags", counts.activeTags],
  ] as const;
});

const businessCards = computed(() => {
  const business = overview.value?.business;
  if (!business) return [];
  return [
    ["Orders", business.orders, "/admin/idc/orders"],
    ["Bills", business.bills, "/admin/idc/finance"],
    ["Transactions", business.transactions, "/admin/idc/finance"],
    ["Servers", business.servers, "/admin/idc/resource/overview"],
    [
      "Contracts",
      business.contracts,
      "/admin/idc/compliance/electronic-contract",
    ],
    ["TaxInvoices", business.taxInvoices, "/admin/idc/compliance/tax-invoices"],
    ["Tickets", business.tickets, "/admin/idc/support"],
    ["Notifications", business.notifications, "/admin/system/message/notice"],
    ["Audits", business.audits, "/admin/system/log/operation"],
  ] as const;
});

const customerBusinessSurfaces = computed(() => {
  const business = overview.value?.business;
  if (!business) return [];
  return [
    {
      count: business.servers,
      key: "servers",
      label: tr("businessServers"),
      path: "/admin/idc/resource/overview",
    },
    {
      count: business.tickets,
      key: "tickets",
      label: tr("businessTickets"),
      path: "/admin/idc/support",
    },
    {
      count: business.contracts,
      key: "contracts",
      label: tr("businessContracts"),
      path: "/admin/idc/compliance/electronic-contract",
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
      path: "/admin/system/message/notice",
    },
    {
      count: business.audits,
      key: "audits",
      label: tr("businessAudits"),
      path: "/admin/system/log/operation",
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

function verificationColor(status = "") {
  return status === "verified"
    ? "success"
    : status === "pending"
      ? "processing"
      : status === "unverified"
        ? "default"
        : "error";
}

const [Drawer, drawerApi] = useVbenDrawer({
  footer: false,
  async onOpenChange(open) {
    const sequence = ++overviewSequence;
    if (!open) {
      overview.value = undefined;
      referrer.value = undefined;
      assistanceSession.value = undefined;
      groupCursor.value = "";
      tagCursor.value = "";
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
    const data = drawerApi.getData<{ customerNo?: string }>();
    if (!data?.customerNo) return;
    drawerApi.setState({ loading: true });
    try {
      const result = await customerOverview(data.customerNo);
      if (sequence !== overviewSequence) return;
      overview.value = result;
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
  <Drawer :title="title" class="w-[min(1180px,calc(100vw-24px))]">
    <div
      v-if="overview"
      class="customer-overview"
      data-testid="customer-overview-drawer"
    >
      <Card size="small" :title="overview.customer.displayName">
        <template #extra>
          <div class="overview-state">
            <Badge
              :status="
                overview.customer.status === 'active' ? 'success' : 'warning'
              "
              :text="
                $t(
                  `plugin.idc-core.customer.status.${overview.customer.status}`,
                )
              "
            />
            <Tag :color="verificationColor(overview.customer.verification)">{{
              $t(
                `plugin.idc-core.customer.verification.${overview.customer.verification}`,
              )
            }}</Tag>
            <Button
              v-if="
                canManageStatus && overview.customer.status !== 'anonymized'
              "
              data-testid="customer-status-open"
              size="small"
              @click="openStatusEditor"
              >{{ tr("editStatus") }}</Button
            >
            <Button
              v-if="canAssist"
              data-testid="customer-assistance-open"
              size="small"
              @click="openAssistance"
              >{{ $t("plugin.idc-core.customer.assistance.open") }}</Button
            >
          </div>
        </template>
        <span class="customer-number">{{
          overview.customer.customerCode || overview.customer.customerNo
        }}</span>
      </Card>

      <Descriptions bordered :column="2" size="small">
        <Descriptions.Item :label="tr('customerId')">{{
          overview.customer.customerCode || "—"
        }}</Descriptions.Item>
        <Descriptions.Item :label="tr('phone')">{{
          overview.customer.primaryPhone || "—"
        }}</Descriptions.Item>
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
        <Descriptions.Item :label="tr('accountStatus')">{{
          $t(`plugin.idc-core.customer.status.${overview.customer.status}`)
        }}</Descriptions.Item>
        <Descriptions.Item :label="tr('verificationStatus')">{{
          $t(
            `plugin.idc-core.customer.verification.${overview.customer.verification}`,
          )
        }}</Descriptions.Item>
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
            <ContractPolicyEditor
              v-if="
                overview.customer.type === 'enterprise' && canEditContractPolicy
              "
              :level="'enterprise'"
              :policy-code="
                overview.customer.enterpriseContractPolicyCode || 'none'
              "
              :target-no="overview.customer.customerNo"
              :template-no="overview.customer.enterpriseContractTemplateNo"
              :version="overview.customer.version"
              @updated="applyContractPolicy"
            />

            <section class="overview-section">
              <div class="section-title">{{ tr("balances") }}</div>
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

            <section class="overview-section">
              <div class="section-title">{{ tr("counts") }}</div>
              <div class="count-grid">
                <article
                  v-for="[key, value] in countCards"
                  :key="key"
                  class="count-card"
                >
                  <strong>{{ value }}</strong
                  ><span>{{ tr(key) }}</span>
                </article>
              </div>
            </section>

            <section class="overview-section">
              <div class="section-title">{{ tr("business") }}</div>
              <div class="count-grid">
                <article
                  v-for="[key, value, path] in businessCards"
                  :key="key"
                  class="count-card"
                >
                  <strong>{{ value }}</strong
                  ><span>{{ tr(`business${key}`) }}</span>
                  <Button
                    :href="`${path}?customerNo=${encodeURIComponent(overview.customer.customerNo)}`"
                    size="small"
                    target="_blank"
                    type="link"
                    >{{ tr("manage") }}</Button
                  >
                </article>
              </div>
            </section>

            <section class="overview-section segment-grid">
              <div>
                <div class="section-title">{{ tr("groups") }}</div>
                <div v-if="overview.groups.items.length" class="segment-list">
                  <Tag
                    v-for="item in overview.groups.items"
                    :key="item.number"
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
                <div class="section-title">{{ tr("tags") }}</div>
                <div v-if="overview.tags.items.length" class="segment-list">
                  <Tag
                    v-for="item in overview.tags.items"
                    :key="item.number"
                    :color="item.colorToken || 'blue'"
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

        <TabPane key="services">
          <template #tab
            ><span data-testid="customer-360-tab-services"
              >{{ tr("businessServers") }} ({{
                overview.business.servers
              }})</span
            ></template
          >
          <div class="business-surface" data-testid="customer-360-services">
            <div class="business-surface-grid">
              <article
                v-for="item in customerBusinessSurface(['servers'])"
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

        <TabPane key="contracts">
          <template #tab
            ><span data-testid="customer-360-tab-contracts"
              >{{ tr("businessContracts") }} /
              {{ tr("businessTaxInvoices") }} ({{
                overview.business.contracts + overview.business.taxInvoices
              }})</span
            ></template
          >
          <div class="business-surface" data-testid="customer-360-contracts">
            <div class="business-surface-grid">
              <article
                v-for="item in customerBusinessSurface([
                  'contracts',
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

        <TabPane key="records">
          <template #tab
            ><span data-testid="customer-360-tab-records"
              >{{ tr("businessNotifications") }} /
              {{ tr("businessAudits") }} ({{
                overview.business.notifications + overview.business.audits
              }})</span
            ></template
          >
          <div class="business-surface" data-testid="customer-360-records">
            <div class="business-surface-grid">
              <article
                v-for="item in customerBusinessSurface([
                  'notifications',
                  'audits',
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

        <TabPane key="activity" :tab="tr('tabs.activity')"
          ><CustomerActivityPanel
            :customer-no="overview.customer.customerNo"
            @changed="refreshOverview"
        /></TabPane>
      </Tabs>
    </div>

    <VbenModalCompat
      v-model:open="assistanceOpen"
      :confirm-loading="assistanceSaving"
      :title="$t('plugin.idc-core.customer.assistance.title')"
      @ok="startAssistance"
    >
      <Alert
        :message="$t('plugin.idc-core.customer.assistance.warningTitle')"
        :description="
          $t('plugin.idc-core.customer.assistance.warningDescription')
        "
        class="mb-4"
        show-icon
        type="warning"
      />
      <AssistanceForm />
      <Checkbox
        v-model:checked="assistanceConfirmed"
        data-testid="customer-assistance-confirm"
        >{{ $t("plugin.idc-core.customer.assistance.confirm") }}</Checkbox
      >
    </VbenModalCompat>
    <VbenModalCompat
      v-model:open="statusOpen"
      :confirm-loading="statusSaving"
      :title="tr('editStatusTitle')"
      @ok="saveStatus"
    >
      <Alert
        :message="tr('statusWarning')"
        class="mb-4"
        show-icon
        type="warning"
      />
      <StatusForm />
    </VbenModalCompat>
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
