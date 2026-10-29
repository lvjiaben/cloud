<script setup lang="ts">
import type { CatalogCategory, CatalogProductDetail } from '../catalog-client';
import type { ProductDirectory } from '../product-directory-client';
import type { VbenFormProps } from '@vben/common-ui';
import { watch, ref } from 'vue';
import { preferences } from '@vben/preferences';
import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';
import { catalogCategoryList } from '../catalog-client';
import { listProductDirectoryOptions } from '../product-directory-client';
import { useAccess } from '../plugin-access';

// Sales category and member navigation are independent assignments. Missing
// candidate permission or a failed read must never clear either assignment.
const props = defineProps<{product: CatalogProductDetail['product']; writable: boolean}>();
const {hasAccessByCodes} = useAccess();
const memberMenus = ref<ProductDirectory[]>([]);
const categories = ref<CatalogCategory[]>([]);
const categoriesReady = ref(false);
const menusReady = ref(false);
const saving = ref(false);
const dirty = ref(false);
// Preserve the original revision while unsaved values remain in this form.
const expectedVersion = ref(0);
let storedSignature = '';
function signature(product:CatalogProductDetail['product']) {
  const {categoryNo,directoryNo,nameZhCn,nameEnUs,summaryZhCn,summaryEnUs}=product;
  return JSON.stringify([categoryNo,directoryNo || '',nameZhCn,nameEnUs,summaryZhCn,summaryEnUs]);
}
const tr = (key: string) => $t(`plugin.idc-core.catalog.description.${key}`);
const localized = (item: {nameZhCn: string; nameEnUs: string}) => preferences.app.locale === 'zh-CN' ? item.nameZhCn || item.nameEnUs : item.nameEnUs || item.nameZhCn;
function categoryOptions() {
  const byNo = new Map(categories.value.map(item => [item.categoryNo, item]));
  const options = categories.value.filter(item => item.status === 'active' && !item.deleted).map(item => {
    const parent = byNo.get(item.parentCategoryNo);
    return {value:item.categoryNo, label:parent ? `${localized(parent)} / ${localized(item)}` : localized(item)};
  });
  if (!options.some(item => item.value === props.product.categoryNo)) {
    options.unshift({value:props.product.categoryNo, label:localized({nameZhCn:props.product.categoryNameZhCn, nameEnUs:props.product.categoryNameEnUs})});
  }
  return options;
}
function schema(): VbenFormProps['schema'] {
  const required = z.string().trim().min(1, tr('required'));
  return [
    ...(hasAccessByCodes(['idc:admin:catalog:category:list']) ? [{fieldName:'categoryNo', component:'Select', label:$t('plugin.idc-core.catalog.detail.category'), componentProps:{'data-testid':'catalog-description-category', options:categoryOptions(), showSearch:true, optionFilterProp:'label', disabled:!props.writable || saving.value || !categoriesReady.value}, rules:z.string().min(1)}] : []),
    ...(hasAccessByCodes(['idc:admin:product-directory:list']) ? [{fieldName:'directoryNo', component:'Select', label:$t('plugin.idc-core.productDirectory.productMenu'), componentProps:{allowClear:false, 'data-testid':'catalog-description-member-menu', disabled:!props.writable || saving.value || !menusReady.value, options:memberMenus.value.filter(item => item.statusCode === 'enabled' && (!item.navigationType || item.navigationType === 'product') || item.directoryNo === props.product.directoryNo).map(item => ({value:item.directoryNo, label:localized(item)}))}, rules:z.string().trim().min(1, tr('required'))}] : []),
    ...(['nameZhCn', 'nameEnUs', 'summaryZhCn', 'summaryEnUs'] as const).map((fieldName, index) => ({fieldName, label:tr(fieldName), component:index < 2 ? 'Input' : 'Textarea', componentProps:{'data-testid':['catalog-description-name-zh','catalog-description-name-en','catalog-description-summary-zh','catalog-description-summary-en'][index], maxlength:index < 2 ? 256 : 1024, ...(index < 2 ? {} : {rows:3})}, rules:required})),
  ];
}
const [Form, formApi] = useVbenForm({handleValuesChange: values => { dirty.value = signature(values as CatalogProductDetail['product']) !== storedSignature; }, schema:[], showDefaultActions:false, layout:'horizontal', commonConfig:{labelWidth:130, componentProps:{class:'w-full'}}, wrapperClass:'grid-cols-1'});
watch(() => props.product.productNo, async () => {
  expectedVersion.value = props.product.descriptionVersion ?? 0;
  storedSignature = signature(props.product);
  categoriesReady.value = false;
  menusReady.value = false;
  await formApi.setState({schema:schema()});
  const {nameZhCn, nameEnUs, summaryZhCn, summaryEnUs, directoryNo, categoryNo} = props.product;
  await formApi.setValues({nameZhCn, nameEnUs, summaryZhCn, summaryEnUs, directoryNo:directoryNo || undefined, categoryNo});
  await Promise.allSettled([
    ...(hasAccessByCodes(['idc:admin:catalog:category:list']) ? [catalogCategoryList().then(result => {categories.value=result.categories; categoriesReady.value=true;})] : []),
    ...(hasAccessByCodes(['idc:admin:product-directory:list']) ? [listProductDirectoryOptions().then(result => {memberMenus.value=result; menusReady.value=true;})] : []),
  ]);
  await formApi.setState({schema:schema()});
}, {immediate:true});
watch(() => props.product.descriptionVersion, version => {
  // Another section may change the product revision without changing these
  // stored fields. Advance only that anchor and preserve all unsaved inputs.
  if (storedSignature === signature(props.product)) expectedVersion.value = version ?? expectedVersion.value;
});
watch(() => preferences.app.locale, () => formApi.setState({schema:schema()}));
watch(() => props.writable && !saving.value, enabled => formApi.setState({schema:schema(), commonConfig:{componentProps:{class:'w-full', disabled:!enabled}}}), {immediate:true});
async function prepare() {
  if (!props.writable || !(await formApi.validate()).valid) return;
  const {directoryNo, categoryNo, ...copy} = await formApi.getValues<{categoryNo?:string;directoryNo?:string;nameZhCn:string;nameEnUs:string;summaryZhCn:string;summaryEnUs:string}>();
  return {...copy,
    ...(hasAccessByCodes(['idc:admin:catalog:category:list']) && categoriesReady.value ? {categoryNo} : {}),
    ...(hasAccessByCodes(['idc:admin:product-directory:list']) && menusReady.value ? {directoryNo:directoryNo || ''} : {}),
    version:expectedVersion.value};
}
defineExpose({prepare, dirty, isDirty:async () => signature(await formApi.getValues() as CatalogProductDetail['product']) !== storedSignature});
</script>

<template>
  <div style="max-width:760px" data-testid="catalog-description-editor">
    <Form />
  </div>
</template>
