<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/provider-capacity",
  title: "Provider Capacity Observations",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ProviderCapacityChannelOption,
  ProviderCapacityKind,
  ProviderCapacityOperation,
  ProviderCapacityOperationListParams,
  ProviderCapacityStatus,
} from "./provider-capacity-client";

import { computed, onMounted, ref } from "vue";

import { useAccess } from "./plugin-access";
import { Page, useVbenModal } from "@vben/common-ui";

import { Badge, Card, Col, Row, Space, Statistic, Tag, message } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import ProviderCapacitySyncCreateModal from "./components/provider-capacity-refresh-modal.vue";
import { providerCapacityOperationList } from "./provider-capacity-client";
import { createResourcePage } from "./resource-page";

const syncPermission = "idc:admin:resource:provider-capacity:refresh";
const { hasAccessByCodes } = useAccess();
const canSynchronize = computed(() => hasAccessByCodes([syncPermission]));

const operations = ref<ProviderCapacityOperation[]>([]);
const channels = ref<ProviderCapacityChannelOption[]>([]);
const directory = createResourcePage<ProviderCapacityOperation>({
  onPage: (page) => gridApi.setGridOptions({ pagerConfig: { currentPage: page.currentPage, pageSize: page.pageSize, total: page.total } }),
  onDenied: () => {
    operations.value = [];
    channels.value = [];
    syncModalApi.setData({ channels: [], selectedChannelNo: "" });
    void syncModalApi.close();
  },
  onFailure: () => message.error($t("plugin.idc-resource.pagination.loadFailed")),
});

const metrics = computed(() => ({
  collecting: operations.value.filter((item) => item.status === "collecting")
    .length,
  complete: operations.value.filter((item) => item.status === "complete")
    .length,
  failed: operations.value.filter((item) => item.status === "failed").length,
  items: operations.value.reduce((total, item) => total + item.itemCount, 0),
}));

