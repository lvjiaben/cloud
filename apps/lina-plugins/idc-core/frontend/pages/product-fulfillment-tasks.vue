<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/product-fulfillment/tasks",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  FulfillmentRecoveryEditor,
  FulfillmentRecoveryConfiguration,
  ProductFulfillmentEvent,
  ProductFulfillmentReasonOption,
  ProductFulfillmentTaskDetail,
  ProductFulfillmentTaskSummary,
} from "./product-fulfillment-client";

import { useRoute, useRouter } from "vue-router";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { createNumberedPage } from "./numbered-page";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Divider,
  Empty,
  message,
  Space,
  Spin,
  Tag,
  Timeline,
  TimelineItem,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import VbenModalCompat from "./components/vben-modal-compat.vue";
import { useAccess } from "./plugin-access";
import {
  inspectFulfillmentRecoveryConfiguration,
  approveAndStartProductFulfillment,
  concludeManualProductFulfillment,
  newFulfillmentActionKey,
  productFulfillmentTask,
  productFulfillmentTasks,
  rejectProductFulfillmentReview,
  takeoverProductFulfillment,
  recoverProductFulfillmentPreparation,
  repairProductFulfillmentManagement,
  refundFailedFulfillment,
  verifyFulfillmentNonAcceptance,
} from "./product-fulfillment-client";
import {
  createFulfillmentActionAttempt,
  fulfillmentEventTranslationKey,
  fulfillmentFailureTranslationKey,
  fulfillmentPhaseTranslationKey,
  fulfillmentEventReasonLabel,
  fulfillmentModeOptions,
  fulfillmentModeTranslationKey,
  fulfillmentQuantityText,
  fulfillmentStatusColor,
  fulfillmentStatusOptions,
  fulfillmentStatusTranslationKey,
  requiresFulfillmentActionNote,
} from "./product-fulfillment-state";
import {
  customerDisplay,
  searchCustomerOptions,
  type CustomerSelectOption,
} from "./customer-selection";

type ActionMode =
  | "verify_non_acceptance"
  | "refund_failed"
  | "manual_completed"
  | "manual_unable"
  | "review_approve"
  | "review_reject"
  | "recover_preparation"
  | "takeover";

const { hasAccessByCodes } = useAccess();
const router = useRouter();
const route = useRoute();
let routeFilterReady = false;
const canRefund = computed(() =>
  hasAccessByCodes(["idc:admin:billing:refund:review"]),
);
const canList = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:task:list"]),
);
const canQuery = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:task:query"]),
);
const canReviewApprove = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:review-approve"]),
);
const canReviewReject = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:review-reject"]),
);
const canTakeover = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:takeover"]),
);
const canManualConclude = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:manual-conclude"]),
);
const canRecover = computed(() =>
  hasAccessByCodes(["idc:admin:product-fulfillment:recover"]),
);

const loading = ref(false);
const detailLoading = ref(false);
const submitting = ref(false);
const loadError = ref(false);
const rows = ref<ProductFulfillmentTaskSummary[]>([]);
const detail = ref<ProductFulfillmentTaskDetail>();
const abnormalExecution = computed(
  () =>
    !!detail.value?.execution &&
    ["manual_processing", "not_completed"].includes(detail.value.statusCode),
);
const recoveryExplanation = computed(() => {
  if (!detail.value) return "";
  if (detail.value.canRecoverPreparation) return tr("diagnostic.ready");
  if (
    ["result_unknown", "manual_takeover"].includes(
      detail.value.execution?.statusCode || "",
    ) ||
    detail.value.execution?.phaseCode === "frozen_unknown"
  )
    return tr("diagnostic.blockedUnknown");
  return tr("diagnostic.blockedOther");
});
const detailOpen = ref(false);
const actionOpen = ref(false);
const recoveryEditor = ref<FulfillmentRecoveryEditor>();
const recoveryPasswordLength = ref(0);
const recoveryPasswordCharacters = ref(["lowercase", "uppercase", "digits"]);
const recoveryLoading = ref(false);
const recoveryError = ref(false);
let recoveryLoadSequence = 0;
const recoveryDesiredSelection = ref<FulfillmentRecoveryConfiguration>();
let recoveryTimer: ReturnType<typeof setTimeout> | undefined;
let recoveryReads = 0;
function stopRecoveryReads() {
  if (recoveryTimer) clearTimeout(recoveryTimer);
  recoveryTimer = undefined;
}
function scheduleRecoveryRead() {
  stopRecoveryReads();
  const editor = recoveryEditor.value;
  if (
    !editor ||
    !actionOpen.value ||
    actionMode.value !== "recover_preparation" ||
    recoveryReads >= 20 ||
    recoveryError.value
  )
    return;
  if (
    editor.regions.length &&
    editor.operatingSystems.length &&
    (!recoveryDesiredSelection.value?.region || editor.lines.length)
  )
    return;
  recoveryTimer = setTimeout(() => {
    if (document.hidden || submitting.value || !actionOpen.value) return;
    recoveryReads++;
    void loadRecoveryEditor(recoveryDesiredSelection.value, true);
  }, 2000);
}
const recoveryReady = computed(
  () =>
    !!recoveryEditor.value?.selection.interfaceInstanceNo &&
    !!recoveryEditor.value.selection.region &&
    !!recoveryEditor.value.selection.line &&
    !!recoveryEditor.value.selection.operatingSystem &&
    !recoveryLoading.value &&
    !recoveryError.value,
);

