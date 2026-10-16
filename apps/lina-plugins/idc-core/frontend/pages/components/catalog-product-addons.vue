<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { Alert, Button, Card, Select, message } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { catalogProductList } from "../catalog-client";
const props = defineProps<{ productNo: string; version: number; writable: boolean }>();
const emit = defineEmits<{ saved: [] }>();
type Offer = { productNo: string; nameZhCN: string; nameEnUS: string };
const chosen = ref<string[]>([]);
const saved = ref<Offer[]>([]);
const candidates = ref<Offer[]>([]);
const busy = ref(false);
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
async function save() {
  if (busy.value || !props.writable || failed.value) return;
  busy.value = true;
  try {
    await requestClient.put(api(), { version: props.version, productNos: chosen.value });
    message.success(tr("saved")); emit("saved");
  } finally { busy.value = false; }
}
watch(() => props.productNo, load, { immediate: true });
onBeforeUnmount(() => { ++sequence; if (timer) clearTimeout(timer); });
</script>
<template>
  <Card :title="tr('adminTitle')" :loading="loading" data-testid="catalog-addons">
    <Alert class="mb-4" type="info" show-icon :message="tr('adminHint')" />
    <Alert v-if="failed" class="mb-4" type="error" show-icon :message="tr('failed')"><template #action><Button @click="load">{{ tr('refresh') }}</Button></template></Alert>
    <Select v-model:value="chosen" mode="multiple" class="w-full" :disabled="!writable || busy || failed" :options="options()" show-search :filter-option="false" :max-tag-count="'responsive'" :placeholder="tr('search')" data-testid="catalog-addon-products" @search="delayedSearch" />
    <Button v-if="writable" class="mt-4" type="primary" :disabled="failed" :loading="busy" data-testid="catalog-addons-save" @click="save">{{ tr('save') }}</Button>
  </Card>
</template>
