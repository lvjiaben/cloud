<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/tasks",
  title: "Provider Tasks",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  ProviderTaskItem,
  ProviderTaskStatus,
} from "./provider-task-client";

import { computed, onMounted, ref, watch } from "vue";

import { useAccess } from "./plugin-access";
import { preferences } from "@vben/preferences";
import { Page, useVbenDrawer } from "@vben/common-ui";

import { Badge, Tag, message } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import ProviderTaskDetailDrawer from "./components/provider-task-detail-drawer.vue";
import { providerTaskList } from "./provider-task-client";
import { createResourcePage } from "./resource-page";

const queryPermission = "idc:admin:resource:task:query";
const taskStatuses: ProviderTaskStatus[] = [
  "pending",
  "submitting",
  "accepted",
  "running",
  "retry_wait",
  "result_unknown",
  "manual_takeover",
  "succeeded",
  "failed",
  "canceled",
];
const { hasAccessByCodes } = useAccess();
const canQuery = computed(() => hasAccessByCodes([queryPermission]));

const tasks = ref<ProviderTaskItem[]>([]);
const directory = createResourcePage<ProviderTaskItem>({
  onPage: (page) => gridApi.setGridOptions({ pagerConfig: { currentPage: page.currentPage, pageSize: page.pageSize, total: page.total } }),
  onDenied: () => {
    tasks.value = [];
    taskDetailDrawerApi.setData({ taskNo: undefined });
    void taskDetailDrawerApi.close();
  },
  onFailure: () => message.error($t("plugin.idc-resource.pagination.loadFailed")),
});

const channelOptions = computed(() =>
  [
    ...new Map(
      tasks.value.map((item) => [item.channelNo, item.channelName]),
    ).entries(),
  ]
    .filter(([, name]) => Boolean(name))
    .map(([value, label]) => ({ label, value })),
);

const metrics = computed(() => ({
  active: tasks.value.filter((item) =>
    ["accepted", "pending", "retry_wait", "running", "submitting"].includes(
      item.status,
    ),
  ).length,
  manual: tasks.value.filter((item) => item.status === "manual_takeover")
    .length,
  terminal: tasks.value.filter((item) =>
    ["canceled", "failed", "succeeded"].includes(item.status),
  ).length,
  unknown: tasks.value.filter((item) => item.status === "result_unknown")
    .length,
}));

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: () => ({
        allowClear: true,
        "data-testid": "provider-task-search-channel",
        options: channelOptions.value,
        placeholder: $t("plugin.idc-resource.tasks.search.channelPlaceholder"),
      }),
      fieldName: "channelNo",
      label: $t("plugin.idc-resource.tasks.search.channel"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "provider-task-search-status",
        options: [
          {
            label: $t("plugin.idc-resource.tasks.search.all"),
            value: "",
          },
          ...taskStatuses.map((value) => ({
            label: $t(`plugin.idc-resource.tasks.status.${value}`),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "status",
      label: $t("plugin.idc-resource.tasks.search.status"),
    },
  ];
}

function columns(): VxeGridProps["columns"] {
  const result: VxeGridProps["columns"] = [
    {
      field: "taskNo",
      minWidth: 240,
      slots: { default: "task" },
      title: $t("plugin.idc-resource.tasks.table.task"),
    },
    {
      field: "status",
      minWidth: 250,
      slots: { default: "status" },
      title: $t("plugin.idc-resource.tasks.table.status"),
    },
    {
      field: "channelName",
      minWidth: 185,
      slots: { default: "channel" },
      title: $t("plugin.idc-resource.tasks.table.channel"),
    },
    {
      field: "attemptCount",
      slots: { default: "attempts" },
      title: $t("plugin.idc-resource.tasks.table.attempts"),
      width: 118,
    },
    {
      field: "observationAttemptCount",
      title: $t("plugin.idc-resource.tasks.table.observations"),
      width: 128,
    },
    {
      field: "updatedAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      minWidth: 176,
      title: $t("plugin.idc-resource.tasks.table.updatedAt"),
    },
  ];
  if (canQuery.value) {
    result.push({
      field: "action",
      fixed: "right",
      slots: { default: "action" },
      title: $t("plugin.idc-resource.tasks.table.action"),
      width: 108,
    });
  }
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 86 },
    schema: [],
    wrapperClass: "plugin-admin-search",
    layout: "inline",
    actionLayout: "inline",
    actionPosition: "left",
    showCollapseButton: false,
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    id: "idc-resource-provider-tasks",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, pageSizes: [10, 20, 50, 100], total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: {page: {currentPage: number; pageSize: number}}, values: Record<string, unknown> = {}) => {
          const filters = {
            channelNo: String(values.channelNo || "").trim(),
            status: String(values.status || "") as ProviderTaskStatus | "",
          };
          const signature = JSON.stringify(filters);
          return directory.query(page, signature,
            (currentPage, limit) => providerTaskList({ ...filters, page: currentPage, limit }),
            (result) => { tasks.value = result.items; });
        },
      },
    },
    rowConfig: { keyField: "taskNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
  },
});

const [TaskDetailDrawerRef, taskDetailDrawerApi] = useVbenDrawer({
  connectedComponent: ProviderTaskDetailDrawer,
});

function statusBadge(status: ProviderTaskStatus) {
  if (status === "succeeded") return "success";
  if (["accepted", "running", "submitting"].includes(status))
    return "processing";
  if (status === "result_unknown" || status === "retry_wait") return "warning";
  if (status === "failed" || status === "canceled") return "error";
  return "default";
}

