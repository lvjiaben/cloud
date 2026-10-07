<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { IsolatedTableColumn } from "@linapro/vben-plugin-adapter/isolated";
import type { PluginHostMountContext } from "./host-context";
import StandaloneTable from "./StandaloneTable.vue";
import { messages } from "./messages";
import { createRequest } from "./request";
import { usePaymentPage } from "./payment-page";

type Health = { channelNo: string; displayName: string; providerCode: string; capabilityCode: string; healthCode: string; latestBatchNo: string; latestBusinessDate?: number; unresolvedCount: number; resultUnknown: boolean };
type Batch = { batchNo: string; channelNo: string; capabilityCode: string; statusCode: string; businessDate: number; paymentCount: number; refundCount: number; differenceCount: number; resultUnknown: boolean };
type Difference = { lineNo: string; recordTypeCode: string; attemptNo: string; refundAttemptNo: string; externalReference: string; differenceCode: string; localAmountMinor: string; channelAmountMinor: string; currencyCode: string; localStatusCode: string; channelStatusCode: string; resolutionCode: string; version: number; supplementRequestNo: string; supplementStatusCode: string; supplementSubmitterUserId: number; supplementVersion: number };
type ExceptionFund = { exceptionNo: string; attemptNo: string; orderNo: string; channelNo: string; providerCode: string; currencyCode: string; amountMinor: string; reasonCode: string; statusCode: string; createdAt: number; actionNo: string; actionCode: string; actionStatus: string; targetOrderNo: string; proofReference: string; version: number };
const props = defineProps<{ context: PluginHostMountContext }>();
const locale = computed(() => props.context.locale.toLowerCase().startsWith("en") ? "en-US" : "zh-CN");
const t = (key: keyof typeof messages["zh-CN"]) => messages[locale.value][key] ?? key;
const request = createRequest(() => props.context);
const apiBase = "/x/idc-payment/api/v1";
const isExceptionFunds = computed(() => props.context.routePath.includes("payment-exception-funds"));
const targetOrders = ref<Record<string, string>>({});
const proofFiles = ref<Record<string, File | undefined>>({});
const selectedChannel = ref(""), selectedBatch = ref("");
const error = ref("");
const loadFailed = () => { error.value = t("loadFailed"); };
const healthPage = usePaymentPage<Health>(async (page, pageSize) => {
  const result = await request<{ channels: Health[]; total: number }>(`${apiBase}/admin/payment-reconciliation/channels?page=${page}&pageSize=${pageSize}`);
  return { items: result.channels, total: result.total };
}, loadFailed, clearReconciliation);
const batchPage = usePaymentPage<Batch>(async (page, pageSize) => {
  const result = await request<{ batches: Batch[]; total: number }>(`${apiBase}/admin/payment-reconciliation/batches?page=${page}&pageSize=${pageSize}&channelNo=${encodeURIComponent(selectedChannel.value)}`);
  return { items: result.batches, total: result.total };
}, loadFailed, clearReconciliation);
const differencePage = usePaymentPage<Difference>(async (page, pageSize) => {
  const result = await request<{ differences: Difference[]; total: number }>(`${apiBase}/admin/payment-reconciliation/batches/${encodeURIComponent(selectedBatch.value)}/differences?page=${page}&pageSize=${pageSize}`);
  return { items: result.differences, total: result.total };
}, loadFailed, clearReconciliation);
const exceptionPage = usePaymentPage<ExceptionFund>(async (page, pageSize) => {
  const result = await request<{ funds: ExceptionFund[]; total: number }>(`${apiBase}/admin/payment-exception-funds?page=${page}&pageSize=${pageSize}`);
  return { items: result.funds, total: result.total };
}, loadFailed, () => { targetOrders.value = {}; proofFiles.value = {}; });
const loading = computed(() => [healthPage, batchPage, differencePage, exceptionPage].some(item => item.state.loading));
function clearReconciliation() { healthPage.reset(); batchPage.reset(); differencePage.reset(); selectedBatch.value = ""; }
const rejectionReason = ref("evidence_insufficient");
const rejectionReasons = ["evidence_insufficient", "ownership_mismatch", "amount_or_currency_mismatch", "provider_reference_conflict", "already_confirmed"] as const;
const money = (minor: string, currency: string) => {
  try {
    const value = BigInt(minor);
    const absolute = value < 0n ? -value : value;
    return `${currency} ${value < 0n ? "-" : ""}${absolute / 100n}.${(absolute % 100n).toString().padStart(2, "0")}`;
  } catch {
    return `${currency} —`;
  }
};
const date = (value?: number) => value ? new Intl.DateTimeFormat(locale.value, { dateStyle: "medium" }).format(new Date(value)) : "—";
const statusText = (value: string) => {
  const aliases: Record<string, keyof typeof messages["zh-CN"]> = { differences: "batch_differences", submitted: "submitted_state", succeeded: "succeeded_state", refunded: "refunded_state", pending: "pending_state" };
  const key = aliases[value] ?? value as keyof typeof messages["zh-CN"];
  return messages[locale.value][key] ?? value;
};
async function load() { error.value = ""; await Promise.all([healthPage.load(), loadBatches()]); }
async function loadBatches(page?: number, pageSize?: number) {
  if (!await batchPage.load(page, pageSize)) return;
  if (!batchPage.state.items.some(item => item.batchNo === selectedBatch.value)) selectedBatch.value = batchPage.state.items[0]?.batchNo ?? "";
  else await loadDifferences();
}
async function loadDifferences(page?: number, pageSize?: number) { if (!selectedBatch.value) { differencePage.reset(); return; } await differencePage.load(page, pageSize); }
async function submit(item: Difference) { try { await request(`${apiBase}/admin/payment-reconciliation/differences/${encodeURIComponent(item.lineNo)}/supplements`, { method: "POST", bodyObject: { expectedVersion: item.version } }); await loadDifferences(); } catch { error.value = t("actionFailed"); } }
async function review(item: Difference, decisionCode: "approve" | "reject") { try { await request(`${apiBase}/admin/payment-reconciliation/supplements/${encodeURIComponent(item.supplementRequestNo)}/review`, { method: "POST", bodyObject: { decisionCode, rejectionReasonCode: decisionCode === "reject" ? rejectionReason.value : "", expectedVersion: item.supplementVersion } }); await load(); } catch { error.value = t("actionFailed"); } }
async function loadExceptionFunds() { error.value = ""; await exceptionPage.load(); }
async function allocateFund(item: ExceptionFund) { const targetOrderNo = targetOrders.value[item.exceptionNo]?.trim(); if (!targetOrderNo) { error.value = t("targetRequired"); return; } try { await request(`${apiBase}/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/allocation`, { method: "POST", bodyObject: { targetOrderNo } }); await loadExceptionFunds(); } catch { error.value = t("allocationFailed"); } }
async function refundFund(item: ExceptionFund) { try { await request(`${apiBase}/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/original-refund`, { method: "POST", bodyObject: {} }); await loadExceptionFunds(); } catch { error.value = t("actionFailed"); } }
function selectProof(item: ExceptionFund, event: Event) { proofFiles.value[item.exceptionNo] = (event.target as HTMLInputElement).files?.[0]; }
async function confirmManualRefund(item: ExceptionFund) { const file = proofFiles.value[item.exceptionNo]; if (!file) { error.value = t("proofRequired"); return; } try { const form = new FormData(); form.append("file", file); const uploaded = await request<{ proofReference: string; proofDigest: string }>(`${apiBase}/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/manual-refund-proof`, { method: "POST", body: form }); await request(`${apiBase}/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/manual-refund-confirmation`, { method: "POST", bodyObject: { proofReference: uploaded.proofReference, proofDigest: uploaded.proofDigest, expectedVersion: item.version } }); delete proofFiles.value[item.exceptionNo]; await loadExceptionFunds(); } catch { error.value = t("actionFailed"); } }
watch(selectedChannel, () => { if (!isExceptionFunds.value) { selectedBatch.value = ""; batchPage.reset(); differencePage.reset(); void loadBatches(1); } });
watch(selectedBatch, () => { differencePage.reset(); if (!isExceptionFunds.value) void loadDifferences(1); });
onMounted(() => void (isExceptionFunds.value ? loadExceptionFunds() : load()));
onBeforeUnmount(() => { for (const item of [healthPage, batchPage, differencePage, exceptionPage]) item.invalidate(); });

