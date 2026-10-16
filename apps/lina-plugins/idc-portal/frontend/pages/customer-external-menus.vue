<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/customer-external-menus",
  title: "Customer Center Menus",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { CustomerExternalMenu } from "./external-menu-client";

import { computed, onMounted, ref, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Card, Space, Switch, Tag, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import {
  listCustomerExternalMenus,
  saveCustomerExternalMenu,
  setCustomerExternalMenuEnabled,
} from "./external-menu-client";

const editing = ref<CustomerExternalMenu>();
const tr = (key: string) => $t(`plugin.idc-portal.externalMenu.${key}`);

const iconOptions = computed(() =>
  [
    "lucide:external-link",
    "lucide:book-open",
    "lucide:life-buoy",
    "lucide:message-circle",
    "lucide:circle-help",
    "lucide:file-text",
    "lucide:globe-2",
    "lucide:shopping-bag",
    "lucide:link",
  ].map((value) => ({ label: tr(`icons.${value.replace("lucide:", "")}`), value })),
);

function columns(): VxeGridProps<CustomerExternalMenu>["columns"] {
  return [
    {
      field: "name",
      minWidth: 180,
      slots: { default: "name" },
      title: tr("columns.name"),
    },
    { field: "url", minWidth: 320, title: tr("columns.url") },
    {
      field: "icon",
      minWidth: 180,
      slots: { default: "icon" },
      title: tr("columns.icon"),
    },
    { field: "sortOrder", title: tr("columns.sortOrder"), width: 100 },
    {
      field: "enabled",
      slots: { default: "enabled" },
      title: tr("columns.status"),
      width: 110,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("columns.actions"),
      width: 150,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid<CustomerExternalMenu>({
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: false },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async () => {
          const result = await listCustomerExternalMenus();
          return { items: result.items || [], total: result.items?.length || 0 };
        },
      },
    },
    rowConfig: { keyField: "id" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-customer-external-menus",
  },
});

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
});

const [MenuModal, menuModalApi] = useVbenModal({
  onCancel: () => menuModalApi.close(),
  onConfirm: save,
  onOpenChange: initializeForm,
});

function formSchema() {
  return [
    { component: "Input", fieldName: "nameZh", label: tr("form.nameZh"), rules: z.string().trim().min(1).max(80) },
    { component: "Input", fieldName: "nameEn", label: tr("form.nameEn"), rules: z.string().trim().min(1).max(80) },
    { component: "Select", componentProps: { options: iconOptions.value }, fieldName: "icon", label: tr("form.icon"), rules: z.string().min(1) },
    { component: "Input", componentProps: { placeholder: "https://example.com/help" }, fieldName: "url", label: tr("form.url"), rules: z.string().url().max(2048) },
    { component: "InputNumber", componentProps: { max: 9999, min: 0 }, fieldName: "sortOrder", label: tr("form.sortOrder"), rules: z.number().min(0).max(9999) },
  ];
}

function displayName(item: CustomerExternalMenu) {
  return preferences.app.locale === "zh-CN" ? item.nameZh : item.nameEn;
}

function openEditor(item?: CustomerExternalMenu) {
  editing.value = item;
  menuModalApi.open();
}

async function initializeForm(open: boolean) {
  if (!open) return;
  formApi.setState({ schema: formSchema() });
  await formApi.setValues({
    icon: editing.value?.icon || "lucide:external-link",
    nameEn: editing.value?.nameEn || "",
    nameZh: editing.value?.nameZh || "",
    sortOrder: editing.value?.sortOrder ?? 100,
    url: editing.value?.url || "",
  });
  menuModalApi.setState({ title: editing.value ? tr("editTitle") : tr("createTitle") });
}

async function save() {
  const result = await formApi.validate();
  if (!result.valid) return;
  const values = await formApi.getValues();
  menuModalApi.setState({ confirmLoading: true });
  try {
    await saveCustomerExternalMenu({
      icon: String(values.icon),
      id: editing.value?.id,
      nameEn: String(values.nameEn),
      nameZh: String(values.nameZh),
      sortOrder: Number(values.sortOrder),
      url: String(values.url),
      version: editing.value?.version,
    });
    message.success(tr("saved"));
    menuModalApi.close();
    await gridApi.query();
  } finally {
    menuModalApi.setState({ confirmLoading: false });
  }
}

async function changeStatus(item: CustomerExternalMenu, enabled: boolean) {
  await setCustomerExternalMenuEnabled(item, enabled);
  message.success(enabled ? tr("enabled") : tr("disabled"));
  await gridApi.query();
}

watch(
  () => preferences.app.locale,
  () => gridApi.setGridOptions({ columns: columns() }),
);

onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" :auto-content-height="true">
    <div class="external-menu-layout">
      <Alert :message="tr('securityHint')" show-icon type="info" />
      <Card :body-style="{ height: '100%' }" :bordered="true" class="external-menu-card">
        <Grid class="h-full">
          <template #toolbar-tools>
            <Button type="primary" @click="openEditor()">{{ tr("create") }}</Button>
          </template>
          <template #name="{ row }">
            <strong>{{ displayName(row) }}</strong>
            <div class="secondary-name">{{ preferences.app.locale === "zh-CN" ? row.nameEn : row.nameZh }}</div>
          </template>
          <template #icon="{ row }"><Tag>{{ row.icon }}</Tag></template>
          <template #enabled="{ row }">
            <Switch :checked="row.enabled" @change="(checked) => changeStatus(row, Boolean(checked))" />
          </template>
          <template #action="{ row }">
            <Space><ghost-button @click="openEditor(row)">{{ tr("edit") }}</ghost-button></Space>
          </template>
        </Grid>
      </Card>
    </div>
    <MenuModal><Form /></MenuModal>
  </Page>
</template>

<style scoped>
.external-menu-layout { display: grid; height: 100%; min-height: 0; grid-template-rows: auto minmax(0, 1fr); gap: 12px; }
.external-menu-card { min-height: 0; }
.secondary-name { margin-top: 2px; color: hsl(var(--muted-foreground)); font-size: 12px; font-weight: 400; }
</style>
