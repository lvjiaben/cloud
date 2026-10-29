<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/catalog",
  title: "Catalog Management",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import "./catalog-icons";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  CatalogCategory,
  CatalogProductCreateResult,
  CatalogProductListParams,
  CatalogProductSummary,
} from "./catalog-client";

import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import { preferences } from "@vben/preferences";
import { useUserStore } from "@vben/stores";
import { Alert, Badge, Button, Empty, message, Popconfirm, Dropdown, Input, Menu, MenuItem, Space, Tree } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { minorToMajorAmount } from "./catalog-money";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import { useAccess } from "./plugin-access";
import { createNumberedPage } from "./numbered-page";
import { catalogCategoryDelete, catalogProductDelete, catalogCategoryList, catalogProductList, catalogProductDetail, catalogProductSaleStateUpdate } from "./catalog-client";
import CatalogCategoryCreateModal from "./components/catalog-category-create-modal.vue";
import CatalogCategoryEditModal from "./components/catalog-category-edit-modal.vue";
import CatalogSortModal from "./components/catalog-sort-modal.vue";
import CatalogPricesDrawer from "./components/catalog-prices-drawer.vue";
import CatalogProductCopyModal from "./components/catalog-product-copy-modal.vue";
import CatalogProductCreateModal from "./components/catalog-product-create-modal.vue";
import CatalogProductEditorDrawer from "./components/catalog-product-editor-drawer.vue";

const productTotal = ref(0);
const refreshing = ref(false);
const categorySearch = ref("");
const pagination = createNumberedPage<CatalogProductSummary>(
  ({ currentPage, pageSize, total }) => { productTotal.value = total; gridApi.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }); },
  () => { closeEditor(); createModalApi.close(); copyModalApi.close(); },
);
const categories = ref<CatalogCategory[]>([]);
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
    const result = await catalogCategoryList("active");
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
  const values = await gridApi.formApi.getValues();
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}

function openCategoryCreate() {
  categoryCreateApi.setData({ parentCategoryNo: selectedCategory.value || undefined }).open();
}

function openCategoryEdit() {
  if (selectedCategoryRow.value) categoryEditApi.setData(selectedCategoryRow.value).open();
}

async function categorySaved() {
  await reloadCategories();
  await gridApi.query();
}

async function deleteCategory() {
  const row = selectedCategoryRow.value;
  if (!row || categoryMutation.value) return;
  categoryMutation.value = true;
  try {
    await catalogCategoryDelete(row.categoryNo, row.version);
    message.success($t("plugin.idc-core.catalog.category.deleteSucceeded"));
    await reloadCategories();
    await gridApi.query();
  } finally { categoryMutation.value = false; }
}

function openCreateProduct() {
  createModalApi.setData({ categoryNo: selectedCategory.value || undefined }).open();
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
  const query = categorySearch.value.trim().toLocaleLowerCase();
  if (!query) return roots;
  return roots.flatMap(root => {
    if (root.label.toLocaleLowerCase().includes(query)) return [root];
    const children = root.children?.filter(child => child.label.toLocaleLowerCase().includes(query));
    return children?.length ? [{...root, children}] : [];
  });
}

async function handleSearchReset() {
  selectedCategory.value = "";
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
    {component:"Input", componentProps:{allowClear:true,"data-testid":"catalog-search-name",placeholder:$t("plugin.idc-core.catalog.search.namePlaceholder")},fieldName:"namePrefix",label:$t("plugin.idc-core.catalog.search.name")},
    {component:"Select", componentProps:{allowClear:true,"data-testid":"catalog-search-status",placeholder:$t("plugin.idc-core.catalog.screen.saleStatus"),options:["", "active", "draft", "suspended", "retired"].map(value=>({value,label:$t(`plugin.idc-core.catalog.screen.filter.${value||"all"}`)}))},fieldName:"status",label:$t("plugin.idc-core.catalog.screen.saleStatus")},
  ];
}
const [SortModal, sortModalApi] = useVbenModal({ connectedComponent: CatalogSortModal });
const canDeleteProduct = computed(() => ["idc:admin:catalog:product:delete", "idc:admin:catalog:product:query"].every(code=>hasAccessByCodes([code])));
const canSortProducts = computed(() => ["idc:admin:catalog:product:update", "idc:admin:catalog:product:query"].every(code=>hasAccessByCodes([code])));
const tableTitle = computed(() => selectedCategoryRow.value ? localizedName(selectedCategoryRow.value) : $t("plugin.idc-core.catalog.workspace.allProducts"));
async function sorted() { await Promise.all([reloadCategories(), gridApi.reload()]); }
async function refreshWorkspace() {
 if (refreshing.value) return;
 refreshing.value = true;
 try { await Promise.all([reloadCategories(),gridApi.query()]); }
 finally { refreshing.value = false; }
}

