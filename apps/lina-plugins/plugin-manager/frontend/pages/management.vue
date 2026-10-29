<script lang="ts">
export const pluginPageMeta = {
  routePath: "/extension/plugin-manager",
  title: "插件管理",
};
</script>

<script setup lang="ts">
import { defineAsyncComponent, onMounted, ref } from "vue";
import { useAccess } from "@vben/access";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import {
  message,
  Popconfirm,
  Switch,
  Tabs,
  TabPane,
  Tag,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import { notifyPluginRegistryChanged } from "#/plugins/slot-registry";
import { formatTimestamp } from "#/utils/time";
import {
  business,
  categories,
  authorization,
  detail,
  enable,
  installAndStart,
  stopAndRemove,
  listPlugins,
  type Category,
  type PluginRow,
} from "./plugin-client";

const ActionDialog = defineAsyncComponent(
  () => import("./components/action-modal.vue"),
);
const [ActionModal, actionApi] = useVbenModal({
  connectedComponent: ActionDialog,
});
const [DeveloperGuide, developerGuideApi] = useVbenDrawer({
  connectedComponent: defineAsyncComponent(
    () => import("./components/developer-guide.vue"),
  ),
});
const { hasAccessByCodes } = useAccess();
const tabs = ref<Category[]>([]);
const activeTab = ref("");
const error = ref(false);
const changing = ref(new Set<string>());
// Presentation defaults do not restrict the configurable category catalog.
const categoryIcons: Record<string, string> = {
  payment: "lucide:credit-card",
  sms: "lucide:message-square",
  server: "lucide:server",
  cloud: "lucide:cloud",
  website: "lucide:globe",
  customer: "lucide:users-round",
  system: "lucide:settings",
};
const runtimeLabels: Record<string, string> = {
  normal: "正常",
  pending_upgrade: "待更新",
  upgrade_running: "更新中",
  upgrade_failed: "更新失败",
  abnormal: "异常",
};
const runtimeColors: Record<string, string> = {
  normal: "success",
  pending_upgrade: "processing",
  upgrade_running: "processing",
  upgrade_failed: "error",
  abnormal: "error",
};
const allowed = (code: string) => hasAccessByCodes([code]);
const canChangeStatus = (row: PluginRow) =>
  row.installed === 1 &&
  allowed(row.enabled === 1 ? "plugin:disable" : "plugin:enable");
const canChangeBusiness = (row: PluginRow) =>
  row.installed === 1 &&
  allowed(
    row.newBusinessState === "stopped"
      ? "plugin:resume-new-business"
      : "plugin:stop-new-business",
  );

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    schema: [
      { component: "Input", fieldName: "id", label: "标识" },
      { component: "Input", fieldName: "name", label: "名称" },
    ],
    commonConfig: { labelWidth: 48, componentProps: { allowClear: true } },
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    id: "plugin-manager-workspace",
    height: "100%",
    columns: [
      {
        field: "name",
        title: "名称 / 标识",
        minWidth: 240,
        align: "left",
        showOverflow: false,
        slots: { default: "identity" },
      },
      {
        field: "version",
        title: "版本号",
        width: 145,
        slots: { default: "version" },
      },
      {
        field: "enabled",
        title: "状态",
        width: 110,
        slots: { default: "status" },
      },
      {
        field: "newBusinessState",
        title: "业务",
        width: 125,
        slots: { default: "business" },
      },
      {
        field: "runtimeState",
        title: "运行时状态",
        width: 125,
        slots: { default: "runtime" },
      },
      {
        field: "installedAt",
        title: "安装时间",
        width: 175,
        formatter: ({ cellValue }: { cellValue: number | null }) =>
          formatTimestamp(cellValue),
      },
      {
        field: "action",
        title: "操作",
        width: "auto",
        fixed: "right",
        showOverflow: false,
        slots: { default: "action" },
      },
    ],
    rowConfig: { keyField: "id" },
    pagerConfig: { pageSize: 20 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async (
          { page }: { page: { currentPage: number; pageSize: number } },
          values = {},
        ) => {
          if (!activeTab.value) return { items: [], total: 0 };
          return listPlugins({
            ...values,
            category: activeTab.value,
            pageNum: page.currentPage,
            pageSize: page.pageSize,
          });
        },
      },
    },
  },
});

