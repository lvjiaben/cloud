<script setup lang="ts">
import {computed,onMounted,ref} from 'vue';
import {Alert as AAlert,Button as AButton,Card as ACard,Empty as AEmpty,Skeleton as ASkeleton} from 'ant-design-vue';
import ProductPurchaseConfigurator from './ProductPurchaseConfigurator.vue';
import {commerceRequest,loginForWebsite,websiteAccessToken,syncWebsiteSession,websiteSignedIn,websiteCustomerOrigin,purchaseDraft,removePurchaseDraft,type Cart,type Quote} from './website-commerce-client';
const route=useRoute(),localePath=useLocalePath(),{locale}=useI18n();
const zh=computed(()=>locale.value!=='en-US');
const copy={ 'zh-CN':{cart:'购物车',checkout:'确认订单',browse:'继续选购',empty:'购物车还是空的',login:'请先登录后结算',signIn:'登录并继续结算',cancelled:'登录未完成，购物车已保留。',error:'操作暂时无法完成，请刷新购物车后重试。',sessionError:'暂时无法读取用户中心登录状态，请重试。',configurationError:'已选配置暂时无法确认，请重试或在下方补全配置。',retry:'重试',remove:'移除',quantity:'数量',cycles:'计费周期数',review:'核对配置和价格',submit:'确认订单并前往付款',subtotal:'商品小计',discount:'优惠',tax:'税费',total:'应付总额',configuration:'请先完成商品配置',completeConfiguration:'补全配置',reconfigure:'查看商品',expired:'报价已失效，请重新核价。',payment:'订单已创建，正在进入账单付款',note:'确认订单后进入用户中心账单，选择支付方式。',selected:'已选配置',preview:'加入购物车时核价',finalItem:'本项最终应付',changed:'本项最终金额与加入购物车时不同，请核对配置和费用后再确认。',previewNote:'最终金额以结算核价为准。'},'en-US':{cart:'Cart',checkout:'Confirm order',browse:'Continue shopping',empty:'Your cart is empty',login:'Sign in before checking out',signIn:'Sign in and continue',cancelled:'Sign-in was not completed. Your cart is preserved.',error:'Could not complete the operation. Refresh your cart and retry.',sessionError:'Could not read your customer sign-in status. Please retry.',configurationError:'Could not confirm the selected configuration. Retry or complete it below.',retry:'Retry',remove:'Remove',quantity:'Quantity',cycles:'Billing cycles',review:'Review configuration and price',submit:'Place order and proceed to payment',subtotal:'Subtotal',discount:'Discount',tax:'Tax',total:'Total due',configuration:'Complete product configuration first',completeConfiguration:'Complete configuration',reconfigure:'View product',expired:'The quote expired. Refresh pricing.',payment:'Order created. Opening your payment invoice',note:'After confirmation, choose a payment method on your customer invoice.',selected:'Selected configuration',preview:'Price when added',finalItem:'Final item total',changed:'The final item total differs from its original preview. Review the configuration and charges before confirming.',previewNote:'Checkout pricing determines the final amount.'}};
const text=computed(()=>copy[zh.value?'zh-CN':'en-US']);
const cart=ref<Cart>(),quote=ref<Quote>(),loading=ref(true),busy=ref(false),error=ref(''),signedIn=websiteSignedIn,loginCancelled=ref(false),orderNo=ref(''),configuringItem=ref('');
const isCheckout=computed(()=>route.path.replace(/^\/en-US/,'')==='/checkout');
const pending=computed(()=>quote.value?.items.some(item=>item.configurationRequired&&!item.configurationFrozen));
function amount(value:string,currency=quote.value?.currencyCode||cart.value?.currencyCode||'CNY'){
 const digits=new Intl.NumberFormat(locale.value,{style:'currency',currency}).resolvedOptions().maximumFractionDigits??2;
 const minor=BigInt(value||'0'),divisor=10n**BigInt(digits),absolute=minor<0n?-minor:minor;
 return `${currency} ${minor<0n?'-':''}${absolute/divisor}${digits?'.'+String(absolute%divisor).padStart(digits,'0'):''}`;
}
const itemQuotes=computed(()=>new Map((quote.value?.items||[]).map(item=>[item.cartItemNo,item])));
function itemDraft(item:Cart['items'][number]){return purchaseDraft(item.configurationReference);}
function finalItem(item:Cart['items'][number]){const row=itemQuotes.value.get(item.cartItemNo);return row&&(!row.configurationRequired||row.configurationFrozen)&&row.totalAmountMinor!==undefined?row:undefined;}
function priceChanged(item:Cart['items'][number]){const preview=itemDraft(item)?.preview,row=finalItem(item);return Boolean(preview&&row&&preview.currencyCode===quote.value?.currencyCode&&BigInt(preview.totalAmountMinor)!==BigInt(row.totalAmountMinor));}
function name(item:Cart['items'][number]){return zh.value?item.productNameZhCN||item.productNameEnUS:item.productNameEnUS||item.productNameZhCN;}
async function load(){loading.value=true;error.value='';try{
 await syncWebsiteSession();
 if(signedIn.value){await commerceRequest('account/commerce/guest-cart-merge','POST');cart.value=(await commerceRequest<{cart:Cart}>('account/commerce/cart')).cart;}
 else cart.value=(await commerceRequest<{cart:Cart}>('public/commerce/cart')).cart;
 quote.value=undefined;
}catch(cause){signedIn.value=Boolean(websiteAccessToken());error.value=cause instanceof Error&&cause.message==='session_unavailable'?text.value.sessionError:text.value.error;}finally{loading.value=false;}if(isCheckout.value&&signedIn.value&&cart.value?.items.length&&!error.value)await review();}
async function signIn(returnTo?:string){busy.value=true;loginCancelled.value=false;try{if(await loginForWebsite(returnTo)){signedIn.value=true;await load();}}catch{error.value=text.value.sessionError;}finally{busy.value=false;}}
async function remove(item:Cart['items'][number]){if(!cart.value||busy.value)return;busy.value=true;error.value='';try{
 const base=signedIn.value?'account':'public';const query=new URLSearchParams({expectedCartVersion:String(cart.value.version),expectedItemVersion:String(item.version)});
 cart.value=(await commerceRequest<{cart:Cart}>(`${base}/commerce/cart-items/${encodeURIComponent(item.cartItemNo)}?${query}`,'DELETE')).cart;removePurchaseDraft(item.configurationReference);quote.value=undefined;if(isCheckout.value&&cart.value.items.length)await createQuote();
}catch{signedIn.value=Boolean(websiteAccessToken());error.value=text.value.error;}finally{busy.value=false;}}
async function refreshQuoteState(){
 const current=(await commerceRequest<{cart:Cart}>('account/commerce/cart')).cart;
 if(current){cart.value=current;if(current.latestQuote)quote.value=current.latestQuote;}
}
async function createQuote(){
 if(!cart.value)return;
 quote.value=(await commerceRequest<{quote:Quote}>('account/commerce/quotes','POST',{expectedCartVersion:cart.value.version,assetNumbers:[]})).quote;
 cart.value.version=quote.value.cartVersion;
 for(const item of quote.value.items){
  if(!item.configurationRequired||item.configurationFrozen)continue;
  const reference=cart.value.items.find(row=>row.cartItemNo===item.cartItemNo)?.configurationReference;
  const draft=purchaseDraft(reference);
  if(!draft)continue;
  try{await commerceRequest(`account/commerce/quote-items/${encodeURIComponent(item.quoteItemNo)}/purchase-configuration`,'POST',{publicSlug:draft.publicSlug,configurationRevision:draft.configurationRevision,values:draft.values});}
  catch{error.value=text.value.configurationError;}
 }
 await refreshQuoteState();
}
async function review(){
 if(!signedIn.value){await signIn(localePath('/checkout'));if(!signedIn.value)return;}
 if(!cart.value||quote.value)return;
 if(!isCheckout.value){await navigateTo(localePath('/checkout'));return;}
 busy.value=true;error.value='';
 try{await createQuote();}catch{signedIn.value=Boolean(websiteAccessToken());error.value=text.value.error;}finally{busy.value=false;}
}
async function configurationSaved(){error.value='';configuringItem.value='';try{await refreshQuoteState();}catch{error.value=text.value.error;}}
async function submit(){if(!quote.value||pending.value||busy.value)return;if(quote.value.expiresAt<=Date.now()){error.value=text.value.expired;return;}busy.value=true;error.value='';try{
 const key='lina.website.checkout.'+quote.value.quoteNo;let action=sessionStorage.getItem(key);if(!action){action=crypto.randomUUID();sessionStorage.setItem(key,action);}
 const result=await commerceRequest<{checkout:{orderNo:string;invoiceNo:string}}>('account/commerce/orders','POST',{quoteNo:quote.value.quoteNo,idempotencyKey:action});orderNo.value=result.checkout.orderNo;
 // The invoice is a customer-template route at this origin. The login popup
 // established that template's real session; never put tokens into a redirect.
 for(const item of cart.value?.items||[])removePurchaseDraft(item.configurationReference);
 window.location.assign((await websiteCustomerOrigin())+'/account/billing/invoices/'+encodeURIComponent(result.checkout.invoiceNo)+'?pay=1');
}catch{signedIn.value=Boolean(websiteAccessToken());error.value=text.value.error;}finally{busy.value=false;}}
onMounted(load);
</script>
<template>
 <section class="website-checkout" data-testid="website-cart">
  <header><div><h1>{{isCheckout?text.checkout:text.cart}}</h1><p>{{text.note}}</p></div><NuxtLink :to="localePath('/products')">{{text.browse}}</NuxtLink></header>
  <AAlert v-if="error" :message="error" type="error" show-icon class="notice"><template #action><AButton @click="load">{{text.retry}}</AButton></template></AAlert>
  <ASkeleton v-if="loading" active />
  <template v-else>
   <AAlert v-if="!signedIn" :message="text.login" type="info" show-icon class="notice"><template #action><AButton :loading="busy" @click="signIn()">{{text.signIn}}</AButton></template></AAlert>
   <AAlert v-if="loginCancelled" :message="text.cancelled" type="warning" show-icon />
   <AEmpty v-if="!cart?.items?.length" :description="text.empty" />
   <div v-else class="checkout-layout">
    <div class="items"><ACard v-for="item in cart.items" :key="item.cartItemNo" :data-testid="'website-cart-item-'+item.cartItemNo"><h2>{{name(item)}}</h2><p>{{text.quantity}}：{{item.quantity}} · {{text.cycles}}：{{item.purchaseCycles}}</p><div v-if="itemDraft(item)?.summary?.length" class="selected-configuration" data-testid="website-cart-configuration"><h3>{{text.selected}}</h3><dl><div v-for="(field,index) in itemDraft(item)!.summary" :key="index"><dt>{{zh?field.labelZhCN:field.labelEnUS}}</dt><dd>{{zh?field.valueZhCN:field.valueEnUS}}</dd></div></dl></div><p v-if="itemDraft(item)?.preview" class="item-price" data-testid="website-cart-preview">{{text.preview}}：{{amount(itemDraft(item)!.preview!.totalAmountMinor,itemDraft(item)!.preview!.currencyCode)}} · {{zh?itemDraft(item)!.preview!.termZhCN:itemDraft(item)!.preview!.termEnUS}}<small>{{text.previewNote}}</small></p><p v-if="finalItem(item)" class="item-price final-price" data-testid="website-cart-final-item">{{text.finalItem}}：{{amount(finalItem(item)!.totalAmountMinor)}}</p><AAlert v-if="priceChanged(item)" type="warning" show-icon :message="text.changed" /><AButton v-if="quote?.items.some(row=>row.cartItemNo===item.cartItemNo&&row.configurationRequired&&!row.configurationFrozen)&&configuringItem!==item.cartItemNo" @click="configuringItem=item.cartItemNo">{{text.completeConfiguration}}</AButton><ProductPurchaseConfigurator v-if="configuringItem===item.cartItemNo&&quote?.items.some(row=>row.cartItemNo===item.cartItemNo&&row.configurationRequired&&!row.configurationFrozen)&&item.publicSlug" :public-slug="item.publicSlug" :quantity="item.quantity" :server-login-token="item.serverLoginToken" :server-login-hostname="item.serverLoginHostname" :server-login-password-configured="item.serverLoginPasswordConfigured" :currencies="[cart.currencyCode]" :locale="zh?'zh-CN':'en-US'" :quote-item-no="quote.items.find(row=>row.cartItemNo===item.cartItemNo)?.quoteItemNo" @configured="configurationSaved"/><div class="item-actions"><NuxtLink :to="localePath('/products/'+encodeURIComponent(item.publicSlug))">{{text.reconfigure}}</NuxtLink><AButton :disabled="busy" danger @click="remove(item)">{{text.remove}}</AButton></div></ACard></div>
    <ACard><h2>{{text.checkout}}</h2><dl v-if="quote" data-testid="website-final-quote"><div v-for="row in [['subtotal',quote.subtotalMinor],['discount',quote.discountMinor],['tax',quote.taxAmountMinor],['total',quote.totalAmountMinor]]" :key="row[0]"><dt>{{text[row[0] as 'total']}}</dt><dd>{{amount(row[1]!)}}</dd></div></dl>
     <AAlert v-if="pending" :message="text.configuration" type="warning" show-icon />
     <AButton v-if="!quote" :loading="busy" type="primary" block @click="review">{{signedIn?text.review:text.signIn}}</AButton>
     <AButton v-else :disabled="pending||Boolean(orderNo)" :loading="busy" type="primary" block data-testid="website-submit-order" @click="submit">{{text.submit}}</AButton>
     <p v-if="orderNo">{{text.payment}}：{{orderNo}}</p>
    </ACard>
   </div>
  </template>
 </section>
