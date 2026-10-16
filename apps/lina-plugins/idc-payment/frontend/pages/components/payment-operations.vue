<script setup lang="ts">
import { useAdminGridLayout } from "../../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ExceptionFund,
  PaymentBatch,
  PaymentDifference,
  PaymentHealth,
} from "../payment-client";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Input,
  Select,
  Tag,
  Tabs,
  TabPane,
  Upload,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { fixedRightActionColumn } from "../../../../vben-adapter/frontend/src";
import { $t } from "#/locales";
import { paymentAccessDenied, usePaymentPage } from "../../src/payment-page";
import {
  allocateExceptionFund,
  confirmExceptionFundRefund,
  getExceptionFunds,
  getPaymentBatches,
  getPaymentChannels,
  getPaymentDifferences,
  refundExceptionFund,
  reviewPaymentSupplement,
  submitPaymentSupplement,
  uploadExceptionFundProof,
} from "../payment-client";

const props = defineProps<{ mode: "exceptions" | "reconciliation" }>();
const tr = (key: string) => $t(`plugin.idc-payment.${key}`);
const channelNo = ref("");
const batchNo = ref("");
const activeSection = ref("batches");
const rejectionReason = ref("evidence_insufficient");
const targetOrders = ref<Record<string, string>>({});
const proofFiles = ref<Record<string, File | undefined>>({});
const channelChoices = ref<Array<{ value: string; label: string }>>([]);
let choicePage = 0;
let choiceTotal = 0;
let choicesLoading = false;
let choiceSequence = 0;
const loadFailed = () => message.error(tr("loadFailed"));
const healthDirectory = usePaymentPage<PaymentHealth>(async (page, size) => {
  const result = await getPaymentChannels(page, size);
  return { items: result.channels || [], total: result.total };
}, loadFailed, clearReconciliation);
const batchDirectory = usePaymentPage<PaymentBatch>(async (page, size) => {
  const result = await getPaymentBatches(channelNo.value || "", page, size);
  return { items: result.batches || [], total: result.total };
}, loadFailed, clearReconciliation);
const differenceDirectory = usePaymentPage<PaymentDifference>(async (page, size) => {
  const result = await getPaymentDifferences(batchNo.value, page, size);
  return { items: result.differences || [], total: result.total };
}, loadFailed, clearReconciliation);
const fundDirectory = usePaymentPage<ExceptionFund>(async (page, size) => {
  const result = await getExceptionFunds(page, size);
  return { items: result.funds || [], total: result.total };
}, loadFailed, () => { targetOrders.value = {}; proofFiles.value = {}; });
const batches = computed(() => batchDirectory.state.items);
const loading = computed(() => [healthDirectory, batchDirectory, differenceDirectory, fundDirectory].some(item => item.state.loading));

function clearReconciliation() {
  healthDirectory.reset(); batchDirectory.reset(); differenceDirectory.reset();
  choiceSequence += 1; channelChoices.value = []; choicePage = 0; choiceTotal = 0;
  batchNo.value = "";
}
async function loadChannelChoices() {
  if (choicesLoading || (choicePage > 0 && channelChoices.value.length >= choiceTotal)) return;
  choicesLoading = true;
  const sequence = choiceSequence;
  try {
    const result = await getPaymentChannels(choicePage + 1, 100);
    if (sequence !== choiceSequence) return;
    const choices = new Map(channelChoices.value.map(item => [item.value, item]));
    for (const item of result.channels) choices.set(item.channelNo, { value: item.channelNo, label: item.displayName });
    channelChoices.value = [...choices.values()];
    choicePage += 1; choiceTotal = result.total;
  } catch (error) {
    if (sequence === choiceSequence) { if (paymentAccessDenied(error)) clearReconciliation(); loadFailed(); }
  } finally { choicesLoading = false; }
}
function channelSearchSchema() {
  return [{ fieldName: "channelNo", component: "Select" as const, label: tr("channel"), componentProps: {
    allowClear: true, options: channelChoices.value,
    onDropdownVisibleChange: (open: boolean) => { if (open) void loadChannelChoices(); },
    onPopupScroll: (event: UIEvent) => { const target = event.target as HTMLElement; if (target.scrollHeight - target.scrollTop - target.clientHeight < 40) void loadChannelChoices(); },
    onChange: (value?: string) => { channelNo.value = value || ""; },
  } }];
}
async function searchBatches() {
  const values = await batchApi.formApi.getValues();
  const value = String(values.channelNo || "");
  if (channelNo.value === value) { batchDirectory.reset(); await loadBatches(1); }
  else channelNo.value = value;
}
const rejectionReasons = [
  "evidence_insufficient",
  "ownership_mismatch",
  "amount_or_currency_mismatch",
  "provider_reference_conflict",
  "already_confirmed",
];

