<script setup lang="ts">
import type {
  ContractFailurePolicy,
  ContractReplacementOptions,
  ContractSignerCandidate,
  ContractTemplateCandidateResult,
  ManagementContract,
  ManagementContractDetail,
  PreparedContract,
} from "../electronic-contract-management-client";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormSchema } from "#/adapter/form";

import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from "vue";
import { createNumberedPage } from "../numbered-page";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  Input,
  InputNumber,
  Modal as AntModal,
  Select,
  Space,
  TabPane,
  Tabs,
  Tag,
  Textarea,
  Timeline,
  TimelineItem,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";
import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

import {
  contractFailureCancelOrder,
  contractFailurePolicyGet,
  contractFailurePolicyUpdate,
  contractReissue,
  contractReplacementOptions,
  contractReplacementPreview,
  contractSignerCandidates,
  contractSubmit,
  contractTerminate,
  managementContractGet,
  managementContractList,
  manualContractCreate,
  manualContractOptions,
} from "../electronic-contract-management-client";

const props = defineProps<{
  canCancelOrder: boolean;
  canConfigureFailurePolicy: boolean;
  canCreate: boolean;
  canReissue: boolean;
  canTerminate: boolean;
}>();

const loading = ref(false);
const detailLoading = ref(false);
const saving = ref(false);
const policyLoading = ref(false);
const policySaving = ref(false);
const failurePolicy = ref<ContractFailurePolicy>();
const handlingDays = ref(3);
const cancelledContractNos = ref(new Set<string>());
const items = ref<ManagementContract[]>([]);
const pageState = ref({ currentPage: 1, pageSize: 10, total: 0 });
let loadSequence = 0;
let detailSequence = 0;
const pager = createNumberedPage<ManagementContract>((state) => {
  items.value = state.items;
  pageState.value = state;
  gridApi.setGridOptions({ pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
}, () => {
  detailSequence++;
  detail.value = undefined;
  prepared.value = undefined;
  createOpen.value = replacementOpen.value = terminationOpen.value = false;
});
onBeforeUnmount(() => { loadSequence++; detailSequence++; pager.invalidate(); });
const detail = ref<ManagementContractDetail>();
const prepared = ref<PreparedContract>();
const preparedSubmitMode = ref<"initial" | "reissue">("initial");
const createOpen = ref(false);
const replacementOpen = ref(false);
const terminationOpen = ref(false);
const options = ref<ContractTemplateCandidateResult>();
const replacementOptions = ref<ContractReplacementOptions>();
const signers = ref<ContractSignerCandidate[]>([]);
const filter = reactive({ customerNo: "", orderNo: "", statusCode: "" });
const createForm = reactive({ orderNo: "", signerNo: "", templateNo: "" });
const replacementForm = reactive({ signerNo: "", templateNo: "" });
const terminationForm = reactive({
  originalExpenseSettlement: "",
  originalOtherSettlement: "",
  otherDeals: "",
  reason: "",
  remainInForceItem: "",
});

function statusOptions() { return [
  "draft",
  "pending_launch",
  "signing",
  "completed",
  "rejected",
  "expired",
  "revoked",
  "failed",
  "reissuing",
  "terminated",
].map((value) => ({ label: statusText(value), value })); }

function searchSchema(): VbenFormSchema[] {
  return [
    { component: "Input", fieldName: "orderNo", label: text("fields.orderNo"), componentProps: { allowClear: true, placeholder: text("filters.order") } },
    { component: "Input", fieldName: "customerNo", label: text("fields.customerNo"), componentProps: { allowClear: true, placeholder: text("filters.customer") } },
    { component: "Select", fieldName: "statusCode", label: text("filters.status"), componentProps: { allowClear: true, options: statusOptions(), placeholder: text("filters.status") } },
  ];
}

const templateOptions = computed(() =>
  (options.value?.items ?? []).map((item) => ({
    label: `${item.templateName} · v${item.templateVersion} · ${item.languageCode}`,
    value: item.templateNo,
  })),
);
const signerOptions = computed(() =>
  signers.value.map((item) => ({
    label: `${item.maskedName} · ${item.maskedPhone}`,
    value: item.signerNo,
  })),
);
const canPrepare = computed(
  () =>
    Boolean(options.value?.items.length) &&
    Boolean(createForm.templateNo) &&
    (options.value?.customerType !== "enterprise" ||
      Boolean(createForm.signerNo)),
);
const replacementTemplateOptions = computed(() =>
  (replacementOptions.value?.templates ?? []).map((item) => ({
    label: `${item.templateName} · v${item.templateVersion} · ${item.languageCode}`,
    value: item.templateNo,
  })),
);
const replacementSignerOptions = computed(() =>
  (replacementOptions.value?.signers ?? []).map((item) => ({
    label: `${item.maskedName} · ${item.maskedPhone}`,
    value: item.signerNo,
  })),
);
const canPrepareReplacement = computed(
  () =>
    Boolean(replacementOptions.value?.templates.length) &&
    Boolean(replacementForm.templateNo) &&
    (replacementOptions.value?.customerType !== "enterprise" ||
      Boolean(replacementForm.signerNo)),
);

function key(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}

function text(path: string) {
  return $t(`plugin.idc-compliance.contract.management.${path}`);
}

function statusText(value: string) {
  return text(`statuses.${value}`);
}

function statusColor(value: string) {
  if (value === "completed") return "green";
  if (value === "signing" || value === "reissuing") return "blue";
  if (value === "pending_launch" || value === "draft") return "orange";
  if (value === "terminated") return "default";
  return "red";
}

function formatTime(value?: number) {
  return value
    ? new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(value)
    : "—";
}

function customerReference(value?: string) {
  if (!value || /^CUS_[0-9A-Za-z_-]+$/.test(value)) {
    return text("fields.customerProtected");
  }
  return value;
}

function isTerminalRequiredContract(item: ManagementContract) {
  return (
    item.requiredForFulfillment &&
    ["expired", "failed", "rejected", "revoked"].includes(item.statusCode)
  );
}

async function loadFailurePolicy() {
  policyLoading.value = true;
  try {
    const result = await contractFailurePolicyGet();
    failurePolicy.value = result.item;
    handlingDays.value = result.item.handlingDays;
  } finally {
    policyLoading.value = false;
  }
}

function saveFailurePolicy() {
  if (
    !failurePolicy.value ||
    policySaving.value ||
    handlingDays.value < 0 ||
    handlingDays.value > 30
  ) {
    return;
  }
  AntModal.confirm({
    content: text("failurePolicy.confirmDescription"),
    okText: text("failurePolicy.confirmAction"),
    title: text("failurePolicy.confirmTitle"),
    async onOk() {
      policySaving.value = true;
      try {
        const result = await contractFailurePolicyUpdate({
          expectedVersion: failurePolicy.value!.policyVersion,
          handlingDays: handlingDays.value,
          idempotencyKey: key("contract-failure-policy"),
        });
        failurePolicy.value = result.item;
        handlingDays.value = result.item.handlingDays;
        message.success(text("messages.failurePolicySaved"));
      } finally {
        policySaving.value = false;
      }
    },
  });
}

function cancelFailedContractOrder(item: ManagementContract) {
  if (!isTerminalRequiredContract(item) || saving.value) return;
  AntModal.confirm({
    content: text("failureCancellation.confirmDescription"),
    okButtonProps: { danger: true },
    okText: text("failureCancellation.confirmAction"),
    title: text("failureCancellation.confirmTitle"),
    async onOk() {
      saving.value = true;
      try {
        const result = await contractFailureCancelOrder(item.contractNo);
        if (result.item.statusCode === "manual_review") {
          message.warning(text("messages.failureCancellationManual"));
          return;
        }
        cancelledContractNos.value = new Set(cancelledContractNos.value).add(
          item.contractNo,
        );
        message.success(
          text(
            result.item.statusCode === "refund_pending"
              ? "messages.failureCancellationRefundPending"
              : "messages.failureCancellationCompleted",
          ),
        );
      } finally {
        saving.value = false;
      }
    },
  });
}

function formatMoney(value: string, currency: string) {
  const digits =
    new Intl.NumberFormat(undefined, {
      currency,
      style: "currency",
    }).resolvedOptions().maximumFractionDigits ?? 2;
  const scale = 10n ** BigInt(digits);
  const minor = BigInt(value);
  return new Intl.NumberFormat(undefined, {
    currency,
    maximumFractionDigits: digits,
    minimumFractionDigits: digits,
    style: "currency",
  }).format(Number(minor) / Number(scale));
}

function shortHash(value: string) {
  return value ? `${value.slice(0, 12)}…${value.slice(-8)}` : "—";
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
}

function catalogText(
  catalog: Record<string, string>,
  value: string,
  fallback: string,
) {
  const item = catalog[value];
  return item ? text(item) : text(fallback);
}

function fileTypeText(value: string) {
  return catalogText(
    {
      blockchain_metadata: "fileTypes.blockchain_metadata",
      event_timeline: "fileTypes.event_timeline",
      evidence_certificate: "fileTypes.evidence_certificate",
      final_pdf: "fileTypes.final_pdf",
      sign_report: "fileTypes.sign_report",
      termination_pdf: "fileTypes.termination_pdf",
    },
    value,
    "fileTypes.other",
  );
}

function fileStatusText(value: string) {
  return catalogText(
    {
      archived: "fileStatuses.archived",
      failed: "fileStatuses.failed",
      legal_hold: "fileStatuses.legal_hold",
      pending: "fileStatuses.pending",
    },
    value,
    "fileStatuses.unknown",
  );
}

function jobStatusText(value: string) {
  return catalogText(
    {
      archived: "jobStatuses.archived",
      pending: "jobStatuses.pending",
      processing: "jobStatuses.processing",
      queued: "jobStatuses.queued",
      result_unknown: "jobStatuses.result_unknown",
      skipped: "jobStatuses.skipped",
    },
    value,
    "jobStatuses.unknown",
  );
}

function archiveReasonText(value: string) {
  return catalogText(
    {
      report_not_enabled: "archiveReasons.report_not_enabled",
      supplier_capability_unavailable:
        "archiveReasons.supplier_capability_unavailable",
      supplier_subject_unavailable:
        "archiveReasons.supplier_subject_unavailable",
    },
    value,
    "archiveReasons.other",
  );
}

function periodUnitText(value: string) {
  return catalogText(
    {
      day: "termUnits.day",
      hour: "termUnits.hour",
      month: "termUnits.month",
      once: "termUnits.once",
      year: "termUnits.year",
    },
    value,
    "termUnits.other",
  );
}

function formatServiceTerm(record: PreparedContract["lines"][number]) {
  return `${record.purchaseCycles} × ${record.periodCount} ${periodUnitText(record.periodUnitCode)}`;
}

function flowPurposeText(value: string) {
  return catalogText(
    {
      initial: "flowPurposes.initial",
      reissue: "flowPurposes.reissue",
      termination: "flowPurposes.termination",
    },
    value,
    "flowPurposes.other",
  );
}

function eventSourceText(value: string) {
  return catalogText(
    {
      callback: "eventSources.callback",
      manual: "eventSources.manual",
      query: "eventSources.query",
      recovery: "eventSources.recovery",
    },
    value,
    "eventSources.other",
  );
}

function eventReasonText(value: string) {
  return catalogText(
    {
      customer_refused: "eventReasons.customer_refused",
      signing_completed: "eventReasons.signing_completed",
      signing_expired: "eventReasons.signing_expired",
    },
    value,
    "eventReasons.other",
  );
}

function mediaTypeText(value: string) {
  return value === "application/pdf"
    ? text("mediaTypes.pdf")
    : text("mediaTypes.file");
}

async function load(currentPage = pageState.value.currentPage, pageSize = pageState.value.pageSize) {
  const sequence = ++loadSequence;
  loading.value = true;
  try {
    const filters = {
      customerNo: filter.customerNo.trim() || undefined,
      orderNo: filter.orderNo.trim() || undefined,
      statusCode: filter.statusCode || undefined,
    };
    await pager.query({ currentPage, pageSize }, (page, pageSize) => managementContractList({ ...filters, page, pageSize }));
  } finally {
    if(sequence === loadSequence) loading.value = false;
  }
}

async function openDetail(contractNo: string) {
  const sequence = ++detailSequence;
  prepared.value = undefined;
  detailLoading.value = true;
  detail.value = undefined;
  try {
    const result = await managementContractGet(contractNo);
    if (sequence === detailSequence) detail.value = result;
  } finally {
    if (sequence === detailSequence) detailLoading.value = false;
  }
}

async function resetFilters() {
  await gridApi.formApi.resetForm();
  filter.customerNo = "";
  filter.orderNo = "";
  filter.statusCode = "";
  void load(1);
}

async function search(values: Record<string, unknown>) {
  filter.customerNo = String(values.customerNo || "").trim();
  filter.orderNo = String(values.orderNo || "").trim();
  filter.statusCode = String(values.statusCode || "");
  await load(1);
}

function openCreate() {
  createForm.orderNo = "";
  createForm.signerNo = "";
  createForm.templateNo = "";
  options.value = undefined;
  signers.value = [];
  createOpen.value = true;
}

async function loadOptions() {
  const orderNo = createForm.orderNo.trim();
  if (!orderNo) return;
  saving.value = true;
  try {
    options.value = await manualContractOptions(orderNo);
    createForm.templateNo = "";
    createForm.signerNo = "";
    signers.value = [];
    if (options.value.customerType === "enterprise") {
      signers.value = (
        await contractSignerCandidates(options.value.customerNo)
      ).items;
    }
  } finally {
    saving.value = false;
  }
}

function prepareContract() {
  if (!canPrepare.value || saving.value) return;
  AntModal.confirm({
    content: text("create.lockWarning"),
    okText: text("create.confirmPreview"),
    title: text("create.confirmTitle"),
    async onOk() {
      saving.value = true;
      try {
        const result = await manualContractCreate({
          idempotencyKey: key("manual-contract-preview"),
          orderNo: options.value!.orderNo,
          signerNo: createForm.signerNo,
          templateNo: createForm.templateNo,
        });
        prepared.value = result.item;
        preparedSubmitMode.value = "initial";
        createOpen.value = false;
        message.success(text("messages.previewCreated"));
        await load(1);
      } finally {
        saving.value = false;
      }
    },
  });
}

async function submitContract(
  contractNo: string,
  expectedVersion: number,
  mode: "initial" | "reissue" = "initial",
) {
  if (saving.value) return;
  saving.value = true;
  try {
    if (mode === "reissue") {
      await contractReissue(
        contractNo,
        expectedVersion,
        key("contract-replacement-submit"),
      );
    } else {
      await contractSubmit(
        contractNo,
        expectedVersion,
        key("contract-submit-signing"),
      );
    }
    prepared.value = undefined;
    message.success(
      text(
        mode === "reissue"
          ? "messages.replacementSubmitted"
          : "messages.submitted",
      ),
    );
    await load(1);
    await openDetail(contractNo);
  } finally {
    saving.value = false;
  }
}

function reissueUnchanged(item: ManagementContract) {
  AntModal.confirm({
    content: text("reissue.unchangedDescription"),
    okText: text("actions.reissueUnchanged"),
    title: text("reissue.unchangedTitle"),
    async onOk() {
      await contractReissue(
        item.contractNo,
        item.version,
        key("contract-reissue"),
      );
      message.success(text("messages.reissued"));
      await load(1);
      await openDetail(item.contractNo);
    },
  });
}

async function openChangedReissue(item: ManagementContract) {
  if (saving.value) return;
  replacementOptions.value = undefined;
  replacementForm.templateNo = "";
  replacementForm.signerNo = "";
  saving.value = true;
  try {
    replacementOptions.value = await contractReplacementOptions(
      item.contractNo,
    );
    replacementOpen.value = true;
  } finally {
    saving.value = false;
  }
}

async function prepareReplacement() {
  const options = replacementOptions.value;
  if (!options || !canPrepareReplacement.value || saving.value) return;
  saving.value = true;
  try {
    const result = await contractReplacementPreview(options.contractNo, {
      idempotencyKey: key("contract-replacement-preview"),
      signerNo: replacementForm.signerNo,
      templateNo: replacementForm.templateNo,
    });
    prepared.value = result.item;
    preparedSubmitMode.value = "reissue";
    detail.value = undefined;
    replacementOpen.value = false;
    message.success(text("messages.replacementPreviewCreated"));
    await load(1);
  } finally {
    saving.value = false;
  }
}

function openTermination() {
  Object.assign(terminationForm, {
    originalExpenseSettlement: "",
    originalOtherSettlement: "",
    otherDeals: "",
    reason: "",
    remainInForceItem: "",
  });
  terminationOpen.value = true;
}

function submitTermination() {
  if (!detail.value || !terminationForm.reason.trim() || saving.value) return;
  AntModal.confirm({
    content: text("termination.secondConfirm"),
    okButtonProps: { danger: true },
    okText: text("actions.confirmTerminate"),
    title: text("termination.confirmTitle"),
    async onOk() {
      saving.value = true;
      try {
        const item = detail.value!.item;
        await contractTerminate(item.contractNo, {
          confirmed: true,
          expectedVersion: item.version,
          idempotencyKey: key("contract-termination"),
          originalExpenseSettlement:
            terminationForm.originalExpenseSettlement.trim(),
          originalOtherSettlement:
            terminationForm.originalOtherSettlement.trim(),
          otherDeals: terminationForm.otherDeals.trim(),
          reason: terminationForm.reason.trim(),
          remainInForceItem: terminationForm.remainInForceItem.trim(),
        });
        terminationOpen.value = false;
        message.success(text("messages.terminationSubmitted"));
        await load(1);
        await openDetail(item.contractNo);
      } finally {
        saving.value = false;
      }
    },
  });
}

function columns(): VxeGridProps<ManagementContract>["columns"] {
  return [
    {
      field: "contractNo",
      slots: { default: "contractNo" },
      title: text("columns.contract"),
      minWidth: 300,
    },
    {
      field: "templateName",
      slots: { default: "templateName" },
      title: text("columns.template"),
      minWidth: 250,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: text("columns.amount"),
      width: 120,
    },
    {
      field: "statusCode",
      slots: { default: "statusCode" },
      title: text("columns.status"),
      width: 110,
    },
    {
      field: "updatedAt",
      slots: { default: "updatedAt" },
      title: text("columns.updatedAt"),
      width: 160,
    },
    {
      field: "actions",
      fixed: "right",
      slots: { default: "actions" },
      title: text("columns.actions"),
      width: 100,
    },
  ];
}
function previewColumns(): VxeGridProps<
  PreparedContract["lines"][number]
>["columns"] {
  return [
    { field: "serviceName", title: text("preview.service"), minWidth: 180 },
    { field: "quantity", title: text("preview.quantity"), width: 100 },
    {
      field: "term",
      slots: { default: "term" },
      title: text("preview.term"),
      minWidth: 150,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: text("preview.lineAmount"),
      minWidth: 140,
    },
  ];
}
function flowColumns(): VxeGridProps<any>["columns"] {
  return [
    { field: "flowNo", title: text("fields.flowNo"), minWidth: 180 },
    {
      field: "purposeCode",
      slots: { default: "purposeCode" },
      title: text("fields.purpose"),
      minWidth: 140,
    },
    {
      field: "statusCode",
      slots: { default: "flowStatus" },
      title: text("fields.status"),
      minWidth: 130,
    },
    {
      field: "createdAt",
      slots: { default: "flowCreatedAt" },
      title: text("fields.createdAt"),
      width: 180,
    },
  ];
}
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    handleSubmit: search,
    handleReset: resetFilters,
    showCollapseButton: false,
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-4",
  },
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => load(currentPage, pageSize) },
  gridOptions: {
    columns: columns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10,20,50,100] },
    rowConfig: { keyField: "contractNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-electronic-contracts",
  },
});
const [PreviewGrid, previewGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: previewColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 280,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "orderItemNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-electronic-contract-preview",
  },
});
const [FlowGrid, flowGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: flowColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 300,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "flowNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-electronic-contract-flows",
  },
});
watch(items, (rows) => gridApi.setGridOptions({ data: rows }), { deep: true });
watch(
  () => prepared.value?.lines || [],
  (rows) => previewGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => detail.value?.flows || [],
  (rows) => flowGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => preferences.app.locale,
  () => {
    gridApi.setGridOptions({ columns: columns() });
    gridApi.formApi.setState({ schema: searchSchema() });
    previewGridApi.setGridOptions({ columns: previewColumns() });
    flowGridApi.setGridOptions({ columns: flowColumns() });
  },
);