const confirmedNoCreation = ref(false);
const actionMode = ref<ActionMode>("review_approve");
const actionKey = ref("");
const currentPage = ref(1);
const pageSize = ref(10);
let loadSequence = 0;
let lastRequested = { currentPage: 1, pageSize: 10 };
const pagination = createNumberedPage<ProductFulfillmentTaskSummary>(
  (state) => {
    rows.value = state.items;
    currentPage.value = state.currentPage;
    pageSize.value = state.pageSize;
    gridApi.setGridOptions({
      data: state.items,
      pagerConfig: {
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    });
  },
  () => {
    detailOpen.value = false;
    actionOpen.value = false;
    detail.value = undefined;
  },
);
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try {
    customerOptions.value = await searchCustomerOptions(keyword);
  } finally {
    customerLoading.value = false;
    gridApi.formApi.setState({ schema: searchSchema() });
  }
}

function tr(suffix: string) {
  return $t(`plugin.idc-core.fulfillment.${suffix}`);
}

function localizedName(input: {
  productNameEnUS: string;
  productNameZhCN: string;
}) {
  return preferences.app.locale === "zh-CN"
    ? input.productNameZhCN || input.productNameEnUS
    : input.productNameEnUS || input.productNameZhCN;
}

function localizedReason(input: ProductFulfillmentReasonOption) {
  return preferences.app.locale === "zh-CN"
    ? input.labelZhCN || input.labelEnUS
    : input.labelEnUS || input.labelZhCN;
}

function quantityText(input: ProductFulfillmentTaskSummary) {
  return fulfillmentQuantityText(
    input.granularityCode,
    input.unitSequence,
    input.quantity,
    $t,
  );
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      componentProps: {
        allowClear: true,
        maxlength: 40,
        placeholder: tr("search.orderPlaceholder"),
      },
      fieldName: "orderNo",
      label: tr("search.order"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        dropdownMatchSelectWidth: false,
        popupClassName: "plugin-customer-options",
        filterOption: false,
        loading: customerLoading.value,
        onSearch: searchCustomers,
        options: customerOptions.value,
        placeholder: tr("search.customerPlaceholder"),
        showSearch: true,
      },
      fieldName: "customerNo",
      label: tr("search.customer"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: fulfillmentModeOptions($t),
      },
      fieldName: "fulfillmentModeCode",
      label: tr("search.mode"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: fulfillmentStatusOptions($t),
      },
      fieldName: "statusCode",
      label: tr("search.status"),
    },
  ];
}

function columns(): VxeGridProps<ProductFulfillmentTaskSummary>["columns"] {
  return [
    { field: "orderNo", minWidth: 210, title: tr("table.order") },
    {
      field: "customerDisplayName",
      minWidth: 240,
      slots: { default: "customer" },
      title: tr("table.customer"),
    },
    {
      field: "productName",
      minWidth: 220,
      slots: { default: "product" },
      title: tr("table.product"),
    },
    {
      field: "fulfillmentModeCode",
      minWidth: 160,
      slots: { default: "mode" },
      title: tr("table.mode"),
    },
    {
      field: "quantity",
      minWidth: 150,
      slots: { default: "quantity" },
      title: tr("table.quantity"),
    },
    {
      field: "statusCode",
      minWidth: 140,
      slots: { default: "status" },
      title: tr("table.status"),
    },
    {
      field: "updatedAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: tr("table.updatedAt"),
      width: 180,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("table.action"),
      width: 120,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize,
    }: {
      currentPage: number;
      pageSize: number;
    }) => {
      void load(currentPage, pageSize);
    },
  },
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    schema: [],
    wrapperClass: "plugin-admin-search",
    layout: "inline",
    actionLayout: "inline",
    actionPosition: "left",
    showCollapseButton: false,
  },
  gridOptions: {
    columns: [],
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "intentNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-product-fulfillment-tasks",
  },
});

function actionSchema() {
  const reasons = actionReasons.value.map((item) => ({
    label: localizedReason(item),
    value: item.code,
  }));
  const schema = [
    {
      component: "Select" as const,
      componentProps: {
        options: reasons,
        placeholder: tr("action.reasonPlaceholder"),
      },
      fieldName: "reasonCode",
      label: tr("action.reason"),
      rules: z.string().trim().min(1, tr("action.reasonPlaceholder")),
    },
    {
      component: "Textarea" as const,
      componentProps: { maxlength: 512, rows: 5, showCount: true },
      fieldName: "note",
      label: tr("action.note"),
      rules:
        actionMode.value === "refund_failed" ||
        requiresFulfillmentActionNote(actionMode.value)
          ? z
              .string()
              .trim()
              .min(10, tr("action.noteLength"))
              .max(512, tr("action.noteLength"))
          : z.string().trim().max(512, tr("action.noteMaxLength")).optional(),
    },
  ];
  if (actionMode.value === "recover_preparation")
    return [...recoveryFields(), ...schema];
  return ["refund_failed", "verify_non_acceptance"].includes(actionMode.value)
    ? schema.filter((item) => item.fieldName !== "reasonCode")
    : schema;
}