const money = (minor: string, currency: string) => {
  try {
    const value = BigInt(minor);
    const absolute = value < 0n ? -value : value;
    return `${currency} ${value < 0n ? "-" : ""}${absolute / 100n}.${(absolute % 100n).toString().padStart(2, "0")}`;
  } catch {
    return `${currency} —`;
  }
};
const date = (value?: number) =>
  value
    ? new Intl.DateTimeFormat(preferences.app.locale, {
        dateStyle: "medium",
      }).format(new Date(value))
    : "—";
const status = (value: string) =>
  tr(
    (
      {
        differences: "batch_differences",
        submitted: "submitted_state",
        succeeded: "succeeded_state",
        refunded: "refunded_state",
        pending: "pending_state",
      } as Record<string, string>
    )[value] || value,
  );

function healthColumns(): VxeGridProps<PaymentHealth>["columns"] {
  return [
    {
      field: "displayName",
      slots: { default: "channel" },
      title: tr("channel"),
      minWidth: 180,
    },
    {
      field: "capabilityCode",
      slots: { default: "capability" },
      title: tr("capability"),
      minWidth: 150,
    },
    {
      field: "healthCode",
      slots: { default: "health" },
      title: tr("health"),
      width: 120,
    },
    {
      field: "latestBusinessDate",
      slots: { default: "date" },
      title: tr("latestDate"),
      width: 150,
    },
    { field: "unresolvedCount", title: tr("unresolved"), width: 140 },
  ];
}
function batchColumns(): VxeGridProps<PaymentBatch>["columns"] {
  return [
    {
      field: "businessDate",
      slots: { default: "date" },
      title: tr("latestDate"),
      width: 150,
    },
    { field: "channelNo", title: tr("channel"), minWidth: 160 },
    {
      field: "counts",
      slots: { default: "counts" },
      title: tr("counts"),
      minWidth: 220,
    },
    {
      field: "statusCode",
      slots: { default: "status" },
      title: tr("status"),
      width: 140,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("action"),
      width: 110,
    }),
  ];
}
function differenceColumns(): VxeGridProps<PaymentDifference>["columns"] {
  return [
    {
      field: "differenceCode",
      slots: { default: "status" },
      title: tr("status"),
      minWidth: 190,
    },
    {
      field: "externalReference",
      slots: { default: "transaction" },
      title: tr("transaction"),
      minWidth: 220,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: tr("amount"),
      minWidth: 220,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("action"),
      width: 330,
    }),
  ];
}
function fundColumns(): VxeGridProps<ExceptionFund>["columns"] {
  return [
    {
      field: "statusCode",
      slots: { default: "status" },
      title: tr("status"),
      minWidth: 160,
    },
    {
      field: "orderNo",
      slots: { default: "order" },
      title: tr("originalOrder"),
      minWidth: 180,
    },
    {
      field: "amountMinor",
      slots: { default: "amount" },
      title: tr("amount"),
      width: 220,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("action"),
      width: 390,
    }),
  ];
}

