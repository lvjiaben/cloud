<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog",
  title: "Catalog Management",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  CatalogCategory,
  CatalogProductListParams,
  CatalogProductSummary,
} from "./catalog-client";

import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Badge, Button, Card, Space } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import { useAccess } from "./plugin-access";
import { createNumberedPage } from "./numbered-page";
import { catalogCategoryList, catalogProductList } from "./catalog-client";
import CatalogProductCreateModal from "./components/catalog-product-create-modal.vue";
import CatalogProductDetailDrawer from "./components/catalog-product-detail-drawer.vue";
import CatalogProductPublicationPreviewModal from "./components/catalog-product-publication-preview-modal.vue";

const pagination = createNumberedPage<CatalogProductSummary>(
  ({ currentPage, pageSize, total }) => gridApi.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }),
  () => { detailDrawerApi.close(); publishModalApi.close(); createModalApi.close(); },
);
const categories = ref<CatalogCategory[]>([]);
const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:product:create"]),
);
const canPublish = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:product:publish"]),
);

interface CategoryTreeNode {
  children?: CategoryTreeNode[];
  label: string;
  value: string;
}

function localizedName(
  row: CatalogProductSummary | Pick<CatalogCategory, "nameEnUs" | "nameZhCn">,
) {
  return preferences.app.locale === "zh-CN"
    ? row.nameZhCn || row.nameEnUs
    : row.nameEnUs || row.nameZhCn;
}

function categoryTree() {
  const nodes = new Map<string, CategoryTreeNode>();
  for (const category of categories.value) {
    nodes.set(category.categoryNo, {
      children: [],
      label: localizedName(category),
      value: category.categoryNo,
    });
  }
  const roots: CategoryTreeNode[] = [];
  for (const category of categories.value) {
    const node = nodes.get(category.categoryNo)!;
    const parent = nodes.get(category.parentCategoryNo);
    if (parent) parent.children!.push(node);
    else roots.push(node);
  }
  return roots;
}

async function handleSearchReset() {
  await gridApi.formApi.resetForm();
  const values = gridApi.formApi.form.values;
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

async function handleSearchSubmit(values: Record<string, unknown>) {
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "TreeSelect",
      componentProps: {
        allowClear: true,
        "data-testid": "catalog-search-category",
        placeholder: $t("plugin.idc-core.catalog.search.categoryPlaceholder"),
        showSearch: true,
        treeData: categoryTree(),
        treeDefaultExpandAll: true,
        treeNodeFilterProp: "label",
      },
      fieldName: "categoryNo",
      label: $t("plugin.idc-core.catalog.search.category"),
    },
    {
      component: "Input",
      componentProps: {
        allowClear: true,
        "data-testid": "catalog-search-name",
        placeholder: $t("plugin.idc-core.catalog.search.namePlaceholder"),
      },
      fieldName: "namePrefix",
      label: $t("plugin.idc-core.catalog.search.name"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: ["draft", "active", "suspended", "retired"].map((value) => ({
          label: $t(`plugin.idc-core.catalog.status.${value}`),
          value,
        })),
      },
      fieldName: "status",
      label: $t("plugin.idc-core.catalog.search.status"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: ["manual", "copy", "migration"].map(
          (value) => ({
            label: $t(`plugin.idc-core.catalog.source.${value}`),
            value,
          }),
        ),
      },
      fieldName: "source",
      label: $t("plugin.idc-core.catalog.search.source"),
    },
  ];
}

function columns(): VxeGridProps<CatalogProductSummary>["columns"] {
  return [
    {
      field: "product",
      minWidth: 240,
      slots: { default: "product" },
      title: $t("plugin.idc-core.catalog.table.product"),
    },
    {
      align: "center",
      field: "status",
      headerAlign: "center",
      slots: { default: "status" },
      title: $t("plugin.idc-core.catalog.table.status"),
      width: 180,
    },
    {
      field: "pricing",
      slots: { default: "pricing" },
      title: $t("plugin.idc-core.catalog.table.pricing"),
      width: 220,
    },
    {
      field: "updatedAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-core.catalog.table.updatedAt"),
      width: 180,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: $t("plugin.idc-core.catalog.table.action"),
      width: 210,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-4",
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: { page: { currentPage: number; pageSize: number } }, values = {}) => pagination.query(page, async (currentPage, pageSize) => {
          const filters = values as CatalogProductListParams;
          const result = await catalogProductList({
            ...filters,
            page: currentPage,
            limit: pageSize,
          });
          return { items: result.products, total: result.total };
        }),
      },
    },
    rowConfig: { keyField: "productNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-catalog-products",
  },
});

