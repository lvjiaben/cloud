<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { PluginHostMountContext } from "../../src/host-context";

import { computed, watch } from "vue";
import {
  Alert,
  Button,
  Descriptions,
  DescriptionsItem,
  Space,
  Tabs,
  TabPane,
  Tag,
} from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { GhostButton } from "#/components/global/button";

import { useCreditWorkspace } from "../../src/finance/use-credit-workspace";
import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

const props = defineProps<{ context: PluginHostMountContext }>();
const {
  accountPageRows,
  activeTab,
  applicationPageRows,
  detailOpen,
  error,
  evidence,
  formatDate,
  load,
  loadPage,
  loading,
  money,
  notice,
  openDetail,
  openEvidence,
  openReview,
  pageLoading,
  pageState,
  reviewForm,
  reviewLoading,
  reviewOpen,
  selected,
  statementPageRows,
  status,
  submitReview,
  tr,
} = useCreditWorkspace(props);

const columns = computed<VxeGridProps["columns"]>(() => {
  const customer = {
    field: "customerNo",
    title: tr("finance.credit.customer"),
    minWidth: 120,
  };
  const statusColumn = {
    align: "center" as const,
    field: "statusCode",
    title: tr("finance.credit.status"),
    width: 120,
    slots: { default: "status" },
  };
  if (activeTab.value === "accounts")
    return [
      {
        field: "creditAccountNo",
        title: tr("finance.credit.account"),
        minWidth: 200,
      },
      customer,
      {
        field: "currencyCode",
        title: tr("finance.credit.currency"),
        width: 90,
        align: "center",
      },
      {
        field: "availableLimitMinor",
        title: tr("finance.credit.available"),
        minWidth: 140,
        slots: { default: "money" },
      },
      {
        field: "usedMinor",
        title: tr("finance.credit.used"),
        minWidth: 140,
        slots: { default: "money" },
      },
      statusColumn,
    ];
  if (activeTab.value === "statements")
    return [
      {
        field: "statementNo",
        title: tr("finance.credit.statement"),
        minWidth: 200,
      },
      customer,
      {
        field: "totalAmountMinor",
        title: tr("finance.credit.amount"),
        minWidth: 140,
        slots: { default: "money" },
      },
      {
        field: "outstandingAmountMinor",
        title: tr("finance.credit.outstanding"),
        minWidth: 140,
        slots: { default: "money" },
      },
      {
        field: "paymentDueAt",
        title: tr("finance.credit.due"),
        width: 140,
        slots: { default: "date" },
      },
      statusColumn,
    ];
  return [
    {
      field: "applicationNo",
      title: tr("finance.credit.application"),
      minWidth: 200,
    },
    customer,
    {
      field: "requestedLimitMinor",
      title: tr("finance.credit.amount"),
      minWidth: 140,
      slots: { default: "money" },
    },
    {
      field: "reviewCount",
      title: tr("finance.credit.progress"),
      width: 100,
      align: "center",
      slots: { default: "progress" },
    },
    statusColumn,
    {
      field: "actions",
      title: tr("finance.credit.action"),
      width: 140,
      fixed: "right",
      slots: { default: "actions" },
    },
  ];
});
const rows = computed(() =>
  activeTab.value === "accounts"
    ? accountPageRows.value
    : activeTab.value === "statements"
      ? statementPageRows.value
      : applicationPageRows.value,
);
const rowKey = computed(() =>
  activeTab.value === "accounts"
    ? "creditAccountNo"
    : activeTab.value === "statements"
      ? "statementNo"
      : "applicationNo",
);

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange({ currentPage, pageSize }: { currentPage: number; pageSize: number }) {
      void loadPage(activeTab.value, currentPage, pageSize);
    },
  },
  gridOptions: {
    border: true,
    columns: [],
    data: [],
    height: 300,
    pagerConfig: {
      enabled: true,
      pageSize: 10,
      pageSizes: [10, 20, 50, 100],
    },
    toolbarConfig: { refresh: false, search: false },
    showOverflow: "tooltip",
  },
});
watch(
  () =>
    [
      columns.value,
      rows.value,
      rowKey.value,
      pageState[activeTab.value],
      pageLoading[activeTab.value],
      tr("finance.credit.empty"),
    ] as const,
  ([currentColumns, data, keyField, pagination, busy, emptyText]) =>
    gridApi.setGridOptions({
      columns: currentColumns,
      data,
      emptyText,
      loading: busy,
      pagerConfig: { ...pagination },
      rowConfig: { keyField },
    }),
  { immediate: true },
);

