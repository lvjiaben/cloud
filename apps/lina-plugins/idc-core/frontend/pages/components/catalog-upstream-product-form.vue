<script setup lang="ts">
// Read only the selected interface's bounded directory. A detail is fetched once
// after explicit selection; no row-by-row reads or upstream writes occur here.
import type { VbenFormProps } from '@vben/common-ui';
import type { CatalogUpstreamProductSelection } from '../catalog-client';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { majorToMinorAmount, minorToMajorAmount } from '../catalog-money';
import { preferences } from '@vben/preferences';
import { Alert, Button } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { pluginApiPath, requestClient } from '#/api/request';
import { $t } from '#/locales';
const props = defineProps<{interfaceNo:string; categoryNo:string; reference?:string; productName?:string; savedPricingMode?:'markup'|'manual'|'mixed'; savedMarkupPercent?:string; savedPrices?:CatalogUpstreamProductSelection['prices']; writable:boolean}>();
const emit = defineEmits<{change:[value:CatalogUpstreamProductSelection | undefined]; loading:[value:boolean]}>();
type Price = CatalogUpstreamProductSelection['prices'][number];
type Product = {reference:string;name:string};
type UpstreamTerm = {chargeType:string;periodUnit:string;periodCount:number;subtotalAmount:string;setupAmount:string};
type UpstreamSchedule = Pick<UpstreamTerm,'chargeType'|'periodUnit'|'periodCount'> & {currencyCode:string};
type Detail = {product:Product;basePrices:Price[];options:Array<{name:string}>;priceScheduleComplete:boolean;
 importSnapshot?:{protocolVersion:string;pricingMode?:string;prices:UpstreamSchedule[]};
 peerPurchase?:{configurationMode:string;billingTerms:Array<{currencyCode:string;terms:UpstreamTerm[]}>};
};
const products = ref<Product[]>([]), detail = ref<Detail>();
const remotePricing = computed(() => detail.value?.importSnapshot?.protocolVersion === '3' && detail.value.importSnapshot.pricingMode === 'remote_evaluation');
const priceRows = computed<Price[]>(() => {
 const item=detail.value;if(!item)return [];
 if(!remotePricing.value && item.basePrices?.length)return item.basePrices;
 return (item.importSnapshot?.prices || []).map(schedule=>{
  const period=schedule.chargeType==='one_time'?'once':schedule.periodUnit;
  const sample=item.peerPurchase?.billingTerms?.find(row=>row.currencyCode===schedule.currencyCode)?.terms?.find(term=>term.chargeType===schedule.chargeType&&term.periodUnit===schedule.periodUnit&&term.periodCount===schedule.periodCount);
  let amount='',setupFee='';
  if(sample){try{amount=minorToMajorAmount(sample.subtotalAmount,schedule.currencyCode);setupFee=minorToMajorAmount(sample.setupAmount,schedule.currencyCode);}catch{/* No approximate or invented price replaces an unavailable sample. */}}
  return {currencyCode:schedule.currencyCode,period,periodCount:schedule.periodCount,amount,setupFee};
 });
});
const sampleRows=computed(()=>remotePricing.value?priceRows.value.filter(row=>row.amount!==''&&row.setupFee!==''):[]);
const loading = ref(false), failed = ref(false);
let listSequence = 0, detailSequence = 0, disposed = false;
let keyword = '', total = 0, page = 0;
let chosen = '', changed = false, signature = '', baselineSignature = '', requestKey = '';
let timer:ReturnType<typeof setTimeout> | undefined;
const tr = (key:string) => $t(`plugin.idc-core.catalog.upstream.${key}`);
const priceText=(key:string)=>$t(`plugin.idc-core.catalog.upstreamPricing.${key}`);
function priceLabel(price:Price){return `${price.currencyCode} · ${price.period==='once'?$t('plugin.idc-core.catalog.detail.code.period.once'):`${price.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${price.period}`)}`}`;}
const path = () => pluginApiPath('idc-resource',`admin/resource/provider-configurations/interfaces/${encodeURIComponent(props.interfaceNo)}`);
function schema():VbenFormProps['schema'] {
  const options = products.value.map(item => ({value:item.reference,label:item.name}));
  if (chosen && !options.some(item => item.value === chosen)) options.unshift({value:chosen,label:props.productName || detail.value?.product.name || chosen});
  return [
    {fieldName:'reference',component:'Select',label:tr('product'),componentProps:{'data-testid':'catalog-upstream-product',options,showSearch:true,filterOption:false,loading:loading.value,disabled:!props.writable || !!props.reference,onSearch:search,onPopupScroll:more,onChange:(value:string) => select(value,true)},rules:z.string().trim().min(1)},
    ...(detail.value ? [
      {fieldName:'pricingMode',component:'Select',label:tr('pricingMode'),componentProps:{'data-testid':'catalog-upstream-pricing-mode',options:[...(props.savedPricingMode==='mixed'?[{value:'mixed',label:priceText('mixed'),disabled:true}]:[]),{value:'markup',label:tr('markup')},{value:'manual',label:tr('manual')}],disabled:!props.writable}},
      {fieldName:'markupPercent',component:'Input',label:tr('percent'),componentProps:{'data-testid':'catalog-upstream-markup',suffix:'%',disabled:!props.writable},dependencies:{triggerFields:['pricingMode'],if:(values:Record<string,unknown>) => values.pricingMode === 'markup'},rules:z.string().regex(/^\d+(?:\.\d+)?$/,{message:priceText('percentRequired')})},
      ...priceRows.value.flatMap((price,index) => [
        {fieldName:`amount_${index}`,component:'Input',label:`${priceLabel(price)} · ${priceText('wholePrice')}`,componentProps:{'data-testid':`catalog-upstream-amount-${index}`,disabled:!props.writable},dependencies:{triggerFields:['pricingMode'],if:(values:Record<string,unknown>) => values.pricingMode === 'manual'},rules:z.string().regex(/^\d+(?:\.\d+)?$/)},
        {fieldName:`setup_${index}`,component:'Input',label:tr('setup'),componentProps:{'data-testid':`catalog-upstream-setup-${index}`,disabled:!props.writable},dependencies:{triggerFields:['pricingMode'],if:(values:Record<string,unknown>) => values.pricingMode === 'manual'},rules:z.string().regex(/^\d+(?:\.\d+)?$/)},
      ]),
    ] : []),
  ];
}
const [Form, formApi] = useVbenForm({schema:[],showDefaultActions:false,commonConfig:{labelWidth:130,componentProps:{class:'w-full'}},wrapperClass:'grid-cols-1',handleValuesChange:values => {
  if (!detail.value || loading.value) return;
  publish(values);
}});
function publish(values:Record<string,unknown>) {
  if (!detail.value || values.reference !== chosen) return;
  if(values.pricingMode!=='manual'&&values.pricingMode!=='markup'){emit('change',undefined);return;}
  const pricingMode = values.pricingMode;
  const input = {reference:chosen,categoryNo:props.categoryNo,pricingMode,markupPercent:pricingMode === 'markup' ? String(values.markupPercent ?? '') : '',prices:pricingMode === 'manual' ? priceRows.value.map((price,index) => ({...price,amount:String(values[`amount_${index}`] ?? ''),setupFee:String(values[`setup_${index}`] ?? '')})) : []} as Omit<CatalogUpstreamProductSelection,'requestKey'>;
  const next = pricingSignature(input);
  changed = next !== baselineSignature;
  if(!changed){signature=next;emit('change',undefined);return;}
  if (next !== signature) {signature=next;requestKey=crypto.randomUUID();}
  emit('change',{...input,requestKey});
}
function pricingSignature(input:{pricingMode:string;markupPercent:string;prices:Price[];reference:string;categoryNo:string}) {
 const canonicalAmount=(value:string,currency:string)=>{try{return minorToMajorAmount(majorToMinorAmount(value,currency),currency);}catch{return value;}};
 let percent=input.markupPercent;
 if(/^\d+(?:\.\d+)?$/.test(percent)){const [whole='',fraction='']=percent.split('.');percent=whole.replace(/^0+(?=\d)/,'')+(fraction.replace(/0+$/,'')?'.'+fraction.replace(/0+$/,''):'');}
 return JSON.stringify({...input,markupPercent:percent,prices:input.prices.map(price=>({...price,amount:canonicalAmount(price.amount,price.currencyCode),setupFee:canonicalAmount(price.setupFee,price.currencyCode)}))});
}
function seededValues(reference:string,user:boolean):Record<string,unknown> {
 const mode=user?'markup':props.savedPricingMode || '';
 const prices=user?priceRows.value:priceRows.value.map(row=>props.savedPrices?.find(saved=>saved.currencyCode===row.currencyCode&&saved.period===row.period&&saved.periodCount===row.periodCount) || {...row,amount:'',setupFee:''});
 return {reference,pricingMode:mode,markupPercent:user?'':props.savedMarkupPercent || '',...Object.fromEntries(prices.flatMap((price,index)=>[[`amount_${index}`,price.amount],[`setup_${index}`,price.setupFee]]))};
}
function seedBaseline(values:Record<string,unknown>) {
 // Capture the complete policy without emitting an initialized form as a user edit.
 const pricingMode=String(values.pricingMode || '');
 baselineSignature=pricingSignature({reference:chosen,categoryNo:props.categoryNo,pricingMode,markupPercent:pricingMode==='markup'?String(values.markupPercent??''):'',prices:pricingMode==='manual'?priceRows.value.map((price,index)=>({...price,amount:String(values[`amount_${index}`]??''),setupFee:String(values[`setup_${index}`]??'')})):[]});
 signature=baselineSignature;changed=false;
}
function busy(value:boolean) {loading.value=value;emit('loading',value);}
async function list(append=false) {
  const run=++listSequence, interfaceNo=props.interfaceNo;
  busy(true);failed.value=false;
  try {
    const result=await requestClient.get<{items:Product[];total:number}>(`${path()}/catalog-products`,{params:{page:append?page+1:1,pageSize:20,keyword}});
    if(disposed || run!==listSequence || interfaceNo!==props.interfaceNo)return;
    products.value=append?[...products.value,...result.items]:result.items;total=result.total;page=append?page+1:1;
    await formApi.setState({schema:schema()});
  }catch{if(run===listSequence)failed.value=true;}
  finally{if(run===listSequence){busy(false);await formApi.setState({schema:schema()});}}
}
function search(value:string) {keyword=value.trim();listSequence++;if(timer)clearTimeout(timer);timer=setTimeout(() => void list(),250);}
function more(event:Event) {const element=event.target as HTMLElement;if(!loading.value && products.value.length<total && element.scrollHeight-element.scrollTop-element.clientHeight<32)void list(true);}
async function select(reference:string, user:boolean) {
  const run=++detailSequence, interfaceNo=props.interfaceNo;
  chosen=reference;detail.value=undefined;changed=user;emit('change',undefined);busy(true);failed.value=false;
  try {
    const result=await requestClient.get<{item:Detail}>(`${path()}/catalog-product`,{params:{reference}});
    if(disposed || run!==detailSequence || interfaceNo!==props.interfaceNo)return;
    if(result.item.product.reference!==reference)throw new Error('upstream_product_changed');
    detail.value=result.item;
    await formApi.setState({schema:schema()});
    const values=seededValues(reference,user);
    if(user){baselineSignature='';signature='';}else{seedBaseline(values);}
    await formApi.setValues(values);
  }catch{if(run===detailSequence)failed.value=true;}
  finally{if(run===detailSequence){busy(false);if(user)publish(await formApi.getValues());}}
}
watch(() => props.interfaceNo,async () => {
  listSequence++;detailSequence++;products.value=[];detail.value=undefined;chosen=props.reference || '';changed=false;signature='';baselineSignature='';emit('change',undefined);
  await formApi.resetForm();await formApi.setState({schema:schema()});await formApi.setValues({reference:chosen || undefined});await list();if(props.reference)await select(props.reference,false);
}, {immediate:true});
watch(() => [props.reference,props.savedPricingMode,props.savedMarkupPercent,props.savedPrices,props.categoryNo] as const, async ([reference]) => {
  if(!reference)return;
  if(reference!==chosen || !detail.value){await select(reference,false);return;}
  const values=seededValues(reference,false);
  if(changed){
    const current=await formApi.getValues();
    const oldBaseline=baselineSignature;seedBaseline(values);const newBaseline=baselineSignature;
    seedBaseline(current);const currentSignature=baselineSignature;
    if(currentSignature!==newBaseline){baselineSignature=oldBaseline;changed=true;signature=currentSignature;return;}
  }
  seedBaseline(values);emit('change',undefined);await formApi.setValues(values);
});
watch(() => preferences.app.locale,() => formApi.setState({schema:schema()}));
async function validate() {
 const values=await formApi.getValues();
 if(values.pricingMode!=='markup'&&values.pricingMode!=='manual')return false;
 const fields=['reference',...(values.pricingMode==='markup'?['markupPercent']:priceRows.value.flatMap((_,index)=>[`amount_${index}`,`setup_${index}`]))];
 const results=await Promise.all(fields.map(field=>formApi.validateField(field)));
 return results.every(result=>result.valid);
}
defineExpose({validate});
onBeforeUnmount(() => {disposed=true;listSequence++;detailSequence++;if(timer)clearTimeout(timer);});
</script>
<template>
  <section data-testid="catalog-upstream-selection" class="mt-5">
    <section v-if="sampleRows.length" class="upstream-price-samples" data-testid="catalog-upstream-price-samples">
      <h4>{{priceText('sample')}}</h4>
      <dl><div v-for="price in sampleRows" :key="`${price.currencyCode}-${price.period}-${price.periodCount}`"><dt>{{priceLabel(price)}}</dt><dd>{{price.amount}} {{price.currencyCode}} · {{tr('setup')}} {{price.setupFee}} {{price.currencyCode}}</dd></div></dl>
    </section>
    <Alert v-if="savedPricingMode==='mixed'" class="mb-4" type="info" :message="priceText('mixedHint')" data-testid="catalog-upstream-mixed-pricing" />
    <Form />
    <Alert v-if="failed" type="error" show-icon :message="tr('readFailed')"><template #action><Button @click="chosen?select(chosen,!reference):list()">{{tr('retry')}}</Button></template></Alert>
    <Alert v-if="detail && !remotePricing && (!detail.priceScheduleComplete || detail.options?.length)" type="warning" show-icon :message="tr('incomplete')" />
  </section>
</template>

<style scoped>
.upstream-price-samples { padding:12px 16px; margin-bottom:16px; border:1px solid hsl(var(--border)); border-radius:6px; }
.upstream-price-samples h4 { margin:0 0 8px; font-size:13px; font-weight:500; color:hsl(var(--muted-foreground)); }
.upstream-price-samples dl { margin:0; }
.upstream-price-samples dl>div { display:flex; flex-wrap:wrap; justify-content:space-between; gap:6px 16px; padding:4px 0; }
.upstream-price-samples dd { margin:0; font-variant-numeric:tabular-nums; }
</style>