</template>
<style scoped>
.website-checkout{max-width:1200px;margin:0 auto;padding:32px 24px 64px}.website-checkout header{display:flex;justify-content:space-between;gap:20px;margin-bottom:24px}.website-checkout h1{font-size:26px;margin:0 0 10px}.website-checkout h2{font-size:18px;margin:0 0 16px}.website-checkout p{color:#64748b;line-height:1.7}.notice{margin-bottom:20px}.checkout-layout{display:grid;grid-template-columns:minmax(0,1fr) 360px;gap:24px;align-items:start}.items{display:grid;gap:16px}.selected-configuration{margin:16px 0}.selected-configuration h3{font-size:14px;margin:0}.selected-configuration dl{margin:0}.selected-configuration dl div{padding:6px 0;font-size:14px;font-weight:normal}.selected-configuration dd{overflow-wrap:anywhere}.item-price small{display:block;font-size:12px}.final-price{font-weight:600;color:#0f172a}.item-actions{display:flex;align-items:center;justify-content:space-between;margin-top:20px}dl div{display:flex;justify-content:space-between;gap:20px;padding:12px 0;border-bottom:1px solid #e5e7eb}dl div:last-child{font-size:18px;font-weight:600}dd{margin:0}@media(max-width:760px){.checkout-layout{grid-template-columns:1fr}.website-checkout{padding:24px 16px}.website-checkout header{flex-wrap:wrap}}
</style>