function basePriceAmount(row:CatalogProductSummary) {
 const price=row.basePrice;
 return price ? `${price.currencyCode} ${minorToMajorAmount(price.amountMinor,price.currencyCode)}` : '';
}
function basePricePeriod(row:CatalogProductSummary) {
 const price=row.basePrice;
 if(!price) return '';
 if(price.periodUnit==='once') return $t('plugin.idc-core.catalog.detail.code.period.once');
 const period=`${price.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${price.periodUnit}`)}`;
 return price.chargeType==='trial' ? `${$t('plugin.idc-core.catalog.detail.code.charge.trial')} · ${period}` : period;
}

function columns(): VxeGridProps<CatalogProductSummary>["columns"] {
  return [
    {
      field: "product",
      align: "left",
      minWidth: 300,
      slots: { default: "product" },
      title: $t("plugin.idc-core.catalog.workspace.productColumn"),
    },
    { field: "sortOrder", title: $t("plugin.idc-core.catalog.create.sortOrder"), width: 80, align: "center" },
    {
      field: "category",
      title: $t("plugin.idc-core.catalog.screen.categoryColumn"),
      minWidth: 120,
      slots: { default: "category" },
    },
    {
      align: "left",
      field: "status",
      headerAlign: "center",
      slots: { default: "status" },
      title: $t("plugin.idc-core.catalog.workspace.statusColumn"),
      width: 100,
    },
    {
      field: "pricing",
      slots: { default: "pricing" },
      title: $t("plugin.idc-core.catalog.workspace.basePriceColumn"),
      width: 160,
    },
    {field: "availableQuantity", title: $t("plugin.idc-core.catalog.screen.stock"), width: 100, align: "right", slots: {default: "stock"}},
    {field: "provisionedQuantity", title: $t("plugin.idc-core.catalog.screen.provisioned"), width: 110, align: "right", slots: {default: "provisioned"}},
    {field: "totalQuantity", title: $t("plugin.idc-core.catalog.screen.totalDelivered"), width: 110, align: "right", slots: {default: "totalDelivered"}},
    {
      field: "updatedAt",
      visible: false,
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.idc-core.catalog.table.updatedAt"),
      width: 180,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: $t("plugin.idc-core.catalog.table.action"),
      width: 160,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, hideLabel: true },
    showCollapseButton: false,
    schema: [],
    handleReset: handleSearchReset,
    handleSubmit: handleSearchSubmit,
    wrapperClass: "grid-cols-1 md:grid-cols-3",
    actionWrapperClass: "col-span-1",
  },
  gridOptions: {
    columns: [],
    emptyRender: { name: "Empty" },
    showOverflow: false,
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: { page: { currentPage: number; pageSize: number } }, values = {}) => pagination.query(page, async (currentPage, pageSize) => {
          const filters = values as CatalogProductListParams;
          const result = await catalogProductList({
            ...filters,
            categoryNo:selectedCategory.value || undefined,
            page: currentPage,
            limit: pageSize,
          });
          return { items: result.products, total: result.total };
        }),
      },
    },
    rowConfig: { keyField: "productNo", height: 78 },
    toolbarConfig: { enabled: false, refresh: false, custom: false, zoom: false },
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
const route = useRoute();
const router = useRouter();
const currentUser = useUserStore();
// Only the open drawer identity is remembered in this tab, never form contents
// or authority. Ordinary opens do not create new host routes or reset the grid.
const editorMemoryKey = `idc-core:catalog-editor:${currentUser.userInfo?.userId ?? currentUser.userInfo?.username}:${currentUser.userInfo?.tenantId ?? 0}:${route.path}`;
function rememberedEditor(): {productNo?: string; section?: string} {
  try { const value = JSON.parse(sessionStorage.getItem(editorMemoryKey) || '{}'); return typeof value.productNo === 'string' && value.productNo.length <= 128 ? value : {}; }
  catch { return {}; }
}
const restoredEditor = rememberedEditor();
const editingProductNo = ref(typeof route.query.editProduct === 'string' ? route.query.editProduct : restoredEditor.productNo || '');
const editorInitialSection = ref(typeof route.query.editSection === 'string' ? route.query.editSection : restoredEditor.section || 'basic');
function showEditor(productNo: string, section='basic') {
  editorInitialSection.value = section;
  editingProductNo.value = productNo;
  try {sessionStorage.setItem(editorMemoryKey, JSON.stringify({productNo, section}));} catch { /* Storage restrictions do not prevent editing. */ }
}
function closeEditor() {
  editingProductNo.value = '';
  try {sessionStorage.removeItem(editorMemoryKey);} catch { /* There is no stored business or form data to clear. */ }
  if (route.query.editProduct) {const query={...route.query};delete query.editProduct;delete query.editSection;void router.replace({query});}
}
const [ProductEditorDrawer, productEditorDrawerApi] = useVbenDrawer({connectedComponent: CatalogProductEditorDrawer});
watch(editingProductNo, async productNo => {
  // Connected drawer methods become available after the child has mounted.
  // Restore URL state only after that mount, including the first page load.
  await nextTick();
  if (productNo) productEditorDrawerApi.setData({productNo, section: editorInitialSection.value}).open();
  else productEditorDrawerApi.close();
}, {immediate: true, flush: 'post'});

const [PricesDrawer, pricesDrawerApi] = useVbenDrawer({ connectedComponent: CatalogPricesDrawer });
function openPrices() {
  pricesDrawerApi.setData({ target: "customer" });
  pricesDrawerApi.open();
}
async function continueCreatedProduct(created: CatalogProductCreateResult) {
  await gridApi.query();
  if(hasAccessByCodes(["idc:admin:catalog:product:query"])) showEditor(created.productNo,"fulfillment");
}

const changingSaleState = ref('');
async function changeSaleState(row: CatalogProductSummary) {
 if (changingSaleState.value) return;
 changingSaleState.value = row.productNo;
 try {
  const detail = await catalogProductDetail(row.productNo);
  await catalogProductSaleStateUpdate(row.productNo, {versionNo: detail.product.currentVersionNo || detail.version.versionNo,
   expectedVersion: detail.product.descriptionVersion || 1, active: row.status === 'suspended'});
  message.success($t('plugin.idc-core.catalog.saleState.saved')); await gridApi.query();
 } finally {changingSaleState.value = '';}
}

const deletingProduct = ref('');
async function deleteProduct(row: CatalogProductSummary) {
 if (deletingProduct.value) return;
 deletingProduct.value = row.productNo;
 try {
  const detail = await catalogProductDetail(row.productNo);
  await catalogProductDelete(row.productNo, detail.product.descriptionVersion!);
  if (editingProductNo.value === row.productNo) closeEditor();
  message.success($t('plugin.idc-core.catalog.screen.productDeleted'));
  await gridApi.reload();
 } finally {deletingProduct.value = '';}
}

function openDetail(row: CatalogProductSummary) {
  showEditor(row.productNo);
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
    <ProductEditorDrawer @closed="closeEditor" @updated="gridApi.query" />
    <div class="catalog-page-shell">
    <div class="catalog-workspace">
      <aside v-if="canViewCategories" class="catalog-category-panel" data-testid="catalog-category-panel">
        <div class="flex items-center justify-between gap-2">
          <strong>{{ $t('plugin.idc-core.catalog.workspace.categories') }}</strong>
          <Button v-if="canCreateCategory" size="small" class="catalog-category-add" :aria-label="$t('plugin.idc-core.catalog.actions.createCategory')" :title="$t('plugin.idc-core.catalog.actions.createCategory')" data-testid="catalog-category-create-inline" :disabled="categoryLoading || categoryMutation" @click="openCategoryCreate"><IconifyIcon icon="catalog:plus" /></Button>
        </div>
        <Input v-model:value="categorySearch" allow-clear class="catalog-category-search" :placeholder="$t('plugin.idc-core.catalog.screen.categorySearch')" data-testid="catalog-category-search"><template #prefix><IconifyIcon icon="catalog:search" /></template></Input>
        <Button class="catalog-all-category" data-testid="catalog-category-all" :class="{ 'is-selected': !selectedCategory }" @click="selectCategory([])"><IconifyIcon icon="catalog:layers" />{{ $t('plugin.idc-core.catalog.workspace.allProducts') }}</Button>
        <Alert v-if="categoryError" type="error" :message="$t('plugin.idc-core.catalog.workspace.categoryError')">
          <template #action><Button size="small" @click="reloadCategories">{{ $t('plugin.idc-core.catalog.workspace.retry') }}</Button></template>
        </Alert>
        <div v-else class="catalog-category-tree" :aria-busy="categoryLoading">
          <Tree v-if="categories.length" :tree-data="categoryTree()" :field-names="{ title: 'label' }" :selected-keys="selectedCategory ? [selectedCategory] : []" :default-expand-all="true" :show-line="{ showLeafIcon: false }" block-node data-testid="catalog-category-tree" @select="selectCategory">
            <template #title="{ category }"><span class="catalog-category-node" :data-testid="`catalog-category-select-${category.categoryNo}`"><IconifyIcon icon="catalog:folder" /><span :title="localizedName(category)">{{ localizedName(category) }}</span></span></template>
          </Tree>
          <Empty v-else-if="!categoryLoading" :image="Empty.PRESENTED_IMAGE_SIMPLE" :description="$t('plugin.idc-core.catalog.workspace.noCategories')" />
        </div>
        <div v-if="selectedCategoryRow" class="catalog-category-footer">
          <span class="catalog-category-selected-name">{{ localizedName(selectedCategoryRow) }}</span>
          <Space :wrap="true">
            <Button v-if="canUpdateCategory" size="small" data-testid="catalog-category-edit-inline" :disabled="categoryMutation" @click="openCategoryEdit">{{ $t('plugin.idc-core.catalog.category.edit') }}</Button>
            <Button v-if="canUpdateCategory" size="small" data-testid="catalog-category-sort" @click="sortModalApi.setData({category:selectedCategoryRow}).open()">{{ $t('plugin.idc-core.catalog.screen.sortAction') }}</Button>
            <Popconfirm v-if="canDeleteCategory" :title="$t('plugin.idc-core.catalog.category.deleteConfirm', { name: localizedName(selectedCategoryRow) })" @confirm="deleteCategory">
              <Button size="small" danger data-testid="catalog-category-delete-inline" :disabled="categoryMutation">{{ $t('plugin.idc-core.catalog.category.delete') }}</Button>
            </Popconfirm>
          </Space>
        </div>
      </aside>
    <section class="catalog-products-panel">
      <header class="catalog-products-header">
        <div class="catalog-products-title"><h2>{{ tableTitle }}</h2><span data-testid="catalog-products-total">{{ $t('plugin.idc-core.catalog.screen.productCount', { count: productTotal }) }}</span></div>
        <Space>
          <Button v-if="canCustomPrices" data-testid="catalog-custom-prices-open" @click="openPrices()">{{ $t("plugin.idc-core.salesManagement.customPrices") }}</Button>
          <Button v-if="canCreate" type="primary" data-testid="catalog-product-create-open" @click="openCreateProduct"><IconifyIcon icon="catalog:plus" />{{ $t('plugin.idc-core.catalog.screen.new') }}</Button>
          <Button :aria-label="$t('plugin.idc-core.catalog.screen.refresh')" :title="$t('plugin.idc-core.catalog.screen.refresh')" :loading="refreshing" data-testid="catalog-refresh" @click="refreshWorkspace"><IconifyIcon v-if="!refreshing" icon="catalog:refresh" /></Button>
        </Space>
      </header>
      <Grid class="catalog-product-grid">
        <template #product="{ row }">
          <div
            class="catalog-product-identity"
            :data-testid="`catalog-product-${row.productNo}`"
          >
            <span class="catalog-product-icon"><IconifyIcon icon="catalog:server" /></span>
            <div class="catalog-product-copy"><span class="catalog-product-name" :title="localizedName(row)">{{ localizedName(row) }}</span><span class="catalog-product-reference text-muted-foreground" :title="row.publicSlug">{{ row.publicSlug }}</span></div>
          </div>
        </template>
        <template #category="{ row }"><span class="catalog-category-cell">{{ preferences.app.locale === 'zh-CN' ? row.categoryNameZhCn || row.categoryNameEnUs : row.categoryNameEnUs || row.categoryNameZhCn }}</span></template>
        <template #status="{ row }">
          <div class="flex flex-col items-center gap-1 text-center">
            <Badge
              :text="$t(`plugin.idc-core.catalog.status.${row.status}`)"
              :status="
                row.status === 'active'
                  ? 'success'
                  : 'default'
              "
            />
          </div>
        </template>
        <template #pricing="{ row }">
          <div v-if="row.basePrice" class="flex flex-col gap-1" :data-testid="`catalog-product-price-${row.productNo}`">
            <span class="font-medium tabular-nums">{{ basePriceAmount(row) }} <span class="text-muted-foreground text-xs">/ {{ basePricePeriod(row) }}</span></span>
            <span v-if="row.basePrice.setupFeeMinor !== '0'" class="text-muted-foreground text-xs">{{ $t('plugin.idc-core.catalog.configurationPrice.setupFee') }} {{ row.basePrice.currencyCode }} {{ minorToMajorAmount(row.basePrice.setupFeeMinor,row.basePrice.currencyCode) }}</span>
            <span v-else class="text-muted-foreground text-xs">{{ $t('plugin.idc-core.catalog.workspace.configurationExtra') }}</span>
          </div>
          <span v-else class="text-muted-foreground" :data-testid="`catalog-product-price-${row.productNo}`">{{ $t('plugin.idc-core.catalog.workspace.noBasePrice') }}</span>
        </template>
        <template #stock="{ row }"><span class="catalog-quantity" :class="{'is-empty': row.availableQuantity === 0}" :data-testid="`catalog-product-stock-${row.productNo}`">{{ row.availableQuantity === null ? $t('plugin.idc-core.catalog.screen.unlimited') : row.availableQuantity ?? '—' }}</span></template>
        <template #provisioned="{ row }"><span class="catalog-quantity" :data-testid="`catalog-product-provisioned-${row.productNo}`">{{ row.provisionedQuantity ?? '—' }}</span></template>
        <template #totalDelivered="{ row }"><span class="catalog-quantity text-muted-foreground" :data-testid="`catalog-product-total-${row.productNo}`">{{ row.totalQuantity ?? '—' }}</span></template>
        <template #action="{ row }">
          <Space :size="8" class="catalog-row-actions">
            <ghost-button :data-testid="`catalog-product-detail-${row.productNo}`" @click="openDetail(row)">{{ canEditProduct ? $t("plugin.idc-core.salesManagement.editProduct") : $t("plugin.idc-core.catalog.actions.detail") }}</ghost-button>
            <ghost-button v-if="canCopy && row.versionNo" :data-testid="`catalog-product-copy-${row.productNo}`" @click="copyModalApi.setData(row).open()">{{ $t("plugin.idc-core.catalog.copy.action") }}</ghost-button>
            <Dropdown v-if="canDeleteProduct || canSortProducts || (canPublish && ['active','suspended'].includes(row.status))" :trigger="['click']">
              <ghost-button :aria-label="$t('plugin.idc-core.catalog.screen.more')" :data-testid="`catalog-product-more-${row.productNo}`"><IconifyIcon icon="catalog:ellipsis" /></ghost-button>
              <template #overlay><Menu>
                <MenuItem v-if="canDeleteProduct" danger>
                  <Popconfirm :title="$t('plugin.idc-core.catalog.screen.deleteProductConfirm')" :description="$t('plugin.idc-core.catalog.screen.deleteProductHint')" @confirm="deleteProduct(row)">
                    <span :data-testid="`catalog-product-delete-${row.productNo}`">{{ $t('plugin.idc-core.catalog.screen.deleteProduct') }}</span>
                  </Popconfirm>
                </MenuItem>
                <MenuItem v-if="canSortProducts" @click="sortModalApi.setData({productNo:row.productNo}).open()"><span :data-testid="`catalog-product-sort-${row.productNo}`">{{ $t('plugin.idc-core.catalog.screen.sortAction') }}</span></MenuItem>
                <MenuItem v-if="canPublish && ['active','suspended'].includes(row.status)">
                  <Popconfirm :title="$t(row.status === 'active' ? 'plugin.idc-core.catalog.saleState.stopConfirm' : 'plugin.idc-core.catalog.saleState.resumeConfirm')" @confirm="changeSaleState(row)">
                    <span :data-testid="`catalog-product-sale-state-${row.productNo}`">{{ $t(row.status === 'active' ? 'plugin.idc-core.catalog.saleState.stop' : 'plugin.idc-core.catalog.saleState.resume') }}</span>
                  </Popconfirm>
                </MenuItem>
              </Menu></template>
            </Dropdown>
          </Space>
        </template>
      </Grid>
    </section>
    </div>
    </div>
      <CategoryCreateModal @success="reloadCategories" />
      <CategoryEditModal @success="categorySaved" />
      <CreateModal @success="continueCreatedProduct" />
      <CopyModal @success="gridApi.query" />
      <PricesDrawer />
      <SortModal @success="sorted" />
  </Page>
</template>

<style scoped>
.catalog-page-shell { display:flex; flex-direction:column; min-height:0; height:100%; }
.catalog-workspace { display:flex; flex:1; gap:12px; min-height:0; }
.catalog-category-panel,.catalog-products-panel { border:1px solid hsl(var(--border)); border-radius:8px; background:hsl(var(--card)); }
.catalog-category-panel { display:flex; flex-direction:column; width:248px; flex-shrink:0; padding:20px 14px 14px; }
.catalog-category-panel > div:first-child { padding:0 4px; margin-bottom:20px; }
.catalog-category-add { display:inline-flex; align-items:center; justify-content:center; width:28px; height:28px; padding:0; font-size:19px; }
.catalog-category-panel strong { font-size:16px; font-weight:600; }
.catalog-category-panel > div:first-child :deep(.ant-btn) { border:0; box-shadow:none; color:hsl(var(--primary)); }
.catalog-category-search { margin-bottom:16px; }
.catalog-category-search :deep(.ant-input-prefix) { margin-right:8px; color:hsl(var(--muted-foreground)); }
.catalog-all-category { display:flex; align-items:center; gap:10px; flex-shrink:0; width:100%; border:0; box-shadow:none; justify-content:flex-start; height:42px; margin-bottom:8px; font-weight:500; }
.catalog-all-category.is-selected { color:hsl(var(--primary)); background:hsl(var(--primary)/.08); }
.catalog-category-tree { flex:1; min-height:100px; overflow:auto; padding-top:4px; }
.catalog-category-tree :deep(.ant-tree) { background:transparent; }
.catalog-category-tree :deep(.ant-tree-treenode) { padding:3px 0; }
.catalog-category-tree :deep(.ant-tree-node-content-wrapper) { flex:1; min-width:0; overflow:hidden; line-height:36px; border-radius:5px; padding:0 8px; }
.catalog-category-tree :deep(.ant-tree-title) { display:block; min-width:0; }
.catalog-category-tree :deep(.ant-tree-indent-unit::before), .catalog-category-tree :deep(.ant-tree-switcher-leaf-line::before), .catalog-category-tree :deep(.ant-tree-switcher-leaf-line::after) { border-style:dashed; border-color:hsl(var(--border)); }
.catalog-category-tree :deep(.ant-tree-switcher) { line-height:36px; color:hsl(var(--muted-foreground)); }
.catalog-category-tree :deep(.ant-tree-node-selected) { background:hsl(var(--primary)/.08); color:hsl(var(--primary)); }
.catalog-category-node { display:flex; min-width:0; align-items:center; gap:9px; }
.catalog-category-node > span { display:block; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.catalog-category-node > svg { flex-shrink:0; color:hsl(var(--muted-foreground)); }
.catalog-category-footer { border-top:1px solid hsl(var(--border)); padding:14px 4px 0; margin-top:12px; }
.catalog-category-selected-name { display:block; font-size:12px; color:hsl(var(--muted-foreground)); margin-bottom:10px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.catalog-products-panel { display:flex; flex:1; flex-direction:column; min-width:0; min-height:0; overflow:hidden; }
.catalog-products-header { display:flex; flex-shrink:0; justify-content:space-between; align-items:center; gap:16px; padding:20px 20px 10px; }
.catalog-products-title { display:flex; align-items:center; gap:14px; min-width:0; }
.catalog-products-title h2 { font-size:16px; font-weight:600; margin:0; overflow-wrap:anywhere; }
.catalog-products-title > span { font-size:12px; color:hsl(var(--muted-foreground)); white-space:nowrap; }
.catalog-products-header :deep(.ant-btn) { display:inline-flex; align-items:center; gap:6px; }
.catalog-products-panel :deep(.catalog-product-grid) { height:0; flex:1; min-width:0; min-height:0; padding:0 12px 4px; }
.catalog-products-panel :deep(.vxe-table--header) { background:hsl(var(--muted)/.5); }
.catalog-product-identity { display:flex; align-items:center; gap:12px; min-width:0; text-align:left; }
.catalog-product-icon { display:flex; align-items:center; justify-content:center; flex:0 0 36px; width:36px; height:40px; border-radius:8px; background:hsl(var(--primary)/.06); color:hsl(var(--primary)); font-size:25px; }
.catalog-product-copy { display:flex; flex:1; min-width:0; flex-direction:column; align-items:stretch; gap:4px; }
.catalog-product-name { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-weight:500; }
.catalog-product-reference { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:12px; }
.catalog-category-cell { color:hsl(var(--muted-foreground)); }
.catalog-quantity { font-variant-numeric:tabular-nums; white-space:nowrap; }
.catalog-quantity.is-empty { color:hsl(var(--destructive)); }
.catalog-row-actions { white-space:nowrap; }
.catalog-row-actions :deep(.ant-btn) { border:0; background:transparent; padding:0 3px; box-shadow:none; }
@media (max-width:1100px) { .catalog-category-panel {width:210px;padding:16px 10px;} .catalog-products-header {padding:16px 14px 8px;} }
@media (max-width:700px) { .catalog-workspace {flex-direction:column;} .catalog-category-panel {width:100%;max-height:280px;} .catalog-products-header {align-items:flex-start;flex-wrap:wrap;} .catalog-products-title {flex:1 0 100%;} .catalog-products-header > :deep(.ant-space) {flex-wrap:wrap;} .catalog-category-panel > div:first-child {margin-bottom:10px;} .catalog-products-panel :deep(.catalog-product-grid) {padding:0 4px 4px;} }
</style>
