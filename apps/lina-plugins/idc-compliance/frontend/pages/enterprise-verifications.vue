<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/enterprise-verifications",
  title: "Enterprise Verification",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  EnterpriseCaseItem,
  EnterpriseCaseStatus,
} from "./enterprise-verification-client";

import { computed, onMounted, onBeforeUnmount, ref } from "vue";
import { createNumberedPage } from "./numbered-page";

import { useAccess } from "./plugin-access";
import { Page, useVbenDrawer } from "@vben/common-ui";

import { Badge, Tag } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import MaterialRetentionDrawer from "./components/material-retention-drawer.vue";
import EnterpriseVerificationDetailDrawer from "./components/enterprise-verification-detail-drawer.vue";
import { enterpriseCaseList } from "./enterprise-verification-client";

const statuses: EnterpriseCaseStatus[] = [
  "pending",
  "verifying",
  "review_required",
  "verified",
  "rejected",
  "expired",
  "canceled",
];
const rows = ref<EnterpriseCaseItem[]>([]);
const loading = ref(false);
const pageState = ref({ currentPage: 1, pageSize: 10, total: 0 });
let loadSequence = 0;
let activeFilters = { customerNo: "", statusCode: "" };
const pager = createNumberedPage<EnterpriseCaseItem>((state) => {
  rows.value = state.items;
  pageState.value = state;
  gridApi.setGridOptions({ data: state.items, pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
}, () => detailDrawerApi.close());
onBeforeUnmount(() => { loadSequence++; pager.invalidate(); });

async function load(currentPage = pageState.value.currentPage, pageSize = pageState.value.pageSize) {
  if (!canList.value) return;
  const sequence = ++loadSequence;
  const filters = { ...activeFilters };
  loading.value = true;
  try {
    await pager.query({ currentPage, pageSize }, (page, size) => enterpriseCaseList({
      page, pageSize: size,
      customerNo: filters.customerNo || undefined,
      statusCode: filters.statusCode || undefined,
    }));
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
}

const { hasAccessByCodes } = useAccess();
const canList = computed(() => hasAccessByCodes(["idc:admin:compliance:enterprise-verification:list"]));
const canQuery = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:enterprise-verification:query"]),
);
const metrics = computed(() => ({
  pending: rows.value.filter((item) => item.statusCode === "pending").length,
  reviewing: rows.value.filter((item) =>
    ["verifying", "review_required"].includes(item.statusCode),
  ).length,
  verified: rows.value.filter((item) => item.statusCode === "verified").length,
  reverification: rows.value.filter((item) => item.reverification).length,
}));

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      componentProps: {
        "data-testid": "enterprise-search-customer",
        placeholder: $t(
          "plugin.idc-compliance.enterprise.search.customerPlaceholder",
        ),
      },
      fieldName: "customerNo",
      label: $t("plugin.idc-compliance.enterprise.search.customer"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "enterprise-search-status",
        options: [
          {
            label: $t("plugin.idc-compliance.enterprise.search.all"),
            value: "",
          },
          ...statuses.map((value) => ({
            label: $t(
              `plugin.idc-compliance.enterprise.status.${value}`,
            ),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "statusCode",
      label: $t("plugin.idc-compliance.enterprise.search.status"),
    },
  ];
}

function columns(): VxeGridProps["columns"] {
  const result: VxeGridProps["columns"] = [
    {
      field: "caseNo",
      minWidth: 260,
      slots: { default: "case" },
      title: $t("plugin.idc-compliance.enterprise.table.case"),
    },
    {
      field: "statusCode",
      minWidth: 160,
      slots: { default: "status" },
      title: $t("plugin.idc-compliance.enterprise.table.status"),
    },
    {
      field: "maskedLegalName",
      minWidth: 210,
      slots: { default: "enterprise" },
      title: $t("plugin.idc-compliance.enterprise.table.enterprise"),
    },
    {
      field: "applicantRoleCode",
      minWidth: 160,
      formatter: ({ cellValue }) =>
        $t(`plugin.idc-compliance.enterprise.applicant.${cellValue}`),
      title: $t("plugin.idc-compliance.enterprise.table.applicant"),
    },
    {
      field: "submittedAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      minWidth: 180,
      title: $t("plugin.idc-compliance.enterprise.table.submittedAt"),
    },
  ];
  if (canQuery.value) {
    result.push({
      field: "action",
      fixed: "right",
      slots: { default: "action" },
      title: $t("plugin.idc-compliance.enterprise.table.action"),
      width: 112,
    });
  }
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 92 },
    handleSubmit: async (values) => {
      activeFilters = { customerNo: String(values.customerNo || "").trim(), statusCode: String(values.statusCode || "") };
      await load(1);
    },
    handleReset: async () => {
      await gridApi.formApi.resetForm();
      activeFilters = { customerNo: "", statusCode: "" };
      await load(1);
    },
    schema: [],
    wrapperClass: "plugin-admin-search",
    layout: "inline",
    actionLayout: "inline",
    actionPosition: "left",
    showCollapseButton: false,
  },
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => load(currentPage, pageSize) },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    id: "idc-compliance-enterprise-verifications",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: "caseNo" },
    toolbarConfig: { custom: true, refresh: { queryMethod: () => load() }, zoom: true },
  },
});

const [RetentionDrawer, retentionDrawerApi] = useVbenDrawer({ connectedComponent: MaterialRetentionDrawer });
const canRetention = computed(() => hasAccessByCodes(["idc:admin:compliance:privacy:query"]));

