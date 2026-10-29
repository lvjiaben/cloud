<script setup lang="ts">
import type { CatalogWebsiteSettings } from '../catalog-purchase-links';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { Alert, Button, Input, message } from 'ant-design-vue';
import { pluginApiPath, requestClient } from '#/api/request';
import { $t } from '#/locales';
import { catalogPurchaseLinks } from '../catalog-purchase-links';
const props = defineProps<{slug:string; categoryNo:string}>();
const settings = ref<CatalogWebsiteSettings>();
const loading = ref(false);
const failed = ref(false);
let disposed = false;
const tr = (key:string) => $t(`plugin.idc-core.catalog.purchaseLinks.${key}`);
const links = computed(() => {
  try { return settings.value ? catalogPurchaseLinks(settings.value,props.slug,props.categoryNo) : undefined; }
  catch { return undefined; }
});
async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const result = await requestClient.get<{config?:CatalogWebsiteSettings}>(pluginApiPath('idc-portal','public/customer-center/configuration'), {silentErrorMessage:true});
    if (!disposed) settings.value = result.config;
  } catch { if (!disposed) { settings.value = undefined; failed.value = true; } }
  finally { if (!disposed) loading.value = false; }
}
async function copy(value:string) {
  try { await navigator.clipboard.writeText(value); message.success(tr('copied')); }
  catch { message.error(tr('copyFailed')); }
}
onMounted(load);
onBeforeUnmount(() => { disposed = true; });
</script>
<template>
  <section class="catalog-purchase-links" data-testid="catalog-purchase-links">
    <div v-if="links" class="grid gap-4">
      <div v-for="kind in (['product','group'] as const)" :key="kind" class="purchase-link-row">
        <label :for="`catalog-link-${kind}`">{{ tr(kind) }}</label>
        <Input :id="`catalog-link-${kind}`" :value="links[kind]" readonly :data-testid="`catalog-link-${kind}`" />
        <Button :data-testid="`catalog-link-copy-${kind}`" @click="copy(links[kind])">{{ tr('copy') }}</Button>
      </div>
    </div>
    <Alert v-else :type="failed ? 'error' : 'warning'" :message="tr(failed ? 'readFailed' : 'unconfigured')"><template #action><Button :loading="loading" @click="load">{{ tr('reload') }}</Button></template></Alert>
  </section>
</template>
<style scoped>
.catalog-purchase-links{margin:28px 0;padding-top:24px;border-top:1px solid hsl(var(--border));}
.purchase-link-row{display:grid;grid-template-columns:130px minmax(0,1fr) auto;gap:12px;align-items:center;}
@media(max-width:600px){.purchase-link-row{grid-template-columns:minmax(0,1fr) auto;}.purchase-link-row label{grid-column:1/-1;}}
</style>
