// Reuse the customer application's authoritative browser session, including
// when it is hosted at a different configured origin. Never put tokens in URLs.
import { ref } from 'vue';
import type { PurchaseConfigurationField, PurchaseConfigurationValue, PurchaseConfigurationOption } from './purchase-configuration-client';
export interface CartItem {hasServerLogin?:boolean;serverLoginToken?:string;serverLoginHostname?:string;serverLoginPasswordConfigured?:boolean;cartItemNo:string;configurationReference?:string;productNameZhCN:string;productNameEnUS:string;publicSlug:string;quantity:number;purchaseCycles:number;version:number;configurationRequired?:boolean;configurationFrozen?:boolean}
export interface Cart {reference?:string;cartNo?:string;currencyCode:string;version:number;items:CartItem[];latestQuote?:Quote}
export interface Quote {quoteNo:string;cartVersion:number;currencyCode:string;subtotalMinor:string;discountMinor:string;taxAmountMinor:string;totalAmountMinor:string;expiresAt:number;items:Array<CartItem & {quoteItemNo:string;totalAmountMinor:string}>}
export const websiteSignedIn=ref(false);
let accessToken='';
let checking:Promise<boolean>|undefined;
let customerOrigin='';
let handoffChecked=false;
let handoffSession=false;
export function websiteAccessToken(){return import.meta.client?accessToken:'';}
export function clearWebsiteSession(){accessToken='';websiteSignedIn.value=false;handoffSession=false;}
export async function websiteCustomerOrigin(){
 if(customerOrigin)return customerOrigin;
 const response=await fetch('/__website/configuration',{credentials:'same-origin'});
 if(!response.ok)throw new Error('session_unavailable');
 const config=await response.json();const url=new URL(config.customerOrigin||location.origin);
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw new Error('session_unavailable');
 customerOrigin=url.origin;return customerOrigin;
}
export async function syncWebsiteSession():Promise<boolean>{
 if(!import.meta.client)return false;
 if(checking)return checking;
 checking=(async()=>{
  const origin=await websiteCustomerOrigin();
  if(new URL(origin).hostname!==location.hostname){
   if(handoffSession)return Boolean(accessToken);
   if(!handoffChecked){
    const response=await fetch('/__website/session',{credentials:'same-origin'});
    if(!response.ok)throw new Error('session_unavailable');
    const session=await response.json();handoffChecked=Boolean(session.checked);
    if(handoffChecked){accessToken=session.accessToken||'';websiteSignedIn.value=Boolean(accessToken);handoffSession=Boolean(accessToken);return websiteSignedIn.value;}
   }
  }
  return new Promise<boolean>((resolve,reject)=>{
   const nonce=crypto.randomUUID(),frame=document.createElement('iframe');
   frame.hidden=true;frame.title='Customer session';frame.referrerPolicy='no-referrer';
   const receive=(event:MessageEvent)=>{
    if(event.origin!==origin||event.source!==frame.contentWindow||event.data?.type!=='lina-website-session'||event.data?.nonce!==nonce)return;
    accessToken=typeof event.data.accessToken==='string'?event.data.accessToken:'';
    websiteSignedIn.value=Boolean(accessToken);cleanup();
    if(!accessToken&&!handoffChecked&&new URL(origin).hostname!==location.hostname){
     window.location.replace(origin+'/auth/website-session?probe=1&returnTo='+encodeURIComponent(location.pathname+location.search)+'&websiteOrigin='+encodeURIComponent(location.origin));
    }
    resolve(websiteSignedIn.value);
   };
   const timeout=window.setTimeout(()=>{cleanup();reject(new Error('session_unavailable'));},20000);
   function cleanup(){window.removeEventListener('message',receive);clearTimeout(timeout);frame.remove();}
   window.addEventListener('message',receive);
   frame.src=origin+'/auth/website-session?nonce='+encodeURIComponent(nonce)+'&websiteOrigin='+encodeURIComponent(location.origin);
   document.body.append(frame);
  });
 })().catch(error=>{clearWebsiteSession();throw error;}).finally(()=>{checking=undefined;});
 return checking;
}
export async function commerceRequest<T>(path:string,method='GET',body?:unknown):Promise<T>{
 const token=websiteAccessToken();
 const response=await fetch('/x/idc-core/api/v1/'+path,{method,credentials:'same-origin',headers:{Accept:'application/json',...(body===undefined?{}:{'Content-Type':'application/json'}),...(token?{Authorization:`Bearer ${token}`}:{})},body:body===undefined?undefined:JSON.stringify(body)});
 const envelope=await response.json();
 if(response.status===401||String(envelope.code)==='401'){clearWebsiteSession();throw new Error('login_required');}
 if(!response.ok||String(envelope.code)!=='0')throw new Error(envelope.errorCode||envelope.data?.errorCode||'commerce_failed');
 return envelope.data;
}
export async function loginForWebsite(returnTo=location.pathname+location.search):Promise<boolean>{
 // Navigation is in this tab; a signed-in customer never needs a second login.
 if(await syncWebsiteSession())return true;
 const origin=await websiteCustomerOrigin();
 window.location.assign(origin+'/auth/website-session?login=1&returnTo='+encodeURIComponent(returnTo)+'&websiteOrigin='+encodeURIComponent(location.origin));
 return false;
}
export interface PurchaseDraft {
 publicSlug:string;
 configurationRevision:string;
 values:PurchaseConfigurationValue[];
 summary?: Array<{labelZhCN:string;labelEnUS:string;valueZhCN:string;valueEnUS:string}>;
 preview?: {currencyCode:string;totalAmountMinor:string;termZhCN:string;termEnUS:string};
}
// This is browser-local display evidence only. Checkout always re-evaluates
// the original values on the server and never trusts the preview as a price.
export function summarizePurchaseConfiguration(fields:PurchaseConfigurationField[],values:PurchaseConfigurationValue[]):NonNullable<PurchaseDraft['summary']> {
 const byToken=new Map(values.map(value=>[value.fieldToken,value]));
 return fields.filter(field=>field.visible).flatMap(field=>{
  const value=byToken.get(field.fieldToken);if(!value)return [];
  const options=new Map<string,PurchaseConfigurationOption>();
  const collect=(items:PurchaseConfigurationOption[])=>{for(const item of items){options.set(item.optionToken,item);if(item.children)collect(item.children);}};
  collect(field.options||[]);
  const selected=value.optionTokens||value.treeOptionTokens||value.addressOptionTokens||[];
  const scalar=value.integerValue||value.decimalValue||value.textValue||value.dateValue||value.dateTimeValue||'';
  const numericLabel=value.integerValue ? field.numericLabels?.[value.integerValue] : undefined;
  const scalarWithUnit=field.unit && scalar ? `${scalar} ${field.unit}` : scalar;
  const zh=selected.length?selected.map(token=>options.get(token)?.labelZhCN||'已选择').join(' / '):value.booleanValue!==undefined?(value.booleanValue?'是':'否'):value.fileTokens?.length?`已选择 ${value.fileTokens.length} 个文件`:numericLabel?.zhCN||scalarWithUnit;
  const en=selected.length?selected.map(token=>options.get(token)?.labelEnUS||'Selected').join(' / '):value.booleanValue!==undefined?(value.booleanValue?'Yes':'No'):value.fileTokens?.length?`${value.fileTokens.length} files selected`:numericLabel?.enUS||scalarWithUnit;
  return zh?[{labelZhCN:field.labelZhCN,labelEnUS:field.labelEnUS,valueZhCN:zh,valueEnUS:en}]:[];
 });
}
const draftKey='lina.website.purchase-drafts';
export function purchaseDraft(reference?:string):PurchaseDraft|undefined{
 if(!reference)return;
 try{return JSON.parse(sessionStorage.getItem(draftKey)||'{}')[reference];}catch{return;}
}
export function savePurchaseDraft(reference:string,draft:PurchaseDraft){
 const drafts=JSON.parse(sessionStorage.getItem(draftKey)||'{}');drafts[reference]=draft;
 const entries=Object.entries(drafts).slice(-100);sessionStorage.setItem(draftKey,JSON.stringify(Object.fromEntries(entries)));
}
export function removePurchaseDraft(reference?:string){if(!reference)return;try{const drafts=JSON.parse(sessionStorage.getItem(draftKey)||'{}');delete drafts[reference];sessionStorage.setItem(draftKey,JSON.stringify(drafts));}catch{/* No recoverable draft. */}}