const gridBase = {
  height: "100%",
  pagerConfig: { enabled: true, currentPage: 1, pageSize: 20, pageSizes: [10, 20, 50, 100], total: 0 },
  toolbarConfig: { custom: true, refresh: false, zoom: true },
};
const [HealthGrid, healthApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }) => void healthDirectory.load(currentPage, pageSize) },
  gridOptions: {
    ...gridBase,
    columns: healthColumns(),
    data: [],
    rowConfig: { keyField: "channelNo" },
    id: "payment-channel-health",
  },
});
const [BatchGrid, batchApi] = useVbenVxeGrid({
  formOptions: { schema: channelSearchSchema(), handleSubmit: searchBatches, handleReset: searchBatches, wrapperClass: "grid-cols-1 md:grid-cols-2" },
  gridEvents: { pageChange: ({ currentPage, pageSize }) => void loadBatches(currentPage, pageSize) },
  gridOptions: {
    ...gridBase,
    columns: batchColumns(),
    data: [],
    rowConfig: { keyField: "batchNo" },
    id: "payment-reconciliation-batches",
  },
});
const [DifferenceGrid, differenceApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }) => void loadDifferences(currentPage, pageSize) },
  gridOptions: {
    ...gridBase,
    columns: differenceColumns(),
    data: [],
    rowConfig: { keyField: "lineNo" },
    id: "payment-reconciliation-differences",
  },
});
const [FundGrid, fundApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }) => void fundDirectory.load(currentPage, pageSize) },
  gridOptions: {
    ...gridBase,
    columns: fundColumns(),
    data: [],
    rowConfig: { keyField: "exceptionNo" },
    id: "payment-exception-funds",
  },
});

for (const [directory, api] of [[healthDirectory, healthApi], [batchDirectory, batchApi], [differenceDirectory, differenceApi], [fundDirectory, fundApi]] as const) {
  watch(() => ({ ...directory.state }), state => api.setGridOptions({
    data: state.items, loading: state.loading,
    pagerConfig: { currentPage: state.currentPage, pageSize: state.pageSize, total: state.total },
  }));
}
watch(
  () => preferences.app.locale,
  () => {
    healthApi.setGridOptions({ columns: healthColumns() });
    batchApi.setGridOptions({ columns: batchColumns() });
    differenceApi.setGridOptions({ columns: differenceColumns() });
    fundApi.setGridOptions({ columns: fundColumns() });
    batchApi.formApi.setState({ schema: channelSearchSchema() });
  },
);
watch(channelChoices, () => batchApi.formApi.setState({ schema: channelSearchSchema() }));

async function loadReconciliation() {
  await Promise.all([healthDirectory.load(), loadBatches()]);
}
async function loadBatches(page?: number, size?: number) {
  if (!await batchDirectory.load(page, size)) return;
  if (!batches.value.some(item => item.batchNo === batchNo.value)) {
    batchNo.value = batches.value[0]?.batchNo || "";
  } else await loadDifferences();
}
async function loadDifferences(page?: number, size?: number) {
  if (!batchNo.value) {
    differenceDirectory.reset();
    return;
  }
  await differenceDirectory.load(page, size);
}
async function loadFunds() {
  await fundDirectory.load();
}
async function submit(item: PaymentDifference) {
  try {
    await submitPaymentSupplement(item);
    await loadDifferences();
  } catch {
    message.error(tr("actionFailed"));
  }
}
async function review(item: PaymentDifference, decision: "approve" | "reject") {
  try {
    await reviewPaymentSupplement(
      item,
      decision,
      decision === "reject" ? rejectionReason.value : "",
    );
    await loadReconciliation();
  } catch {
    message.error(tr("actionFailed"));
  }
}
async function allocate(item: ExceptionFund) {
  const target = targetOrders.value[item.exceptionNo]?.trim();
  if (!target) {
    message.warning(tr("targetRequired"));
    return;
  }
  try {
    await allocateExceptionFund(item, target);
    await loadFunds();
  } catch {
    message.error(tr("allocationFailed"));
  }
}
async function refund(item: ExceptionFund) {
  try {
    await refundExceptionFund(item);
    await loadFunds();
  } catch {
    message.error(tr("actionFailed"));
  }
}
function beforeUpload(item: ExceptionFund, file: File) {
  proofFiles.value[item.exceptionNo] = file;
  return false;
}
async function confirmRefund(item: ExceptionFund) {
  const file = proofFiles.value[item.exceptionNo];
  if (!file) {
    message.warning(tr("proofRequired"));
    return;
  }
  try {
    const form = new FormData();
    form.append("file", file);
    const proof = await uploadExceptionFundProof(item, form);
    await confirmExceptionFundRefund(
      item,
      proof.proofReference,
      proof.proofDigest,
    );
    delete proofFiles.value[item.exceptionNo];
    await loadFunds();
  } catch {
    message.error(tr("actionFailed"));
  }
}
watch(channelNo, () => {
  batchNo.value = "";
  batchDirectory.reset();
  differenceDirectory.reset();
  void loadBatches(1);
});
watch(batchNo, () => { differenceDirectory.reset(); void loadDifferences(1); });
onBeforeUnmount(() => { choiceSequence += 1; for (const item of [healthDirectory, batchDirectory, differenceDirectory, fundDirectory]) item.invalidate(); });
onMounted(
  () => void (props.mode === "exceptions" ? loadFunds() : loadReconciliation()),
);
const layoutRoot = useAdminGridLayout(healthApi, batchApi, differenceApi, fundApi);
function viewDifferences(no: string) { batchNo.value = no; activeSection.value = "differences"; }
</script>

