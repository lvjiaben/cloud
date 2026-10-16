<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { Input, Switch } from 'ant-design-vue';
import { $t } from '#/locales';
const props = defineProps<{ publicOnly?: boolean }>();
const keyword = ref('');
const showAll = ref(!props.publicOnly);
const document = ref<Record<string, any> | null>(null);
const failed = ref(false);
const limit = ref(50);
const expanded = ref(new Set<string>());
const contractCache = new Map<string, string>();
watch([keyword, showAll],()=>{limit.value=50;expanded.value=new Set();});
async function load() {
  failed.value = false;
  try { const response=await fetch('/api.json'); if(!response.ok)throw new Error(); const body=await response.json(); if(!body.paths)throw new Error();document.value=body; } catch { failed.value=true; }
}
onMounted(load);
const entries = computed(() => Object.entries(document.value?.paths??{}).flatMap(([path,methods])=>Object.entries(methods as Record<string,any>).filter(([method])=>['get','post','put','patch','delete'].includes(method)).map(([method,operation])=>({path,method:method.toUpperCase(),operation}))).filter((entry)=> {
  return (showAll.value||entry.path.includes('/public/')||entry.path.includes('/site/configuration')) && (entry.path+' '+entry.operation.summary).toLowerCase().includes(keyword.value.toLowerCase());
}));
function contract(entry:{operation:any; method:string; path:string}) {
 const key=entry.method+entry.path; if(contractCache.has(key))return contractCache.get(key);
 const schemas:Record<string,any>={};
 function collect(value:any){if(!value||typeof value!=='object')return; if(typeof value.$ref==='string'&&value.$ref.startsWith('#/components/schemas/')){const name=value.$ref.slice(21);if(!(name in schemas)){schemas[name]=document.value?.components?.schemas?.[name];collect(schemas[name]);}} for(const item of Object.values(value))collect(item);}
 collect(entry.operation);const result=JSON.stringify({operation:entry.operation,schemas},null,2);contractCache.set(key,result);return result;
}
const download = ref('');
watch([document, keyword, showAll], () => {
 if (download.value) URL.revokeObjectURL(download.value);
 const paths=Object.fromEntries(entries.value.map(e=>[e.path,document.value?.paths[e.path]]));
 download.value=URL.createObjectURL(new Blob([JSON.stringify({openapi:document.value?.openapi,info:document.value?.info,paths,components:document.value?.components},null,2)],{type:'application/json'}));
});
onBeforeUnmount(()=>{if(download.value)URL.revokeObjectURL(download.value);});
</script>
<template>
 <section class="space-y-3">
  <div class="flex gap-4"><a href="/api.json" target="_blank" rel="noopener">OpenAPI 3.0</a><a v-if="document" :href="download" download="template-api-contracts.json">{{ $t('plugin.idc-portal.templateApi.download') }}</a></div>
  <label class="flex items-center gap-2"><Switch v-model:checked="showAll" />{{ $t('plugin.idc-portal.templateApi.all') }}</label>
  <Input v-model:value="keyword" :placeholder="$t('plugin.idc-portal.templateApi.search')" allow-clear />
  <p>{{ $t('plugin.idc-portal.templateApi.source') }}</p>
  <button v-if="failed" type="button" @click="load">{{ $t('plugin.idc-portal.templateApi.retry') }}</button>
  <div class="max-h-[500px] overflow-y-auto">
   <details v-for="entry in entries.slice(0,limit)" :key="entry.method+entry.path" class="border-b py-2" @toggle="($event.target as HTMLDetailsElement).open ? expanded.add(entry.method+entry.path) : expanded.delete(entry.method+entry.path)">
    <summary class="cursor-pointer break-all"><strong>{{ entry.method }}</strong> {{ entry.path }}<div>{{ entry.operation.summary }}</div></summary>
    <pre v-if="expanded.has(entry.method+entry.path)" class="overflow-x-auto whitespace-pre text-xs">{{ contract(entry) }}</pre>
   </details>
  </div>
  <div class="flex gap-4"><span>{{ Math.min(limit,entries.length) }} / {{ entries.length }}</span><button v-if="entries.length>limit" type="button" @click="limit+=50">+50</button></div>
 </section>
</template>
