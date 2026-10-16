<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog/automation-interfaces",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  AutomationInterface,
  AutomationInterfaceListParams,
} from "./catalog-automation-client";

import { computed, onMounted, ref, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Badge,
  Button,
  Card,
  message,
  Popconfirm,
  Result,
  Space,
  Tooltip,
} from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import { useAccess } from "./plugin-access";
import {
  automationInterfaceDelete,
  automationInterfaceList,
  automationInterfaceSetNewBusinessState,
} from "./catalog-automation-client";
import CatalogAutomationInterfaceCreateModal from "./components/catalog-automation-interface-create-modal.vue";
import CatalogAutomationInterfaceRenameModal from "./components/catalog-automation-interface-rename-modal.vue";
import CatalogConfigurationEditButton from "./components/catalog-configuration-edit-button.vue";

const actionInFlight = ref("");
const { hasAccessByCodes } = useAccess();
const canView = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:automation-interface:view"]),
);
const canManage = computed(
  () =>
    canView.value &&
    hasAccessByCodes(["idc:admin:catalog:automation-interface:manage"]),
);

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      componentProps: {
        allowClear: true,
        "data-testid": "automation-interface-search-name",
        maxlength: 128,
        placeholder: $t(
          "plugin.idc-core.catalogAutomation.search.namePlaceholder",
        ),
      },
      fieldName: "namePrefix",
      label: $t("plugin.idc-core.catalogAutomation.search.name"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        "data-testid": "automation-interface-search-state",
        options: ["accepting", "stopped"].map((value) => ({
          label: $t(`plugin.idc-core.catalogAutomation.state.${value}`),
          value,
        })),
      },
      fieldName: "newBusinessState",
      label: $t("plugin.idc-core.catalogAutomation.search.state"),
    },
  ];
}

function columns(): VxeGridProps<AutomationInterface>["columns"] {
  const result: VxeGridProps<AutomationInterface>["columns"] = [
    {
      field: "displayName",
      minWidth: 260,
      title: $t("plugin.idc-core.catalogAutomation.table.name"),
    },
    {
      align: "center",
      field: "newBusinessState",
      headerAlign: "center",
      slots: { default: "state" },
      title: $t("plugin.idc-core.catalogAutomation.table.state"),
      width: 170,
    },
    {
      field: "createdAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-core.catalogAutomation.table.createdAt"),
      width: 180,
    },
    {
      field: "changedAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-core.catalogAutomation.table.changedAt"),
      width: 180,
    },
  ];
  if (canManage.value) {
    result.push(
      fixedRightActionColumn({
        field: "action",
        slots: { default: "action" },
        title: $t("plugin.idc-core.catalogAutomation.table.action"),
        width: 360,
      }),
    );
  }
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid<AutomationInterface>({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
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
    pagerConfig: {},
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async (
          { page }: { page: { currentPage: number; pageSize: number } },
          values = {},
        ) => {
          const filters = values as AutomationInterfaceListParams;
          const result = await automationInterfaceList({
            namePrefix: filters.namePrefix?.trim() || undefined,
            newBusinessState: filters.newBusinessState || undefined,
            page: page.currentPage,
            pageSize: page.pageSize,
          });
          return { items: result.items, total: result.total };
        },
      },
    },
    rowConfig: { keyField: "interfaceInstanceNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-catalog-automation-interfaces",
  },
});

const [CreateModal, createModalApi] = useVbenModal({
  connectedComponent: CatalogAutomationInterfaceCreateModal,
});
const [RenameModal, renameModalApi] = useVbenModal({
  connectedComponent: CatalogAutomationInterfaceRenameModal,
});

watch(
  () => preferences.app.locale,
  async () => {
    const values = await gridApi.formApi.getValues();
    gridApi.formApi.setState({ schema: searchSchema() });
    await gridApi.formApi.setValues(values);
    gridApi.setGridOptions({ columns: columns() });
  },
);

function openRename(row: AutomationInterface) {
  renameModalApi.setData(row);
  renameModalApi.open();
}

async function runAction(
  key: string,
  action: () => Promise<unknown>,
  successKey: string,
) {
  if (actionInFlight.value) return;
  actionInFlight.value = key;
  try {
    await action();
    message.success($t(successKey));
    await gridApi.query();
  } catch {
    // The request layer has already shown the localized business error.
    // Resolve the action here so Ant Design closes the confirmation overlay.
  } finally {
    actionInFlight.value = "";
  }
}

function stopNewBusiness(row: AutomationInterface) {
  return runAction(
    `stop:${row.interfaceInstanceNo}`,
    () =>
      automationInterfaceSetNewBusinessState(row.interfaceInstanceNo, {
        expectedVersion: row.version,
        newBusinessState: "stopped",
      }),
    "plugin.idc-core.catalogAutomation.actions.stopSucceeded",
  );
}

function resumeNewBusiness(row: AutomationInterface) {
  return runAction(
    `resume:${row.interfaceInstanceNo}`,
    () =>
      automationInterfaceSetNewBusinessState(row.interfaceInstanceNo, {
        expectedVersion: row.version,
        newBusinessState: "accepting",
      }),
    "plugin.idc-core.catalogAutomation.actions.resumeSucceeded",
  );
}