let reloadingCategory = false;
let reloadCategoryAgain = false;
async function reloadCategory() {
  reloadCategoryAgain = true;
  if (reloadingCategory) return;
  reloadingCategory = true;
  try {
    // Vxe ignores reloads while its proxy is loading. Coalesce rapid tab
    // changes and load the final selection after the current request settles.
    while (reloadCategoryAgain) {
      reloadCategoryAgain = false;
      await gridApi.reload();
    }
  } finally {
    reloadingCategory = false;
  }
}

async function loadCategories() {
  error.value = false;
  try {
    tabs.value = await categories();
    activeTab.value = tabs.value[0]?.id ?? "";
    await reloadCategory();
  } catch {
    error.value = true;
  }
}
function openAction(mode: "upgrade" | "upload", row?: PluginRow) {
  actionApi.setData({ mode, row });
  actionApi.open();
}
async function refresh() {
  try {
    await notifyPluginRegistryChanged();
  } finally {
    await gridApi.query();
  }
}
async function runAction(row: PluginRow, operation: "install" | "uninstall") {
  if (changing.value.has(row.id) || !allowed(`plugin:${operation}`)) return;
  changing.value.add(row.id);
  try {
    if (operation === "install") await installAndStart(row.id);
    else await stopAndRemove(row.id, "uninstall");
    message.success(
      operation === "install" ? "插件已安装并启用" : "插件已卸载",
    );
  } catch {
    // The public request adapter displays the server's actionable error.
  } finally {
    try {
      await refresh();
    } finally {
      changing.value.delete(row.id);
    }
  }
}
async function toggleStatus(row: PluginRow, checked: boolean) {
  if (changing.value.has(row.id) || !canChangeStatus(row)) return;
  changing.value.add(row.id);
  try {
    if (checked) {
      const current = await detail(row.id);
      await enable(row.id, authorization(current.requestedHostServices ?? []));
    } else await stopAndRemove(row.id, "disable");
    message.success(checked ? "插件已启用" : "插件已停用");
  } catch {
    // Refresh partial results as well, so retry always starts from real state.
  } finally {
    try {
      await refresh();
    } finally {
      changing.value.delete(row.id);
    }
  }
}
async function toggleBusiness(row: PluginRow) {
  if (changing.value.has(row.id) || !canChangeBusiness(row)) return;
  changing.value.add(row.id);
  try {
    await business(row.id, row.newBusinessState === "stopped");
    message.success("业务状态已更新");
  } finally {
    try {
      await gridApi.query();
    } finally {
      changing.value.delete(row.id);
    }
  }
}
const layoutRoot = useAdminGridLayout(gridApi);
onMounted(loadCategories);
</script>

