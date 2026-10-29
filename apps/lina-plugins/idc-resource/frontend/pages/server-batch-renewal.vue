<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/server-renewals",
  title: "Server Renewals",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ServerRenewalCandidate,
  ServerRenewalPreview,
  ServerRenewalPreviewItem,
  ServerRenewalResultItem,
} from "./server-batch-renewal-client";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Space,
  Tag,
  message,
} from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { useAccess } from "./plugin-access";
import { createResourcePage } from "./resource-page";
import {
  confirmServerRenewals,
  listServerRenewalCandidates,
  previewServerRenewals,
} from "./server-batch-renewal-client";

const props = defineProps<{ entitlementNo?: string }>();
const tr = (key: string, params?: Record<string, number | string>) =>
  $t(`plugin.idc-resource.serverRenewal.${key}`, params ?? {});
const rows = ref<ServerRenewalCandidate[]>([]);
const selected = ref<string[]>([]);
const loading = ref(false);
const preview = ref<ServerRenewalPreview>();
const results = ref<ServerRenewalResultItem[]>([]);
const confirmed = ref(false);
let accessRevision = 0;
let disposed = false;
let previewEntitlementNo: string | undefined;
const directory = createResourcePage<ServerRenewalCandidate>({
  onPage: (page) => candidateGridApi.setGridOptions({ pagerConfig: { currentPage: page.currentPage, pageSize: page.pageSize, total: page.total } }),
  onDenied: () => {
    ++accessRevision;
    rows.value = [];
    selected.value = [];
    preview.value = undefined;
    results.value = [];
    confirmed.value = false;
    void previewModalApi.close();
    void resultModalApi.close();
  },
  onFailure: () => message.error(tr("loadFailed")),
});
const { hasAccessByCodes } = useAccess();
const canView = computed(() => hasAccessByCodes(["idc:admin:resource:server-batch:view"]));
const canRenew = computed(() =>
  canView.value && hasAccessByCodes(["idc:admin:resource:server-batch:safe"]),
);
const selectedCount = computed(() => selected.value.length);

const [RenewalForm, renewalFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [
    {
      component: "InputNumber",
      componentProps: { max: 120, min: 1, precision: 0 },
      defaultValue: 1,
      fieldName: "renewalCycles",
      label: tr("renewalCycles"),
      rules: z.number().int().min(1).max(120),
    },
  ],
  showDefaultActions: false,
});

function candidateColumns(): VxeGridProps<ServerRenewalCandidate>["columns"] {
  return [
    {
      field: "selection",
      slots: { default: "selection" },
      title: "",
      width: 54,
    },
    { field: "serverNo", minWidth: 250, title: tr("serverNo") },
    {
      field: "productName",
      minWidth: 220,
      slots: { default: "productName" },
      title: tr("product"),
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("actions"),
      width: 150,
    }),
  ];
}

function previewColumns(): VxeGridProps<ServerRenewalPreviewItem>["columns"] {
  return [
    { field: "serverNo", minWidth: 250, title: tr("serverNo") },
    {
      field: "eligibility",
      slots: { default: "eligibility" },
      title: tr("eligibility"),
      width: 140,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: tr("amount"),
      width: 250,
    },
    {
      field: "originalExpiresAt",
      slots: { default: "originalExpiresAt" },
      title: tr("currentExpiry"),
      minWidth: 180,
    },
    {
      field: "projectedExpiresAt",
      slots: { default: "projectedExpiresAt" },
      title: tr("projectedExpiry"),
      minWidth: 180,
    },
  ];
}

function resultColumns(): VxeGridProps<ServerRenewalResultItem>["columns"] {
  return [
    { field: "serverNo", minWidth: 250, title: tr("serverNo") },
    {
      field: "result",
      slots: { default: "result" },
      title: tr("result"),
      width: 150,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: tr("amount"),
      width: 250,
    },
  ];
}

const [CandidateGrid, candidateGridApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }: {currentPage: number; pageSize: number}) => void load({ currentPage, pageSize }) },
  gridOptions: {
    columns: candidateColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, pageSizes: [10, 20, 50, 100], total: 0 },
    rowConfig: { keyField: "serverNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-server-renewal-candidates",
  },
});
const [PreviewGrid, previewGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: previewColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 360,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "serverNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-server-renewal-preview",
  },
});
const [ResultGrid, resultGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: resultColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 360,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "serverNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-server-renewal-results",
  },
});

const [PreviewModal, previewModalApi] = useVbenModal({
  onCancel: () => previewModalApi.close(),
  onConfirm: execute,
});
const [ResultModal, resultModalApi] = useVbenModal({
  onCancel: () => resultModalApi.close(),
  onConfirm: () => resultModalApi.close(),
});