function taskStatusLabel(status: string) {
  const known = new Set(taskStatuses);
  return $t(
    `plugin.idc-resource.tasks.status.${known.has(status as ProviderTaskStatus) ? status : "unknown"}`,
  );
}

const capabilityMessageKeys: Record<string, string> = {
  "balance.query": "balanceQuery",
  "capability.discovery": "capabilityDiscovery",
  "cost.query": "costQuery",
  "provider-capacity.list": "providerCapacity",
  "server.backup": "serverBackup",
  "server.console": "serverConsole",
  "server.expiry.query": "serverExpiryQuery",
  "server.ip.change": "serverIPChange",
  "server.nat": "serverNAT",
  "server.password.reset": "serverPasswordReset",
  "server.provision": "serverProvision",
  "server.query": "serverQuery",
  "server.reboot": "serverReboot",
  "server.reinstall": "serverReinstall",
  "server.renew": "serverRenew",
  "server.rescue.enter": "serverRescueEnter",
  "server.rescue.exit": "serverRescueExit",
  "server.resize": "serverResize",
  "server.resume": "serverResume",
  "server.security-group": "serverSecurityGroup",
  "server.snapshot": "serverSnapshot",
  "server.start": "serverStart",
  "server.stop": "serverStop",
  "server.suspend": "serverSuspend",
  "server.terminate": "serverTerminate",
  "server.traffic-package": "serverTrafficPackage",
  "server.virtual-media": "serverVirtualMedia",
  "ticket.manage": "ticketManage",
};

function capabilityLabel(code: string) {
  return $t(
    `plugin.idc-resource.tasks.capability.${capabilityMessageKeys[code] || "unknown"}`,
  );
}

function openDetail(row: ProviderTaskItem) {
  taskDetailDrawerApi.setData({ taskNo: row.taskNo });
  taskDetailDrawerApi.open();
}

async function onTakeover() {
  await gridApi.query();
}

onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
  refreshLabels();
});
function refreshLabels() {
  gridApi.setGridOptions({ columns: columns() });
  gridApi.setState({ formOptions: { schema: searchSchema(), commonConfig: { labelWidth: preferences.app.locale === "en-US" ? 120 : 86 } } });
}
watch(() => $t("plugin.idc-resource.tasks.search.status"), refreshLabels);
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true" data-testid="provider-task-page">
    <div :ref="layoutRoot" class="task-shell">
      <section class="task-metrics">
        <article>
          <span>{{ $t("plugin.idc-resource.tasks.metrics.active") }}</span
          ><strong>{{ metrics.active }}</strong>
        </article>
        <article class="metric-warning">
          <span>{{ $t("plugin.idc-resource.tasks.metrics.unknown") }}</span
          ><strong>{{ metrics.unknown }}</strong>
        </article>
        <article class="metric-purple">
          <span>{{ $t("plugin.idc-resource.tasks.metrics.manual") }}</span
          ><strong>{{ metrics.manual }}</strong>
        </article>
        <article>
          <span>{{ $t("plugin.idc-resource.tasks.metrics.terminal") }}</span
          ><strong>{{ metrics.terminal }}</strong>
        </article>
      </section>

      <section class="task-grid">
        <Grid class="w-full min-w-0">
          <template #task="{ row }">
            <div class="task-cell">
              <strong>{{ capabilityLabel(row.capabilityCode) }}</strong>
            </div>
          </template>
          <template #status="{ row }">
            <div class="task-status" :data-testid="`provider-task-status-${row.taskNo}`">
              <Badge
                :status="statusBadge(row.status)"
                :text="taskStatusLabel(row.status)"
              />
              <Tag v-if="row.resultUnknown" color="orange">{{
                $t("plugin.idc-resource.tasks.hardFence")
              }}</Tag>
            </div>
          </template>
          <template #channel="{ row }">
            <div class="channel-cell">
              <strong>{{
                row.channelName ||
                $t("plugin.idc-resource.tasks.channelUnavailable")
              }}</strong>
            </div>
          </template>
          <template #attempts="{ row }">
            <span>{{ row.attemptCount }} / {{ row.maxAttempts }}</span>
          </template>
          <template #action="{ row }">
            <a-button
              size="small"
              type="link"
              :data-testid="`provider-task-detail-${row.taskNo}`"
              @click="openDetail(row)"
              >{{ $t("plugin.idc-resource.tasks.viewEvidence") }}</a-button
            >
          </template>
        </Grid>
      </section>
    </div>
    <TaskDetailDrawerRef @success="onTakeover" />
  </Page>
</template>

<style scoped>
.task-status { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; }
.task-status :deep(.ant-tag) { margin: 0; }
.task-shell {
  display: grid;
  gap: 12px;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}
.task-metrics {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.task-metrics article {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 12px 16px;
}
.task-metrics span {
  color: hsl(var(--muted-foreground));
  display: block;
  font-size: 12px;
  margin-bottom: 4px;
}
.task-metrics strong {
  font-size: 24px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.metric-warning strong {
  color: rgb(217 119 6);
}
.metric-purple strong {
  color: rgb(147 51 234);
}
.task-grid {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: block;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  padding: 4px;
}
.task-grid :deep(.vxe-grid) {
  flex: 1 1 0%;
  max-width: 100%;
  min-height: 0;
  width: 100%;
}
.task-cell,
.channel-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.task-pagination {
  align-items: center;
  border-top: 1px solid hsl(var(--border));
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding: 12px;
}
.task-pagination span {
  color: hsl(var(--muted-foreground));
  font:
    600 12px ui-monospace,
    monospace;
}
@media (max-width: 900px) {
  .task-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .task-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
