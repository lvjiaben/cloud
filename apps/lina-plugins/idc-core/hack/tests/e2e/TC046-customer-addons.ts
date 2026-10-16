// These UI contract tests use isolated HTTP fixtures; real SQL delivery is tested separately.
import {readFileSync} from 'node:fs';
import {test,expect} from '@host-tests/fixtures/auth';
import {mockCustomerAPIs} from '../../../../../lina-vben/apps/web-customer/tests/support/customer-api-mock';
import {CustomerAddonPage} from '../pages/CustomerAddonPage';
test.use({baseURL:'http://127.0.0.1:6032'});
test.describe('TC-46 客户附加商品',()=>{
  for (const mode of ['payment','blocked','reset']) test(`TC-46${mode==='payment'?'a: 配置冻结与付款重试':mode==='blocked'?'b: 已有购物车保护':'c: 重新选择只移除本次商品'}`,async({page})=>{
    await mockCustomerAPIs(page);
    const messages:Record<string,any>={};
    for (const file of ['plugin','assisted-create','addons']) {
      const flat=JSON.parse(readFileSync(new URL(`../../../manifest/i18n/zh-CN/${file}.json`,import.meta.url),'utf8'));
      for(const [key,value] of Object.entries(flat)) { const parts=key.split('.');let node=messages;for(const part of parts.slice(0,-1))node=node[part]??={};node[parts.at(-1)!]=value; }
    }
    await page.route('**/i18n/runtime/messages**',route=>route.fulfill({json:{code:0,data:{messages}}}));
    await page.route('**/api/v1/plugins/dynamic**',route=>route.fulfill({json:{code:0,data:{list:[{id:'idc-core',installed:1,enabled:1,version:'v0.1.2',generation:1,runtimeState:'normal',type:'source',customerSurfaces:[{pluginId:'idc-core',pluginType:'source',key:'addons',version:'v1',title:'plugin.idc-core.addons.title',slot:'customer.products',routePath:'/account/plugins/idc-core/addons',entry:'customer-addons.vue',status:'active',bridgeCapabilities:['addons.customer.v1']}]}]}}}));
    let adds=0,quotes=0,freezes=0,orders=0,payments=0,other=mode==='blocked';const removed:string[]=[];const keys:string[]=[];
    const config={configurationRevision:'r'.repeat(43),fields:[{fieldToken:'cpu',type:'integer',labelZhCN:'处理器核心数',labelEnUS:'CPU',visible:true,required:true,disabled:false,minimumInteger:'2',maximumInteger:'16'}],values:[{fieldToken:'cpu',type:'integer',integerValue:'4'}]};
    const cart=()=>({version:1+adds+removed.length,items:[...(adds&&!removed.length?[{cartItemNo:'addon_item',version:1,productNameZhCN:'运维服务'}]:[]),...(other?[{cartItemNo:'other_item',version:1,productNameZhCN:'其他商品'}]:[])]});
    await page.route('**/x/idc-core/api/v1/**',async route=>{
      const path=new URL(route.request().url()).pathname;const method=route.request().method();let data:any={};let code=0;
      if(path.endsWith('/purchase-configuration')){if(method==='POST'){freezes++;expect(route.request().postDataJSON().values).toEqual(config.values);if(mode==='payment'&&freezes===1)code=500;else data={configurationFrozen:true};}else data=config;}
      else if(path.endsWith('/evaluate'))data={...config,valid:true,issues:[]};
      else if(path.endsWith('/quote'))data={terms:[{billingTermToken:'month',labelZhCN:'月付',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',setupAmount:'0',subtotalAmount:'3000',taxAmount:'0',totalAmount:'3000'}]};
      else if(path.endsWith('/public/catalog/products'))data={products:[],availableCurrencies:['CNY'],nextCursor:''};
      else if(path.endsWith('/commerce/addon-targets'))data={targets:[{entitlementNo:'ent_owned',productNameZhCN:'我的服务器',productNameEnUS:'My server',addons:[{publicSlug:'ops-service',nameZhCN:'运维服务',nameEnUS:'Operations'}]}]};
      else if(path.endsWith('/commerce/cart')||path.endsWith('/commerce/carts'))data={cart:cart()};
      else if(path.endsWith('/commerce/cart-items')){adds++;data={cart:cart()};}
      else if(method==='DELETE'){removed.push(path.split('/').at(-1)!);expect(removed).toEqual(['addon_item']);data={cart:cart()};}
      else if(path.endsWith('/commerce/quotes')){quotes++;expect(route.request().postDataJSON().targetEntitlementNo).toBe('ent_owned');data={quote:{quoteNo:'QTE_addon',currencyCode:'CNY',totalAmountMinor:'3000',items:[{quoteItemNo:'QIT_addon'}]}};}
      else if(path.endsWith('/commerce/orders')){orders++;expect(freezes).toBeGreaterThan(0);keys.push(route.request().postDataJSON().idempotencyKey);data={checkout:{orderNo:'ORD_addon',invoiceNo:'INV_addon',statusCode:'pending_payment',totalAmountMinor:'3000'}};}
      else if(path.endsWith('/balance-payments')){payments++;keys.push(route.request().postDataJSON().idempotencyKey);if(payments===1)code=500;}
      else {await route.fallback();return;}
      await route.fulfill({json:{code,data,message:code?'余额不足，请充值后重试':'success'}});
    });
    const view=new CustomerAddonPage(page);await view.login();await view.goto();
    await expect(page.getByTestId('addon-target')).toContainText('我的服务器');await view.screenshot('initial');
    if(mode==='blocked'){await expect(page.getByTestId('addon-quote')).toBeDisabled();expect(adds).toBe(0);expect(removed).toEqual([]);return;}
    await view.choose();await expect(page.getByText('月付',{exact:true})).toBeVisible();await view.quote();
    if(mode==='payment'){await expect(page.getByText(/操作未完成，请检查提示后重试/)).toBeVisible();await view.screenshot('freeze-failed');await view.quote();expect(quotes).toBe(1);expect(freezes).toBe(2);}
    await expect(page.getByTestId('addon-quote-summary')).toContainText('应付金额：CNY 30.00');expect(payments).toBe(0);await view.screenshot('quote');
    if(mode==='reset'){other=true;await view.reset();await expect(page.getByText('其他商品',{exact:true})).toBeVisible();expect(removed).toEqual(['addon_item']);await expect(page.getByTestId('addon-quote')).toBeDisabled();return;}
    await view.pay();await expect(page.getByTestId('addon-quote-summary')).toContainText('ORD_addon');await view.screenshot('payment-failed');await view.pay();
    await expect(page.getByTestId('addon-paid')).toContainText('付款成功，附加商品已进入交付队列');expect(orders).toBe(1);expect(payments).toBe(2);expect(keys[1]).toBe(keys[2]);await expect(page.locator('body')).not.toContainText('plugin.idc-core.addons.');await view.screenshot('paid');
  });
});