const [ReviewForm, reviewFormApi] = useVbenForm({
  showDefaultActions: false,
  schema: [],
});
watch(reviewOpen, async (open) => {
  if (!open) return;
  reviewFormApi.setState({
    schema: [
      {
        fieldName: "decisionCode",
        component: "RadioGroup",
        componentProps: {
          optionType: "button",
          buttonStyle: "solid",
          options: [
            { label: tr("finance.credit.approve"), value: "approved" },
            { label: tr("finance.credit.reject"), value: "rejected" },
          ],
        },
      },
      {
        fieldName: "comment",
        component: "Textarea",
        label: tr("finance.credit.comment"),
        componentProps: { placeholder: tr("finance.credit.comment"), rows: 4 },
        rules: "required",
      },
      {
        fieldName: "evidence",
        component: "Textarea",
        label: tr("finance.credit.evidenceBasis"),
        componentProps: {
          placeholder: tr("finance.credit.evidenceBasis"),
          rows: 4,
        },
        rules: "required",
      },
    ],
  });
  await reviewFormApi.setValues({ ...reviewForm });
});
async function confirmReview() {
  if (!(await reviewFormApi.validate()).valid) return;
  const values = await reviewFormApi.getValues();
  reviewForm.comment = String(values.comment ?? "");
  reviewForm.evidence = String(values.evidence ?? "");
  reviewForm.decisionCode =
    values.decisionCode === "rejected" ? "rejected" : "approved";
  await submitReview();
}
</script>

<template>
  <section
    class="credit-management-workspace"
    data-testid="credit-management-panel"
  >
    <Alert v-if="error" :message="error" show-icon type="error">
      <template #action
        ><Button size="small" @click="load">{{
          tr("finance.credit.retry")
        }}</Button></template
      >
    </Alert>
    <Alert v-if="notice" :message="notice" show-icon />
    <Tabs v-model:active-key="activeTab">
      <TabPane key="applications"
        ><template #tab
          ><span data-testid="credit-applications-tab">{{
            tr("finance.credit.applications")
          }}</span></template
        ></TabPane
      >
      <TabPane key="accounts" :tab="tr('finance.credit.accounts')" />
      <TabPane key="statements" :tab="tr('finance.credit.statements')" />
      <template #rightExtra
        ><Button :loading="loading" @click="load">{{
          tr("finance.credit.retry")
        }}</Button></template
      >
    </Tabs>
    <Grid :key="activeTab" data-testid="credit-management-grid">
      <template #money="{ column, row }">{{
        money(row[column.field], row.currencyCode)
      }}</template>
      <template #status="{ row }"
        ><Tag>{{ status(row.statusCode) }}</Tag></template
      >
      <template #date="{ row }">{{ formatDate(row.paymentDueAt) }}</template>
      <template #progress="{ row }"
        >{{ row.reviewCount }}/{{ row.requiredApprovalCount }}</template
      >
      <template #actions="{ row }"
        ><Space size="small">
          <GhostButton
            data-testid="credit-application-detail"
            @click="openDetail(row)"
            >{{ tr("finance.credit.detail") }}</GhostButton
          >
          <GhostButton
            v-if="row.statusCode === 'pending_review'"
            data-testid="credit-application-review"
            @click="openReview(row)"
            >{{ tr("finance.credit.review") }}</GhostButton
          >
        </Space></template
      >
    </Grid>
    <VbenDrawerCompat
      v-model:open="detailOpen"
      :title="tr('finance.credit.detailTitle')"
      :width="760"
    >
      <section v-if="selected" data-testid="credit-application-detail-drawer">
        <Descriptions bordered :column="2" size="small">
          <DescriptionsItem :label="tr('finance.credit.application')">{{
            selected.applicationNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('finance.credit.customer')">{{
            selected.customerNo
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('finance.credit.amount')">{{
            money(selected.requestedLimitMinor, selected.currencyCode)
          }}</DescriptionsItem>
          <DescriptionsItem :label="tr('finance.credit.progress')"
            >{{ selected.reviewCount }}/{{
              selected.requiredApprovalCount
            }}</DescriptionsItem
          >
        </Descriptions>
        <h3 class="my-4">{{ tr("finance.credit.evidence") }}</h3>
        <Space wrap
          ><Button
            v-for="item in evidence"
            :key="item.uploadNo"
            data-testid="credit-evidence-open"
            @click="openEvidence(item)"
            >{{ item.fileName }} ·
            {{ Math.ceil(item.sizeBytes / 1024) }} KiB</Button
          ></Space
        >
      </section>
    </VbenDrawerCompat>
    <VbenModalCompat
      v-model:open="reviewOpen"
      :title="tr('finance.credit.reviewTitle')"
      :ok-text="tr('finance.credit.submit')"
      :confirm-loading="reviewLoading"
      @ok="confirmReview"
    >
      <div v-if="selected" data-testid="credit-review-modal"><ReviewForm /></div>
    </VbenModalCompat>
  </section>
</template>

<style scoped>
.credit-management-workspace {
  min-width: 0;
  max-width: 100%;
}
</style>
