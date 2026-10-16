<script setup lang="ts">
import { ref } from 'vue';
import type { Locale } from './public-i18n';

defineOptions({ name: 'PublicNavigation' });
export type PublicNavigationItem = { label: { enUS: string; zhCN: string }; href: string; group?: boolean; customer?: boolean; external?: boolean; children?: PublicNavigationItem[] };
const props = defineProps<{ items: PublicNavigationItem[]; locale: Locale; customerOrigin: string; footer?: boolean }>();
const localePath = useLocalePath();
const expanded = ref<string[]>([]);
const key = (item: PublicNavigationItem) => item.href + item.label.zhCN;
function toggle(item: PublicNavigationItem) { const id=key(item); expanded.value=expanded.value.includes(id) ? expanded.value.filter(value=>value!==id) : [...expanded.value,id]; }
function destination(item: PublicNavigationItem) { return item.customer ? new URL(item.href, props.customerOrigin).toString() : item.external ? item.href : localePath(item.href); }
</script>
<template>
 <ul :class="['public-navigation-tree', { 'is-footer': footer }]">
  <li v-for="item in items" :key="key(item)" :class="{ expanded: expanded.includes(key(item)) }" @keydown.esc.stop="expanded = expanded.filter(value => value !== key(item))">
   <button v-if="item.group" type="button" :aria-expanded="expanded.includes(key(item))" @click="toggle(item)">{{ locale === 'en-US' ? item.label.enUS : item.label.zhCN }} <span aria-hidden="true">⌄</span></button>
   <a v-else-if="item.customer || item.external" :href="destination(item)" :target="item.external ? '_blank' : undefined" :rel="item.external ? 'noopener noreferrer' : undefined">{{ locale === 'en-US' ? item.label.enUS : item.label.zhCN }}</a>
   <NuxtLink v-else :to="destination(item)">{{ locale === 'en-US' ? item.label.enUS : item.label.zhCN }}</NuxtLink>
   <PublicNavigation v-if="item.children?.length" :items="item.children" :locale="locale" :customer-origin="customerOrigin" :footer="footer" />
  </li>
 </ul>
</template>
<style scoped>
.public-navigation-tree { display:flex; align-items:center; gap:18px; list-style:none; padding:0; margin:0; }
.public-navigation-tree li { position:relative; }
.public-navigation-tree a, .public-navigation-tree button { display:block; padding:10px 0; color:inherit; white-space:nowrap; }
.public-navigation-tree button { border:0; background:none; font:inherit; cursor:pointer; text-align:left; }
.public-navigation-tree .public-navigation-tree { display:none; position:absolute; z-index:20; top:100%; left:0; min-width:160px; padding:8px 16px; background:#fff; color:#182230; border:1px solid #e5e7eb; box-shadow:0 4px 12px #00000014; }
.public-navigation-tree li.expanded > .public-navigation-tree, .public-navigation-tree li:has(> a):hover > .public-navigation-tree, .public-navigation-tree li:has(> a):focus-within > .public-navigation-tree { display:flex; flex-direction:column; align-items:stretch; gap:0; }
.public-navigation-tree .public-navigation-tree .public-navigation-tree { top:0; left:100%; }
.public-navigation-tree.is-footer { flex-wrap:wrap; gap:24px; align-items:flex-start; }
.is-footer .public-navigation-tree { display:flex; position:static; padding:0; min-width:0; flex-direction:column; align-items:flex-start; gap:0; background:none; color:inherit; border:0; box-shadow:none; }
.is-footer a { padding:4px 0; }
@media(max-width:640px) { .public-navigation-tree { gap:14px; flex-wrap:wrap; } .public-navigation-tree .public-navigation-tree { position:static; min-width:0; box-shadow:none; padding:0 12px; } }
</style>
