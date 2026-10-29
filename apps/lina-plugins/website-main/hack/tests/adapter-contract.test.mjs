// Controlled adapter regression tests only. Responses below are test inputs,
// never published products, backend seed data or evidence of real transactions.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {parseHTML} from 'linkedom';

async function settle(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
async function adapter(t,page,respond){
 const {document,window}=parseHTML(await readFile(new URL('../../frontend/public/'+page+'.html',import.meta.url),'utf8'));
 const calls=[],redirects=[],stored=new Map();
 let completeRedirect;const redirected=new Promise(resolve=>{completeRedirect=resolve;});
 const sessionStorage={getItem:key=>stored.get(String(key))??null,setItem:(key,value)=>stored.set(String(key),String(value)),removeItem:key=>stored.delete(String(key)),clear:()=>stored.clear()};
 // LinkeDOM exposes a getter-only select.value; browsers also support assignment.
 // Scope the missing behavior to this document rather than changing its shared prototype.
 const createElement=document.createElement.bind(document);
 document.createElement=(tag,...args)=>{
  const element=createElement(tag,...args);
  if(tag.toLowerCase()==='select')Object.defineProperty(element,'value',{
   configurable:true,
   get(){return [...this.options].find(option=>option.selected)?.value||'';},
   set(value){for(const option of this.options)option.selected=false;const selected=[...this.options].find(option=>option.value===String(value));if(selected)selected.selected=true;},
  });
  if(tag.toLowerCase()==='select')Object.defineProperty(element,'selectedOptions',{get(){return [...this.options].filter(option=>option.selected);}});
  return element;
 };
 const properties={document,window,location:{href:'http://localhost:16682/'+page+'.html?product=contract-input',protocol:'http:',origin:'http://localhost:16682',hostname:'localhost',pathname:'/'+page+'.html',search:'?product=contract-input',assign:u=>{redirects.push(u);completeRedirect(u);}},history:{replaceState(){}},Option:function(text,value){const opt=document.createElement('option');opt.textContent=text;opt.value=value;return opt;},fetch:async(path,options)=>{calls.push({path,body:options.body?JSON.parse(options.body):undefined});if(path==='/__website/session')return {ok:true,status:200,json:async()=>({checked:true,accessToken:'controlled-adapter-session'})};const data=await respond(path,options);return {ok:true,status:200,json:async()=>({code:0,data})};}};
 // LinkeDOM has no layout engine; browser layout is covered by Playwright.
 properties.ResizeObserver=class{observe(){} disconnect(){}};
 properties.MutationObserver=class{observe(){} disconnect(){}};
 properties.innerWidth=1280;
 for(const element of document.querySelectorAll('*'))element.getBoundingClientRect=()=>({left:0,width:1280,height:72});
 window.__LINA_TEMPLATE__={customerUrl:'http://localhost:6032'};
 properties.sessionStorage=sessionStorage;
 const previous=new Map(Object.keys(properties).map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
 for(const [key,value]of Object.entries(properties))Object.defineProperty(globalThis,key,{value,configurable:true,writable:true});
 t.after(()=>{window.dispatchEvent(new window.Event('pagehide'));for(const [key,descriptor]of previous)if(descriptor)Object.defineProperty(globalThis,key,descriptor);else delete globalThis[key];});
 if(document.querySelector('form'))document.querySelector('form').reportValidity=()=>true;
 await import(new URL('../../frontend/public/lina/site.js?test='+Math.random(),import.meta.url));await settle();
 return {document,window,calls,redirects,redirected};
}
const brand={siteName:{zhCN:'契约测试输入'},navigation:[],footerNavigation:[],footer:{zhCN:''}};
const revision='r'.repeat(43);
const view={configurationRevision:revision,fields:[],values:[]};
function purchaseResponses(status='final',stale=false,currencies=['CNY']){return path=>{
 if(path.includes('/brand'))return {brand};
 if(path.endsWith('/evaluate'))return {...view,valid:true};
 if(path.endsWith('/quote'))return {configurationRevision:stale?'s'.repeat(43):revision,currencyCode:'CNY',terms:[{billingTermToken:'opaque-test-term',labelZhCN:'契约输入周期',minimumPurchaseCycles:2,status,totalAmount:'9007199254740993'}]};
 if(path.endsWith('/purchase-configuration'))return view;
 if(path.endsWith('/commerce/carts')||path.endsWith('/commerce/cart-items'))return {cart:{version:1,items:[{configurationReference:'controlled-unit-input',publicSlug:'contract-input'}]}};
 return {product:{publicSlug:'contract-input',nameZhCN:'契约输入商品',availability:'available',currencies}};
};}
test('controlled adapter: first backend currency, selected term and hidden counts reach cart API',{timeout:3000},async t=>{
 const app=await adapter(t,'configure',purchaseResponses('final',false,['CNY','USD']));const d=app.document;
 d.querySelector('#addCartForm').reportValidity=()=>true;
 const term=d.querySelector('[data-quote] button');assert.equal(term.textContent,'契约输入周期');
 assert.equal(d.querySelector('[data-total-amount]').textContent,'¥90071992547409.93');
 assert.equal(d.querySelector('[data-purchase-controls] [aria-label="币种"]'),null);
 assert.equal(d.querySelector('[data-purchase-controls] .sky-config-row'),null);
 term.dispatchEvent(new app.window.Event('click'));d.querySelector('[name=quantity]').value='3';d.querySelector('[name=cycles]').value='4';
 d.querySelector('#addCartForm').dispatchEvent(new app.window.Event('submit',{cancelable:true}));await settle();
 await app.redirected;
 const sent=app.calls.find(call=>call.path.endsWith('/commerce/cart-items'));
 assert.deepEqual(sent.body,{expectedCartVersion:1,quantity:1,purchaseCycles:4,publicSlug:'contract-input',configurationRevision:revision,billingTermToken:'opaque-test-term',currencyCode:'CNY',values:[]});
 assert.deepEqual(app.redirects,['/cart.html?lang=zh-CN']);
});
test('controlled adapter: usage and customer-context terms cannot be added as final prices',async t=>{
 for(const [status,text]of [['pending_usage',/按实际用量结算/],['requires_customer_context',/登录/]])await t.test(status,async t=>{
  const app=await adapter(t,'configure',purchaseResponses(status));
  assert.match(app.document.querySelector('[data-quote]').textContent,text);
  // Customer-context terms use same-tab sign-in; no unpriced cart submission.
  if(status==='pending_usage')assert.equal(app.document.querySelector('[data-quote] button').disabled,true);
  else{assert.equal(app.document.querySelector('[data-quote] a'),null);assert.equal(app.document.querySelector('[data-quote] button').disabled,false);}
  assert.equal(app.document.querySelector('[data-add]').disabled,true);
  app.document.querySelector('#addCartForm').dispatchEvent(new app.window.Event('submit',{cancelable:true}));await settle();
  assert.equal(app.calls.some(call=>call.path.endsWith('/commerce/cart-items')),false);
 });
});
test('controlled adapter: mismatched quote revision never enables a stale purchase',async t=>{
 const app=await adapter(t,'configure',purchaseResponses('final',true));
 assert.equal(app.document.querySelector('[data-add]').disabled,true);
 assert.equal(app.document.querySelector('[data-quote]').children.length,0);
 assert.match(app.document.querySelector('[data-status]').textContent,/重新读取配置/);
});
test('controlled adapter: tree selection sends one opaque token while retaining parent and child choices',async t=>{
 const field={fieldToken:'tree-field',type:'tree_select',visible:true,labelZhCN:'网络类型',options:[{optionToken:'parent-token',labelZhCN:'父选项',children:[{optionToken:'child-token',labelZhCN:'子选项'}]}]};
 const configured={...view,fields:[field],values:[{fieldToken:field.fieldToken,type:field.type,treeOptionTokens:['child-token']}]};
 const normal=purchaseResponses();
 const app=await adapter(t,'configure',path=>path.endsWith('/purchase-configuration')?configured:path.endsWith('/evaluate')?{...configured,valid:true}:normal(path));
 const select=app.document.querySelector('#field-tree-field');
 assert.equal(select.multiple,false);assert.equal(select.value,'child-token');
 assert.deepEqual([...select.options].map(o=>o.value),['parent-token','child-token']);
 select.value='parent-token';select.dispatchEvent(new app.window.Event('change'));
 await new Promise(resolve=>setTimeout(resolve,350));await settle();
 const sent=app.calls.filter(c=>c.path.endsWith('/evaluate')).at(-1);
 assert.deepEqual(sent.body.values,[{fieldToken:field.fieldToken,type:field.type,treeOptionTokens:['parent-token']}]);
});

