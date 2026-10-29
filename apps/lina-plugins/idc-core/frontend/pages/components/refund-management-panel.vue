<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormSchema } from "#/adapter/form";
import type { VbenFormProps } from "@vben/common-ui";

import type { RefundDetail, RefundSummary } from "../refund-client";

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
const props = defineProps<{initialRefundNo?: string}>();

import { createNumberedPage } from "../numbered-page";

import { useAccess } from "../plugin-access";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  message,
  Space,
  Tag,
} from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

import { getRefundDetail, listRefunds, reviewRefund } from "../refund-client";
import {
  customerDisplay,
  searchCustomerOptions,
  type CustomerSelectOption,
} from "../customer-selection";

const { hasAccessByCodes } = useAccess();
const canList = computed(() =>
  hasAccessByCodes(["idc:admin:billing:refund:list"]),
);
const canQuery = computed(() =>
  hasAccessByCodes(["idc:admin:billing:refund:query"]),
);
const canReview = computed(() =>
  hasAccessByCodes(["idc:admin:billing:refund:review"]),
);

const rows = ref<RefundSummary[]>([]);
const loading = ref(false);
const errorMessage = ref("");
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);
const currentPage = ref(1);
const pageSize = ref(10);
let sequence = 0;
const pagination = createNumberedPage<RefundSummary>(
  (state) => {
    rows.value = state.items;
    currentPage.value = state.currentPage;
    pageSize.value = state.pageSize;
    gridApi.setGridOptions({
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
    reviewOpen.value = false;
    detail.value = undefined;
    reviewTarget.value = undefined;
  },
);
const detailOpen = ref(false);
const detailLoading = ref(false);
const detail = ref<RefundDetail>();
const reviewOpen = ref(false);
const reviewLoading = ref(false);
const reviewTarget = ref<RefundSummary>();

function columns(): VxeGridProps<RefundSummary>["columns"] {
  return [
    {
      field: "refundNo",
      slots: { default: "refundNo" },
      title: t("refunds.table.refund"),
      width: 190,
    },
    {
      field: "customerDisplayName",
      slots: { default: "customer" },
      title: t("table.customer"),
      minWidth: 240,
    },
    {
      field: "status",
      slots: { default: "status" },
      title: t("table.status"),
      width: 130,
    },
    {
      field: "amount",
      slots: { default: "amount" },
      title: t("table.amount"),
      width: 140,
    },
    {
      field: "submittedAt",
      slots: { default: "submittedAt" },
      title: t("refunds.table.submittedAt"),
      width: 165,
    },
    {
      fixed: "right",
      field: "action",
      slots: { default: "action" },
      title: t("refunds.table.action"),
      width: 170,
    },
  ];
}
function searchSchema(): VbenFormProps["schema"] {
  return [
    {
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
    },
    {
      component: "Select",
      componentProps: { allowClear: true, options: statusOptions.value },
      defaultValue: "pending_review",
      fieldName: "statusCode",
      label: t("table.status"),
    },
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
    }) => void loadRows(currentPage, pageSize),
  },
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    handleReset: search,
    handleSubmit: search,
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-2",
  },
  gridOptions: {
    columns: columns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: "refundNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-refunds",
  },
});
watch(rows, (items) => gridApi.setGridOptions({ data: items }), { deep: true });
watch(
  () => preferences.app.locale,
  () => {
    gridApi.formApi.setState({ schema: searchSchema() });
    gridApi.setGridOptions({ columns: columns() });
  },
);

const statusOptions = computed(() =>
  [
    "pending_review",
    "approved",
    "awaiting_balance_consent",
    "executing",
    "succeeded",
    "failed",
    "result_unknown",
    "manual_confirmation",
    "rejected",
  ].map((value) => ({ label: t(`status.${value}`), value })),
);

function t(key: string, values?: Record<string, number | string>) {
  return $t(`plugin.idc-core.finance.${key}`, values ?? {});
}

function reviewFormSchema(): VbenFormSchema[] {
  return [
    {
      component: "RadioGroup" as const,
      componentProps: {
        buttonStyle: "solid",
        optionType: "button",
        options: [
          { label: t("action.approve"), value: "approve" },
          { label: t("action.reject"), value: "reject" },
        ],
      },
      defaultValue: "approve",
      fieldName: "decisionCode",
      label: t("action.decision"),
      rules: z.enum(["approve", "reject"]),
    },
    {
      component: "Textarea" as const,
      componentProps: { rows: 3 },
      fieldName: "comment",
      label: t("action.comment"),
    },
    {
      component: "Textarea" as const,
      componentProps: {
        "data-testid": "refund-review-evidence",
        rows: 4,
      },
      fieldName: "evidence",
      label: t("refunds.evidenceRequired"),
    },
  ];
}