function invalidateContext() {
  ++accessRevision;
  directory.invalidate();
  rows.value = [];
  selected.value = [];
  preview.value = undefined;
  previewEntitlementNo = undefined;
  results.value = [];
  confirmed.value = false;
  loading.value = false;
  previewModalApi.unlock();
  void previewModalApi.close();
  void resultModalApi.close();
}
watch([() => props.entitlementNo, canView, canRenew], () => {
  invalidateContext();
  if (!disposed && canView.value) void load(directory.current);
});
onBeforeUnmount(() => { disposed = true; invalidateContext(); });

watch(rows, (value) => candidateGridApi.setGridOptions({ data: value }), {
  deep: true,
});
watch(
  preview,
  (value) => previewGridApi.setGridOptions({ data: value?.items || [] }),
  { deep: true },
);
watch(results, (value) => resultGridApi.setGridOptions({ data: value }), {
  deep: true,
});
watch(
  () => preferences.app.locale,
  () => {
    candidateGridApi.setGridOptions({ columns: candidateColumns() });
    previewGridApi.setGridOptions({ columns: previewColumns() });
    resultGridApi.setGridOptions({ columns: resultColumns() });
  },
);

function product(item: { productNameEnUS: string; productNameZhCN: string }) {
  return preferences.app.locale === "zh-CN"
    ? item.productNameZhCN
    : item.productNameEnUS;
}
function money(minor: string, currency: string) {
  if (!/^[A-Z]{3}$/.test(currency)) return "-";
  try {
    const value = BigInt(minor);
    const formatter = new Intl.NumberFormat(preferences.app.locale, {
      currency,
      style: "currency",
    });
    const digits = formatter.resolvedOptions().maximumFractionDigits;
    if (digits === undefined) return "—";
    const scale = BigInt(`1${"0".repeat(digits)}`);
    const absolute = value < 0n ? -value : value;
    const fraction = (absolute % scale).toString().padStart(digits, "0");
    const formatted = formatter
      .formatToParts(absolute / scale)
      .map((part) => (part.type === "fraction" ? fraction : part.value))
      .join("");
    return value < 0n ? `-${formatted}` : formatted;
  } catch {
    return "-";
  }
}
function toggle(row: ServerRenewalCandidate, checked: boolean) {
  if (checked) {
    if (selected.value.length >= 20) {
      message.warning(tr("maximumSelected"));
      return;
    }
    if (!selected.value.includes(row.serverNo))
      selected.value = [...selected.value, row.serverNo];
  } else
    selected.value = selected.value.filter(
      (serverNo) => serverNo !== row.serverNo,
    );
}
async function selectAndPreview(row: ServerRenewalCandidate) {
  selected.value = [row.serverNo];
  await createPreview();
}
async function load(requested: {currentPage: number; pageSize: number} = directory.current, resetSelection = false) {
  if (disposed || !canView.value) return;
  const revision = accessRevision;
  const entitlementNo = props.entitlementNo;
  loading.value = true;
  try {
    await directory.query(requested, "",
      (page, limit) => listServerRenewalCandidates({ page, limit, entitlementNo }),
      (response) => {
        rows.value = response.items;
        if (resetSelection) selected.value = [];
      });
  } finally {
    if (revision === accessRevision) loading.value = false;
  }
}
async function createPreview() {
  if (disposed || !canRenew.value || loading.value) return;
  const revision = ++accessRevision;
  const entitlementNo = props.entitlementNo;
  const serverNos = [...selected.value];
  if (!selected.value.length) {
    message.warning(tr("selectRequired"));
    return;
  }
  const { valid } = await renewalFormApi.validate();
  if (!valid || revision !== accessRevision || !canRenew.value || disposed) return;
  const values = await renewalFormApi.getValues<{ renewalCycles: number }>();
  if (revision !== accessRevision || !canRenew.value || disposed) return;
  loading.value = true;
  try {
    const result = await previewServerRenewals({
      entitlementNo,
      renewalCycles: values.renewalCycles,
      requestKey: `server-renewal-${crypto.randomUUID()}`,
      serverNos,
    });
    if (revision !== accessRevision || disposed || !canRenew.value || entitlementNo !== props.entitlementNo) return;
    preview.value = result;
    previewEntitlementNo = entitlementNo;
    confirmed.value = false;
    previewModalApi.open();
  } catch {
    if (revision === accessRevision) message.error(tr("previewFailed"));
  } finally {
    if (revision === accessRevision) loading.value = false;
  }
}
async function execute() {
  if (disposed || !canRenew.value || previewEntitlementNo !== props.entitlementNo) return;
  const revision = accessRevision;
  if (!preview.value || !confirmed.value) {
    message.warning(tr("confirmationRequired"));
    return;
  }
  previewModalApi.lock();
  try {
    const response = await confirmServerRenewals({
      entitlementNo: props.entitlementNo,
      confirmationDigest: preview.value.confirmationDigest,
      renewalCycles: preview.value.renewalCycles,
      requestKey: preview.value.requestKey,
      serverNos: preview.value.items.map((item) => item.serverNo),
    });
    if (revision !== accessRevision) return;
    results.value = response.items;
    previewModalApi.close();
    resultModalApi.open();
  } catch {
    if (revision === accessRevision) message.error(tr("confirmationFailed"));
  } finally {
    if (revision === accessRevision) previewModalApi.unlock();
  }
}