function deleteInterface(row: AutomationInterface) {
  return runAction(
    `delete:${row.interfaceInstanceNo}`,
    () => automationInterfaceDelete(row.interfaceInstanceNo, row.version),
    "plugin.idc-core.catalogAutomation.actions.deleteSucceeded",
  );
}

onMounted(async () => {
  if (!canView.value) return;
  gridApi.formApi.setState({ schema: searchSchema() });
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page"
    :auto-content-height="true"
    data-testid="catalog-automation-interface-page"
  >
    <Card
      v-if="canView"
      :body-style="{ height: '100%' }"
      :bordered="false"
      class="h-full"
    >
      <Grid class="h-full">
        <template #toolbar-tools>
          <Space>
            <Button
              v-if="canManage"
              data-testid="automation-interface-create-open"
              type="primary"
              @click="createModalApi.open()"
            >
              {{ $t("plugin.idc-core.catalogAutomation.actions.create") }}
            </Button>
          </Space>
        </template>

        <template #state="{ row }">
          <Badge
            :status="
              row.newBusinessState === 'accepting' ? 'success' : 'warning'
            "
            :text="
              $t(
                `plugin.idc-core.catalogAutomation.state.${row.newBusinessState}`,
              )
            "
          />
        </template>

        <template #action="{ row }">
          <Space v-if="canManage" :size="4">
            <CatalogConfigurationEditButton
              :disabled="Boolean(actionInFlight)"
              :interface-instance-no="row.interfaceInstanceNo"
              @saved="gridApi.query"
            />
            <ghost-button
              :data-testid="`automation-interface-rename-${row.interfaceInstanceNo}`"
              :disabled="Boolean(actionInFlight)"
              @click="openRename(row)"
            >
              {{ $t("plugin.idc-core.catalogAutomation.actions.rename") }}
            </ghost-button>
            <Popconfirm
              v-if="row.newBusinessState === 'accepting'"
              :cancel-text="
                $t('plugin.idc-core.catalogAutomation.actions.cancel')
              "
              :ok-text="
                $t('plugin.idc-core.catalogAutomation.actions.confirmStop')
              "
              :title="
                $t('plugin.idc-core.catalogAutomation.actions.stopConfirm')
              "
              @confirm="stopNewBusiness(row)"
            >
              <ghost-button
                :data-testid="`automation-interface-stop-${row.interfaceInstanceNo}`"
                :disabled="Boolean(actionInFlight)"
              >
                {{ $t("plugin.idc-core.catalogAutomation.actions.stop") }}
              </ghost-button>
            </Popconfirm>
            <Popconfirm
              v-else
              :cancel-text="
                $t('plugin.idc-core.catalogAutomation.actions.cancel')
              "
              :ok-text="
                $t('plugin.idc-core.catalogAutomation.actions.confirmResume')
              "
              :title="
                $t('plugin.idc-core.catalogAutomation.actions.resumeConfirm')
              "
              @confirm="resumeNewBusiness(row)"
            >
              <ghost-button
                :data-testid="`automation-interface-resume-${row.interfaceInstanceNo}`"
                :disabled="Boolean(actionInFlight)"
              >
                {{ $t("plugin.idc-core.catalogAutomation.actions.resume") }}
              </ghost-button>
            </Popconfirm>
            <Popconfirm
              v-if="row.canDelete"
              :cancel-text="
                $t('plugin.idc-core.catalogAutomation.actions.cancel')
              "
              :ok-text="
                $t('plugin.idc-core.catalogAutomation.actions.confirmDelete')
              "
              :title="
                $t('plugin.idc-core.catalogAutomation.actions.deleteConfirm')
              "
              @confirm="deleteInterface(row)"
            >
              <ghost-button
                danger
                :data-testid="`automation-interface-delete-${row.interfaceInstanceNo}`"
                :disabled="Boolean(actionInFlight)"
              >
                {{ $t("plugin.idc-core.catalogAutomation.actions.delete") }}
              </ghost-button>
            </Popconfirm>
            <Tooltip
              v-else
              :title="
                $t(
                  'plugin.idc-core.catalogAutomation.actions.deleteUnavailable',
                )
              "
            >
              <span>
                <ghost-button
                  danger
                  :data-testid="`automation-interface-delete-${row.interfaceInstanceNo}`"
                  disabled
                >
                  {{ $t("plugin.idc-core.catalogAutomation.actions.delete") }}
                </ghost-button>
              </span>
            </Tooltip>
          </Space>
        </template>
      </Grid>

      <CreateModal @success="gridApi.query" />
      <RenameModal @success="gridApi.query" />
    </Card>
    <Card v-else :bordered="false">
      <Result
        data-testid="catalog-automation-interface-forbidden"
        status="403"
        :sub-title="$t('plugin.idc-core.catalogAutomation.access.deniedHelp')"
        :title="$t('plugin.idc-core.catalogAutomation.access.denied')"
      />
    </Card>
  </Page>
</template>