const [ReviewForm, reviewFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 review-form",
});

function statusColor(value: string) {
  if (["approved", "succeeded"].includes(value)) return "success";
  if (["failed", "rejected"].includes(value)) return "error";
  if (["awaiting_balance_consent", "result_unknown"].includes(value))
    return "warning";
  return "processing";
}

function formatMinor(value: string, currencyCode: string) {
  if (!/^\d+$/.test(value)) return "—";
  let digits = 2;
  try {
    digits =
      new Intl.NumberFormat("en", {
        currency: currencyCode,
        style: "currency",
      }).resolvedOptions().maximumFractionDigits ?? 2;
  } catch {
    digits = 2;
  }
  const padded = value.padStart(digits + 1, "0");
  return digits
    ? `${currencyCode} ${padded.slice(0, -digits)}.${padded.slice(-digits)}`
    : `${currencyCode} ${padded}`;
}

function localizedDescription(item: RefundDetail["items"][number]) {
  return preferences.app.locale === "zh-CN"
    ? item.descriptionZhCN || item.descriptionEnUS
    : item.descriptionEnUS || item.descriptionZhCN;
}

async function loadRows(pageNumber = currentPage.value, size = pageSize.value) {
  if (!canList.value) return;
  const currentSequence = ++sequence;
  loading.value = true;
  errorMessage.value = "";
  await pagination.query(
    { currentPage: pageNumber, pageSize: size },
    async (page, limit) => {
      try {
        const values = await gridApi.formApi.getValues();
        const result = await listRefunds({
          page,
          customerNo: String(values.customerNo || "").trim() || undefined,
          limit,
          statusCode: String(values.statusCode || "") || undefined,
        });
        return { items: result.refunds, total: result.total };
      } catch (failure) {
        if (currentSequence === sequence)
          errorMessage.value = t("refunds.loadFailed");
        throw failure;
      } finally {
        if (currentSequence === sequence) loading.value = false;
      }
    },
  );
}

function search() {
  void loadRows(1);
}

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try {
    customerOptions.value = await searchCustomerOptions(keyword);
    gridApi.formApi.setState({ schema: searchSchema() });
  } finally {
    customerLoading.value = false;
  }
}

async function openDetail(refund: Pick<RefundSummary, "refundNo">) {
  if (!canQuery.value) return;
  detailOpen.value = true;
  detailLoading.value = true;
  detail.value = undefined;
  try {
    detail.value = await getRefundDetail(refund.refundNo);
  } catch {
    message.error(t("refunds.detailFailed"));
  } finally {
    detailLoading.value = false;
  }
}

async function openReview(refund: RefundSummary) {
  reviewTarget.value = refund;
  reviewOpen.value = true;
  await nextTick();
  await reviewFormApi.setState({ schema: reviewFormSchema() });
  await reviewFormApi.resetForm();
  await reviewFormApi.setFieldValue("decisionCode", "approve");
}

async function digest(value: string, required: boolean) {
  const canonical = value.trim();
  if (!canonical) {
    if (required) throw new Error("digest-required");
    return "";
  }
  const result = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  return [...new Uint8Array(result)]
    .map((item) => item.toString(16).padStart(2, "0"))
    .join("");
}

function requestNo() {
  return `refund-review-${crypto.randomUUID()}`;
}

async function submitReview() {
  if (!reviewTarget.value) return;
  const { valid } = await reviewFormApi.validate();
  if (!valid) return;
  const values = await reviewFormApi.getValues<{
    comment?: string;
    decisionCode: "approve" | "reject";
    evidence: string;
  }>();
  reviewLoading.value = true;
  try {
    await reviewRefund(reviewTarget.value, {
      commentDigest: await digest(values.comment || "", false),
      decisionCode: values.decisionCode,
      evidenceDigest: await digest(values.evidence || "", false),
      expectedVersion: reviewTarget.value.version,
      requestNo: requestNo(),
    });
    reviewOpen.value = false;
    message.success(t("refunds.reviewSaved"));
    await loadRows();
  } catch {
    message.error(
      reviewTarget.value.highAmount && values.decisionCode === "approve"
        ? t("refunds.reauthFailed")
        : t("refunds.reviewFailed"),
    );
    throw new Error("refund-review-failed");
  } finally {
    reviewLoading.value = false;
  }
}

watch(() => props.initialRefundNo, refundNo => {
  if (refundNo && /^[A-Za-z0-9_:-]{1,40}$/.test(refundNo)) void openDetail({refundNo});
}, {immediate: true});

onBeforeUnmount(() => {
  sequence += 1;
  pagination.invalidate();
});
onMounted(() => {
  gridApi.formApi.setState({ schema: searchSchema() });
  void loadRows();
  void searchCustomers();
});
</script>

