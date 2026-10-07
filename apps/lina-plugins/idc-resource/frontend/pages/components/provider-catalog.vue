<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { ProviderCatalogPrice, ProviderCatalogState } from "../provider-catalog-state";

import { nextTick, onBeforeUnmount, onMounted, shallowRef, watch } from "vue";
import { preferences } from "@vben/preferences";
import { Alert, Button, Spin } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { getProviderCatalogProduct, getProviderCatalogProducts, providerCatalogImportClient } from "../provider-catalog-client";
import { createCatalogImporter } from "../provider-catalog-import";
import { createProviderCatalogState, providerCatalogPriceRows } from "../provider-catalog-state";

const props = defineProps<{ interfaceNo: string }>();
const tr = (key: string) => $t(`plugin.idc-resource.providerCatalog.${key}`);
const state = shallowRef<ProviderCatalogState>();
const importRoot = shallowRef<HTMLElement>();
const importError = shallowRef("");
const importLoading = shallowRef(false);
let importer: ReturnType<typeof createCatalogImporter> | undefined;
async function importProduct() {
  if (importLoading.value || !state.value?.detail || !importRoot.value) return;
  importer?.destroy();
  importer = createCatalogImporter(importRoot.value, { client: providerCatalogImportClient, interfaceNo: () => props.interfaceNo, locale: () => preferences.app.locale, tr, onError: value => { importError.value = value; } });
  importLoading.value = true;
  try { await importer.open(state.value.detail); } finally { importLoading.value = false; }
}
const catalog = createProviderCatalogState((value) => { state.value = value; });
state.value = catalog.current;

function searchSchema() {
  return [{ component: "Input", fieldName: "keyword", label: tr("search"), componentProps: { allowClear: true, maxlength: 128, "data-testid": "provider-catalog-search" } }];
}
function productColumns(): VxeGridProps["columns"] {
  return [
    { field: "name", title: tr("name"), minWidth: 180 },
    { field: "description", title: tr("description"), minWidth: 240 },
    { field: "action", title: tr("actions"), width: 110, fixed: "right", slots: { default: "action" } },
  ];
}
function priceColumns(options = false): VxeGridProps["columns"] {
  return [
    ...(options ? [
      { field: "groupName", title: tr("configuration"), minWidth: 150 },
      { field: "choiceName", title: tr("choice"), minWidth: 150 },
    ] : []),
    { field: "currencyCode", title: tr("currency"), width: 100 },
    { field: "period", title: tr("period"), minWidth: 150, formatter: ({ row }: { row: ProviderCatalogPrice }) => row.period === "once" ? tr("periods.once") : `${row.periodCount} ${tr(`periods.${row.period}`)}` },
    { field: "amount", title: tr(options ? "surcharge" : "basePrice"), minWidth: 160 },
    { field: "setupFee", title: tr("setupFee"), minWidth: 150 },
  ];
}
const [ProductGrid, productGridApi] = useVbenVxeGrid({
  formOptions: { commonConfig: { labelWidth: 90 }, schema: [], wrapperClass: "grid-cols-1" },
  gridOptions: {
    columns: [], height: 360, id: "idc-resource-upstream-products", rowConfig: { keyField: "reference" },
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 20, total: 0, pageSizes: [10, 20, 50, 100] },
    toolbarConfig: { refresh: true },
    proxyConfig: { autoLoad: false, ajax: {
      query: async ({ page }: { page: { currentPage: number; pageSize: number } }, values: Record<string, unknown> = {}) => {
        const result = await catalog.query(props.interfaceNo, page, String(values.keyword ?? ""), getProviderCatalogProducts);
        productGridApi.setGridOptions({ pagerConfig: { currentPage: result.currentPage, pageSize: result.pageSize, total: result.total } });
        return { items: result.items, total: result.total };
      },
    } },
  },
});
const [BaseGrid, baseGridApi] = useVbenVxeGrid({ gridOptions: { columns: [], height: 210, rowConfig: { keyField: "rowKey" }, pagerConfig: { enabled: false } } });
const [OptionGrid, optionGridApi] = useVbenVxeGrid({ gridOptions: { columns: [], height: 280, rowConfig: { keyField: "rowKey" }, pagerConfig: { enabled: false } } });

function updateLabels() {
  productGridApi.setState({ formOptions: { schema: searchSchema() } });
  productGridApi.setGridOptions({ columns: productColumns(), emptyText: tr("empty") });
  baseGridApi.setGridOptions({ columns: priceColumns(), emptyText: tr("noPrices") });
  optionGridApi.setGridOptions({ columns: priceColumns(true), emptyText: tr("noSurcharges") });
}
watch(() => state.value?.detail, async (detail) => {
  await nextTick();
  const rows = providerCatalogPriceRows(detail);
  baseGridApi.setGridOptions({ data: rows.base });
  optionGridApi.setGridOptions({ data: rows.options });
});
watch(() => preferences.app.locale, updateLabels);
watch(() => props.interfaceNo, async () => {
  importer?.destroy();
  catalog.reset(props.interfaceNo);
  await productGridApi.formApi.resetForm();
  await productGridApi.reload();
});
onMounted(async () => { updateLabels(); await productGridApi.query(); });
onBeforeUnmount(() => { importer?.destroy(); catalog.reset(); });
</script>

<template>
  <div data-testid="provider-catalog">
    <Alert class="mb-3" type="info" show-icon :message="tr('readOnlyHint')" />
    <Alert v-if="state?.listFailed" class="mb-3" type="error" show-icon :message="tr('loadFailed')" />
    <ProductGrid>
      <template #toolbar-actions><Button type="primary" :loading="importLoading" :disabled="!state?.detail" data-action="provider-catalog-import" @click="importProduct">{{ tr('importTitle') }}</Button></template>
      <template #action="{ row }"><Button size="small" type="link" @click="catalog.select(row.reference, getProviderCatalogProduct)">{{ tr('viewPrices') }}</Button></template>
    </ProductGrid>
    <Spin :spinning="state?.detailLoading ?? false">
      <Alert v-if="state?.detailFailed" class="mt-3" type="error" show-icon :message="tr('detailFailed')" />
      <div v-if="state?.detail" class="mt-4" data-testid="provider-catalog-prices">
        <Alert v-if="importError" class="mb-3" type="error" :message="importError" show-icon />
        <h3 class="mb-2 font-medium">{{ state.detail.product.name }}</h3>
        <p v-if="state.detail.product.description" class="mb-3 whitespace-pre-wrap">{{ state.detail.product.description }}</p>
        <Alert class="mb-3" type="info" show-icon :message="tr('basePriceHint')" />
        <Alert v-if="!state.detail.priceScheduleComplete" class="mb-3" type="warning" show-icon :message="tr('incompleteHint')" />
        <h4 class="mb-2 font-medium">{{ tr('basePrices') }}</h4>
        <BaseGrid />
        <h4 class="mb-2 mt-4 font-medium">{{ tr('optionPrices') }}</h4>
        <OptionGrid />
      </div>
    </Spin>
    <div ref="importRoot" />
  </div>
</template>
