<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/health",
  title: "Provider Health",
};
</script>

<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ProviderChannelHealth,
  ProviderHealthCode,
  ProviderHealthObservation,
} from "./health-client";

import { computed, onMounted, ref } from "vue";

import { useAccess } from "./plugin-access";
import { Page, useVbenModal } from "@vben/common-ui";

import { Badge, Tag, message } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import ProviderHealthPolicyModal from "./components/provider-health-policy-modal.vue";
import { providerHealthList } from "./health-client";
import { createResourcePage } from "./resource-page";

const { hasAccessByCodes } = useAccess();
const canUpdatePolicy = computed(() =>
  hasAccessByCodes(["idc:admin:resource:health-policy:write"]),
);
const rows = ref<ProviderChannelHealth[]>([]);
const directory = createResourcePage<ProviderChannelHealth>({
  onPage: (page) => gridApi.setGridOptions({ pagerConfig: { currentPage: page.currentPage, pageSize: page.pageSize, total: page.total } }),
  onDenied: () => {
    rows.value = [];
    policyModalApi.setData({ item: undefined });
    void policyModalApi.close();
  },
  onFailure: () => message.error($t("plugin.idc-resource.pagination.loadFailed")),
});

const metrics = computed(() => ({
  protected: rows.value.filter((item) => item.circuitOpen).length,
  degraded: rows.value.filter((item) => item.healthCode === "degraded").length,
  healthy: rows.value.filter((item) => item.healthCode === "healthy").length,
  unknown: rows.value.filter((item) => item.healthCode === "unknown").length,
}));

function columns(): VxeGridProps["columns"] {
  const result: VxeGridProps["columns"] = [
    {
      field: "channelName",
      minWidth: 210,
      slots: { default: "channel" },
      title: $t("plugin.idc-resource.health.table.channel"),
    },
    {
      field: "healthCode",
      width: 145,
      slots: { default: "health" },
      title: $t("plugin.idc-resource.health.table.health"),
    },
    {
      field: "observations",
      minWidth: 340,
      slots: { default: "observations" },
      title: $t("plugin.idc-resource.health.table.signals"),
    },
    {
      field: "window",
      minWidth: 190,
      slots: { default: "window" },
      title: $t("plugin.idc-resource.health.table.window"),
    },
    {
      field: "protection",
      minWidth: 210,
      slots: { default: "protection" },
      title: $t("plugin.idc-resource.health.table.protection"),
    },
  ];
  if (canUpdatePolicy.value) {
    result.push({
      field: "action",
      fixed: "right",
      slots: { default: "action" },
      title: $t("plugin.idc-resource.health.table.action"),
      width: 116,
    });
  }
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    id: "idc-resource-provider-health",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, pageSizes: [10, 20, 50, 100], total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: ({ page }: {page: {currentPage: number; pageSize: number}}) => directory.query(page, "",
          (currentPage, limit) => providerHealthList({ page: currentPage, limit }),
          (result) => { rows.value = result.items; }),
      },
    },
    cellConfig: { height: 78 },
    rowConfig: { keyField: "channelNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
  },
});

const [PolicyModal, policyModalApi] = useVbenModal({
  connectedComponent: ProviderHealthPolicyModal,
});

function openPolicy(item: ProviderChannelHealth) {
  policyModalApi.setData({ item });
  policyModalApi.open();
}

async function policyUpdated() {
  await gridApi.query();
}

function badgeStatus(code: ProviderHealthCode) {
  if (code === "healthy") return "success";
  if (code === "degraded") return "warning";
  if (code === "unavailable") return "error";
  return "default";
}

function healthCodeLabel(code: string) {
  const known = new Set(["degraded", "healthy", "unavailable", "unknown"]);
  return $t(
    `plugin.idc-resource.health.code.${known.has(code) ? code : "unknown"}`,
  );
}

function signalLabel(kind: string, item: ProviderHealthObservation) {
  const latency = item.observedAt ? `${item.latencyMs}ms` : "—";
  const health = healthCodeLabel(item.healthCode);
  return `${$t(`plugin.idc-resource.health.signal.${kind}`)} · ${health} · ${latency}`;
}

function channelStatusLabel(status: string) {
  const known = new Set([
    "degraded",
    "deleted",
    "disabled",
    "enabled",
    "maintenance",
  ]);
  return $t(
    `plugin.idc-resource.health.channelStatus.${known.has(status) ? status : "unknown"}`,
  );
}

function healthSignals(row: ProviderChannelHealth) {
  return [
    { item: row.authentication, kind: "authentication" },
    { item: row.query, kind: "query" },
    { item: row.write, kind: "write" },
    { item: row.providerCapacity, kind: "providerCapacity" },
  ];
}

onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
</script>

<template>
  <Page :auto-content-height="true" data-testid="provider-health-page">
    <div class="health-shell">
      <section class="health-metrics" aria-live="polite">
        <article class="metric metric--danger">
          <span>{{ $t("plugin.idc-resource.health.metrics.protected") }}</span
          ><strong>{{ metrics.protected }}</strong>
        </article>
        <article class="metric metric--warning">
          <span>{{ $t("plugin.idc-resource.health.metrics.degraded") }}</span
          ><strong>{{ metrics.degraded }}</strong>
        </article>
        <article class="metric metric--good">
          <span>{{ $t("plugin.idc-resource.health.metrics.healthy") }}</span
          ><strong>{{ metrics.healthy }}</strong>
        </article>
        <article class="metric">
          <span>{{ $t("plugin.idc-resource.health.metrics.unknown") }}</span
          ><strong>{{ metrics.unknown }}</strong>
        </article>
      </section>

      <div class="health-grid">
        <Grid>
          <template #channel="{ row }">
            <div class="channel-cell">
              <strong>{{ row.channelName }}</strong
              ><span>{{ channelStatusLabel(row.channelStatus) }}</span>
            </div>
          </template>
          <template #health="{ row }"
            ><Badge
              :status="badgeStatus(row.healthCode)"
              :text="healthCodeLabel(row.healthCode)"
          /></template>
          <template #observations="{ row }">
            <div class="signals">
              <Tag
                v-for="signal in healthSignals(row)"
                :key="signal.kind"
                :color="
                  signal.item.healthCode === 'healthy'
                    ? 'green'
                    : signal.item.healthCode === 'degraded'
                      ? 'orange'
                      : 'default'
                "
                >{{ signalLabel(signal.kind, signal.item) }}</Tag
              >
            </div>
          </template>
          <template #window="{ row }"
            ><div class="window-cell">
              <strong>{{ row.failureCount }} / {{ row.sampleCount }}</strong
              ><span>{{
                $t("plugin.idc-resource.health.window.unknown", {
                  count: row.unknownCount,
                })
              }}</span
              ><span>{{
                $t("plugin.idc-resource.health.window.consecutive", {
                  count: row.consecutiveFailures,
                })
              }}</span>
            </div></template
          >
          <template #protection="{ row }">
            <div class="protection-cell" :class="{ active: row.circuitOpen }">
              <strong>{{
                row.circuitOpen
                  ? $t("plugin.idc-resource.health.protected")
                  : $t("plugin.idc-resource.health.writesOpen")
              }}</strong
              ><span>{{
                row.writeProtectedUntil
                  ? formatTimestamp(row.writeProtectedUntil)
                  : $t("plugin.idc-resource.health.noDeadline")
              }}</span
              ><span>{{
                $t("plugin.idc-resource.health.protectionDuration", {
                  seconds: row.policy.protectionDurationSeconds,
                })
              }}</span>
            </div>
          </template>
          <template #action="{ row }"
            ><a-button
              :data-testid="`provider-health-policy-${row.channelNo}`"
              size="small"
              type="link"
              @click="openPolicy(row)"
              >{{ $t("plugin.idc-resource.health.editPolicy") }}</a-button
            ></template
          >
        </Grid>
      </div>
    </div>
    <PolicyModal @success="policyUpdated" />
  </Page>
</template>

<style scoped>
.health-shell {
  --cyan: 34 211 238;
  display: grid;
  gap: 18px;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}
.health-metrics {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.metric {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 17px 20px;
}
.metric span {
  color: hsl(var(--muted-foreground));
  display: block;
  font-size: 12px;
  margin-bottom: 9px;
}
.metric strong {
  font-family: ui-monospace, monospace;
  font-size: 22px;
  font-weight: 550;
}
.metric--danger strong {
  color: rgb(239 68 68);
}
.metric--warning strong {
  color: rgb(217 119 6);
}
.metric--good strong {
  color: rgb(22 163 74);
}
.health-grid {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  overflow: hidden;
  padding: 4px;
  height: 100%;
  min-height: 0;
}

.health-grid :deep(.vxe-grid) {
  height: 100%;
  min-height: 0;
}
.channel-cell,
.window-cell,
.protection-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.channel-cell span,
.window-cell span,
.protection-cell span,
.protection-cell code {
  color: hsl(var(--muted-foreground));
  font-size: 10px;
}
.signals {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding-block: 4px;
}
.window-cell strong {
  color: rgb(8 145 178);
  font-family: ui-monospace, monospace;
  font-size: 18px;
}
.protection-cell.active strong {
  color: rgb(220 38 38);
}
@media (max-width: 900px) {
  .health-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .health-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
