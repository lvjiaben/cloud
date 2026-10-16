import { readFileSync } from 'node:fs';
import { test, expect } from '@host-tests/fixtures/auth';
import { mockCustomerAPIs } from '../../../../../lina-vben/apps/web-customer/tests/support/customer-api-mock';
import { EntitlementUpgradePage } from '../pages/EntitlementUpgradePage';
const prefix = '/x/idc-core/api/v1/';
test.use({ baseURL: process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6032' });
test.describe('TC-45 客户配置升级', () => {
  for (const blocked of [false, true]) test(`TC-45${blocked ? 'b: 保留已有购物车' : 'a: 冻结配置、展示补差、失败重试复用账单'}`, async ({ page }) => {
    await mockCustomerAPIs(page);
    const flat = JSON.parse(readFileSync(new URL('../../../manifest/i18n/zh-CN/plugin.json', import.meta.url), 'utf8'));
    Object.assign(flat, JSON.parse(readFileSync(new URL('../../../manifest/i18n/zh-CN/assisted-create.json', import.meta.url), 'utf8')));
    const messages: Record<string, any> = {};
    for (const [key, value] of Object.entries(flat)) {
      const parts = key.split('.'); let node = messages;
      for (const part of parts.slice(0, -1)) node = node[part] ??= {};
      node[parts.at(-1)!] = value;
    }
    await page.route('**/i18n/runtime/messages**', route => route.fulfill({ json: {code:0,data:{messages}} }));
    await page.route('**/api/v1/plugins/dynamic**', route => route.fulfill({ json: {code:0,data:{list:[{id:'idc-core',installed:1,enabled:1,version:'v0.1.2',generation:1,runtimeState:'normal',type:'source',customerSurfaces:[{pluginId:'idc-core',pluginType:'source',key:'entitlement-upgrades',version:'v1',title:'plugin.idc-core.upgrades.title',slot:'customer.products',routePath:'/account/plugins/idc-core/entitlement-upgrades',entry:'customer-entitlement-upgrades.vue',status:'active',bridgeCapabilities:['entitlement-upgrades.customer.v1']}]}]}} }));
    let adds=0,freezes=0,confirmations=0,payments=0,removals=0;const bodies:any[]=[];
    const upgrade={upgradeNo:'UPG_test',entitlementNo:'ent_test',currencyCode:'CNY',statusCode:'quoted',invoiceNo:'',sourcePeriodMinor:'8000',targetPeriodMinor:'12000',netMinor:'2000',taxMinor:'120',totalMinor:'2120',expiresAt:Date.now()+300000};
    const config={configurationRevision:'r'.repeat(43),fields:[{fieldToken:'cpu',type:'integer',labelZhCN:'处理器核心数',labelEnUS:'CPU',visible:true,required:true,disabled:false,minimumInteger:'2',maximumInteger:'16'}],values:[{fieldToken:'cpu',type:'integer',integerValue:'4'}]};
    const currentCart=()=>({version:1+adds+removals,items:blocked?[{cartItemNo:'other_item',version:1}]:adds>removals?[{cartItemNo:'upgrade_item',version:1}]:[]});
    await page.route(`**${prefix}**`,async route=>{
      const path=new URL(route.request().url()).pathname;const method=route.request().method();let data:any={};let code=0;
      if(path.endsWith('/purchase-configuration')) {if(method==='POST'){freezes++;data={configurationFrozen:true};}else data=config;}
      else if(path.endsWith('/evaluate'))data={...config,valid:true,issues:[]};
      else if(path.endsWith('/quote'))data={terms:[{billingTermToken:'term_month',labelZhCN:'月付',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',setupAmount:'0',subtotalAmount:'12000',taxAmount:'720',totalAmount:'12720'}]};
      else if(path.endsWith('/commerce/carts')||path.endsWith('/commerce/cart'))data={cart:currentCart()};
      else if(path.endsWith('/commerce/cart-items')){adds++;bodies.push(route.request().postDataJSON());data={cart:currentCart()};}
      else if(method==='DELETE'){expect(path).toContain('/cart-items/upgrade_item');removals++;data={cart:currentCart()};}
      else if(path.endsWith('/commerce/quotes'))data={quote:{quoteNo:'QTE_test',items:[{quoteItemNo:'QIT_test'}]}};
      else if(path.endsWith('/ent_test/upgrades')){expect(freezes).toBe(1);data={upgrade:{...upgrade}};}
      else if(path.endsWith('/confirmations')){confirmations++;upgrade.statusCode='pending_payment';upgrade.invoiceNo='UIN_test';data={upgrade:{...upgrade}};}
      else if(path.endsWith('/balance-payments')){payments++;if(payments===1)code=500;else{upgrade.statusCode='ready';data={upgrade:{...upgrade}};}}
      else if(path.endsWith('/entitlement-upgrades'))data={targets:[{entitlementNo:'ent_test',publicSlug:'test-server',nameZhCN:'测试云服务器',nameEnUS:'Test server',currencyCode:'CNY'}],items:confirmations?[{...upgrade}]:[]};
      else { await route.fallback();return; }
      await route.fulfill({json:{code,data,message:code?'余额不足，请充值后重试':'success'}});
    });
    const view=new EntitlementUpgradePage(page);await view.login();await view.goto();
    await expect(page.getByText('测试云服务器',{exact:true})).toBeVisible();await view.screenshot('list');await view.open();await view.chooseProduct();
    await expect(view.dialog).toContainText('处理器核心数');await expect(view.dialog).toContainText('月付');await view.screenshot('configuration');
    if (!blocked) {
      const quantity = page.getByTestId('assisted-item-quantity-0');
      await quantity.fill('2'); await quantity.blur(); await view.preview();
      await expect(page.getByText('一次只能升级一台服务，商品数量请填1。', { exact: true })).toBeVisible();
      expect(adds).toBe(0);
      await quantity.fill('1'); await quantity.blur();
    }
    await view.preview();
    if(blocked){await expect(view.dialog).toContainText('原购物车不会被清空');expect(adds).toBe(0);expect(removals).toBe(0);expect(freezes).toBe(0);await view.screenshot('cart-blocked');return;}
    await expect(page.getByTestId('upgrade-summary')).toContainText('升级后周期费用（不含税）：CNY 120.00');await expect(page.getByTestId('upgrade-summary')).toContainText('本次费用：CNY 21.20');expect(payments).toBe(0);expect(removals).toBe(1);expect(bodies[0].values).toEqual(config.values);await view.screenshot('quote');await view.pay();
    await expect(view.dialog).toContainText('暂时无法处理');expect(confirmations).toBe(1);await view.screenshot('payment-failed');await view.pay();await expect(view.dialog).toBeHidden();expect(confirmations).toBe(1);expect(payments).toBe(2);await expect(page.getByText('等待升级',{exact:true})).toBeVisible();await expect(page.locator('body')).not.toContainText('plugin.idc-core.upgrades.');await view.screenshot('queued');
  });
});