<template>
  <Page
    :ref="layoutRoot"
    :auto-content-height="true"
    data-testid="plugin-manager-page"
  >
    <div class="plugin-manager-layout">
      <div class="plugin-manager-tabs">
        <Tabs v-model:active-key="activeTab" @change="reloadCategory">
          <TabPane v-for="tab in tabs" :key="tab.id">
            <template #tab>
              <span class="inline-flex items-center gap-2">
                <IconifyIcon
                  :icon="categoryIcons[tab.id] || 'lucide:puzzle'"
                  :data-testid="`plugin-category-icon-${tab.id}`"
                  :data-icon="categoryIcons[tab.id] || 'lucide:puzzle'"
                  aria-hidden="true"
                  class="size-4"
                />
                <span>{{ tab.name }}</span>
              </span>
            </template>
          </TabPane>
        </Tabs>
        <a-alert
          v-if="error"
          type="error"
          message="插件分类加载失败，请重试。"
          show-icon
        >
          <template #action
            ><a-button size="small" @click="loadCategories"
              >重试</a-button
            ></template
          >
        </a-alert>
      </div>
      <Grid table-title="插件管理" class="h-full min-h-0 min-w-0">
        <template #toolbar-tools>
          <a-button
            data-testid="plugin-developer-guide-trigger"
            @click="developerGuideApi.open()"
          >
            <IconifyIcon icon="lucide:book-open" class="mr-1 size-4" />开发文档
          </a-button>
          <a-button
            v-if="allowed('plugin:install')"
            type="primary"
            data-testid="plugin-package-install"
            @click="openAction('upload')"
            >安装插件</a-button
          >
        </template>
        <template #identity="{ row }">
          <div
            class="plugin-identity"
            :data-testid="`plugin-identity-${row.id}`"
          >
            <div class="font-medium">{{ row.name }}</div>
            <div class="text-muted-foreground text-xs">{{ row.id }}</div>
          </div>
        </template>
        <template #version="{ row }"
          ><span>{{ row.effectiveVersion || row.version }}</span></template
        >
        <template #status="{ row }">
          <Tag v-if="row.installed !== 1">未安装</Tag>
          <Switch
            v-else
            :checked="row.enabled === 1"
            :disabled="!canChangeStatus(row)"
            :loading="changing.has(row.id)"
            :title="row.enabled === 1 ? '停用时会先自动停止新销售' : '启用插件'"
            checked-children="启用"
            un-checked-children="停用"
            :data-testid="`plugin-status-${row.id}`"
            @change="(checked) => toggleStatus(row, Boolean(checked))"
          />
        </template>
        <template #business="{ row }">
          <span v-if="row.installed !== 1" class="text-muted-foreground"
            >—</span
          >
          <Popconfirm
            v-else
            :title="
              row.newBusinessState === 'stopped'
                ? '恢复该插件接受新业务？'
                : '停止新业务后，已有服务仍可继续使用。确认停止？'
            "
            :disabled="!canChangeBusiness(row) || changing.has(row.id)"
            @confirm="toggleBusiness(row)"
          >
            <a-button
              type="link"
              size="small"
              :disabled="!canChangeBusiness(row) || changing.has(row.id)"
              :data-testid="`plugin-business-${row.id}`"
              >{{
                row.newBusinessState === "stopped" ? "已停止" : "接收中"
              }}</a-button
            >
          </Popconfirm>
        </template>
        <template #runtime="{ row }">
          <Tag
            v-if="row.installed === 1"
            :color="runtimeColors[row.runtimeState] || 'default'"
            >{{ runtimeLabels[row.runtimeState] || "状态未知" }}</Tag
          >
          <span v-else class="text-muted-foreground">—</span>
        </template>
        <template #action="{ row }">
          <div
            :data-testid="`plugin-actions-${row.id}`"
            class="inline-flex items-center gap-2 whitespace-nowrap"
          >
            <Popconfirm
              v-if="row.installed !== 1 && allowed('plugin:install')"
              :title="`确认安装${row.name}？`"
              :disabled="changing.has(row.id)"
              @confirm="runAction(row, 'install')"
            >
              <ghost-button
                size="small"
                :disabled="changing.has(row.id)"
                :loading="changing.has(row.id)"
                :data-testid="`plugin-install-${row.id}`"
                >安装</ghost-button
              >
            </Popconfirm>
            <template v-if="row.installed === 1">
              <ghost-button
                v-if="allowed('plugin:install') && row.upgradeAvailable"
                size="small"
                :disabled="changing.has(row.id)"
                :data-testid="`plugin-upgrade-${row.id}`"
                @click="openAction('upgrade', row)"
                >更新</ghost-button
              >
              <Popconfirm
                v-if="allowed('plugin:uninstall')"
                :title="`确认卸载${row.name}？`"
                :disabled="
                  changing.has(row.id) || row.runtimeState === 'upgrade_running'
                "
                @confirm="runAction(row, 'uninstall')"
              >
                <ghost-button
                  size="small"
                  danger
                  :disabled="
                    changing.has(row.id) ||
                    row.runtimeState === 'upgrade_running'
                  "
                  :loading="changing.has(row.id)"
                  :data-testid="`plugin-uninstall-${row.id}`"
                  >卸载</ghost-button
                >
              </Popconfirm>
            </template>
          </div>
        </template>
      </Grid>
    </div>
    <ActionModal @reload="refresh" />
    <DeveloperGuide />
  </Page>
</template>

<style scoped>
.plugin-manager-layout {
  min-width: 0;
  grid-template-columns: minmax(0, 1fr);
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 12px;
}
.plugin-manager-tabs {
  padding: 0 16px;
  background: hsl(var(--card));
  border-radius: var(--radius);
}
.plugin-manager-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 0;
}
.plugin-identity {
  padding: 8px 0;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
</style>
