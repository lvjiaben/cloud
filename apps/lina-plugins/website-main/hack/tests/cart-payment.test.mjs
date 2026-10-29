import test from 'node:test';
import assert from 'node:assert/strict';
import {createCartPayment,paymentStorageKey,paymentURL,browserModes} from '../../frontend/public/lina/cart-payment.js';
const storage=()=>{const data=new Map();return{getItem:key=>data.get(key),setItem:(key,value)=>data.set(key,value),removeItem:key=>data.delete(key)};};
test('channel payment uses only the authoritative prepared context and never posts twice after unknown submission',async()=>{
 const calls=[],store=storage();
 const api=async(path,method='GET',body)=>{calls.push({path,method,body});if(path.endsWith('payment-preparations'))return{preparation:{ready:true,statusCode:'ready',contextNo:'CTX'}};if(path==='account/payments')throw new Error('lost response');if(path.includes('/invoices/'))return{detail:{invoice:{statusCode:'pending_payment'}}};return{order:{summary:{statusCode:'payment_processing'}}};};
 const flow=createCartPayment(api,{storage:store,key:'owner-a'});flow.begin({orderNo:'ORD',invoiceNo:'INV'});
 const choice={channelNo:'CHANNEL',modeCode:'wechat_native',balance:false};await assert.rejects(flow.pay(choice));
 const restored=createCartPayment(api,{storage:store,key:'owner-a'});await restored.pay(choice);assert.equal(restored.state.phase,'unknown');
 assert.deepEqual(calls.filter(c=>c.path==='account/payments'),[{path:'account/payments',method:'POST',body:{paymentContextNo:'CTX',channelNo:'CHANNEL',modeCode:'wechat_native',balanceAmountMinor:'0'}}]);
 assert.equal(createCartPayment(api,{storage:store,key:'other-owner'}).state,undefined);
});
test('identity/review decisions do not trigger payment writes or impersonate expired authentication',async()=>{
 for(const status of ['awaiting_identity','awaiting_order_review']){const calls=[];const flow=createCartPayment(async(path)=>{calls.push(path);return{preparation:{ready:false,statusCode:status,contextNo:'CTX'}}});flow.begin({orderNo:'ORD',invoiceNo:'INV'});await flow.pay({channelNo:'A',modeCode:'wechat_h5'});assert.equal(flow.state.phase,status);assert.equal(calls.length,1);assert.equal(flow.state.submitted,false);}
});
test('pure balance settles through the owned invoice contract and records canonical success',async()=>{
 const calls=[];const flow=createCartPayment(async(path,method,body)=>{calls.push({path,method,body});return method==='POST'?{payment:{statusCode:'paid',outstandingAmountMinor:'0'}}:{detail:{invoice:{canPayFromBalance:true,statusCode:'pending_payment'}}}});flow.begin({orderNo:'ORD',invoiceNo:'INV'});await flow.pay({balance:true});assert.equal(flow.state.phase,'paid');assert.equal(calls.length,2);assert.equal(calls[1].path,'account/billing/invoices/INV/balance-payments');assert.ok(calls[1].body.idempotencyKey.length>=16);
});
test('payment result is confirmed only from owned server state, not from the browser redirect',async()=>{
 let paid=false;
 const flow=createCartPayment(async(path)=>path.endsWith('payment-preparations')?{preparation:{ready:true,contextNo:'CTX',statusCode:'ready'}}:path==='account/payments'?{attemptNo:'PAY',redirectMethod:'GET',redirectUrl:'https://pay.example/'}:path.includes('/invoices/')?{detail:{invoice:{statusCode:paid?'paid':'pending_payment'}}}:path.startsWith('account/payments/')?{statusCode:'confirming'}:{order:{summary:{statusCode:'payment_processing'}}});
 flow.begin({orderNo:'ORD',invoiceNo:'INV'});await flow.pay({channelNo:'A',modeCode:'alipay_page'});assert.equal(flow.state.phase,'waiting');await flow.check();assert.equal(flow.state.phase,'waiting');paid=true;await flow.check();assert.equal(flow.state.phase,'paid');
});
test('payment handoff URLs reject executable schemes and mode selection excludes identity-only JSAPI',async()=>{
 assert.equal(paymentURL('javascript:alert(1)'),'');assert.equal(paymentURL('https://name:secret@pay.example/'),'');assert.equal(paymentURL('https://pay.example/a'),'https://pay.example/a');assert.deepEqual(browserModes({modes:['wechat_jsapi','jsapi','app','wechat_native','alipay_page']}),['wechat_native','alipay_page']);assert.doesNotMatch(await paymentStorageKey('private-access-token'),/private-access-token/);
});

test('the canonical invoice can refuse balance payment before any financial POST',async()=>{
 const calls=[];const flow=createCartPayment(async(path,method='GET')=>{calls.push({path,method});return{detail:{invoice:{statusCode:'pending_payment',canPayFromBalance:false}}};});flow.begin({orderNo:'ORD',invoiceNo:'INV'});await flow.pay({balance:true});assert.equal(flow.state.phase,'balance_unavailable');assert.equal(flow.state.submitted,false);assert.equal(calls.filter(call=>call.method==='POST').length,0);
});
test('a real unauthorized payment clears the submission lock for an explicit login recovery',async()=>{
 let active=true;const flow=createCartPayment(async path=>{if(path.endsWith('payment-preparations'))return{preparation:{ready:true,statusCode:'ready',contextNo:'CTX'}};active=false;throw new Error('unauthorized');},{authenticated:()=>active});flow.begin({orderNo:'ORD',invoiceNo:'INV'});await assert.rejects(flow.pay({channelNo:'A',modeCode:'wechat_native'}));assert.equal(flow.state.phase,'login_required');assert.equal(flow.state.submitted,false);
});
