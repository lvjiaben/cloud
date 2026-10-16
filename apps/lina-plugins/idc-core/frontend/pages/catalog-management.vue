<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog",
  title: "Catalog Management",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
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
import { Alert, Badge, Button, Empty, message, Popconfirm, Select, Space, Tree } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import { useAccess } from "./plugin-access";
import { createNumberedPage } from "./numbered-page";
import { catalogCategoryDelete, catalogCategoryList, catalogCategoryRestore, catalogProductList } from "./catalog-client";
import CatalogCategoryCreateModal from "./components/catalog-category-create-modal.vue";
import CatalogCategoryEditModal from "./components/catalog-category-edit-modal.vue";
import CatalogPricesDrawer from "./components/catalog-prices-drawer.vue";
import CatalogProductCopyModal from "./components/catalog-product-copy-modal.vue";
import CatalogProductCreateModal from "./components/catalog-product-create-modal.vue";
import CatalogProductDetailDrawer from "./components/catalog-product-detail-drawer.vue";
import CatalogProductPublicationPreviewModal from "./components/catalog-product-publication-preview-modal.vue";

const pagination = createNumberedPage<CatalogProductSummary>(
  ({ currentPage, pageSize, total }) => gridApi.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }),
  () => { detailDrawerApi.close(); publishModalApi.close(); createModalApi.close(); copyModalApi.close(); },
);
const categories = ref<CatalogCategory[]>([]);
const categoryState = ref<"active" | "deleted">("active");
const selectedCategory = ref("");
const categoryLoading = ref(false);
const categoryError = ref(false);
const categoryMutation = ref(false);
let categoryRequest = 0;
const { hasAccessByCodes } = useAccess();
const canViewCategories = computed(() => hasAccessByCodes(["idc:admin:catalog:category:list"]));
const canCreateCategory = computed(() => canViewCategories.value && hasAccessByCodes(["idc:admin:catalog:category:create"]));
const canUpdateCategory = computed(() => hasAccessByCodes(["idc:admin:catalog:category:update"]));
const canDeleteCategory = computed(() => hasAccessByCodes(["idc:admin:catalog:category:delete"]));
const canRestoreCategory = computed(() => hasAccessByCodes(["idc:admin:catalog:category:restore"]));
const canCreate = computed(() =>
  canViewCategories.value && hasAccessByCodes(["idc:admin:catalog:product:create"]),
);
const canCopy = computed(() => ["idc:admin:catalog:product:create", "idc:admin:catalog:product:query", "idc:admin:catalog:billing:write", "idc:admin:catalog:price:write", "idc:admin:catalog:tax:write", "idc:admin:catalog:fulfillment-binding:write", "idc:admin:catalog:configuration:write", "idc:admin:catalog:automation-interface:view", "idc:admin:sales-channel:pricing:manage"].every(code => hasAccessByCodes([code])));
const canManagePrices = computed(() => hasAccessByCodes(["idc:admin:sales-channel:pricing:manage"]));
const canEditProduct = computed(() => hasAccessByCodes(["idc:admin:catalog:product:update"]) || canManagePrices.value);
const canCustomPrices = computed(() => canManagePrices.value && hasAccessByCodes(["idc:admin:customer:query"]));
const canPublish = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:product:publish"]),
);

interface CategoryTreeNode {
  children?: CategoryTreeNode[];
  key: string;
  category: CatalogCategory;
  label: string;
  value: string;
}

const [CategoryCreateModal, categoryCreateApi] = useVbenModal({ connectedComponent: CatalogCategoryCreateModal });
const [CategoryEditModal, categoryEditApi] = useVbenModal({ connectedComponent: CatalogCategoryEditModal });
const selectedCategoryRow = computed(() => categories.value.find((row) => row.categoryNo === selectedCategory.value));

async function reloadCategories() {
  if (!canViewCategories.value) return;
  const sequence = ++categoryRequest;
  categoryLoading.value = true;
  categoryError.value = false;
  try {
    const result = await catalogCategoryList(categoryState.value);
    if (sequence !== categoryRequest) return;
    categories.value = result.categories;
    if (selectedCategory.value && !selectedCategoryRow.value) await selectCategory([]);
    gridApi.formApi.setState({ schema: searchSchema() });
  } catch {
    if (sequence !== categoryRequest) return;
    categoryError.value = true;
    categories.value = [];
    gridApi.formApi.setState({ schema: searchSchema() });
  } finally {
    if (sequence === categoryRequest) categoryLoading.value = false;
  }
}

