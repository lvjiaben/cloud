<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/product-fulfillment/tasks",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ProductFulfillmentEvent,
  ProductFulfillmentReasonOption,
  ProductFulfillmentTaskDetail,
  ProductFulfillmentTaskSummary,
} from "./product-fulfillment-client";

import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { createNumberedPage } from "./numbered-page";
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
  approveAndStartProductFulfillment,
  concludeManualProductFulfillment,
  newFulfillmentActionKey,
  productFulfillmentTask,
  productFulfillmentTasks,
  rejectProductFulfillmentReview,
  takeoverProductFulfillment,
} from "./product-fulfillment-client";
import {
  createFulfillmentActionAttempt,
  fulfillmentEventTranslationKey,
  fulfillmentEventReasonLabel,
  fulfillmentModeOptions,
  fulfillmentModeTranslationKey,
  fulfillmentQuantityText,
  fulfillmentStatusColor,
  fulfillmentStatusOptions,
  fulfillmentStatusTranslationKey,
  requiresFulfillmentActionNote,
} from "./product-fulfillment-state";
import { customerDisplay, searchCustomerOptions, type CustomerSelectOption } from "./customer-selection";

type ActionMode =
  | "manual_completed"
  | "manual_unable"
  | "review_approve"
  | "review_reject"
  | "takeover";

const { hasAccessByCodes } = useAccess();
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

const loading = ref(false);
const detailLoading = ref(false);
const submitting = ref(false);
const loadError = ref(false);
const rows = ref<ProductFulfillmentTaskSummary[]>([]);
const detail = ref<ProductFulfillmentTaskDetail>();
const detailOpen = ref(false);
const actionOpen = ref(false);
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
    gridApi.setGridOptions({ data: state.items, pagerConfig: { currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
  },
  () => { detailOpen.value = false; actionOpen.value = false; detail.value = undefined; },
);
const customerOptions = ref<CustomerSelectOption[]>([]);
const customerLoading = ref(false);

async function searchCustomers(keyword = "") {
  customerLoading.value = true;
  try {
    customerOptions.value = await searchCustomerOptions(keyword);
    gridApi.formApi.setState({ schema: searchSchema() });
  } finally { customerLoading.value = false; }
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
    { field: "customerDisplayName", minWidth: 240, slots: { default: "customer" }, title: tr("table.customer") },
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
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => { void load(currentPage, pageSize); } },
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-4",
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
  return [
    {
      component: "Select" as const,
      componentProps: {
        options: reasons,
        placeholder: tr("action.reasonPlaceholder"),
      },
      fieldName: "reasonCode",
      label: tr("action.reason"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Textarea" as const,
      componentProps: { maxlength: 512, rows: 5, showCount: true },
      fieldName: "note",
      label: tr("action.note"),
      rules: requiresFulfillmentActionNote(actionMode.value)
        ? z.string().trim().min(10).max(512)
        : z.string().trim().max(512).optional(),
    },
  ];
}

const [ActionForm, actionFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

const actionReasons = computed(() => {
  if (!detail.value) return [];
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

async function load(targetPage = currentPage.value, targetSize = pageSize.value) {
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
  search();
}

onBeforeUnmount(() => { loadSequence += 1; pagination.invalidate(); });

function eventReasonLabel(event: ProductFulfillmentEvent) {
  return fulfillmentEventReasonLabel(
    event,
    preferences.app.locale !== "zh-CN",
    tr("common.unavailable"),
  );
}

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

async function openAction(mode: ActionMode) {
  actionMode.value = mode;
  actionKey.value = createFulfillmentActionAttempt(
    newFulfillmentActionKey,
  ).idempotencyKey;
  actionOpen.value = true;
  await nextTick();
  await actionFormApi.setState({ schema: actionSchema() });
  await actionFormApi.resetForm();
}

async function submitAction() {
  if (!detail.value || submitting.value) return;
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
    let response: { task: ProductFulfillmentTaskDetail };
    if (actionMode.value === "takeover") {
      response = await takeoverProductFulfillment(detail.value.intentNo, common);
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
      response = await concludeManualProductFulfillment(
        detail.value.intentNo,
        {
          ...common,
          outcomeCode:
            actionMode.value === "manual_completed"
              ? "completed"
              : "unable_to_complete",
        },
      );
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
  await load();
  await searchCustomers();
});
</script>

<template>
  <Page :auto-content-height="true" data-testid="product-fulfillment-page">
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
          <Button size="small" @click="load(lastRequested.currentPage, lastRequested.pageSize)">{{
            tr("common.retry")
          }}</Button>
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
          <ghost-button
            v-if="canQuery"
            @click="openDetail(row)"
            >{{ tr("common.detail") }}</ghost-button
          >
        </template>
      </Grid>
    </Card>
    <Alert
      v-else
      show-icon
      type="warning"
      :message="tr('message.noPermission')"
    />

    <VbenDrawerCompat
      v-model:open="detailOpen"
      :title="tr('detail.title')"
    >
      <Spin :spinning="detailLoading">
        <template v-if="detail">
          <Alert
            class="mb-4"
            show-icon
            type="info"
            :message="tr('detail.safety')"
          />
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem :label="tr('table.order')">
              {{ detail.orderNo }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.customer')">
              {{ customerDisplay(detail) }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.product')">
              {{ localizedName(detail) }}
            </DescriptionsItem>
            <DescriptionsItem :label="tr('table.mode')">
              {{
                $t(
                  fulfillmentModeTranslationKey(detail.fulfillmentModeCode),
                )
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

          <Space class="mt-4" wrap>
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

          <Divider>{{ tr("detail.history") }}</Divider>
          <Timeline v-if="detail.events.length">
            <TimelineItem
              v-for="(event, index) in detail.events"
              :key="`${event.occurredAt}-${index}`"
            >
              <strong>{{ $t(fulfillmentEventTranslationKey(event.eventCode)) }}</strong>
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
        </template>
      </Spin>
    </VbenDrawerCompat>

    <VbenModalCompat
      v-model:open="actionOpen"
      :confirm-loading="submitting"
      :title="actionTitle"
      @ok="submitAction"
    >
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="tr(`action.help.${actionMode}`)"
      />
      <ActionForm />
    </VbenModalCompat>
  </Page>
</template>