onMounted(() => {
  gridApi.formApi.setState({ schema: searchSchema() });
  void load(1);
  void loadFailurePolicy();
});
</script>

<template>
  <div class="manager-shell" data-testid="electronic-contract-manager">
    <Card
      class="manager-toolbar"
      :bordered="false"
      :title="text('title')"
      size="small"
    >
      <template #extra>
        <Button
          v-if="props.canCreate"
          data-testid="contract-create-open"
          type="primary"
          @click="openCreate"
        >
          {{ text("actions.create") }}
        </Button>
      </template>
    </Card>

    <Card
      class="failure-policy-card"
      :loading="policyLoading"
      :title="text('failurePolicy.title')"
      :bordered="false"
      data-testid="contract-failure-policy"
    >
      <div class="failure-policy-layout">
        <div>
          <p>{{ text("failurePolicy.description") }}</p>
          <small>{{ text("failurePolicy.frozenNote") }}</small>
        </div>
        <div v-if="failurePolicy" class="failure-policy-control">
          <InputNumber
            v-model:value="handlingDays"
            :disabled="!props.canConfigureFailurePolicy"
            :max="30"
            :min="0"
            :precision="0"
            data-testid="contract-failure-policy-days"
          />
          <span>{{ text("failurePolicy.days") }}</span>
          <Button
            v-if="props.canConfigureFailurePolicy"
            :disabled="handlingDays === failurePolicy.handlingDays"
            :loading="policySaving"
            data-testid="contract-failure-policy-save"
            type="primary"
            @click="saveFailurePolicy"
            >{{ text("failurePolicy.save") }}</Button
          >
        </div>
      </div>
    </Card>

    <Card class="directory-card" :bordered="false">
      <Grid :loading="loading" class="min-h-0 min-w-0" data-testid="contract-directory-table">
        <template #contractNo="{ row }"
          ><div class="contract-cell">
            <strong>{{ row.contractNo }}</strong
            ><span
              >{{ row.orderNo }} · {{ customerReference(row.customerNo) }}</span
            >
          </div></template
        >
        <template #templateName="{ row }"
          ><div class="contract-cell">
            <strong>{{ row.templateName }}</strong
            ><span>v{{ row.templateVersion }} · {{ row.languageCode }}</span>
          </div></template
        >
        <template #amount="{ row }">{{
          formatMoney(row.amountMinor, row.currencyCode)
        }}</template>
        <template #statusCode="{ row }"
          ><Tag :color="statusColor(row.statusCode)">{{
            statusText(row.statusCode)
          }}</Tag></template
        >
        <template #updatedAt="{ row }">{{
          formatTime(row.updatedAt)
        }}</template>
        <template #actions="{ row }"
          ><Button type="link" @click="openDetail(row.contractNo)">{{
            text("actions.detail")
          }}</Button></template
        >
      </Grid>
    </Card>

    <VbenModalCompat
      v-model:open="createOpen"
      :confirm-loading="saving"
      :ok-button-props="{ disabled: !canPrepare }"
      :ok-text="text('create.confirmPreview')"
      :title="text('create.title')"
      width="680px"
      @ok="prepareContract"
    >
      <div class="form-stack">
        <Alert
          :description="text('create.lockWarning')"
          :message="text('create.warningTitle')"
          show-icon
          type="warning"
        />
        <label>
          <span>{{ text("create.order") }}</span>
          <div class="compact-input">
            <Input
              v-model:value="createForm.orderNo"
              :disabled="Boolean(options)"
              data-testid="contract-create-order"
            />
            <Button
              :loading="saving"
              data-testid="contract-create-load-order"
              type="primary"
              @click="loadOptions"
            >
              {{ text("actions.loadOrder") }}
            </Button>
          </div>
        </label>
        <Alert
          v-if="options && options.items.length === 0"
          :message="text('create.noCandidates')"
          show-icon
          type="info"
        />
        <template v-if="options?.items.length">
          <div class="order-context">
            <span>{{ customerReference(options.customerNo) }}</span>
            <Tag>{{ text(`subjects.${options.customerType}`) }}</Tag>
          </div>
          <label>
            <span>{{ text("create.template") }}</span>
            <Select
              v-model:value="createForm.templateNo"
              :options="templateOptions"
              :placeholder="text('create.templatePlaceholder')"
              data-testid="contract-create-template"
            />
          </label>
          <label v-if="options.customerType === 'enterprise'">
            <span>{{ text("create.signer") }}</span>
            <Select
              v-model:value="createForm.signerNo"
              :options="signerOptions"
              :placeholder="text('create.signerPlaceholder')"
              data-testid="contract-create-signer"
            />
          </label>
        </template>
      </div>
    </VbenModalCompat>

    <VbenDrawerCompat
      :open="Boolean(prepared || detail || detailLoading)"
      :title="prepared ? text('preview.title') : text('detail.title')"
      width="min(900px, 94vw)"
      @close="
        prepared = undefined;
        detail = undefined;
      "
    >
      <div v-if="detailLoading" class="loading-panel">
        {{ text("loading") }}
      </div>
      <div
        v-else-if="prepared"
        class="detail-stack"
        data-testid="contract-preview"
      >
        <Alert
          :description="text('preview.description')"
          :message="text('preview.immutable')"
          show-icon
          type="info"
        />
        <Descriptions bordered :column="2" size="small">
          <DescriptionsItem :label="text('fields.contractNo')">{{
            prepared.contractNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.orderNo')">{{
            prepared.orderNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.customer')">{{
            prepared.customerName
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.signer')"
            >{{ prepared.signerName || "—" }}
            {{ prepared.signerPhoneMasked }}</DescriptionsItem
          >
          <DescriptionsItem :label="text('fields.template')"
            >{{ prepared.templateName }} · v{{
              prepared.templateVersion
            }}</DescriptionsItem
          >
          <DescriptionsItem :label="text('fields.amount')">{{
            formatMoney(prepared.totalAmountMinor, prepared.currencyCode)
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.expiresAt')">{{
            formatTime(prepared.signingExpiresAt)
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.fulfillment')">{{
            text(prepared.requiredForFulfillment ? "yes" : "no")
          }}</DescriptionsItem>
        </Descriptions>
        <PreviewGrid
          ><template #term="{ row }">{{ formatServiceTerm(row) }}</template
          ><template #amount="{ row }">{{
            formatMoney(row.amountMinor, prepared.currencyCode)
          }}</template></PreviewGrid
        >
        <Button
          block
          data-testid="contract-submit-signing"
          :loading="saving"
          size="large"
          type="primary"
          @click="
            submitContract(
              prepared.contractNo,
              prepared.version,
              preparedSubmitMode,
            )
          "
        >
          {{ text("actions.submitSigning") }}
        </Button>
      </div>
      <div
        v-else-if="detail"
        class="detail-stack"
        data-testid="contract-detail"
      >
        <Alert
          v-if="detail.historyTruncated"
          :message="text('detail.truncated')"
          show-icon
          type="warning"
        />
        <Descriptions bordered :column="2" size="small">
          <DescriptionsItem :label="text('fields.contractNo')">{{
            detail.item.contractNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.status')"
            ><Tag :color="statusColor(detail.item.statusCode)">{{
              statusText(detail.item.statusCode)
            }}</Tag></DescriptionsItem
          >
          <DescriptionsItem :label="text('fields.orderNo')">{{
            detail.item.orderNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.customerNo')">{{
            customerReference(detail.item.customerNo)
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.template')"
            >{{ detail.item.templateName }} · v{{
              detail.item.templateVersion
            }}</DescriptionsItem
          >
          <DescriptionsItem :label="text('fields.amount')">{{
            formatMoney(detail.item.amountMinor, detail.item.currencyCode)
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.expiresAt')">{{
            formatTime(detail.item.signingExpiresAt)
          }}</DescriptionsItem>
          <DescriptionsItem :label="text('fields.updatedAt')">{{
            formatTime(detail.item.updatedAt)
          }}</DescriptionsItem>
        </Descriptions>
        <Space wrap>
          <Button
            v-if="
              props.canCreate && detail.item.statusCode === 'pending_launch'
            "
            data-testid="contract-submit-signing"
            type="primary"
            @click="submitContract(detail.item.contractNo, detail.item.version)"
            >{{ text("actions.submitSigning") }}</Button
          >
          <Button
            v-if="
              props.canReissue &&
              ['rejected', 'expired', 'revoked', 'failed'].includes(
                detail.item.statusCode,
              )
            "
            data-testid="contract-reissue-unchanged"
            @click="reissueUnchanged(detail.item)"
            >{{ text("actions.reissueUnchanged") }}</Button
          >
          <Button
            v-if="
              props.canReissue &&
              ['rejected', 'expired', 'revoked', 'failed'].includes(
                detail.item.statusCode,
              )
            "
            data-testid="contract-reissue-changed"
            @click="openChangedReissue(detail.item)"
            >{{ text("actions.reissueChanged") }}</Button
          >
          <Button
            v-if="props.canTerminate && detail.item.statusCode === 'completed'"
            data-testid="contract-terminate-open"
            danger
            @click="openTermination"
            >{{ text("actions.terminate") }}</Button
          >
          <Button
            v-if="
              props.canCancelOrder && isTerminalRequiredContract(detail.item)
            "
            :disabled="cancelledContractNos.has(detail.item.contractNo)"
            data-testid="contract-failure-cancel-order"
            danger
            @click="cancelFailedContractOrder(detail.item)"
            >{{
              text(
                cancelledContractNos.has(detail.item.contractNo)
                  ? "actions.cancellationRequested"
                  : "actions.cancelFailedOrder",
              )
            }}</Button
          >
        </Space>
        <Tabs>
          <TabPane key="flows" :tab="text('detail.flows')">
            <FlowGrid
              ><template #purposeCode="{ row }">{{
                flowPurposeText(row.purposeCode)
              }}</template
              ><template #flowStatus="{ row }">{{
                statusText(row.statusCode)
              }}</template
              ><template #flowCreatedAt="{ row }">{{
                formatTime(row.createdAt)
              }}</template></FlowGrid
            >
          </TabPane>
          <TabPane key="timeline" :tab="text('detail.timeline')">
            <Empty
              v-if="detail.events.length === 0"
              :description="text('detail.noEvents')"
            />
            <Timeline v-else>
              <TimelineItem
                v-for="event in detail.events"
                :key="`${event.flowNo}-${event.createdAt}`"
              >
                <strong>{{ statusText(event.projectedStatus) }}</strong>
                <p>
                  {{ eventReasonText(event.reasonCode) }} ·
                  {{ eventSourceText(event.sourceKind) }} ·
                  {{ formatTime(event.supplierObservedAt) }}
                </p>
              </TimelineItem>
            </Timeline>
          </TabPane>
          <TabPane key="files" :tab="text('detail.files')">
            <div class="artifact-grid">
              <Card
                v-for="file in detail.files"
                :key="file.fileNo"
                size="small"
              >
                <div class="artifact-head">
                  <strong>{{ fileTypeText(file.fileTypeCode) }}</strong
                  ><Tag
                    :color="file.statusCode === 'archived' ? 'green' : 'orange'"
                    >{{ fileStatusText(file.statusCode) }}</Tag
                  >
                </div>
                <p>
                  {{ mediaTypeText(file.mediaType) }} ·
                  {{ formatBytes(file.byteSize) }}
                </p>
                <small>{{ text("archive.checksum") }}</small>
                <code>{{ shortHash(file.contentSha256) }}</code>
              </Card>
              <Card
                v-for="job in detail.archiveJobs"
                :key="job.jobNo"
                size="small"
              >
                <div class="artifact-head">
                  <strong>{{ fileTypeText(job.artifactTypeCode) }}</strong
                  ><Tag>{{ jobStatusText(job.statusCode) }}</Tag>
                </div>
                <p>
                  {{
                    job.lastReasonCode
                      ? archiveReasonText(job.lastReasonCode)
                      : text("archive.pending")
                  }}
                </p>
                <small>{{ formatTime(job.updatedAt) }}</small>
              </Card>
              <Empty
                v-if="
                  detail.files.length === 0 && detail.archiveJobs.length === 0
                "
                :description="text('detail.noFiles')"
              />
            </div>
          </TabPane>
          <TabPane key="replacements" :tab="text('detail.replacements')">
            <Empty
              v-if="detail.replacements.length === 0"
              :description="text('detail.noReplacements')"
            />
            <Descriptions
              v-for="item in detail.replacements"
              :key="item.replacementNo"
              bordered
              :column="1"
              size="small"
            >
              <DescriptionsItem :label="text('fields.replacement')"
                >{{ item.oldContractNo }} →
                {{ item.newContractNo }}</DescriptionsItem
              >
            </Descriptions>
          </TabPane>
        </Tabs>
      </div>
    </VbenDrawerCompat>

    <VbenModalCompat
      v-model:open="replacementOpen"
      :confirm-loading="saving"
      :ok-button-props="{ disabled: !canPrepareReplacement }"
      :ok-text="text('reissue.createReplacementPreview')"
      :title="text('reissue.changedTitle')"
      width="680px"
      @ok="prepareReplacement"
    >
      <div class="form-stack">
        <Alert
          :description="text('reissue.changedDescription')"
          :message="text('reissue.changedWarning')"
          show-icon
          type="warning"
        />
        <div v-if="replacementOptions" class="order-context">
          <span
            >{{ replacementOptions.orderNo }} ·
            {{ customerReference(replacementOptions.customerNo) }}</span
          >
          <Tag>{{ text(`subjects.${replacementOptions.customerType}`) }}</Tag>
        </div>
        <Alert
          v-if="replacementOptions && replacementOptions.templates.length === 0"
          :message="text('reissue.noTemplates')"
          show-icon
          type="info"
        />
        <label v-if="replacementOptions?.templates.length">
          <span>{{ text("create.template") }}</span>
          <Select
            v-model:value="replacementForm.templateNo"
            :options="replacementTemplateOptions"
            :placeholder="text('create.templatePlaceholder')"
            data-testid="contract-replacement-template"
          />
        </label>
        <label v-if="replacementOptions?.customerType === 'enterprise'">
          <span>{{ text("create.signer") }}</span>
          <Select
            v-model:value="replacementForm.signerNo"
            :options="replacementSignerOptions"
            :placeholder="text('create.signerPlaceholder')"
            data-testid="contract-replacement-signer"
          />
        </label>
      </div>
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="terminationOpen"
      :confirm-loading="saving"
      :ok-button-props="{
        danger: true,
        disabled: !terminationForm.reason.trim(),
      }"
      :ok-text="text('actions.reviewTerminate')"
      :title="text('termination.title')"
      width="680px"
      @ok="submitTermination"
    >
      <div class="form-stack">
        <Alert
          :message="text('termination.originalSigner')"
          show-icon
          type="warning"
        />
        <label
          ><span>{{ text("termination.reason") }} *</span
          ><Textarea
            v-model:value="terminationForm.reason"
            :maxlength="200"
            data-testid="contract-termination-reason"
            show-count
        /></label>
        <label
          ><span>{{ text("termination.remain") }}</span
          ><Textarea
            v-model:value="terminationForm.remainInForceItem"
            :maxlength="200"
        /></label>
        <label
          ><span>{{ text("termination.expense") }}</span
          ><Textarea
            v-model:value="terminationForm.originalExpenseSettlement"
            :maxlength="200"
        /></label>
        <label
          ><span>{{ text("termination.otherSettlement") }}</span
          ><Textarea
            v-model:value="terminationForm.originalOtherSettlement"
            :maxlength="200"
        /></label>
        <label
          ><span>{{ text("termination.otherDeals") }}</span
          ><Textarea
            v-model:value="terminationForm.otherDeals"
            :maxlength="200"
        /></label>
      </div>
    </VbenModalCompat>
  </div>