<template>
  <Alert
    v-if="!canList"
    :message="t('refunds.noListPermission')"
    show-icon
    type="warning"
  />
  <div v-else data-testid="refund-management-panel">
    <Alert
      v-if="errorMessage"
      class="refund-alert"
      :message="errorMessage"
      show-icon
      type="error"
    />
    <Grid :loading="loading">
      <template #customer="{ row }">{{ customerDisplay(row) }}</template>
      <template #refundNo="{ row }"
        ><span class="mono">{{ row.refundNo }}</span></template
      >
      <template #status="{ row }"
        ><Tag :color="statusColor(row.statusCode)">{{
          t(`status.${row.statusCode}`)
        }}</Tag></template
      >
      <template #amount="{ row }"
        ><strong>{{
          formatMinor(row.amountMinor, row.currencyCode)
        }}</strong></template
      >
      <template #submittedAt="{ row }">{{
        formatTimestamp(row.submittedAt)
      }}</template>
      <template #action="{ row }"
        ><Space
          ><Button
            v-if="canQuery"
            data-testid="refund-detail-open"
            size="small"
            type="link"
            @click="openDetail(row)"
            >{{ t("detail.button") }}</Button
          ><Button
            v-if="canReview && row.statusCode === 'pending_review'"
            data-testid="refund-review-open"
            size="small"
            type="link"
            @click="openReview(row)"
            >{{ t("refunds.review") }}</Button
          ></Space
        ></template
      >
    </Grid>
    <VbenDrawerCompat
      v-model:open="detailOpen"
      :title="t('refunds.detailTitle')"
      :width="760"
    >
      <div data-testid="refund-detail-drawer-content">
        <div v-if="detailLoading">{{ t("detail.loading") }}</div>
        <template v-else-if="detail">
          <Descriptions :column="2" bordered size="small">
            <DescriptionsItem :label="t('refunds.table.refund')">
              {{ detail.refund.refundNo }}
            </DescriptionsItem>
            <DescriptionsItem :label="t('table.customer')">
              {{ customerDisplay(detail.refund) }}
            </DescriptionsItem>
            <DescriptionsItem :label="t('table.status')">
              {{ t(`status.${detail.refund.statusCode}`) }}
            </DescriptionsItem>
            <DescriptionsItem :label="t('table.amount')">
              {{
                formatMinor(
                  detail.refund.amountMinor,
                  detail.refund.currencyCode,
                )
              }}
            </DescriptionsItem>
            <DescriptionsItem :label="t('refunds.reasonLabel')" :span="2">
              {{ t(`refunds.reason.${detail.refund.reasonCode}`) }}
              <span v-if="detail.refund.reasonDetail">
                · {{ detail.refund.reasonDetail }}
              </span>
            </DescriptionsItem>
          </Descriptions>
          <Card
            v-for="item in detail.items"
            :key="item.refundItemNo"
            class="refund-item-card"
          >
            <div class="item-heading">
              <div>
                <strong>{{ localizedDescription(item) }}</strong>
                <div class="item-quantity">
                  {{ t("refunds.quantity", { quantity: item.quantity }) }}
                </div>
              </div>
              <strong>
                {{ formatMinor(item.amountMinor, detail.refund.currencyCode) }}
              </strong>
            </div>
            <Alert
              v-for="impact in item.impacts"
              :key="impact.entitlementNo"
              class="refund-impact"
              :message="
                impact.requiresTermination
                  ? t('refunds.dataDeletion')
                  : t('refunds.noDataDeletion')
              "
              :description="`${t('table.status')}: ${t(`status.${impact.statusCode}`)}`"
              show-icon
              :type="impact.requiresTermination ? 'warning' : 'info'"
            />
          </Card>
        </template>
      </div>
    </VbenDrawerCompat>

    <VbenModalCompat
      v-model:open="reviewOpen"
      :confirm-loading="reviewLoading"
      :title="t('refunds.reviewTitle')"
      @ok="submitReview"
    >
      <div data-testid="refund-review-modal-content">
        <Alert
          v-if="reviewTarget?.highAmount"
          :message="t('refunds.reauthNotice')"
          show-icon
          type="warning"
        />
        <ReviewForm />
      </div>
    </VbenModalCompat>
  </div>
</template>

<style scoped>
.refund-toolbar {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(180px, 0.7fr) auto;
  gap: 10px;
  margin-bottom: 16px;
}
.refund-alert,
.review-form,
.refund-item-card {
  margin-top: 16px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.pager,
.item-heading {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}
.pager {
  margin-top: 18px;
  justify-content: flex-end;
}
.refund-impact {
  margin-top: 10px;
}
.item-quantity {
  margin-top: 4px;
  color: var(--ant-color-text-secondary, #8c8c8c);
  font-size: 12px;
}
@media (max-width: 640px) {
  .refund-toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
