<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog/categories",
  title: "Catalog Categories",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { CatalogCategory } from "./catalog-client";

import { computed, onMounted, watch } from "vue";
import { useVbenModal, Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Button, Card, message, Popconfirm, Space } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import { useAccess } from "./plugin-access";
import {
  catalogCategoryDelete,
  catalogCategoryList,
  catalogCategoryRestore,
} from "./catalog-client";
import CatalogCategoryCreateModal from "./components/catalog-category-create-modal.vue";
import CatalogCategoryEditModal from "./components/catalog-category-edit-modal.vue";

const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:category:create"]),
);
const canUpdate = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:category:update"]),
);
const canDelete = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:category:delete"]),
);
const canRestore = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:category:restore"]),
);

const [CreateModal, createModalApi] = useVbenModal({
  connectedComponent: CatalogCategoryCreateModal,
});
const [EditModal, editModalApi] = useVbenModal({
  connectedComponent: CatalogCategoryEditModal,
});

function localizedName(category: CatalogCategory) {
  return preferences.app.locale === "zh-CN"
    ? category.nameZhCn || category.nameEnUs
    : category.nameEnUs || category.nameZhCn;
}

function searchSchema(): VbenFormProps["schema"] {
  return [
  {
    component: "Input",
    componentProps: {
      allowClear: true,
      placeholder: $t("plugin.idc-core.catalog.category.searchPlaceholder"),
    },
    fieldName: "name",
    label: $t("plugin.idc-core.catalog.category.name"),
  },
  {
    component: "Select",
    componentProps: {
      allowClear: false,
      options: [
        {
          label: $t("plugin.idc-core.catalog.category.active"),
          value: "active",
        },
        {
          label: $t("plugin.idc-core.catalog.category.deleted"),
          value: "deleted",
        },
      ],
    },
    defaultValue: "active",
    fieldName: "deletionState",
    label: $t("plugin.idc-core.catalog.table.status"),
  },
  ];
}

function columns(): VxeGridProps<CatalogCategory>["columns"] {
  return [
  {
    field: "name",
    minWidth: 220,
    slots: { default: "name" },
    title: $t("plugin.idc-core.catalog.category.name"),
    treeNode: true,
  },
  {
    field: "sortOrder",
    title: $t("plugin.idc-core.catalog.create.sortOrder"),
    width: 130,
  },
  fixedRightActionColumn({
    field: "action",
    slots: { default: "action" },
    title: $t("plugin.idc-core.catalog.category.actions"),
    width: 180,
  }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid<CatalogCategory>({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-2",
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: false },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async (_params, values = {}) => {
          const deletionState = String(values.deletionState || "active") as
            | "active"
            | "deleted";
          const keyword = String(values.name || "")
            .trim()
            .toLocaleLowerCase();
          const result = await catalogCategoryList(deletionState);
          const items = keyword
            ? result.categories.filter((item) =>
                `${item.nameZhCn} ${item.nameEnUs}`
                  .toLocaleLowerCase()
                  .includes(keyword),
              )
            : result.categories;
          return { items, total: items.length };
        },
      },
    },
    rowConfig: { keyField: "categoryNo" },
    treeConfig: {
      parentField: "parentCategoryNo",
      rowField: "categoryNo",
      transform: true,
    },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-catalog-categories",
  },
});

watch(
  () => preferences.app.locale,
  () => {
    gridApi.formApi.setState({ schema: searchSchema() });
    gridApi.setGridOptions({ columns: columns() });
  },
);

function openEdit(row: CatalogCategory) {
  editModalApi.setData(row);
  editModalApi.open();
}

async function deleteCategory(row: CatalogCategory) {
  await catalogCategoryDelete(row.categoryNo, row.version);
  message.success($t("plugin.idc-core.catalog.category.deleteSucceeded"));
  await gridApi.query();
}

async function restoreCategory(row: CatalogCategory) {
  await catalogCategoryRestore(row.categoryNo, row.version);
  message.success($t("plugin.idc-core.catalog.category.restoreSucceeded"));
  await gridApi.query();
}

onMounted(async () => {
  gridApi.formApi.setState({ schema: searchSchema() });
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
</script>

<template>
  <Page :auto-content-height="true" data-testid="catalog-category-page">
    <Card :body-style="{ height: '100%' }" :bordered="false" class="h-full">
      <Grid class="h-full">
        <template #toolbar-tools>
          <Button
            v-if="canCreate"
            data-testid="catalog-category-create-open"
            type="primary"
            @click="createModalApi.open()"
          >
            {{ $t("plugin.idc-core.catalog.actions.createCategory") }}
          </Button>
        </template>
        <template #name="{ row }">
          <span>{{ localizedName(row) }}</span>
        </template>
        <template #action="{ row }">
          <Space>
            <ghost-button
              v-if="!row.deleted && canUpdate"
              :data-testid="`catalog-category-edit-${row.categoryNo}`"
              @click="openEdit(row)"
            >
              {{ $t("plugin.idc-core.catalog.category.edit") }}
            </ghost-button>
            <Popconfirm
              v-if="!row.deleted && canDelete"
              :title="$t('plugin.idc-core.catalog.category.deleteConfirm', { name: localizedName(row) })"
              @confirm="deleteCategory(row)"
            >
              <ghost-button danger @click.stop="">
                {{ $t("plugin.idc-core.catalog.category.delete") }}
              </ghost-button>
            </Popconfirm>
            <Popconfirm
              v-if="row.deleted && canRestore"
              :title="$t('plugin.idc-core.catalog.category.restoreConfirm', { name: localizedName(row) })"
              @confirm="restoreCategory(row)"
            >
              <ghost-button @click.stop="">
                {{ $t("plugin.idc-core.catalog.category.restore") }}
              </ghost-button>
            </Popconfirm>
          </Space>
        </template>
      </Grid>
      <CreateModal @success="gridApi.query" />
      <EditModal @success="gridApi.query" />
    </Card>
  </Page>
</template>
