<script setup lang="ts">
import type { CatalogProductVersionInventory } from '../catalog-client';
import type { VbenFormProps } from '@vben/common-ui';
import { computed, ref, watch } from 'vue';
import { preferences } from '@vben/preferences';
import { Alert, Button } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';

// Keep pending stock edits across tabs. An equivalent internal working copy
// may advance the write anchor; a different stored stock value may not.
const props = defineProps<{inventory:CatalogProductVersionInventory; versionNo:string; writable:boolean}>();
const saving = ref(false);
const dirty = ref(false);
const conflict = ref(false);
const expectedVersion = ref(0);
const editingVersionNo = ref('');
const baseline = ref('');
const representable = computed(() => Number.isSafeInteger(props.inventory.sellableQuantity) && props.inventory.sellableQuantity >= 0);
const tr = (key:string) => $t(`plugin.idc-core.catalog.inventory.${key}`);
const signature = (item:CatalogProductVersionInventory) => JSON.stringify([item.trackingEnabled,item.sellableQuantity]);
function schema():VbenFormProps['schema'] {
  return [
    {component:'Switch', componentProps:{'data-testid':'catalog-inventory-tracking',style:{width:'44px'}}, fieldName:'trackingEnabled', label:tr('trackingEnabled')},
    {component:'InputNumber', componentProps:{'data-testid':'catalog-inventory-quantity', min:0, max:Number.MAX_SAFE_INTEGER, precision:0}, dependencies:{show:values => values.trackingEnabled === true, triggerFields:['trackingEnabled']}, fieldName:'sellableQuantity', label:tr('sellableQuantity'), rules:z.number().int().min(0).max(Number.MAX_SAFE_INTEGER)},
  ];
}
const [Form, formApi] = useVbenForm({handleValuesChange: values => { dirty.value = signature(values as CatalogProductVersionInventory) !== baseline.value; }, schema:[], commonConfig:{labelWidth:130, componentProps:{class:'w-full'}}, wrapperClass:'grid-cols-1', showDefaultActions:false});
async function readCurrent() {
  baseline.value = signature(props.inventory);
  expectedVersion.value = props.inventory.version;
  editingVersionNo.value = props.versionNo;
  conflict.value = false;
  await formApi.setValues({trackingEnabled:props.inventory.trackingEnabled,sellableQuantity:props.inventory.sellableQuantity});
}
watch(() => [props.versionNo,props.inventory] as const, async () => {
  if (!baseline.value) {await readCurrent();return;}
  if (baseline.value !== signature(props.inventory)) {conflict.value=true;return;}
  editingVersionNo.value=props.versionNo;
  expectedVersion.value=props.inventory.version;
}, {immediate:true, deep:true});
watch(() => [props.writable,saving.value,preferences.app.locale,representable.value], () => formApi.setState({schema:schema(), commonConfig:{componentProps:{class:'w-full',disabled:!props.writable || saving.value || !representable.value}}}), {immediate:true});
async function prepare() {
  if (!props.writable) return {valid:true};
  if (conflict.value || !representable.value || !(await formApi.validate()).valid) return {valid:false};
  const values = await formApi.getValues<{trackingEnabled:boolean;sellableQuantity:number}>();
  return {valid:true, input:signature({...values,version:expectedVersion.value}) === baseline.value ? undefined : {...values,expectedVersion:expectedVersion.value}};
}
defineExpose({prepare, dirty, isDirty:async () => signature(await formApi.getValues() as CatalogProductVersionInventory) !== baseline.value});
</script>

<template>
  <div style="max-width:760px" data-testid="catalog-inventory-editor">
    <Alert v-if="conflict" type="warning" show-icon :message="tr('conflict')" class="mb-4" data-testid="catalog-inventory-conflict"><template #action><Button data-testid="catalog-inventory-reload" @click="readCurrent">{{ tr('reload') }}</Button></template></Alert>
    <Alert v-if="!representable" type="warning" show-icon :message="tr('quantityTooLarge')" class="mb-4" />
    <Form />
  </div>
</template>