function textNode(tag: string, text: string) {
  const node = document.createElement(tag);
  node.textContent = text;
  return node;
}
function actionButton(label: string, action: () => unknown, danger = false) {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  button.className = danger ? "standalone-table-action danger" : "standalone-table-action";
  button.addEventListener("click", () => { void action(); });
  return button;
}
function selectionButton(label: string, selected: boolean, action: () => unknown) {
  const button = actionButton(label, action);
  button.setAttribute("aria-pressed", String(selected));
  return button;
}
function reconciliationStatus(status: string, resultUnknown: boolean) {
  const nodes = [textNode("span", statusText(status))];
  if (resultUnknown) {
    const warning = textNode("small", t("unknown"));
    warning.className = "unknown";
    nodes.push(warning);
  }
  return nodes;
}
function differenceActions(item: Difference) {
  if (item.differenceCode === "local_missing" && item.channelStatusCode === "succeeded" && !item.supplementRequestNo) {
    return actionButton(t("submit"), () => submit(item));
  }
  if (item.supplementStatusCode !== "pending_review") {
    return statusText(item.supplementStatusCode || item.resolutionCode);
  }
  const container = document.createElement("div");
  container.className = "review";
  const reason = document.createElement("select");
  for (const value of rejectionReasons) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = t(value);
    reason.append(option);
  }
  reason.value = rejectionReason.value;
  reason.addEventListener("change", () => { rejectionReason.value = reason.value; });
  container.append(textNode("span", t("pending_review")), actionButton(t("approve"), () => review(item, "approve")), reason, actionButton(t("reject"), () => { rejectionReason.value = reason.value; return review(item, "reject"); }, true));
  return container;
}
function exceptionActions(item: ExceptionFund) {
  if (item.actionCode && item.actionStatus !== "manual_pending") return statusText(item.actionStatus || item.actionCode);
  const container = document.createElement("div");
  container.className = "fund-actions";
  container.dataset.testid = `payment-exception-fund-${item.exceptionNo}`;
  const label = document.createElement("label");
  const input = document.createElement("input");
  if (!item.actionCode) {
    label.append(t("targetOrder"));
    input.placeholder = t("targetOrderPlaceholder");
    input.value = targetOrders.value[item.exceptionNo] ?? "";
    input.addEventListener("input", () => { targetOrders.value[item.exceptionNo] = input.value; });
    label.append(input);
    container.append(label, actionButton(t("allocate"), () => allocateFund(item)), actionButton(t("originalRefund"), () => refundFund(item), true));
  } else {
    label.append(t("proofFile"));
    input.type = "file";
    input.addEventListener("change", event => selectProof(item, event));
    label.append(input);
    container.append(label, actionButton(t("confirmRefund"), () => confirmManualRefund(item)));
  }
  return container;
}
const differenceColumns = computed<IsolatedTableColumn<Difference>[]>(() => [
  { field: "differenceCode", title: t("status"), minWidth: 180, cell: item => [textNode("strong", statusText(item.differenceCode)), textNode("small", `${statusText(item.localStatusCode) || "—"} → ${statusText(item.channelStatusCode) || "—"}`)] },
  { field: "externalReference", title: t("transaction"), minWidth: 220, cell: item => [textNode("code", item.externalReference || "—"), textNode("small", item.attemptNo || item.refundAttemptNo)] },
  { field: "localAmountMinor", title: t("amount"), minWidth: 220, cell: item => `${money(item.localAmountMinor, item.currencyCode)} / ${money(item.channelAmountMinor, item.currencyCode)}` },
  { field: "actions", title: t("action"), minWidth: 380, fixed: "right", cell: differenceActions },
]);
const exceptionColumns = computed<IsolatedTableColumn<ExceptionFund>[]>(() => [
  { field: "statusCode", title: t("status"), minWidth: 200, cell: item => [textNode("strong", statusText(item.actionStatus || item.statusCode)), textNode("small", `${item.providerCode} · ${item.channelNo}`)] },
  { field: "orderNo", title: t("originalOrder"), minWidth: 220, cell: item => [textNode("code", item.orderNo), textNode("small", item.attemptNo)] },
  { field: "amountMinor", title: t("amount"), minWidth: 160, cell: item => money(item.amountMinor, item.currencyCode) },
  { field: "actions", title: t("action"), minWidth: 380, fixed: "right", cell: exceptionActions },
]);
const healthColumns = computed<IsolatedTableColumn<Health>[]>(() => {
  const selection = selectedChannel.value;
  return [
  { field: "displayName", title: t("channel"), minWidth: 180, cell: item => [selectionButton(item.displayName, selection === item.channelNo, () => { selectedChannel.value = selectedChannel.value === item.channelNo ? "" : item.channelNo; }), textNode("small", item.providerCode)] },
  { field: "capabilityCode", title: t("capability"), minWidth: 180, cell: item => statusText(item.capabilityCode) },
  { field: "healthCode", title: t("health"), minWidth: 130, cell: item => reconciliationStatus(item.healthCode, item.resultUnknown) },
  { field: "latestBusinessDate", title: t("latestDate"), minWidth: 150, cell: item => date(item.latestBusinessDate) },
  { field: "unresolvedCount", title: t("unresolved"), minWidth: 140 },
];
});
const batchColumns = computed<IsolatedTableColumn<Batch>[]>(() => {
  const selection = selectedBatch.value;
  return [
  { field: "businessDate", title: t("latestDate"), minWidth: 150, cell: item => date(item.businessDate) },
  { field: "channelNo", title: t("channel"), minWidth: 180 },
  { field: "paymentCount", title: t("counts"), minWidth: 180, cell: item => `${item.paymentCount} / ${item.refundCount} / ${item.differenceCount}` },
  { field: "statusCode", title: t("status"), minWidth: 140, cell: item => reconciliationStatus(item.statusCode, item.resultUnknown) },
  { field: "action", title: t("action"), minWidth: 110, fixed: "right", cell: item => selectionButton(t("differences"), selection === item.batchNo, () => { selectedBatch.value = item.batchNo; }) },
];
});
const pager = (state: { currentPage: number; pageSize: number; total: number }) => ({ currentPage: state.currentPage, pageSize: state.pageSize, total: state.total, pageSizes: [10, 20, 50, 100] });
</script>