<template>
  <div :ref="layoutRoot" class="payment-operations-layout">
  <Tabs v-if="mode === 'reconciliation'" v-model:active-key="activeSection" class="plugin-admin-tabs payment-operations-tabs" data-testid="payment-reconciliation-tabs">
  <TabPane key="health" :tab="tr('channelHealth')" force-render>
    <Card class="payment-directory" size="small" :bordered="false"
      ><HealthGrid data-testid="payment-health-table"
        ><template #toolbar-actions><Button :loading="loading" @click="loadReconciliation">{{ tr("refresh") }}</Button></template
        ><template #channel="{ row }"
          ><div>{{ row.displayName }}</div>
          <div class="text-xs text-gray-500">
            {{ row.providerCode }}
          </div></template
        ><template #capability="{ row }">{{ tr(row.capabilityCode) }}</template
        ><template #health="{ row }"
          ><Tag>{{ tr(row.healthCode) }}</Tag></template
        ><template #date="{ row }">{{
          date(row.latestBusinessDate)
        }}</template></HealthGrid
      ></Card
    >
  </TabPane>
  <TabPane key="batches" :tab="tr('batches')" force-render>
    <Card class="payment-directory" size="small" :bordered="false"
      ><BatchGrid data-testid="payment-batches-table"
        ><template #date="{ row }">{{ date(row.businessDate) }}</template
        ><template #counts="{ row }"
          >{{ row.paymentCount }} / {{ row.refundCount }} /
          {{ row.differenceCount }}</template
        ><template #status="{ row }"
          ><Tag>{{ status(row.statusCode) }}</Tag></template
        ><template #action="{ row }"
          ><Button size="small" type="link" @click="viewDifferences(row.batchNo)">{{
            tr("differences")
          }}</Button></template
        ></BatchGrid
      ></Card
    >
  </TabPane>
  <TabPane key="differences" :tab="tr('differences')" force-render>
    <Card class="payment-directory" size="small" :bordered="false"
      ><DifferenceGrid data-testid="payment-differences-table"
        ><template #status="{ row }"
          ><div>{{ status(row.differenceCode) }}</div>
          <div class="text-xs text-gray-500">
            {{ status(row.localStatusCode) }} →
            {{ status(row.channelStatusCode) }}
          </div></template
        ><template #transaction="{ row }"
          ><div>{{ row.externalReference || "—" }}</div>
          <div class="text-xs text-gray-500">
            {{ row.attemptNo || row.refundAttemptNo }}
          </div></template
        ><template #amount="{ row }"
          >{{ money(row.localAmountMinor, row.currencyCode) }} /
          {{ money(row.channelAmountMinor, row.currencyCode) }}</template
        ><template #action="{ row }"
          ><div class="flex items-center gap-2 whitespace-nowrap">
            <Button
              v-if="
                row.differenceCode === 'local_missing' &&
                row.channelStatusCode === 'succeeded' &&
                !row.supplementRequestNo
              "
              size="small"
              type="link"
              @click="submit(row)"
              >{{ tr("submit") }}</Button
            ><template v-else-if="row.supplementStatusCode === 'pending_review'"
              ><Button
                size="small"
                type="link"
                @click="review(row, 'approve')"
                >{{ tr("approve") }}</Button
              ><Select
                v-model:value="rejectionReason"
                class="w-36"
                size="small"
                :options="
                  rejectionReasons.map((value) => ({ value, label: tr(value) }))
                "
              /><Button
                danger
                size="small"
                type="link"
                @click="review(row, 'reject')"
                >{{ tr("reject") }}</Button
              ></template
            ><span v-else>{{
              row.supplementStatusCode
                ? status(row.supplementStatusCode)
                : status(row.resolutionCode)
            }}</span>
          </div></template
        ></DifferenceGrid
      ></Card
    >
  </TabPane>
  </Tabs>
  <Card v-else class="payment-directory" size="small" :bordered="false"
    ><template #title>{{ tr("exceptionTitle") }}</template
    ><template #extra
      ><Button :loading="loading" @click="loadFunds">{{
        tr("refresh")
      }}</Button></template
    ><Alert
      class="mb-4"
      :message="tr('exceptionHelp')"
      show-icon
      type="info"
    /><FundGrid data-testid="payment-funds-table"
      ><template #status="{ row }"
        ><div :data-testid="`payment-exception-fund-${row.exceptionNo}`">
          {{ status(row.actionStatus || row.statusCode) }}
        </div>
        <div class="text-xs text-gray-500">
          {{ row.providerCode }} · {{ row.channelNo }}
        </div></template
      ><template #order="{ row }"
        ><div>{{ row.orderNo }}</div>
        <div class="text-xs text-gray-500">{{ row.attemptNo }}</div></template
      ><template #amount="{ row }">{{
        money(row.amountMinor, row.currencyCode)
      }}</template
      ><template #action="{ row }"
        ><div :data-testid="`payment-exception-fund-action-${row.exceptionNo}`">
          <div
            v-if="!row.actionCode"
            class="flex min-w-[360px] items-center gap-2 whitespace-nowrap"
          >
            <Input
              v-model:value="targetOrders[row.exceptionNo]"
              class="w-48 shrink-0"
              style="flex: 0 0 12rem"
              :placeholder="tr('targetOrderPlaceholder')"
            /><Button size="small" type="link" @click="allocate(row)">{{
              tr("allocate")
            }}</Button
            ><Button danger size="small" type="link" @click="refund(row)">{{
              tr("originalRefund")
            }}</Button>
          </div>
          <div
            v-else-if="row.actionStatus === 'manual_pending'"
            class="flex items-center gap-2 whitespace-nowrap"
          >
            <Upload
              :before-upload="(file) => beforeUpload(row, file)"
              :max-count="1"
              :show-upload-list="true"
              ><Button size="small">{{ tr("proofFile") }}</Button></Upload
            ><Button size="small" type="link" @click="confirmRefund(row)">{{
              tr("confirmRefund")
            }}</Button>
          </div>
          <span v-else>{{ tr(row.actionStatus || row.actionCode) }}</span>
        </div></template
      ></FundGrid
    ></Card
  >
  </div>
</template>

<style scoped>
.payment-operations-layout { height: 100%; min-height: 0; min-width: 0; }
.payment-operations-tabs { height: 100%; }
.payment-operations-tabs :deep(.ant-tabs-content-holder),
.payment-operations-tabs :deep(.ant-tabs-content),
.payment-operations-tabs :deep(.ant-tabs-tabpane-active) { height: 100%; min-height: 0; }
.payment-directory { display: flex; flex-direction: column; height: 100%; min-height: 0; min-width: 0; }
.payment-directory :deep(.ant-card-head) { flex: 0 0 auto; }
.payment-directory :deep(.ant-card-body) { display: flex; flex-direction: column; flex: 1; min-height: 0; padding: 0; }
.payment-directory :deep(.ant-card-body > .h-full.bg-card) { flex: 1; min-height: 0; }
.payment-directory :deep(.ant-alert) { margin: 12px; }
</style>