onMounted(() => void load());
const layoutRoot = useAdminGridLayout(candidateGridApi, previewGridApi, resultGridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" auto-content-height data-testid="server-batch-renewal-page">
    <Card class="renewal-card h-full" :title="tr('title')" :loading="loading">
      <template #extra>
        <Space>
          <span>{{ tr("selectedCount", { count: selectedCount }) }}</span>
          <RenewalForm class="min-w-72" />
          <Button
            v-if="canRenew"
            data-testid="server-renewal-preview"
            type="primary"
            :disabled="!selectedCount"
            @click="createPreview"
            >{{ tr("previewSelected") }}</Button
          >
          <Button data-testid="server-renewal-refresh" @click="load(directory.current, true)">{{ tr("refresh") }}</Button>
        </Space>
      </template>
      <Alert class="mb-4" show-icon type="info" :message="tr('guidance')" />
      <CandidateGrid>
        <template #selection="{ row }"
          ><Checkbox
            :checked="selected.includes(row.serverNo)"
            :disabled="!canRenew"
            @change="toggle(row, $event.target.checked)"
        /></template>
        <template #productName="{ row }">{{ product(row) }}</template>
        <template #action="{ row }"
          ><Button v-if="canRenew" type="link" @click="selectAndPreview(row)">{{
            tr("previewOne")
          }}</Button></template
        >
        <template #empty
          ><span>{{ tr("empty") }}</span></template
        >
      </CandidateGrid>
    </Card>

    <PreviewModal :title="tr('confirmTitle')" class="resource-renewal-preview">
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="tr('confirmHint')"
      />
      <PreviewGrid v-if="preview">
        <template #eligibility="{ row }"
          ><Tag :color="row.eligible ? 'green' : 'red'">{{
            tr(
              row.eligible
                ? "eligible"
                : `failure.${row.failureCode || "unknown"}`,
            )
          }}</Tag></template
        >
        <template #amount="{ row }">{{
          row.eligible ? money(row.totalAmountMinor, row.currencyCode) : "-"
        }}</template>
        <template #originalExpiresAt="{ row }">{{
          row.originalExpiresAt ? formatTimestamp(row.originalExpiresAt) : "-"
        }}</template>
        <template #projectedExpiresAt="{ row }">{{
          row.projectedExpiresAt ? formatTimestamp(row.projectedExpiresAt) : "-"
        }}</template>
      </PreviewGrid>
      <Checkbox v-model:checked="confirmed" class="mt-4">{{
        tr("confirmCheckbox")
      }}</Checkbox>
    </PreviewModal>

    <ResultModal :title="tr('resultTitle')" class="resource-renewal-result">
      <ResultGrid v-if="results.length">
        <template #result="{ row }"
          ><Tag :color="row.succeeded ? 'green' : 'red'">{{
            tr(
              row.succeeded
                ? "succeeded"
                : `failure.${row.failureCode || "unknown"}`,
            )
          }}</Tag></template
        >
        <template #amount="{ row }">{{
          row.currencyCode ? money(row.totalAmountMinor, row.currencyCode) : "-"
        }}</template>
      </ResultGrid>
    </ResultModal>
  </Page>
</template>

<style scoped>
:global(.resource-renewal-preview) {
  width: min(1000px, calc(100vw - 32px));
}
:global(.resource-renewal-result) {
  width: min(800px, calc(100vw - 32px));
}
.renewal-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.renewal-card :deep(.ant-card-head) {
  flex: 0 0 auto;
}

.renewal-card :deep(.ant-card-body) {
  display: flex;
  flex: 1 1 0%;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.renewal-card :deep(.vxe-grid) {
  flex: 1 1 0%;
  max-width: 100%;
  min-height: 0;
  width: 100%;
}
</style>
