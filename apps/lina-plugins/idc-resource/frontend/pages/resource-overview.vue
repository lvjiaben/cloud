<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/overview",
  title: "Resource Overview",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ResourceFinanceFact,
  ResourceOverviewPool,
  ResourceOverviewSummary,
} from "./resource-overview-client";

import { onMounted, ref, watch } from "vue";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Col,
  Row,
  Statistic,
  Tag,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { getResourceOverview } from "./resource-overview-client";
import { isResourceAccessDenied } from "./resource-page";

const loading = ref(false);
const failed = ref(false);
const summary = ref<ResourceOverviewSummary>();
const poolPage = ref({ currentPage: 1, pageSize: 10, total: 0 });
const financePage = ref({ currentPage: 1, pageSize: 10, total: 0 });
let generation = 0;

function poolColumns(): VxeGridProps<ResourceOverviewPool>["columns"] {
  return [
  {
    field: "displayName",
    slots: { default: "pool" },
    title: $t("plugin.idc-resource.overview.pool"),
  },
  {
    field: "channelName",
    slots: { default: "channel" },
    title: $t("plugin.idc-resource.overview.channel"),
  },
  {
    field: "resourceKind",
    slots: { default: "kind" },
    title: $t("plugin.idc-resource.overview.kind"),
  },
  {
    field: "status",
    slots: { default: "status" },
    title: $t("plugin.idc-resource.overview.status"),
  },
  { field: "routing", slots: { default: "routing" }, title: $t("plugin.idc-resource.overview.routing") },
  {
    field: "updatedAt",
    slots: { default: "updatedAt" },
    title: $t("plugin.idc-resource.overview.updatedAt"),
  },
  ];
}

function financeColumns(): VxeGridProps<ResourceFinanceFact>["columns"] {
  return [
  {
    field: "kind",
    slots: { default: "kind" },
    title: $t("plugin.idc-resource.overview.factType"),
  },
  {
    field: "channelName",
    title: $t("plugin.idc-resource.overview.channel"),
  },
  { field: "value", slots: { default: "value" }, title: $t("plugin.idc-resource.overview.value") },
  {
    field: "observedAt",
    slots: { default: "observedAt" },
    title: $t("plugin.idc-resource.overview.observedAt"),
  },
  ];
}

const [PoolGrid, poolGridApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }: {currentPage: number; pageSize: number}) => void load("pool", { currentPage, pageSize }) },
  gridOptions: {
    columns: poolColumns(), data: [], emptyRender: { name: "Empty" }, height: "100%",
    pagerConfig: { enabled: true, ...poolPage.value, pageSizes: [10, 20, 50, 100] }, rowConfig: { keyField: "poolNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-resource-overview-pools",
  },
});
const [FinanceGrid, financeGridApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }: {currentPage: number; pageSize: number}) => void load("finance", { currentPage, pageSize }) },
  gridOptions: {
    columns: financeColumns(), data: [], emptyRender: { name: "Empty" }, height: "100%",
    pagerConfig: { enabled: true, ...financePage.value, pageSizes: [10, 20, 50, 100] }, rowConfig: { keyField: "snapshotNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-resource-overview-finance",
  },
});
watch(summary, (value) => {
  poolGridApi.setGridOptions({ data: value?.pools || [] });
  financeGridApi.setGridOptions({ data: value?.finance || [] });
}, { deep: true });
watch(() => preferences.app.locale, () => {
  poolGridApi.setGridOptions({ columns: poolColumns() });
  financeGridApi.setGridOptions({ columns: financeColumns() });
});

async function load(table?: "pool" | "finance", requested?: {currentPage: number; pageSize: number}) {
  const attempt = ++generation;
  const nextPool = { ...poolPage.value };
  const nextFinance = { ...financePage.value };
  if (requested) {
    const target = table === "pool" ? nextPool : nextFinance;
    target.currentPage = requested.pageSize !== target.pageSize ? 1 : requested.currentPage;
    target.pageSize = requested.pageSize;
  }
  loading.value = true;
  failed.value = false;
  try {
    const result = (await getResourceOverview({poolPage: nextPool.currentPage, poolPageSize: nextPool.pageSize,
      financePage: nextFinance.currentPage, financePageSize: nextFinance.pageSize})).summary;
    if (attempt !== generation) return;
    if (!Number.isSafeInteger(result.poolTotal) || result.poolTotal < result.pools.length ||
        !Number.isSafeInteger(result.financeTotal) || result.financeTotal < result.finance.length ||
        result.pools.length > nextPool.pageSize || result.finance.length > nextFinance.pageSize) {
      throw new Error("Invalid resource overview page");
    }
    summary.value = result;
    poolPage.value = { ...nextPool, total: result.poolTotal };
    financePage.value = { ...nextFinance, total: result.financeTotal };
  } catch (error) {
    if (attempt !== generation) return;
    if (isResourceAccessDenied(error)) {
      summary.value = undefined;
      poolPage.value = { ...poolPage.value, currentPage: 1, total: 0 };
      financePage.value = { ...financePage.value, currentPage: 1, total: 0 };
    }
    failed.value = true;
  } finally {
    if (attempt === generation) loading.value = false;
    poolGridApi.setGridOptions({ pagerConfig: poolPage.value });
    financeGridApi.setGridOptions({ pagerConfig: financePage.value });
  }
}