watch(
  () => preferences.app.locale,
  () => {
    gridApi.formApi.setState({ schema: searchSchema() });
    gridApi.setGridOptions({ columns: columns() });
  },
);

const [CreateModal, createModalApi] = useVbenModal({
  connectedComponent: CatalogProductCreateModal,
});
const [DetailDrawer, detailDrawerApi] = useVbenDrawer({
  connectedComponent: CatalogProductDetailDrawer,
});
const [PublishModal, publishModalApi] = useVbenModal({
  connectedComponent: CatalogProductPublicationPreviewModal,
});

function openDetail(row: CatalogProductSummary) {
  detailDrawerApi.setData({ productNo: row.productNo });
  detailDrawerApi.open();
}
function openPublish(row: CatalogProductSummary) {
  publishModalApi.setData({ row });
  publishModalApi.open();
}
onBeforeUnmount(() => pagination.invalidate());

onMounted(async () => {
  const result = await catalogCategoryList("active");
  categories.value = result.categories;
  gridApi.formApi.setState({
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    schema: searchSchema(),
  });
  gridApi.setGridOptions({ columns: columns() });
  await gridApi.query();
});
</script>

<template>
  <Page :auto-content-height="true" data-testid="catalog-management-page">
    <Card :body-style="{ height: '100%' }" :bordered="false" class="h-full">
      <Grid class="h-full">
        <template #toolbar-tools>
          <Space>
            <Button
              v-if="canCreate"
              data-testid="catalog-product-create-open"
              type="primary"
              @click="createModalApi.open()"
            >
              {{ $t("plugin.idc-core.catalog.actions.createProduct") }}
            </Button>
          </Space>
        </template>
        <template #product="{ row }">
          <div
            class="flex flex-col gap-1"
            :data-testid="`catalog-product-${row.productNo}`"
          >
            <span class="font-medium">{{ localizedName(row) }}</span>
            <span class="text-muted-foreground"
              >v{{ row.versionNumber }} · {{ row.publicSlug }}</span
            >
          </div>
        </template>
        <template #status="{ row }">
          <div class="flex flex-col items-center gap-1 text-center">
            <Badge
              :text="$t(`plugin.idc-core.catalog.status.${row.status}`)"
              :status="
                row.status === 'active'
                  ? 'success'
                  : row.status === 'draft'
                    ? 'processing'
                    : 'default'
              "
            />
            <span class="text-muted-foreground">{{
              $t(`plugin.idc-core.catalog.source.${row.source}`)
            }}</span>
          </div>
        </template>
        <template #pricing="{ row }">{{
          $t("plugin.idc-core.catalog.table.pricingMeta", {
            currencies: row.currencyCount,
            rules: row.billingRuleCount,
          })
        }}</template>
        <template #action="{ row }">
          <Space>
            <ghost-button
              :data-testid="`catalog-product-detail-${row.productNo}`"
              @click="openDetail(row)"
              >{{ $t("plugin.idc-core.catalog.actions.detail") }}</ghost-button
            >
            <ghost-button
              v-if="canPublish && row.versionStatus === 'draft'"
              :data-testid="`catalog-product-publish-${row.productNo}`"
              @click="openPublish(row)"
              >{{ $t("plugin.idc-core.catalog.actions.publish") }}</ghost-button
            >
          </Space>
        </template>
      </Grid>
      <CreateModal @success="gridApi.query" />
      <DetailDrawer />
      <PublishModal @success="gridApi.query" />
    </Card>
  </Page>
</template>
