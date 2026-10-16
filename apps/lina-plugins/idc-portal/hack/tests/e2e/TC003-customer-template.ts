import path from 'node:path';
import { CustomerTemplatePage } from '../pages/CustomerTemplatePage';
import { mkdir } from 'node:fs/promises';
import { readFileSync } from 'node:fs';
import { test as base, expect, type Page } from '@playwright/test';
import { config } from '../../../../../../hack/tests/fixtures/config';

const endpoint = '/x/idc-portal/api/v1/admin/customer-center/configuration';
const test = base.extend<{adminPage: Page}>({
 adminPage: async ({page},use)=>{
  const login=await page.request.post(config.apiBaseURL+'auth/login',{data:{username:config.adminUser,password:config.adminPass,clientType:'desktop'}});
  const session=await login.json();
  expect(session.code).toBe(0);
  const state=JSON.parse(readFileSync(path.resolve('temp/storage-state/admin.json'),'utf8'));
  const entries=state.origins[0].localStorage;
  const access=entries.find((item:{name:string})=>item.name.endsWith('core-access'));
  const data=JSON.parse(access.value);
  data.accessToken=session.data.accessToken;data.refreshToken=session.data.refreshToken;access.value=JSON.stringify(data);
  await page.addInitScript(items=>{if(sessionStorage.getItem('website-test-session'))return;for(const item of items)localStorage.setItem(item.name,item.value);sessionStorage.setItem('website-test-session','1');},entries);
  await page.route(/\/(?:api\/v1|x\/[^/]+\/api\/v1)\//,async route=>{const url=new URL(route.request().url());const response=await route.fetch({url:new URL(url.pathname+url.search,config.backendBaseURL).href});await route.fulfill({response});});
  await use(page);
 }
});

test.describe('TC-3 用户中心模板', () => {
 test('TC-3a: 品牌和模板设置保存后客户登录页生效，开发文档提供真实接口', async ({adminPage}) => {
  const view=new CustomerTemplatePage(adminPage);await view.open();
  const token=await adminPage.evaluate(()=>{const k=Object.keys(localStorage).find(k=>k.endsWith('-core-access'));return k?JSON.parse(localStorage.getItem(k)??'{}').accessToken:'';});
  const headers={Authorization:'Bearer '+token};
  async function call(method='GET',data?:unknown){return (await adminPage.request.fetch(config.backendBaseURL+endpoint,{method,headers,data})).json();}
  const original=(await call()).data;
  const selected=original.plugins.find((item:any)=>item.pluginId==='customer-default');
  expect(selected).toBeTruthy();
  const input={selectedPlugin:'customer-default',common:{...original.common,siteName:'模板品牌验收',siteTitle:'客户中心模板验收'},config:{primarycolor:'#1273cc'}};
  try {
   expect((await call('PUT',input)).code).toBe(0);
   await adminPage.reload();
   await view.assertConfiguration();
   await view.openGuide();
   await view.searchOrders();
   await mkdir(path.resolve('../../temp/20261016'),{recursive:true});
   await adminPage.screenshot({path:path.resolve('../../temp/20261016/121500-customer-template-api-guide.png'),fullPage:true});
   const customer=await adminPage.context().newPage();
   const customerView=new CustomerTemplatePage(customer);await customerView.assertCustomerLogin();
   await expect(customer).toHaveTitle(/客户中心模板验收|模板品牌验收/);
   await expect(customer.locator('html')).toHaveAttribute('data-customer-template','customer-default');
   await customer.screenshot({path:path.resolve('../../temp/20261016/121600-customer-template-login.png'),fullPage:true});
   await customer.reload();
   await expect(customer.locator('html')).toHaveAttribute('data-customer-template','customer-default');
   const customers=await (await adminPage.request.get(config.backendBaseURL+'/x/idc-core/api/v1/admin/customers?status=active&limit=1',{headers})).json();
   const subject=customers.data.customers[0];expect(subject).toBeTruthy();
   const assistance=await (await adminPage.request.post(config.backendBaseURL+'/x/idc-core/api/v1/admin/customers/'+subject.customerNo+'/assistance-sessions',{headers,data:{reason:'用户中心模板只读业务验收，不修改客户数据',idempotencyKey:'customer-template-'+Date.now()}})).json();
   expect(assistance.code).toBe(0);
   const launch=new URL(assistance.data.launchUrl);await customer.goto('http://127.0.0.1:6032'+launch.pathname+launch.search);
   await expect(customer.getByTestId('customer-assistance-banner')).toBeVisible();
   try {
    for(const [route,testId] of [['/account/servers','customer-servers-page'],['/account/orders','customer-commerce-page'],['/account/billing','customer-finance-page'],['/account/support','customer-support-page']]){
     await customerView.openBusiness(route,testId);await customerView.screenshot('121700-customer-'+route.split('/').pop());
    }
   } finally {await adminPage.request.post(config.backendBaseURL+'/x/idc-core/api/v1/admin/customer-assistance/'+assistance.data.sessionNo+'/end',{headers,data:{}});}
   await customer.close();
   const bad={...input,selectedPlugin:'not-installed'};
   expect((await call('PUT',bad)).code).not.toBe(0);
   expect((await call()).data.common.siteName).toBe(input.common.siteName);
  } finally {
   const restore={selectedPlugin:original.selectedPlugin||'customer-default',common:original.common.siteName?original.common:{...original.common,siteName:'客户中心'},config:original.config??{}};
   expect((await call('PUT',restore)).code).toBe(0);
  }
 });
});
