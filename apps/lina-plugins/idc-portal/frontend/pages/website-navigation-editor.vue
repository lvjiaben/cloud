<script setup lang="ts">
import type { WebsiteNavigation, WebsiteNavigationItem, WebsitePage } from './website-configuration-client';
import { computed, ref, watch } from 'vue';
import { useVbenModal } from '@vben/common-ui';
import { Button, Popconfirm, Radio, RadioGroup, message } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';

const props = defineProps<{ modelValue: WebsiteNavigation; pages: WebsitePage[]; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: WebsiteNavigation] }>();
const tr = (key: string) => $t(`plugin.idc-portal.website.nav.${key}`);
const position = ref<'header' | 'footer'>('header');
const editing = ref<WebsiteNavigationItem>();
const items = computed(() => props.modelValue[position.value] ?? []);
const destinations = computed(() => [{ label: tr('group'), value: '__group' }, ...props.pages.map(page => ({ label: `${page.nameZh} (${page.path})`, value: page.id })), { label: tr('login'), value: 'customer-login' }, { label: tr('register'), value: 'customer-register' }, { label: tr('external'), value: '__url' }]);
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions: {
 columns: [
  { field: 'nameZh', title: tr('name'), minWidth: 150 },
  { field: 'destination', title: tr('destination'), minWidth: 180, slots: { default: 'destination' } },
  { field: 'parentId', title: tr('parent'), minWidth: 130, slots: { default: 'parent' } },
  { field: 'sortOrder', title: tr('order'), width: 90 },
  { field: 'action', title: tr('actions'), fixed: 'right', width: 140, slots: { default: 'action' } },
 ], height: 340, pagerConfig: { enabled: false }, rowConfig: { keyField: 'id' },
 toolbarConfig: { refresh: false, custom: false },
} });
const [Form, formApi] = useVbenForm({ commonConfig: { componentProps: { class: 'w-full' }, labelWidth: 100 }, schema: [], showDefaultActions: false });
const [Editor, modalApi] = useVbenModal({ onConfirm: save, onOpenChange: initialize });
watch(items, value => gridApi.setGridOptions({ data: [...value].sort((a,b) => a.sortOrder - b.sortOrder) }), { immediate: true, deep: true });

function parentOptions() {
 const excluded = new Set<string>();
 if (editing.value) {
  excluded.add(editing.value.id);
  for (let pass=0;pass<3;pass++) for (const item of items.value) if (excluded.has(item.parentId)) excluded.add(item.id);
 }
 return [{ label: tr('root'), value: '' }, ...items.value.filter(item => !excluded.has(item.id)).map(item => ({ label: item.nameZh, value: item.id }))];
}
async function initialize(open: boolean) {
 if (!open) return;
 formApi.setState({ schema: [
  { fieldName: 'nameZh', component: 'Input', label: tr('nameZh'), componentProps: { 'aria-label': tr('nameZh') }, rules: z.string().trim().min(1).max(80) },
  { fieldName: 'nameEn', component: 'Input', label: tr('nameEn'), componentProps: { 'aria-label': tr('nameEn') }, rules: z.string().trim().min(1).max(80) },
  { fieldName: 'destination', component: 'Select', label: tr('destination'), componentProps: { options: destinations.value, 'aria-label': tr('destination') }, rules: 'required' },
  { fieldName: 'url', component: 'Input', label: tr('url'), componentProps: { placeholder: 'https://example.com' }, dependencies: { triggerFields: ['destination'], show: values => values.destination === '__url' } },
  { fieldName: 'parentId', component: 'Select', label: tr('parent'), componentProps: { options: parentOptions() } },
  { fieldName: 'sortOrder', component: 'InputNumber', label: tr('order'), componentProps: { min: 0, max: 9999 }, rules: z.number().min(0).max(9999) },
 ] });
 await formApi.resetForm();
 await formApi.setValues(editing.value ? { ...editing.value, destination: editing.value.group ? '__group' : editing.value.pageId || '__url' } : { nameZh: '', nameEn: '', destination: props.pages[0]?.id ?? 'customer-login', url: '', parentId: '', sortOrder: items.value.length });
 modalApi.setState({ title: editing.value ? tr('edit') : tr('add') });
}
function open(item?: WebsiteNavigationItem) { editing.value = item; modalApi.open(); }
function saveItems(next: WebsiteNavigationItem[]) { emit('update:modelValue', { ...props.modelValue, [position.value]: next }); }
function remove(item: WebsiteNavigationItem) { saveItems(items.value.filter(row => row.id !== item.id).map(row => row.parentId === item.id ? { ...row, parentId: item.parentId } : row)); }
async function save() {
 if (!(await formApi.validate()).valid) return;
 const values = await formApi.getValues();
 const url = values.destination === '__url' ? String(values.url ?? '').trim() : '';
 if (values.destination === '__url') {
  try { const parsed = new URL(url); if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) throw new Error(); } catch { message.error(tr('invalidURL')); return; }
 }
 const item: WebsiteNavigationItem = { id: editing.value?.id ?? crypto.randomUUID(), group: values.destination === '__group', nameZh: String(values.nameZh).trim(), nameEn: String(values.nameEn).trim(), pageId: ['__url', '__group'].includes(values.destination) ? '' : String(values.destination), url, parentId: String(values.parentId ?? ''), sortOrder: Number(values.sortOrder) };
 const next = [...items.value.filter(row => row.id !== item.id), item];
 const byId = new Map(next.map(row => [row.id, row]));
 for (const row of next) { let parent = row.parentId; let depth=0; const seen = new Set([row.id]); while (parent) { if (seen.has(parent) || !byId.has(parent) || ++depth > 2) { message.error(tr('depth')); return; } seen.add(parent); parent = byId.get(parent)!.parentId; } }
 saveItems(next); modalApi.close();
}
function destination(item: WebsiteNavigationItem) { return item.group ? tr('group') : item.pageId ? destinations.value.find(row => row.value === item.pageId)?.label ?? tr('unavailable') : item.url; }
</script>

<template>
 <div class="website-navigation-editor">
  <div class="mb-3 flex items-center justify-between gap-3">
   <RadioGroup v-model:value="position" option-type="button" button-style="solid"><Radio value="header">{{ tr('header') }}</Radio><Radio value="footer">{{ tr('footer') }}</Radio></RadioGroup>
   <Button type="primary" :disabled="disabled || items.length >= 100" @click="open()">{{ tr('add') }}</Button>
  </div>
  <p class="mb-3 text-muted-foreground">{{ tr('hint') }}</p>
  <Grid>
   <template #destination="{ row }">{{ destination(row) }}</template>
   <template #parent="{ row }">{{ items.find(item => item.id === row.parentId)?.nameZh ?? tr('root') }}</template>
   <template #action="{ row }"><ghost-button :disabled="disabled" @click="open(row)">{{ tr('edit') }}</ghost-button><Popconfirm :title="tr('confirmDelete')" @confirm="remove(row)"><ghost-button danger :disabled="disabled">{{ tr('delete') }}</ghost-button></Popconfirm></template>
  </Grid>
  <Editor class="w-[480px]"><Form /></Editor>
 </div>
</template>
