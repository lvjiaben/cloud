// Payment writes occur only on an explicit customer action. Unknown submissions are read-only.
export const browserModes=channel=>(channel.modes||[]).filter(mode=>!['jsapi','app','wechat_jsapi'].includes(mode));
export function paymentURL(value){try{const url=new URL(value);return ['http:','https:'].includes(url.protocol)&&!url.username&&!url.password?url.href:'';}catch{return '';}}
export async function paymentStorageKey(identity){const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(identity));return 'website-main-payment-'+Array.from(new Uint8Array(digest),v=>v.toString(16).padStart(2,'0')).join('');}
export function createCartPayment(request,{storage,key,authenticated=()=>true}={}){
 let current;
 try{const saved=JSON.parse(storage?.getItem(key)||'null');if(saved?.orderNo&&saved?.invoiceNo)current=saved;}catch{}
 const save=()=>{try{if(current)storage?.setItem(key,JSON.stringify(current));else storage?.removeItem(key);}catch{/* In-memory state still prevents duplicate writes while this page is open. */}};
 const paid=invoice=>invoice?.statusCode==='paid';
 async function check(){
  if(!current)return;
  const paths=[`account/billing/invoices/${encodeURIComponent(current.invoiceNo)}`,`account/commerce/orders/${encodeURIComponent(current.orderNo)}`];
  if(current.created?.attemptNo)paths.push(`account/payments/${encodeURIComponent(current.created.attemptNo)}`);
  const results=await Promise.allSettled(paths.map(path=>request(path,'GET',undefined,undefined,path.startsWith('account/payments/')?'idc-payment':'idc-core')));
  const invoice=results[0].status==='fulfilled'?results[0].value.detail?.invoice:undefined;
  const order=results[1].status==='fulfilled'?results[1].value.order:undefined;
  const attempt=results[2]?.status==='fulfilled'?results[2].value:undefined;
  if(invoice)current.invoice=invoice;
  if(attempt)current.attempt=attempt;
  if(paid(invoice)||paid(order?.invoice)){current.phase='paid';save();try{storage?.removeItem(key);}catch{}return current;}
  if(!authenticated()){current.phase='login_required';save();return current;}
  if(results.every(result=>result.status==='rejected'))throw results[0].reason;
  if(attempt&&['failed','closed','expired'].includes(attempt.statusCode)){
   current.phase='failed';current.submitted=false;
  }else if(attempt&&['manual_review','disputed','late_success','result_unknown'].includes(attempt.statusCode))current.phase='unknown';
  else if(current.submitted)current.phase=current.created?'waiting':'unknown';
  if(invoice?.statusCode==='cancelled'||order?.summary?.statusCode==='cancelled')current.phase='cancelled';
  save();return current;
 }
 async function pay(choice){
  if(!current)return;
  if(current.submitted)return check();
  current.choice=choice;save();
  if(choice.balance){
   const {detail}=await request(`account/billing/invoices/${encodeURIComponent(current.invoiceNo)}`);
   current.invoice=detail?.invoice;
   if(paid(current.invoice)){current.phase='paid';save();try{storage?.removeItem(key);}catch{}return current;}
   if(!current.invoice?.canPayFromBalance){current.phase='balance_unavailable';save();return current;}
  }else{
   const path=current.preparation?.contextNo?`account/commerce/payment-preparations/${encodeURIComponent(current.preparation.contextNo)}`:`account/commerce/orders/${encodeURIComponent(current.orderNo)}/payment-preparations`;
   const result=await request(path,current.preparation?.contextNo?'PUT':'POST');
   current.preparation=result.preparation;current.phase=result.preparation.statusCode;save();
   if(!result.preparation.ready)return current;
  }
  // Persist before issuing a financial POST. A lost response must never cause an automatic replay.
  current.submitted=true;current.phase='unknown';current.created=undefined;current.attempt=undefined;save();
  try{
   if(choice.balance){
    current.balanceKey ||= crypto.randomUUID();save();
    const result=await request(`account/billing/invoices/${encodeURIComponent(current.invoiceNo)}/balance-payments`,'POST',{idempotencyKey:current.balanceKey});
    current.invoice=result.payment;current.phase=paid(result.payment)?'paid':'waiting';
   }else{
    current.created=await request('account/payments','POST',{paymentContextNo:current.preparation.contextNo,channelNo:choice.channelNo,modeCode:choice.modeCode,balanceAmountMinor:'0'},undefined,'idc-payment');
    current.phase='waiting';
   }
   save();if(current.phase==='paid')try{storage?.removeItem(key);}catch{}
   return current;
  }catch(error){
   if(!authenticated()){current.submitted=false;current.phase='login_required';}
   save();throw error;
  }
 }
 return {
  get state(){return current;},
  begin(order,ui){if(current&&current.orderNo===order.orderNo)return current;current={orderNo:order.orderNo,invoiceNo:order.invoiceNo,phase:'order_created',submitted:false,ui};save();return current;},
  pay,check,
 };
}