function allOption() {
  return {
    label: $t("plugin.idc-resource.providerCapacity.search.all"),
    value: "",
  };
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: {
        "data-testid": "provider-capacity-search-channel",
        options: [
          allOption(),
          ...channels.value.map((channel) => ({
            label: channel.displayName,
            value: channel.channelNo,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "channelNo",
      label: $t("plugin.idc-resource.providerCapacity.search.channel"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "provider-capacity-search-kind",
        options: [
          allOption(),
          ...["region", "line", "plan", "image", "stock"].map((value) => ({
            label: $t(`plugin.idc-resource.providerCapacity.kind.${value}`),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "kind",
      label: $t("plugin.idc-resource.providerCapacity.search.kind"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "provider-capacity-search-status",
        options: [
          allOption(),
          ...["collecting", "complete", "failed", "superseded"].map(
            (value) => ({
              label: $t(`plugin.idc-resource.providerCapacity.status.${value}`),
              value,
            }),
          ),
        ],
      },
      defaultValue: "",
      fieldName: "status",
      label: $t("plugin.idc-resource.providerCapacity.search.status"),
    },
  ];
}

function columns(): VxeGridProps["columns"] {
  const result: VxeGridProps["columns"] = [
    {
      field: "channelName",
      minWidth: 230,
      slots: { default: "channel" },
      title: $t("plugin.idc-resource.providerCapacity.table.channel"),
    },
    {
      field: "kind",
      slots: { default: "kind" },
      title: $t("plugin.idc-resource.providerCapacity.table.kind"),
      width: 118,
    },
    {
      field: "status",
      minWidth: 150,
      slots: { default: "status" },
      title: $t("plugin.idc-resource.providerCapacity.table.status"),
    },
    {
      field: "pageCount",
      minWidth: 150,
      slots: { default: "progress" },
      title: $t("plugin.idc-resource.providerCapacity.table.progress"),
    },
    {
      field: "requestedAt",
      minWidth: 178,
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-resource.providerCapacity.table.requestedAt"),
    },
    {
      field: "lastObservedAt",
      minWidth: 178,
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-resource.providerCapacity.table.observedAt"),
    },
    {
      field: "failureCode",
      minWidth: 190,
      slots: { default: "failure" },
      title: $t("plugin.idc-resource.providerCapacity.table.result"),
    },
  ];
  if (canSynchronize.value) {
    result.push({
      field: "action",
      fixed: "right",
      slots: { default: "action" },
      title: $t("plugin.idc-resource.providerCapacity.table.action"),
      width: 140,
    });
  }
  return result;
}

function normalizeFilters(
  values: Record<string, unknown>,
): ProviderCapacityOperationListParams {
  return {
    channelNo: String(values.channelNo || "").trim() || undefined,
    kind: String(values.kind || "") as "" | ProviderCapacityKind,
    status: String(values.status || "") as "" | ProviderCapacityStatus,
  };
}

function filterSignature(filters: ProviderCapacityOperationListParams) {
  return JSON.stringify([
    filters.channelNo || "",
    filters.kind || "",
    filters.status || "",
  ]);
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: {
      componentProps: { allowClear: true },
      labelWidth: 82,
    },
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-3",
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    id: "idc-resource-provider-capacity-operations",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, pageSizes: [10, 20, 50, 100], total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: {page: {currentPage: number; pageSize: number}}, formValues: Record<string, unknown> = {}) => {
          const filters = normalizeFilters(formValues);
          const signature = filterSignature(filters);
          return directory.query(page, signature,
            (currentPage, limit) => providerCapacityOperationList({ ...filters, page: currentPage, limit }),
            (result) => {
              operations.value = result.items;
              channels.value = result.channels;
            });
        },
      },
    },
    rowConfig: { keyField: "observationNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
  },
});

const [ProviderCapacitySyncCreateModalRef, syncModalApi] = useVbenModal({
  connectedComponent: ProviderCapacitySyncCreateModal,
});

function statusBadge(status: ProviderCapacityStatus) {
  if (status === "complete") return "success";
  if (status === "collecting") return "processing";
  if (status === "failed") return "error";
  return "default";
}

function failureLabel(code: string) {
  const knownCodes = new Set([
    "provider_authentication",
    "provider_internal",
    "provider_invalid_response",
    "provider_timeout",
    "provider_transport",
    "provider_unavailable",
    "provider_unknown",
    "synchronization_failed",
  ]);
  const normalizedCode = String(code || "").trim();
  const messageKey = knownCodes.has(normalizedCode)
    ? normalizedCode
    : "safe_unknown";
  return $t(
    `plugin.idc-resource.providerCapacity.result.failure.${messageKey}`,
  );
}

function openSync(selectedChannelNo = "") {
  syncModalApi.setData({
    channels: channels.value,
    selectedChannelNo,
  });
  syncModalApi.open();
}

async function onSyncCreated() {
  await gridApi.reload();
}

onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
  gridApi.setState({ formOptions: { schema: searchSchema() } });
});
</script>

<template>
  <Page
    :auto-content-height="true"
    data-testid="provider-capacity-operations-page"
  >
    <div class="provider-capacity-shell">
      <Card size="small">
        <Row :gutter="[16, 16]" aria-live="polite">
          <Col :lg="6" :sm="12" :xs="24">
            <Statistic
              :title="
                $t('plugin.idc-resource.providerCapacity.metrics.collecting')
              "
              :value="metrics.collecting"
            />
          </Col>
          <Col :lg="6" :sm="12" :xs="24">
            <Statistic
              :title="
                $t('plugin.idc-resource.providerCapacity.metrics.complete')
              "
              :value="metrics.complete"
            />
          </Col>
          <Col :lg="6" :sm="12" :xs="24">
            <Statistic
              :title="$t('plugin.idc-resource.providerCapacity.metrics.failed')"
              :value="metrics.failed"
            />
          </Col>
          <Col :lg="6" :sm="12" :xs="24">
            <Statistic
              :title="$t('plugin.idc-resource.providerCapacity.metrics.items')"
              :value="metrics.items"
            />
          </Col>
        </Row>
      </Card>

      <Card class="provider-capacity-grid-card" size="small">
        <Grid>
          <template #toolbar-tools>
            <Space>
              <a-button
                v-if="canSynchronize"
                data-testid="provider-capacity-sync-open"
                :disabled="channels.length === 0"
                type="primary"
                @click="openSync()"
              >
                {{
                  $t("plugin.idc-resource.providerCapacity.actions.synchronize")
                }}
              </a-button>
            </Space>
          </template>

          <template #channel="{ row }">
            <div class="channel-cell">
              <strong>{{ row.channelName }}</strong>
            </div>
          </template>

          <template #kind="{ row }">
            <Tag color="cyan">
              {{ $t(`plugin.idc-resource.providerCapacity.kind.${row.kind}`) }}
            </Tag>
          </template>

          <template #status="{ row }">
            <div
              class="status-cell"
              :data-testid="`provider-capacity-status-${row.observationNo}`"
            >
              <Badge
                :status="statusBadge(row.status)"
                :text="
                  $t(
                    `plugin.idc-resource.providerCapacity.status.${row.status}`,
                  )
                "
              />
              <span>{{
                $t(
                  `plugin.idc-resource.providerCapacity.trigger.${row.trigger}`,
                )
              }}</span>
            </div>
          </template>

          <template #progress="{ row }">
            <div class="progress-cell">
              <strong>{{ row.itemCount }}</strong>
              <span>{{
                $t("plugin.idc-resource.providerCapacity.table.pages", {
                  count: row.pageCount,
                })
              }}</span>
            </div>
          </template>

          <template #failure="{ row }">
            <span v-if="row.failureCode" class="failure-code">{{
              failureLabel(row.failureCode)
            }}</span>
            <span v-else class="result-ok">{{
              row.status === "collecting"
                ? $t("plugin.idc-resource.providerCapacity.result.pending")
                : $t("plugin.idc-resource.providerCapacity.result.clean")
            }}</span>
          </template>

          <template #action="{ row }">
            <a-button
              :data-testid="`provider-capacity-sync-channel-${row.channelNo}`"
              size="small"
              type="link"
              @click="openSync(row.channelNo)"
            >
              {{ $t("plugin.idc-resource.providerCapacity.actions.syncAgain") }}
            </a-button>
          </template>
        </Grid>
      </Card>
    </div>

    <ProviderCapacitySyncCreateModalRef @success="onSyncCreated" />
  </Page>
</template>

<style scoped>
.provider-capacity-shell {
  display: grid;
  gap: 16px;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}

.provider-capacity-grid-card {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.provider-capacity-grid-card :deep(.ant-card-body) {
  display: flex;
  height: 100%;
  min-height: 0;
  padding: 0;
  width: 100%;
}

.provider-capacity-grid-card :deep(.ant-card-body > div) {
  flex: 1 1 0%;
  min-width: 0;
  width: 100%;
}

.provider-capacity-grid-card :deep(.vxe-grid) {
  flex: 1;
  max-width: 100%;
  min-height: 0;
  width: 100%;
}

.channel-cell,
.status-cell,
.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.channel-cell strong {
  color: hsl(var(--foreground));
  font-size: 13px;
}

.channel-cell code,
.channel-cell span {
  color: hsl(var(--muted-foreground));
  font-family: ui-monospace, monospace;
  font-size: 10px;
}

.status-cell span,
.progress-cell span {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}

.progress-cell strong {
  color: rgb(8 145 178);
  font-family: ui-monospace, monospace;
  font-size: 18px;
}

.failure-code {
  background: rgb(239 68 68 / 8%);
  border: 1px solid rgb(239 68 68 / 20%);
  border-radius: 0;
  color: rgb(220 38 38);
  font-size: 10px;
  padding: 4px 7px;
}

.result-ok {
  color: rgb(22 163 74);
  font-size: 12px;
}
</style>