</template>

<style scoped>
.detail-stack,
.form-stack {
  display: grid;
  gap: 18px;
}
.manager-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.manager-toolbar {
  flex: none;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
}
.failure-policy-card,
.directory-card {
  border: 1px solid hsl(var(--border));
  min-width: 0;
}
.manager-toolbar :deep(.ant-card-body) { display: none; }
.failure-policy-card { flex: none; max-height: 160px; overflow: auto; }
.directory-card { flex: 1; min-height: 0; overflow: hidden; }
.directory-card :deep(.ant-card-body) { height: 100%; min-height: 0; min-width: 0; padding: 0; }
.failure-policy-card :deep(.ant-card-head),
.failure-policy-card :deep(.ant-card-body) {
  padding: 8px 12px;
}
.failure-policy-card :deep(.ant-card-head) { min-height: 40px; }
.failure-policy-layout {
  align-items: center;
  display: flex;
  gap: 24px;
  justify-content: space-between;
}
.failure-policy-layout p {
  margin: 0 0 6px;
}
.failure-policy-layout small {
  color: hsl(var(--muted-foreground));
}
.failure-policy-control {
  align-items: center;
  display: flex;
  flex: none;
  gap: 10px;
}
.failure-policy-control :deep(.ant-input-number) {
  width: 92px;
}
:deep(.contract-cell) {
  display: grid;
  gap: 3px;
}
:deep(.contract-cell span) {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.pager {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 16px;
}
.form-stack label {
  display: grid;
  gap: 7px;
}
.form-stack label > span {
  font-size: 12px;
  font-weight: 700;
}
.compact-input {
  display: flex;
}
.compact-input :deep(.ant-input) {
  border-bottom-right-radius: 0;
  border-top-right-radius: 0;
}
.compact-input :deep(.ant-btn) {
  border-bottom-left-radius: 0;
  border-top-left-radius: 0;
}
.order-context,
.artifact-head {
  align-items: center;
  display: flex;
  gap: 10px;
  justify-content: space-between;
}
.loading-panel {
  color: hsl(var(--muted-foreground));
  padding: 48px;
  text-align: center;
}
.artifact-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.artifact-grid p {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 10px 0;
}
.artifact-grid code {
  font-size: 11px;
}
@media (max-width: 900px) {
  .failure-policy-layout {
    align-items: flex-start;
    flex-direction: column;
  }
  .artifact-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 600px) {
  .failure-policy-control {
    align-items: stretch;
    flex-direction: column;
    width: 100%;
  }
  .failure-policy-control :deep(.ant-input-number) {
    width: 100%;
  }
}
</style>