function factLabel(kind: string) {
  const known = new Set(["balance", "cost", "expiry"]);
  return $t(
    `plugin.idc-resource.overview.fact.${known.has(kind) ? kind : "unknown"}`,
  );
}

function resourceKindLabel(kind: string) {
  const known = new Set(["bare_metal", "cloud_server", "virtual_server"]);
  return $t(
    `plugin.idc-resource.overview.resourceKind.${known.has(kind) ? kind : "unknown"}`,
  );
}

function poolStatusLabel(status: string) {
  const known = new Set([
    "deleted",
    "disabled",
    "draining",
    "enabled",
    "maintenance",
  ]);
  return $t(
    `plugin.idc-resource.overview.poolStatus.${known.has(status) ? status : "unknown"}`,
  );
}

function factValue(item: ResourceFinanceFact) {
  if (item.kind === "expiry")
    return item.expiresAt ? formatTimestamp(item.expiresAt) : "—";
  return item.amount && item.currencyCode
    ? `${item.amount} ${item.currencyCode}`
    : "—";
}

onMounted(() => void load());
const layoutRoot = useAdminGridLayout(poolGridApi, financeGridApi);
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true" data-testid="resource-overview-page">
    <div :ref="layoutRoot" class="overview-shell">
    <div class="overview-heading">
      <Button :loading="loading" @click="load()">{{ $t("plugin.idc-resource.overview.refresh") }}</Button>
    </div>
    <Alert
      v-if="failed"
      class="mb-4"
      show-icon
      type="error"
      :message="$t('plugin.idc-resource.overview.failed')"
    />
    <Row v-if="summary" :gutter="[12, 12]" class="overview-metrics">
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><Statistic
            :title="$t('plugin.idc-resource.overview.channels')"
            :value="summary.enabledChannels" /></Card
      ></Col>
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><Statistic
            :title="$t('plugin.idc-resource.overview.pools')"
            :value="summary.enabledPools" /></Card
      ></Col>
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><Statistic
            :title="$t('plugin.idc-resource.overview.mappings')"
            :value="summary.activeMappings" /></Card
      ></Col>
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><Statistic
            :title="$t('plugin.idc-resource.overview.pending')"
            :value="summary.pendingTasks" /></Card
      ></Col>
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><Statistic
            :title="$t('plugin.idc-resource.overview.unknown')"
            :value="summary.unknownTasks" /></Card
      ></Col>
      <Col :xs="12" :md="8" :xl="4"
        ><Card
          ><p class="mb-2 text-muted-foreground">{{ $t('plugin.idc-resource.overview.providerCapacityAt') }}</p>
          <span class="text-base">{{ summary.latestProviderCapacityAt ? formatTimestamp(summary.latestProviderCapacityAt) : '—' }}</span></Card
      ></Col>
    </Row>
    <div class="overview-lists">
    <Card class="overview-directory" size="small">
      <PoolGrid :loading="loading" :table-title="$t('plugin.idc-resource.overview.poolTitle')">
        <template #pool="{ row }"><strong>{{ row.displayName }}</strong></template>
        <template #channel="{ row }">{{ row.channelName }}</template>
        <template #status="{ row }"><Tag>{{ poolStatusLabel(row.status) }}</Tag></template>
        <template #kind="{ row }">{{ resourceKindLabel(row.resourceKind) }}</template>
        <template #routing="{ row }">{{ row.routingPriority }} / {{ row.routingWeight }}</template>
        <template #updatedAt="{ row }">{{ formatTimestamp(row.updatedAt) }}</template>
      </PoolGrid>
    </Card>
    <Card class="overview-directory" size="small">
      <FinanceGrid :loading="loading" :table-title="$t('plugin.idc-resource.overview.financeTitle')">
        <template #kind="{ row }"><Tag>{{ factLabel(row.kind) }}</Tag></template>
        <template #value="{ row }">{{ factValue(row) }}</template>
        <template #observedAt="{ row }">{{ formatTimestamp(row.observedAt) }}</template>
      </FinanceGrid>
    </Card>
    </div>
    </div>
  </Page>
</template>

<style scoped>
.overview-shell { display: flex; flex-direction: column; gap: 12px; height: 100%; min-height: 0; }
.overview-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.overview-heading h2 { margin: 0; font-size: 16px; font-weight: 600; }
.overview-heading p { margin: 4px 0 0; font-size: 13px; color: hsl(var(--muted-foreground)); }
.overview-metrics { flex: 0 0 auto; }
.overview-metrics :deep(.ant-card-body) { padding: 12px 16px; }
.overview-metrics :deep(.ant-statistic-content) { font-size: 24px; }
.overview-lists { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); gap: 12px; flex: 1; min-height: 0; }
.overview-directory { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.overview-directory :deep(.ant-card-head) { flex: 0 0 auto; }
.overview-directory :deep(.ant-card-body) { flex: 1; min-height: 0; padding: 0; overflow: hidden; }
@media (max-width: 1100px) {
  .overview-shell { height: auto; }
  .overview-lists { grid-template-columns: minmax(0, 1fr); flex: none; }
  .overview-directory { height: 380px; }
}
</style>
