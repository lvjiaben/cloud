<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Alert, Button, Select } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { catalogProductList } from "../catalog-client";
const props = defineProps<{ productNo: string; version: number; writable: boolean }>();
type Offer = { productNo: string; nameZhCN: string; nameEnUS: string };
const chosen = ref<string[]>([]);
const saved = ref<Offer[]>([]);
const candidates = ref<Offer[]>([]);
const loading = ref(false);
const failed = ref(false);
let sequence = 0;
let timer: ReturnType<typeof setTimeout> | undefined;
const tr = (key: string) => $t(`plugin.idc-core.addons.${key}`);
const api = () => pluginApiPath("idc-core", `admin/catalog/products/${encodeURIComponent(props.productNo)}/addons`);
function options() {
  return [...new Map([...saved.value, ...candidates.value].filter(row => row.productNo !== props.productNo).map(row => [row.productNo, row])).values()]
    .map(row => ({ value: row.productNo, label: preferences.app.locale === "zh-CN" ? row.nameZhCN || row.nameEnUS : row.nameEnUS || row.nameZhCN }));
}
async function load() {
  const current = ++sequence;
  loading.value = true; failed.value = false;
  try {
    const result = await requestClient.get<{ addons: Offer[] }>(api());
    if (current !== sequence) return;
    saved.value = result.addons; chosen.value = result.addons.map(row => row.productNo);
    await search("");
  } catch { if (current === sequence) failed.value = true; }
  finally { if (current === sequence) loading.value = false; }
}
async function search(keyword: string) {
  const current = sequence;
  const result = await catalogProductList({ page: 1, limit: 50, namePrefix: keyword.trim() });
  if (current === sequence) candidates.value = result.products.map(row => ({ productNo: row.productNo, nameZhCN: row.nameZhCn, nameEnUS: row.nameEnUs }));
}
function delayedSearch(keyword: string) {
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => void search(keyword).catch(() => { failed.value = true; }), 250);
}
function prepare() {
  if (!props.writable) return {valid:true};
  if (loading.value || failed.value) return {valid:false};
  const baseline = saved.value.map(row => row.productNo).sort();
  return {valid:true, input:JSON.stringify([...chosen.value].sort()) === JSON.stringify(baseline) ? undefined : [...chosen.value]};
}
defineExpose({prepare, dirty:computed(() => JSON.stringify([...chosen.value].sort()) !== JSON.stringify(saved.value.map(row=>row.productNo).sort()))});
watch(() => props.productNo, load, { immediate: true });
onBeforeUnmount(() => { ++sequence; if (timer) clearTimeout(timer); });
</script>
<template>
  <section class="catalog-addon-row" data-testid="catalog-addons">
    <label>{{ tr('adminTitle') }}</label>
    <div class="min-w-0 flex-1">
    <Alert v-if="failed" class="mb-4" type="error" show-icon :message="tr('failed')"><template #action><Button @click="load">{{ tr('refresh') }}</Button></template></Alert>
    <Select v-model:value="chosen" mode="multiple" class="w-full" :disabled="!writable || loading || failed" :options="options()" show-search :filter-option="false" :max-tag-count="'responsive'" :placeholder="tr('search')" data-testid="catalog-addon-products" @search="delayedSearch" />
    </div>
  </section>
</template>

<style scoped>
.catalog-addon-row {display:flex;gap:8px;align-items:flex-start;max-width:760px;margin:12px 0 24px;}
.catalog-addon-row > label {width:122px;flex:none;text-align:right;line-height:32px;}
</style>
