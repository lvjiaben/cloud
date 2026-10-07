<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, shallowRef, watch } from "vue";
import type { IsolatedTableColumn, IsolatedTableOptions } from "@linapro/vben-plugin-adapter/isolated";
import type { PluginHostMountContext } from "./host-context";
import type { ProviderCatalogState, ProviderCatalogProduct, ProviderCatalogPrice } from "../pages/provider-catalog-state";
import { createProviderCatalogState, providerCatalogPriceRows } from "../pages/provider-catalog-state";
import { createCatalogImporter } from "../pages/provider-catalog-import";
import { createAPI } from "./api";
import StandaloneTable from "./StandaloneTable.vue";

const props = defineProps<{ context: PluginHostMountContext; interfaceNo: string }>();
const api = createAPI(() => props.context);
const tr = (key: string) => props.context.t(`plugin.idc-resource.providerCatalog.${key}`, key);
const state = shallowRef<ProviderCatalogState>();
const catalog = createProviderCatalogState((value) => { state.value = value; });
state.value = catalog.current;
const rows = computed(() => providerCatalogPriceRows(state.value?.detail));
const searchKeyword = computed(() => state.value?.keyword ?? "");
const tableFailed = shallowRef(false);
const importRoot = shallowRef<HTMLElement>();
const importError = shallowRef("");
const importLoading = shallowRef(false);
let importer: ReturnType<typeof createCatalogImporter> | undefined;
async function importProduct() {
  if (importLoading.value || !state.value?.detail || !importRoot.value) return;
  importer?.destroy();
  importer = createCatalogImporter(importRoot.value, { client: api, interfaceNo: () => props.interfaceNo, locale: () => props.context.locale, tr, onError: value => { importError.value = value; } });
  importLoading.value = true;
  try { await importer.open(state.value.detail); } finally { importLoading.value = false; }
}
const importActions = computed(() => [{ key: "provider-catalog-import", label: tr("importTitle"), disabled: !state.value?.detail, loading: importLoading.value, onClick: importProduct }]);

async function query(page = { currentPage: 1, pageSize: state.value?.pageSize ?? 20 }, keyword = state.value?.keyword ?? "") {
  tableFailed.value = false;
  await catalog.query(props.interfaceNo, page, keyword, api.catalogProducts);
}
const pagination = computed(() => ({
  currentPage: state.value?.currentPage ?? 1, pageSize: state.value?.pageSize ?? 20, total: state.value?.total ?? 0,
  onChange: (page: { currentPage: number; pageSize: number }) => { void query(page); },
}));
const search = computed<IsolatedTableOptions<ProviderCatalogProduct & Record<string, unknown>>["search"]>(() => ({
  fields: [{ name: "keyword", label: tr("search"), component: "Input", props: { allowClear: true, maxlength: 128, "data-testid": "provider-catalog-search" } }],
  values: { keyword: searchKeyword.value },
  onSubmit: (values) => query({ currentPage: 1, pageSize: state.value?.pageSize ?? 20 }, String(values.keyword ?? "")),
}));
const columns = computed<IsolatedTableColumn<ProviderCatalogProduct & Record<string, unknown>>[]>(() => [
  { field: "name", title: tr("name"), minWidth: 180 },
  { field: "description", title: tr("description"), minWidth: 240 },
  { field: "action", title: tr("actions"), width: 120, fixed: "right", cell: (row) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "button";
    button.textContent = tr("viewPrices");
    button.onclick = () => { void catalog.select(row.reference, api.catalogProduct); };
    return button;
  } },
]);
function priceColumns(options = false): IsolatedTableColumn<ProviderCatalogPrice & Record<string, unknown>>[] {
  return [
    ...(options ? [
      { field: "groupName", title: tr("configuration"), minWidth: 150 },
      { field: "choiceName", title: tr("choice"), minWidth: 150 },
    ] : []),
    { field: "currencyCode", title: tr("currency"), width: 100 },
    { field: "period", title: tr("period"), minWidth: 150, cell: (row) => row.period === "once" ? tr("periods.once") : `${row.periodCount} ${tr(`periods.${row.period}`)}` },
    { field: "amount", title: tr(options ? "surcharge" : "basePrice"), minWidth: 160 },
    { field: "setupFee", title: tr("setupFee"), minWidth: 150 },
  ];
}
const baseColumns = computed(() => priceColumns());
const optionColumns = computed(() => priceColumns(true));
watch(() => props.interfaceNo, async () => { importer?.destroy(); catalog.reset(props.interfaceNo); await query(); });
onMounted(() => void query());
onBeforeUnmount(() => { importer?.destroy(); catalog.reset(); });
</script>

<template>
  <section data-testid="provider-catalog">
    <p>{{ tr('readOnlyHint') }}</p>
    <p v-if="state?.listFailed || tableFailed" role="alert" class="notice error">{{ tr('loadFailed') }}</p>
    <p v-if="importError" role="alert" class="notice error">{{ importError }}</p>
    <StandaloneTable :columns="columns" :rows="state?.items ?? []" row-key="reference" :height="360" :pagination="pagination" :search="search" :actions="importActions" :locale="context.locale" :empty-text="tr('empty')" :loading="state?.listLoading ?? false" @failed="tableFailed = true" />
    <p v-if="state?.detailLoading" role="status">{{ tr('loadingPrices') }}</p>
    <p v-if="state?.detailFailed" role="alert" class="notice error">{{ tr('detailFailed') }}</p>
    <section v-if="state?.detail" data-testid="provider-catalog-prices">
      <h3>{{ state.detail.product.name }}</h3>
      <p v-if="state.detail.product.description">{{ state.detail.product.description }}</p>
      <p>{{ tr('basePriceHint') }}</p>
      <p v-if="!state.detail.priceScheduleComplete" role="alert" class="notice error">{{ tr('incompleteHint') }}</p>
      <h4>{{ tr('basePrices') }}</h4>
      <StandaloneTable :columns="baseColumns" :rows="rows.base" row-key="rowKey" :height="210" :locale="context.locale" :empty-text="tr('noPrices')" :loading="false" @failed="tableFailed = true" />
      <h4>{{ tr('optionPrices') }}</h4>
      <StandaloneTable :columns="optionColumns" :rows="rows.options" row-key="rowKey" :height="280" :locale="context.locale" :empty-text="tr('noSurcharges')" :loading="false" @failed="tableFailed = true" />
    </section>
    <div ref="importRoot" />
  </section>
</template>