const [DetailDrawerRef, detailDrawerApi] = useVbenDrawer({
  connectedComponent: EnterpriseVerificationDetailDrawer,
});

function statusBadge(status: EnterpriseCaseStatus) {
  if (status === "verified") return "success";
  if (status === "verifying") return "processing";
  if (status === "pending" || status === "review_required") return "warning";
  if (status === "rejected" || status === "expired") return "error";
  return "default";
}

function openDetail(row: EnterpriseCaseItem) {
  detailDrawerApi.setData({ caseNo: row.caseNo });
  detailDrawerApi.open();
}

onMounted(async () => {
  if (!canList.value) return;
  gridApi.setGridOptions({ columns: columns() });
  gridApi.formApi.setState({ schema: searchSchema() });
  await load();
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" :auto-content-height="true" class="plugin-admin-page min-w-0 w-full" content-class="min-h-0 min-w-0" data-testid="enterprise-verification-page">
    <div class="enterprise-shell">

      <div v-if="canRetention" class="flex justify-end"><a-button data-testid="material-retention-open" @click="retentionDrawerApi.open()">{{ $t("plugin.idc-compliance.materialRetention.title") }}</a-button></div>
      <section v-if="canList" class="metric-rail">
        <article>
          <span>{{
            $t("plugin.idc-compliance.enterprise.metrics.pending")
          }}</span
          ><strong>{{ metrics.pending }}</strong>
        </article>
        <article class="metric-amber">
          <span>{{
            $t("plugin.idc-compliance.enterprise.metrics.reviewing")
          }}</span
          ><strong>{{ metrics.reviewing }}</strong>
        </article>
        <article class="metric-green">
          <span>{{
            $t("plugin.idc-compliance.enterprise.metrics.verified")
          }}</span
          ><strong>{{ metrics.verified }}</strong>
        </article>
        <article class="metric-purple">
          <span>{{
            $t(
              "plugin.idc-compliance.enterprise.metrics.reverification",
            )
          }}</span
          ><strong>{{ metrics.reverification }}</strong>
        </article>
      </section>

      <section v-if="canList" class="enterprise-grid">
        <Grid :loading="loading" class="min-h-0 min-w-0">
          <template #case="{ row }">
            <div class="case-cell">
              <strong>{{ row.caseNo }}</strong
              ><span>{{ row.customerNo }}</span>
            </div>
          </template>
          <template #status="{ row }">
            <Badge
              :status="statusBadge(row.statusCode)"
              :text="
                $t(
                  `plugin.idc-compliance.enterprise.status.${row.statusCode}`,
                )
              "
            />
            <Tag v-if="row.reverification" color="purple">{{
              $t("plugin.idc-compliance.enterprise.reverification")
            }}</Tag>
          </template>
          <template #enterprise="{ row }">
            <div class="case-cell">
              <strong>{{ row.maskedLegalName }}</strong
              ><code>{{ row.maskedRegistrationId }}</code>
            </div>
          </template>
          <template #action="{ row }">
            <a-button
              size="small"
              type="link"
              :data-testid="`enterprise-case-detail-${row.caseNo}`"
              @click="openDetail(row)"
            >
              {{ $t("plugin.idc-compliance.enterprise.view") }}
            </a-button>
          </template>
        </Grid>
      </section>
      <DetailDrawerRef @success="load()" />
      <RetentionDrawer />
    </div>
  </Page>
</template>

<style scoped>
.enterprise-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.enterprise-header {
  flex: none;
  border-bottom: 1px solid #d8dee5;
  padding: 2px 2px 8px;
}
.enterprise-header small {
  color: #1677ff;
  font-size: 12px;
  font-weight: 700;
}
.enterprise-header h1 {
  color: #0f172a;
  font-size: 16px;
  line-height: 1.35;
  margin: 5px 0 6px;
}
.enterprise-header p {
  color: #64748b;
  line-height: 1.7;
  margin: 0;
  max-width: 920px;
}
.metric-rail {
  flex: none;
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.metric-rail article {
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 0;
  box-shadow: none;
  display: grid;
  gap: 6px;
  padding: 8px 12px;
}
.metric-rail span {
  color: #64748b;
  font-size: 12px;
  font-weight: 650;
}
.metric-rail strong {
  color: #0f172a;
  font:
    750 20px ui-monospace,
    monospace;
}
.metric-amber {
  border-top: 3px solid #f59e0b !important;
}
.metric-green {
  border-top: 3px solid #10b981 !important;
}
.metric-purple {
  border-top: 3px solid #8b5cf6 !important;
}
.enterprise-grid {
  flex: 1;
  min-height: 0;
  min-width: 0;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 0;
  overflow: hidden;
  padding: 6px;
}
.case-cell {
  display: grid;
  gap: 3px;
}
.case-cell strong {
  color: #0f172a;
}
.case-cell span,
.case-cell code {
  color: #64748b;
  font-size: 12px;
}
.pagination {
  align-items: center;
  border-top: 1px solid #e2e8f0;
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding: 14px 10px 8px;
}
.pagination span {
  background: #0f172a;
  border-radius: 0;
  color: white;
  font:
    700 12px ui-monospace,
    monospace;
  min-width: 32px;
  padding: 6px;
  text-align: center;
}
@media (max-width: 900px) {
  .metric-rail {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