const [ActionForm, actionFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

const actionReasons = computed(() => {
  if (!detail.value) return [];
  if (actionMode.value === "recover_preparation")
    return detail.value.recoveryReasons ?? [];
  if (actionMode.value === "takeover") return detail.value.takeoverReasons;
  if (actionMode.value === "review_approve") {
    return detail.value.reviewApprovalReasons;
  }
  if (actionMode.value === "review_reject") {
    return detail.value.reviewRejectionReasons;
  }
  return actionMode.value === "manual_completed"
    ? detail.value.manualCompletedReasons
    : detail.value.manualUnableReasons;
});

const actionTitle = computed(() => tr(`action.title.${actionMode.value}`));

async function load(
  targetPage = currentPage.value,
  targetSize = pageSize.value,
) {
  if (!canList.value) return;
  const sequence = ++loadSequence;
  lastRequested = { currentPage: targetPage, pageSize: targetSize };
  loading.value = true;
  loadError.value = false;
  await pagination.query(lastRequested, async (pageNumber, limit) => {
    const values = await gridApi.formApi.getValues();
    try {
      const page = await productFulfillmentTasks({
        page: pageNumber,
        customerNo: String(values.customerNo || "").trim() || undefined,
        fulfillmentModeCode:
          String(values.fulfillmentModeCode || "") || undefined,
        limit,
        orderNo: String(values.orderNo || "").trim() || undefined,
        statusCode: String(values.statusCode || "") || undefined,
      });
      return { items: page.tasks, total: page.total };
    } catch (error) {
      if (sequence === loadSequence) loadError.value = true;
      throw error;
    }
  });
  if (sequence === loadSequence) loading.value = false;
}

function search() {
  void load(1);
}

function handleSearchSubmit() {
  search();
}

function handleSearchReset() {
  if (route.query.orderNo != null) {
    void router.replace({ query: { ...route.query, orderNo: undefined } });
    return;
  }
  search();
}

// A route change chooses a new read scope. Do not preserve the old order's
// confirmed rows when the new scope fails; ordinary paging still preserves them.
async function applyOrderRouteFilter() {
  if (!canList.value) return;
  const requested = route.query.orderNo;
  pagination.reset();
  detailOpen.value = false;
  actionOpen.value = false;
  detail.value = undefined;
  await gridApi.formApi.setValues({
    orderNo: requested == null ? "" : String(requested),
    customerNo: undefined,
    fulfillmentModeCode: undefined,
    statusCode: undefined,
  });
  await load(1);
}

watch(
  () => [route.path, route.query.orderNo],
  async () => {
    if (routeFilterReady && route.path === "/idc/product-fulfillment/tasks")
      await applyOrderRouteFilter();
  },
);

onBeforeUnmount(() => {
  stopRecoveryReads();
  loadSequence += 1;
  recoveryLoadSequence++;
  pagination.invalidate();
});
watch(actionOpen, (open) => {
  if (!open) {
    stopRecoveryReads();
    recoveryLoadSequence++;
  }
});

function eventReasonLabel(event: ProductFulfillmentEvent) {
  return fulfillmentEventReasonLabel(
    event,
    preferences.app.locale !== "zh-CN",
    tr("common.unavailable"),
  );
}

function formatHTTPResponse(text: string) {
  try {
    return JSON.stringify(JSON.parse(text), null, 2) ?? text;
  } catch {
    return text;
  }
}

const callDiagnostic = computed(() => {
  const stages = new Set([
    "request_validation",
    "authentication",
    "request_preparation",
    "http_transport",
    "response_processing",
    "provider_dispatch",
    "provider_result",
    "owner_setup",
  ]);
  try {
    const value: unknown = JSON.parse(
      detail.value?.execution?.responseExcerpt || "null",
    );
    if (
      !value ||
      typeof value !== "object" ||
      !("stage" in value) ||
      typeof value.stage !== "string" ||
      !stages.has(value.stage)
    )
      return undefined;
    return {
      stage: value.stage,
      started:
        "businessRequestStarted" in value &&
        typeof value.businessRequestStarted === "boolean"
          ? value.businessRequestStarted
          : undefined,
    };
  } catch {
    return undefined;
  }
});

async function openDetail(row: ProductFulfillmentTaskSummary) {
  if (!canQuery.value) return;
  detailOpen.value = true;
  detailLoading.value = true;
  detail.value = undefined;
  try {
    detail.value = (await productFulfillmentTask(row.intentNo)).task;
  } catch {
    message.error(tr("message.detailFailed"));
  } finally {
    detailLoading.value = false;
  }
}

type RecoveryField =
  | "interfaceInstanceNo"
  | "region"
  | "line"
  | "operatingSystem";

function recoveryFields(): NonNullable<VbenFormProps["schema"]> {
  const editor = recoveryEditor.value;
  const routing: NonNullable<VbenFormProps["schema"]> = [
    {
      key: "interfaceInstanceNo",
      fieldName: "recoveryInterfaceNo",
      options: editor?.interfaces,
      label: "interface",
    },
    {
      key: "region",
      fieldName: "recoveryRegion",
      options: editor?.regions,
      label: "region",
    },
    {
      key: "line",
      fieldName: "recoveryLine",
      options: editor?.lines,
      label: "line",
    },
    {
      key: "operatingSystem",
      fieldName: "recoveryOperatingSystem",
      options: editor?.operatingSystems,
      label: "image",
    },
  ].map((field) => ({
    component: "Select",
    fieldName: field.fieldName,
    label: tr(`correction.${field.label}`),
    rules: z.string().min(1, tr("correction.choose")),
    componentProps: {
      disabled:
        (recoveryLoading.value && field.key !== "interfaceInstanceNo") ||
        !editor ||
        submitting.value,
      loading: recoveryLoading.value && field.key !== "interfaceInstanceNo",
      allowClear: false,
      showSearch: true,
      optionFilterProp: "label",
      dropdownStyle: { zIndex: 2300 },
      options: (field.options || []).map((option) => ({
        value: option.value,
        label:
          preferences.app.locale === "zh-CN"
            ? option.labelZhCN
            : option.labelEnUS,
      })),
      onChange: (value: string) =>
        changeRecoverySelection(field.key as RecoveryField, value),
    },
  }));
  return [...routing, {
    component: "Select", fieldName: "recoveryPasswordLength",
    label: tr("correction.passwordLength"),
    componentProps: {
      disabled: submitting.value,
      "data-testid": "recovery-password-length",
      virtual: false,
      dropdownStyle: { zIndex: 2300 },
      options: [{ value: 0, label: tr("correction.keepPasswordRules") },
        ...Array.from({ length: 15 }, (_, index) => ({ value: index + 6, label: String(index + 6) }))],
      onChange: (value: number) => { recoveryPasswordLength.value = value; },
    },
  }, {
    component: "CheckboxGroup", fieldName: "recoveryPasswordCharacters",
    label: tr("correction.passwordCharacters"),
    componentProps: {
      disabled: submitting.value,
      options: ["lowercase", "uppercase", "digits", "symbols"].map(value => ({ value, label: tr(`correction.passwordClass.${value}`) })),
      onChange: (value: string[]) => { recoveryPasswordCharacters.value = value; },
    },
    help: tr("correction.passwordHelp"),
  }];
}

async function loadRecoveryEditor(
  selection?: FulfillmentRecoveryConfiguration,
  quiet = false,
) {
  if (!detail.value) return;
  stopRecoveryReads();
  if (!quiet) recoveryReads = 0;
  if (selection) recoveryDesiredSelection.value = { ...selection };
  const sequence = ++recoveryLoadSequence;
  if (!quiet) recoveryLoading.value = true;
  recoveryError.value = false;
  if (!quiet) await actionFormApi.setState({ schema: actionSchema() });
  try {
    const editor = await inspectFulfillmentRecoveryConfiguration(
      detail.value.intentNo,
      selection,
    );
    if (sequence !== recoveryLoadSequence) return;
    if (!editor?.selection) throw new Error("recovery editor unavailable");
    const latestValues = await actionFormApi.getValues();
    if (!selection && !quiet) {
      recoveryPasswordLength.value = editor.selection.generationLength || 0;
      recoveryPasswordCharacters.value = editor.selection.generationCharacters?.split(",") || ["lowercase", "uppercase", "digits"];
    }
    if (!recoveryDesiredSelection.value)
      recoveryDesiredSelection.value = { ...editor.original };
    if (recoveryEditor.value)
      editor.original = {
        ...editor.original,
        regionLabel:
          recoveryEditor.value.original.regionLabel ||
          editor.original.regionLabel,
        lineLabel:
          recoveryEditor.value.original.lineLabel || editor.original.lineLabel,
        operatingSystemLabel:
          recoveryEditor.value.original.operatingSystemLabel ||
          editor.original.operatingSystemLabel,
      };
    recoveryEditor.value = editor;
    await actionFormApi.setValues({
      ...latestValues,
      recoveryInterfaceNo: editor.selection.interfaceInstanceNo,
      recoveryRegion: editor.selection.region,
      recoveryLine: editor.selection.line,
      recoveryOperatingSystem: editor.selection.operatingSystem,
      recoveryPasswordLength: recoveryPasswordLength.value,
      recoveryPasswordCharacters: recoveryPasswordCharacters.value,
    });
  } catch {
    if (sequence === recoveryLoadSequence) recoveryError.value = true;
  } finally {
    if (sequence === recoveryLoadSequence) {
      recoveryLoading.value = false;
      await actionFormApi.setState({ schema: actionSchema() });
      scheduleRecoveryRead();
    }
  }
}

async function changeRecoverySelection(field: RecoveryField, value: string) {
  if (
    !recoveryEditor.value ||
    (recoveryLoading.value && field !== "interfaceInstanceNo")
  )
    return;
  const selection = { ...recoveryEditor.value.selection, [field]: value };
  if (field === "interfaceInstanceNo") {
    selection.region = "";
    selection.line = "";
    selection.operatingSystem = "";
  }
  if (field === "region") selection.line = "";
  await loadRecoveryEditor(selection);
}

const managementRepairLoading = ref(false);
const managementRepairError = ref(false);
let managementRepairAttempt:
  | { intentNo: string; expectedVersion: number; idempotencyKey: string }
  | undefined;
async function repairManagement() {
  const task = detail.value;
  if (
    !task?.canRepairManagement ||
    !canRecover.value ||
    managementRepairLoading.value
  )
    return;
  if (
    !managementRepairAttempt ||
    managementRepairAttempt.intentNo !== task.intentNo ||
    managementRepairAttempt.expectedVersion !== task.version
  )
    managementRepairAttempt = {
      intentNo: task.intentNo,
      expectedVersion: task.version,
      idempotencyKey: newFulfillmentActionKey(),
    };
  const attempt = managementRepairAttempt;
  managementRepairLoading.value = true;
  managementRepairError.value = false;
  try {
    const response = await repairProductFulfillmentManagement(
      attempt.intentNo,
      {
        expectedVersion: attempt.expectedVersion,
        idempotencyKey: attempt.idempotencyKey,
      },
    );
    if (detail.value?.intentNo === attempt.intentNo)
      detail.value = response.task;
    managementRepairAttempt = undefined;
    message.success(tr("management.repairQueued"));
  } catch {
    if (detail.value?.intentNo === attempt.intentNo)
      managementRepairError.value = true;
  } finally {
    managementRepairLoading.value = false;
  }
}
watch(
  () => detail.value?.intentNo,
  () => {
    managementRepairError.value = false;
  },
);

async function openAction(mode: ActionMode) {
  if (
    mode === "recover_preparation" &&
    (!canRecover.value || !detail.value?.canRecoverPreparation)
  )
    return;
  if (
    mode === "refund_failed" &&
    (!canRefund.value || !detail.value?.canRequestRefund)
  )
    return;
  recoveryLoadSequence++;
  recoveryEditor.value = undefined;
  recoveryError.value = false;
  recoveryLoading.value = false;
  stopRecoveryReads();
  recoveryReads = 0;
  recoveryDesiredSelection.value = undefined;
  if (
    mode === "verify_non_acceptance" &&
    (!canRecover.value || !detail.value?.canVerifyNonAcceptance)
  )
    return;
  confirmedNoCreation.value = false;
  actionMode.value = mode;
  actionKey.value = createFulfillmentActionAttempt(
    newFulfillmentActionKey,
  ).idempotencyKey;
  actionOpen.value = true;
  await nextTick();
  await actionFormApi.setState({ schema: actionSchema() });
  await actionFormApi.resetForm();
  if (mode === "recover_preparation") {
    recoveryPasswordLength.value = 0;
    recoveryPasswordCharacters.value = ["lowercase", "uppercase", "digits"];
    await loadRecoveryEditor();
  }
}

async function submitAction() {
  if (!detail.value || submitting.value) return;
  if (
    actionMode.value === "verify_non_acceptance" &&
    !confirmedNoCreation.value
  )
    return;
  if (actionMode.value === "recover_preparation" && !recoveryReady.value)
    return;
  const { valid } = await actionFormApi.validate();
  if (!valid) return;
  const values = await actionFormApi.getValues<{
    note: string;
    reasonCode: string;
  }>();
  const common = {
    expectedVersion: detail.value.version,
    idempotencyKey: actionKey.value,
    note: String(values.note ?? "").trim(),
    reasonCode: values.reasonCode,
  };
  submitting.value = true;
  try {
    if (actionMode.value === "refund_failed") {
      const result = await refundFailedFulfillment(
        detail.value.intentNo,
        common,
      );
      actionOpen.value = false;
      actionKey.value = "";
      message.success(tr("message.refundRequested"));
      await router.push({
        path: "/idc/finance",
        query: { tab: "refunds", refundNo: result.refund.refundNo },
      });
      return;
    }
    let response: { task: ProductFulfillmentTaskDetail };
    if (actionMode.value === "verify_non_acceptance") {
      response = await verifyFulfillmentNonAcceptance(detail.value.intentNo, {
        ...common,
        confirmedNoCreation: confirmedNoCreation.value,
      });
    } else if (actionMode.value === "recover_preparation") {
      response = await recoverProductFulfillmentPreparation(
        detail.value.intentNo,
        { ...common, recoveryConfiguration: {
          ...recoveryEditor.value!.selection,
          generationLength: recoveryPasswordLength.value,
          generationCharacters: recoveryPasswordLength.value ? recoveryPasswordCharacters.value.join(",") : "",
        } },
      );
    } else if (actionMode.value === "takeover") {
      response = await takeoverProductFulfillment(
        detail.value.intentNo,
        common,
      );
    } else if (actionMode.value === "review_approve") {
      response = await approveAndStartProductFulfillment(
        detail.value.intentNo,
        common,
      );
    } else if (actionMode.value === "review_reject") {
      response = await rejectProductFulfillmentReview(
        detail.value.intentNo,
        common,
      );
    } else {
      response = await concludeManualProductFulfillment(detail.value.intentNo, {
        ...common,
        outcomeCode:
          actionMode.value === "manual_completed"
            ? "completed"
            : "unable_to_complete",
      });
    }
    detail.value = response.task;
    actionOpen.value = false;
    actionKey.value = "";
    message.success(tr("message.actionSucceeded"));
    await load();
  } catch {
    message.error(tr("message.actionFailed"));
  } finally {
    submitting.value = false;
  }
}

watch(
  () => preferences.app.locale,
  async () => {
    const searchValues = await gridApi.formApi.getValues();
    gridApi.formApi.setState({ schema: searchSchema() });
    await gridApi.formApi.setValues(searchValues);
    gridApi.setGridOptions({ columns: columns() });
    if (actionOpen.value) {
      const actionValues = await actionFormApi.getValues();
      await actionFormApi.setState({ schema: actionSchema() });
      await actionFormApi.setValues(actionValues);
    }
  },
);

onMounted(async () => {
  if (!canList.value) return;
  gridApi.formApi.setState({ schema: searchSchema() });
  gridApi.setGridOptions({ columns: columns() });
  await nextTick();
  routeFilterReady = true;
  await applyOrderRouteFilter();
  await searchCustomers();
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page
    :ref="layoutRoot"
    class="plugin-admin-page"
    :auto-content-height="true"
    data-testid="product-fulfillment-page"
  >
    <Card
      v-if="canList"
      :body-style="{ height: '100%' }"
      :bordered="false"
      class="h-full"
    >
      <Alert
        v-if="loadError"
        class="mb-4"
        show-icon
        type="error"
        :message="tr('message.loadFailed')"
      >
        <template #action>
          <Button
            size="small"
            @click="load(lastRequested.currentPage, lastRequested.pageSize)"
            >{{ tr("common.retry") }}</Button
          >
        </template>
      </Alert>
      <Grid class="h-full">
        <template #product="{ row }">
          {{ localizedName(row) }}
        </template>
        <template #customer="{ row }">{{ customerDisplay(row) }}</template>
        <template #mode="{ row }">
          {{ $t(fulfillmentModeTranslationKey(row.fulfillmentModeCode)) }}
        </template>
        <template #quantity="{ row }">
          {{ quantityText(row) }}
        </template>
        <template #status="{ row }">
          <Tag :color="fulfillmentStatusColor(row.statusCode)">
            {{ $t(fulfillmentStatusTranslationKey(row.statusCode)) }}
          </Tag>
        </template>
        <template #action="{ row }">
          <ghost-button v-if="canQuery" @click="openDetail(row)">{{
            tr("common.detail")
          }}</ghost-button>
        </template>
      </Grid>
    </Card>
    <Alert
      v-else
      show-icon
      type="warning"
      :message="tr('message.noPermission')"
    />

    <VbenDrawerCompat v-model:open="detailOpen" :title="tr('detail.title')">
      <Spin :spinning="detailLoading">
        <div v-if="detail" class="fulfillment-details">
          <Alert
            class="mb-4"
            show-icon
            type="info"
            :message="tr('detail.safety')"
          />
          <Descriptions bordered :column="{ xs: 1, sm: 2 }" size="small">
            <DescriptionsItem :label="tr('table.order')">
              {{ detail.orderNo }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.customer')">
              {{ customerDisplay(detail) }}
            </DescriptionsItem>
            <DescriptionsItem
              v-if="detail?.targetEntitlementNo"
              :label="$t('plugin.idc-core.addons.service')"
              >{{ detail?.targetEntitlementNo }}</DescriptionsItem
            >
            <DescriptionsItem :label="tr('table.product')">
              {{ localizedName(detail) }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.mode')">
              {{
                $t(fulfillmentModeTranslationKey(detail.fulfillmentModeCode))
              }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.quantity')">
              {{ quantityText(detail) }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.status')">
              <Tag :color="fulfillmentStatusColor(detail.statusCode)">
                {{ $t(fulfillmentStatusTranslationKey(detail.statusCode)) }}
              </Tag>
            </DescriptionsItem>
            <DescriptionsItem :label="tr('detail.settlement')">
              {{
                detail.financiallySettled
                  ? tr("detail.settled")
                  : tr("detail.notSettled")
              }}
            </DescriptionsItem>
          </Descriptions>

          <Alert
            v-if="detail.statusCode === 'manual_processing'"
            class="mt-4"
            show-icon
            type="warning"
            :message="tr('detail.manualAttention')"
          />
          <Space class="mt-4" wrap>
            <Button
              v-if="canRecover && detail.canVerifyNonAcceptance"
              @click="openAction('verify_non_acceptance')"
              >{{ tr("verification.action") }}</Button
            >
            <Button
              v-if="canRefund && (detail.canRequestRefund || abnormalExecution)"
              :disabled="!detail.canRequestRefund"
              @click="openAction('refund_failed')"
              >{{ tr("action.refundFailed") }}</Button
            >
            <Button
              v-if="
                canRecover &&
                (detail.canRecoverPreparation || abnormalExecution)
              "
              :disabled="!detail.canRecoverPreparation"
              type="primary"
              @click="openAction('recover_preparation')"
              >{{ tr("action.recoverPreparation") }}</Button
            >
            <Button
              v-if="canReviewApprove && detail.canReviewApprove"
              type="primary"
              @click="openAction('review_approve')"
              >{{ tr("action.reviewApprove") }}</Button
            >
            <Button
              v-if="canReviewReject && detail.canReviewReject"
              danger
              @click="openAction('review_reject')"
              >{{ tr("action.reviewReject") }}</Button
            >
            <Button
              v-if="canTakeover && detail.canTakeover"
              @click="openAction('takeover')"
              >{{ tr("action.takeover") }}</Button
            >
            <Button
              v-if="canManualConclude && detail.canManualConclude"
              type="primary"
              @click="openAction('manual_completed')"
              >{{ tr("action.completed") }}</Button
            >
            <Button
              v-if="canManualConclude && detail.canManualConclude"
              danger
              @click="openAction('manual_unable')"
              >{{ tr("action.unable") }}</Button
            >
          </Space>
          <section
            v-if="detail.managementPreparation"
            class="mt-4"
            data-testid="fulfillment-management-preparation"
          >
            <Divider>{{ tr("management.title") }}</Divider>
            <Alert
              show-icon
              :type="
                detail.managementPreparation.statusCode === 'needs_attention'
                  ? 'warning'
                  : 'info'
              "
              :message="
                tr(`management.${detail.managementPreparation.statusCode}`)
              "
            />
            <Descriptions bordered :column="1" size="small" class="mt-3">
              <DescriptionsItem :label="tr('management.attempts')">{{
                detail.managementPreparation.attemptCount
              }}</DescriptionsItem>
              <DescriptionsItem
                v-if="detail.managementPreparation.failureCode"
                :label="tr('management.reason')"
                >{{
                  tr(
                    `management.failures.${detail.managementPreparation.failureCode}`,
                  )
                }}</DescriptionsItem
              >
              <DescriptionsItem :label="tr('management.noteLabel')">{{
                tr("management.note")
              }}</DescriptionsItem>
            </Descriptions>
            <Alert
              v-if="managementRepairError"
              class="mt-3"
              type="error"
              show-icon
              :message="tr('management.repairFailed')"
            />
            <Button
              v-if="canRecover && detail.canRepairManagement"
              class="mt-3"
              type="primary"
              :loading="managementRepairLoading"
              @click="repairManagement"
              >{{ tr("management.repair") }}</Button
            >
          </section>
          <section
            v-if="abnormalExecution"
            class="mt-4"
            data-testid="fulfillment-execution-diagnostic"
          >
            <Divider>{{ tr("diagnostic.title") }}</Divider>
            <Descriptions
              bordered
              :column="1"
              size="small"
              class="fulfillment-diagnostic-facts"
            >
              <DescriptionsItem :label="tr('diagnostic.reasonLabel')">{{
                detail.execution?.failureCode
                  ? $t(fulfillmentFailureTranslationKey(detail.execution))
                  : callDiagnostic
                    ? tr(`diagnostic.callStage.${callDiagnostic.stage}`)
                    : $t(fulfillmentFailureTranslationKey(detail.execution))
              }}</DescriptionsItem>
              <DescriptionsItem :label="tr('diagnostic.phaseLabel')">{{
                $t(fulfillmentPhaseTranslationKey(detail.execution!.phaseCode))
              }}</DescriptionsItem>
              <DescriptionsItem :label="tr('diagnostic.attempts')">{{
                detail.execution!.attemptCount
              }}</DescriptionsItem>
              <DescriptionsItem :label="tr('diagnostic.nextStep')">{{
                recoveryExplanation
              }}</DescriptionsItem>
            </Descriptions>
            <Alert
              v-if="
                detail.execution?.failureCode === 'provider_request_not_sent'
              "
              class="mt-3"
              type="info"
              show-icon
              data-testid="fulfillment-confirmed-not-sent"
              :message="tr('diagnostic.notSentConfirmed')"
              :description="tr('diagnostic.notSentChoices')"
            />
            <details
              v-if="detail.execution?.responseExcerpt"
              open
              class="mt-3"
              data-testid="fulfillment-http-response"
            >
              <summary>{{ tr("diagnostic.response") }}</summary>
              <Alert
                v-if="callDiagnostic"
                class="mt-2"
                type="warning"
                show-icon
                data-testid="fulfillment-call-failure"
                :message="tr(`diagnostic.callStage.${callDiagnostic.stage}`)"
                :description="
                  tr(
                    callDiagnostic.started === true
                      ? 'diagnostic.sendAttempted'
                      : callDiagnostic.started === false
                        ? 'diagnostic.notSent'
                        : 'diagnostic.sendUnconfirmed',
                  )
                "
              />
              <pre
                class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all rounded bg-muted p-3 text-sm"
                >{{ formatHTTPResponse(detail.execution.responseExcerpt) }}</pre
              >
            </details>
            <p
              v-else-if="
                detail.execution?.failureCode !== 'provider_request_not_sent'
              "
              class="mt-2 text-muted-foreground"
            >
              {{ tr("diagnostic.responseMissing") }}
            </p>
          </section>

          <section
            v-if="detail.nonAcceptanceVerifications?.length"
            class="mt-4"
            data-testid="fulfillment-non-acceptance-history"
          >
            <h3 class="mb-3 font-semibold">{{ tr("verification.history") }}</h3>
            <Card
              v-for="record in detail.nonAcceptanceVerifications"
              :key="record.providerTaskNo"
              class="mb-3"
              size="small"
              :title="formatTimestamp(record.verifiedAt)"
            >
              <p>{{ tr("verification.originalRetained") }}</p>
              <p class="mt-2 whitespace-pre-wrap">{{ record.note }}</p>
              <p class="mt-2">
                {{ tr("verification.operator") }} #{{ record.actorUserId }}
              </p>
            </Card>
          </section>
          <section
            v-if="detail.recoveryConfigurations?.length"
            class="mt-4"
            data-testid="fulfillment-recovery-configurations"
          >
            <Divider>{{ tr("correction.history") }}</Divider>
            <Card
              v-for="(record, index) in detail.recoveryConfigurations"
              :key="index"
              class="mb-3"
              size="small"
              :title="formatTimestamp(record.occurredAt)"
            >
              <Descriptions :column="{ xs: 1, sm: 2 }" size="small">
                <DescriptionsItem :label="tr('correction.interface')">{{
                  record.configuration.interfaceLabel
                }}</DescriptionsItem>
                <DescriptionsItem :label="tr('correction.region')">{{
                  record.configuration.regionLabel
                }}</DescriptionsItem>
                <DescriptionsItem :label="tr('correction.line')">{{
                  record.configuration.lineLabel
                }}</DescriptionsItem>
                <DescriptionsItem :label="tr('correction.image')">{{
                  record.configuration.operatingSystemLabel
                }}</DescriptionsItem>
                <DescriptionsItem v-if="record.configuration.generationLength" :label="tr('correction.passwordLength')">{{ record.configuration.generationLength }}</DescriptionsItem>
                <DescriptionsItem v-if="record.configuration.generationCharacters" :label="tr('correction.passwordCharacters')">{{ record.configuration.generationCharacters.split(',').map(value => tr(`correction.passwordClass.${value}`)).join(' / ') }}</DescriptionsItem>
              </Descriptions>
            </Card>
          </section>
          <Divider>{{ tr("detail.history") }}</Divider>
          <Timeline v-if="detail.events.length">
            <TimelineItem
              v-for="(event, index) in detail.events"
              :key="`${event.occurredAt}-${index}`"
            >
              <strong>{{
                $t(fulfillmentEventTranslationKey(event.eventCode))
              }}</strong>
              <div>{{ formatTimestamp(event.occurredAt) }}</div>
              <div v-if="event.operatorDisplayName">
                {{ event.operatorDisplayName }}
              </div>
              <div v-if="event.reasonCode">
                {{ eventReasonLabel(event) }}
              </div>
              <div v-if="event.safeNote">{{ event.safeNote }}</div>
            </TimelineItem>
          </Timeline>
          <Empty v-else :description="tr('detail.noHistory')" />
        </div>
      </Spin>
    </VbenDrawerCompat>

    <VbenModalCompat
      v-model:open="actionOpen"
      :confirm-loading="submitting"
      :ok-button-props="{
        disabled:
          (actionMode === 'recover_preparation' && !recoveryReady) ||
          (actionMode === 'verify_non_acceptance' && !confirmedNoCreation),
      }"
      :title="actionTitle"
      @ok="submitAction"
    >
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="tr(`action.help.${actionMode}`)"
      />
      <Checkbox
        v-if="actionMode === 'verify_non_acceptance'"
        v-model:checked="confirmedNoCreation"
        class="mb-4"
        :disabled="submitting"
        >{{ tr("verification.confirmation") }}</Checkbox
      >
      <template v-if="actionMode === 'recover_preparation'">
        <Alert
          class="mb-4"
          type="info"
          show-icon
          :message="tr('correction.terms')"
          :description="tr('correction.help')"
        />
        <Descriptions
          v-if="recoveryEditor"
          bordered
          :column="{ xs: 1, sm: 2 }"
          size="small"
          class="mb-4 fulfillment-specifications"
          data-testid="recovery-paid-specifications"
        >
          <DescriptionsItem
            v-for="spec in recoveryEditor.specifications"
            :key="spec.key"
            :label="tr(`correction.spec.${spec.key}`)"
            >{{ spec.value }}</DescriptionsItem
          >
          <DescriptionsItem :label="tr('correction.originalInterface')">{{
            recoveryEditor.original.interfaceLabel || tr("common.unavailable")
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('correction.originalRegion')">{{
            recoveryEditor.original.regionLabel ||
            tr("correction.originalUnavailable")
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('correction.originalLine')">{{
            recoveryEditor.original.lineLabel ||
            tr("correction.originalUnavailable")
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('correction.originalImage')">{{
            recoveryEditor.original.operatingSystemLabel ||
            tr("correction.originalUnavailable")
          }}</DescriptionsItem>
        </Descriptions>
        <Alert
          v-if="recoveryError"
          class="mb-4"
          show-icon
          type="error"
          :message="tr('correction.loadFailed')"
          ><template #action
            ><Button @click="loadRecoveryEditor(recoveryDesiredSelection)">{{
              tr("common.retry")
            }}</Button></template
          ></Alert
        >
        <Alert
          v-else-if="
            recoveryEditor &&
            (!recoveryEditor.regions.length ||
              !recoveryEditor.operatingSystems.length ||
              (recoveryEditor.selection.region && !recoveryEditor.lines.length))
          "
          class="mb-4"
          show-icon
          type="warning"
          :message="tr('correction.optionsPending')"
          ><template #action
            ><Button
              :loading="recoveryLoading"
              @click="loadRecoveryEditor(recoveryDesiredSelection)"
              >{{ tr("common.retry") }}</Button
            ></template
          ></Alert
        >
      </template>
      <ActionForm />
    </VbenModalCompat>
  </Page>
</template>

<style scoped>
.fulfillment-details {
  min-width: 0;
  overflow-wrap: anywhere;
}
.fulfillment-details :deep(.ant-descriptions-view table),
.fulfillment-specifications :deep(.ant-descriptions-view table) {
  width: 100%;
  table-layout: fixed;
}
.fulfillment-details :deep(.ant-descriptions-item-content),
.fulfillment-details :deep(.ant-descriptions-item-label),
.fulfillment-specifications :deep(.ant-descriptions-item-content),
.fulfillment-specifications :deep(.ant-descriptions-item-label) {
  white-space: normal;
  overflow-wrap: anywhere;
}
.fulfillment-diagnostic-facts :deep(.ant-descriptions-item-label) {
  width: 148px;
}
@media (max-width: 600px) {
  .fulfillment-diagnostic-facts :deep(.ant-descriptions-item-label) {
    width: 100px;
  }
}
</style>
