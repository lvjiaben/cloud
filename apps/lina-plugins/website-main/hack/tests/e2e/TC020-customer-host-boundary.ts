import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/WebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032',apiOrigin:'http://127.0.0.1:6030'});
import {readFile} from 'node:fs/promises';

// Controlled browser contracts only. Every possible cart/handoff write is
// intercepted before it can reach the backend; no business data is created.
const slug='controlled-host-boundary';
const root='/x/idc-core/api/v1/public/catalog/products/'+slug;
const revision='r'.repeat(43);
const note={fieldToken:'note',type:'single_line',labelZhCN:'交付备注',visible:true};
const values=[{fieldToken:'note',type:'single_line',textValue:'保留当前选择'}];

test.describe('TC-20 用户中心主机安全边界',()=>{
 for(const scenario of [{flow:'cart',boundary:'host',id:'a'},{flow:'handoff',boundary:'host',id:'b'},{flow:'cart',boundary:'protocol',id:'c'},{flow:'cart',boundary:'same',id:'d'}] as const){
  const {flow,boundary,id}=scenario;
  test(`TC-20${id}: ${boundary==='same'?'同主机同协议允许受控加车交接':`${boundary==='host'?'主机':'协议'}不一致时拒绝${flow==='cart'?'加车':'配置保存'}，保留输入且无写请求`}`,async({page,baseURL})=>{
   const site=new WebsitePage(page);let writes=0;
   // Current source regression only; this does not deploy or upgrade the plugin.
   await page.route('**/lina/site.js',async route=>route.fulfill({contentType:'application/javascript',body:await readFile(new URL('../../../frontend/public/lina/site.js',import.meta.url),'utf8')}));
   const view={configurationRevision:revision,fields:flow==='handoff'?[note,{fieldToken:'file',type:'file',labelZhCN:'附件',visible:true,required:true}]:[note],values};
   await page.route('**'+root,route=>route.fulfill({json:{code:0,data:{product:{publicSlug:slug,nameZhCN:'受控主机边界验证',availability:'available',currencies:['CNY']}}}}));
   await page.route('**'+root+'/purchase-configuration',route=>route.fulfill({json:{code:0,data:view}}));
   await page.route('**'+root+'/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{...view,valid:flow==='cart'}}}));
   await page.route('**'+root+'/purchase-configuration/quote',route=>route.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'controlled-term',labelZhCN:'受控周期',minimumPurchaseCycles:1,status:'final',totalAmount:'100'}]}}}));
   await page.route('**/public/commerce/**',route=>{writes++;return route.abort();});
   await page.route('**/public/purchase-configuration-handoffs',route=>{writes++;return route.abort();});
   let submitted:unknown;
   if(boundary==='same'){
    await page.route('**/public/commerce/carts',route=>{writes++;return route.fulfill({json:{code:0,data:{cart:{reference:'controlled'}}}});});
    await page.route('**/public/commerce/cart-items',route=>{writes++;submitted=route.request().postDataJSON();return route.fulfill({json:{code:0,data:{cart:{reference:'controlled'}}}});});
    await page.route('**/account/cart',route=>route.fulfill({contentType:'text/html',body:'<!doctype html><meta charset="utf-8"><h1>受控购物车目标：未创建真实购物车</h1>'}));
   }
   await site.open('/configure.html?product='+slug);
   const customer=new URL(baseURL!);if(boundary==='host')customer.hostname='127.0.0.1';else if(boundary==='protocol')customer.protocol='https:';customer.port='6032';
   await page.evaluate(origin=>{(window as any).__LINA_TEMPLATE__.customerUrl=origin;},customer.origin);
   if(flow==='cart')await site.chooseTerm();
   await site.setPurchaseCount('quantity','3');await site.setPurchaseCount('cycles','2');
   if(flow==='cart')await site.submit();else await site.saveConfiguration();
   if(boundary==='same'){
    await expect(page).toHaveURL(new URL('/account/cart','http://'+new URL(baseURL!).hostname+':6032').href);
    expect(writes).toBe(2);expect(submitted).toEqual({quantity:3,purchaseCycles:2,publicSlug:slug,configurationRevision:revision,billingTermToken:'controlled-term',currencyCode:'CNY',values});
    await expect(page.getByRole('heading')).toHaveText('受控购物车目标：未创建真实购物车');return;
   }
   await expect(page.locator('[data-status]')).toHaveText(flow==='cart'&&boundary==='host'?'官网与用户中心主机不一致，暂不能安全交接购物车，请联系管理员。':'官网与用户中心地址不支持安全交接，请联系管理员。');
   expect(writes).toBe(0);await expect(page).toHaveURL(/\/configure.html\?product=controlled-host-boundary$/);
   await expect(page.getByLabel('交付备注')).toHaveValue('保留当前选择');
   await expect(page.locator('[name=quantity]')).toHaveValue('3');await expect(page.locator('[name=cycles]')).toHaveValue('2');
   await expect(page.locator('#addCartForm')).not.toHaveAttribute('inert','');
   await site.capture('controlled-'+boundary+'-boundary-'+flow);
  });
 }
});