async function selectCategory(keys: (string | number)[]) {
  selectedCategory.value = String(keys[0] ?? "");
  await gridApi.formApi.setValues({ categoryNo: selectedCategory.value });
  const values = await gridApi.formApi.getValues();
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

async function changeCategoryState() {
  await selectCategory([]);
  await reloadCategories();
}

function openCategoryCreate() {
  categoryCreateApi.setData({ parentCategoryNo: categoryState.value === "active" ? selectedCategory.value : undefined }).open();
}

function openCategoryEdit() {
  if (selectedCategoryRow.value) categoryEditApi.setData(selectedCategoryRow.value).open();
}

async function categorySaved() {
  await reloadCategories();
  await gridApi.query();
}

async function mutateCategory(action: "delete" | "restore") {
  const row = selectedCategoryRow.value;
  if (!row || categoryMutation.value) return;
  categoryMutation.value = true;
  try {
    if (action === "delete") await catalogCategoryDelete(row.categoryNo, row.version);
    else await catalogCategoryRestore(row.categoryNo, row.version);
    message.success($t(`plugin.idc-core.catalog.category.${action}Succeeded`));
    await reloadCategories();
    await gridApi.query();
  } finally { categoryMutation.value = false; }
}

function openCreateProduct() {
  createModalApi.setData({ categoryNo: categoryState.value === "active" ? selectedCategory.value : undefined }).open();
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
      key: category.categoryNo,
      category,
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
  selectedCategory.value = "";
  await gridApi.formApi.resetForm();
  const values = gridApi.formApi.form.values;
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

async function handleSearchSubmit(values: Record<string, unknown>) {
  selectedCategory.value = String(values.categoryNo ?? "");
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    ...(canViewCategories.value ? [{
      component: "TreeSelect",
      componentProps: {
        allowClear: true,
        "data-testid": "catalog-search-category",
        placeholder: $t("plugin.idc-core.catalog.search.categoryPlaceholder"),
        showSearch: true,
        treeData: categoryState.value === "active" ? categoryTree() : [],
        treeDefaultExpandAll: true,
        treeNodeFilterProp: "label",
      },
      fieldName: "categoryNo",
      label: $t("plugin.idc-core.catalog.search.category"),
    }] : []),
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
      width: 190,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 80 },
    schema: [],
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    wrapperClass: "grid-cols-1 md:grid-cols-2 2xl:grid-cols-3",
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    height: "auto",
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
const [PricesDrawer, pricesDrawerApi] = useVbenDrawer({ connectedComponent: CatalogPricesDrawer });
function openPrices() {
  pricesDrawerApi.setData({ target: "customer" });
  pricesDrawerApi.open();
}
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
onBeforeUnmount(() => { pagination.invalidate(); categoryRequest++; });

onMounted(async () => {

  gridApi.formApi.setState({
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    schema: searchSchema(),
  });
  gridApi.setGridOptions({ columns: columns() });
  await reloadCategories();
  await gridApi.query();
});
const [CopyModal, copyModalApi] = useVbenModal({ connectedComponent: CatalogProductCopyModal });
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" :auto-content-height="true" data-testid="catalog-management-page">
    <div class="catalog-workspace">
      <aside v-if="canViewCategories" class="catalog-category-panel" data-testid="catalog-category-panel">
        <div class="flex items-center justify-between gap-2">
          <strong>{{ $t('plugin.idc-core.catalog.workspace.categories') }}</strong>
          <Button v-if="canCreateCategory" size="small" data-testid="catalog-category-create-inline" :disabled="categoryLoading || categoryMutation" @click="openCategoryCreate">{{ $t('plugin.idc-core.catalog.actions.createCategory') }}</Button>
        </div>
        <p class="text-muted-foreground my-3 text-xs">{{ $t('plugin.idc-core.catalog.workspace.categoryHint') }}</p>
        <Select v-model:value="categoryState" class="mb-3 w-full" data-testid="catalog-category-state" :disabled="categoryLoading || categoryMutation" :options="['active', 'deleted'].map(value => ({ value, label: $t(`plugin.idc-core.catalog.category.${value}`) }))" @change="changeCategoryState" />
        <Button class="mb-2 w-full" data-testid="catalog-category-all" :type="selectedCategory ? 'default' : 'primary'" @click="selectCategory([])">{{ $t('plugin.idc-core.catalog.workspace.allProducts') }}</Button>
        <Alert v-if="categoryError" type="error" :message="$t('plugin.idc-core.catalog.workspace.categoryError')">
          <template #action><Button size="small" @click="reloadCategories">{{ $t('plugin.idc-core.catalog.workspace.retry') }}</Button></template>
        </Alert>
        <div v-else class="catalog-category-tree" :aria-busy="categoryLoading">
          <Tree v-if="categories.length" :key="categoryState" :tree-data="categoryTree()" :field-names="{ title: 'label' }" :selected-keys="selectedCategory ? [selectedCategory] : []" :default-expand-all="true" block-node data-testid="catalog-category-tree" @select="selectCategory">
            <template #title="{ category }"><span :data-testid="`catalog-category-select-${category.categoryNo}`">{{ localizedName(category) }}</span></template>
          </Tree>
          <Empty v-else-if="!categoryLoading" :image="Empty.PRESENTED_IMAGE_SIMPLE" :description="$t('plugin.idc-core.catalog.workspace.noCategories')" />
        </div>
        <Space v-if="selectedCategoryRow" class="mt-3 flex-wrap" :wrap="true">
          <Button v-if="!selectedCategoryRow.deleted && canUpdateCategory" size="small" data-testid="catalog-category-edit-inline" :disabled="categoryMutation" @click="openCategoryEdit">{{ $t('plugin.idc-core.catalog.category.edit') }}</Button>
          <Popconfirm v-if="!selectedCategoryRow.deleted && canDeleteCategory" :title="$t('plugin.idc-core.catalog.category.deleteConfirm', { name: localizedName(selectedCategoryRow) })" @confirm="mutateCategory('delete')">
            <Button size="small" danger data-testid="catalog-category-delete-inline" :disabled="categoryMutation">{{ $t('plugin.idc-core.catalog.category.delete') }}</Button>
          </Popconfirm>
          <Popconfirm v-if="selectedCategoryRow.deleted && canRestoreCategory" :title="$t('plugin.idc-core.catalog.category.restoreConfirm', { name: localizedName(selectedCategoryRow) })" @confirm="mutateCategory('restore')">
            <Button size="small" data-testid="catalog-category-restore-inline" :disabled="categoryMutation">{{ $t('plugin.idc-core.catalog.category.restore') }}</Button>
          </Popconfirm>
        </Space>
      </aside>
    <Grid class="catalog-product-grid" :table-title="$t('plugin.idc-core.salesManagement.productTableTitle')">
        <template #toolbar-tools>
          <Space>
            <Button v-if="canCustomPrices" data-testid="catalog-custom-prices-open" @click="openPrices()">{{ $t("plugin.idc-core.salesManagement.customPrices") }}</Button>
            <Button
              v-if="canCreate"
              data-testid="catalog-product-create-open"
              type="primary"
              @click="openCreateProduct"
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
            <ghost-button v-if="canCopy && row.versionNo" :data-testid="`catalog-product-copy-${row.productNo}`" @click="copyModalApi.setData(row).open()">{{ $t("plugin.idc-core.catalog.copy.action") }}</ghost-button>
            <ghost-button
              :data-testid="`catalog-product-detail-${row.productNo}`"
              @click="openDetail(row)"
              >{{ canEditProduct ? $t("plugin.idc-core.salesManagement.editProduct") : $t("plugin.idc-core.catalog.actions.detail") }}</ghost-button
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
    </div>
      <CategoryCreateModal @success="reloadCategories" />
      <CategoryEditModal @success="categorySaved" />
      <CreateModal @success="gridApi.query" />
      <CopyModal @success="gridApi.query" />
      <DetailDrawer />
      <PricesDrawer />
      <PublishModal @success="gridApi.query" />
  </Page>
</template>

<style scoped>
.catalog-workspace { display: flex; gap: 16px; min-height: 0; height: 100%; }
.catalog-category-panel { display: flex; flex-direction: column; width: 260px; flex-shrink: 0; padding: 16px; border-radius: 8px; background: hsl(var(--card)); border: 1px solid hsl(var(--border)); }
.catalog-category-tree { flex: 1; min-height: 100px; overflow: auto; }
.catalog-product-grid { flex: 1; min-width: 0; min-height: 0; }
@media (max-width: 900px) { .catalog-category-panel { width: 210px; padding: 12px; } }
@media (max-width: 640px) { .catalog-workspace { flex-direction: column; } .catalog-category-panel { width: 100%; max-height: 240px; } }
</style>