<template>
  <main class="workspace">
    <div class="page-toolbar"><button class="secondary" :disabled="loading" @click="isExceptionFunds ? loadExceptionFunds() : load()">{{ t("refresh") }}</button></div>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>
    <template v-if="!isExceptionFunds">
    <section><h2>{{ t("channelHealth") }}</h2><StandaloneTable :columns="healthColumns" :rows="healthPage.state.items" row-key="channelNo" :locale="locale" :empty-text="t('empty')" :loading="healthPage.state.loading" :pagination="pager(healthPage.state)" @page-change="healthPage.load($event.currentPage, $event.pageSize)" @failed="loadFailed" /></section>
    <section><h2>{{ t("batches") }}</h2><StandaloneTable :columns="batchColumns" :rows="batchPage.state.items" row-key="batchNo" :locale="locale" :empty-text="t('empty')" :loading="batchPage.state.loading" :pagination="pager(batchPage.state)" @page-change="loadBatches($event.currentPage, $event.pageSize)" @failed="loadFailed" /></section>
    <section><h2>{{ t("differences") }}</h2><StandaloneTable :columns="differenceColumns" :rows="differencePage.state.items" row-key="lineNo" :locale="locale" :empty-text="t('empty')" :loading="differencePage.state.loading" :pagination="pager(differencePage.state)" @page-change="loadDifferences($event.currentPage, $event.pageSize)" @failed="loadFailed" /></section>
    </template>
    <section v-else><p class="help">{{ t("exceptionHelp") }}</p><StandaloneTable :columns="exceptionColumns" :rows="exceptionPage.state.items" row-key="exceptionNo" :locale="locale" :empty-text="t('empty')" :loading="exceptionPage.state.loading" :pagination="pager(exceptionPage.state)" @page-change="exceptionPage.load($event.currentPage, $event.pageSize)" @failed="loadFailed" /></section>
  </main>
</template>
