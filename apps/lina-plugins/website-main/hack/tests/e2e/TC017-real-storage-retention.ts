import {test,expect} from '@playwright/test';
import {access,utimes} from 'node:fs/promises';
import {resolve} from 'node:path';

test.describe('TC-17 真实私有存储和定时清理',()=>{
 test('TC-17a: 真实保存的老对象由运行中的清理任务删除，新对象保留',async({playwright})=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','只允许隔离本地验证');
  test.setTimeout(110000);
  const origin=process.env.WEBSITE_TEST_URL!;
  expect(new URL(origin).port).toBe('16687');
  const root=resolve(process.env.WEBSITE_QA_STORAGE_ROOT!);
  expect(root.replaceAll('\\','/')).toBe('D:/project1/temp/20261017/website-validation/packages/.capability-storage/plugins/idc-core/platform/purchase-handoff');
  const oldContext=await playwright.request.newContext();
  const activeContext=await playwright.request.newContext();
  try{
   const base=origin+'/x/idc-core/api/v1/public';
   const config=await (await oldContext.get(base+'/catalog/products/isolated-integration-only/purchase-configuration')).json();expect(config.code).toBe(0);
   const input={publicSlug:'isolated-integration-only',configurationRevision:config.data.configurationRevision,values:config.data.values,currencyCode:'CNY',billingTermToken:'',quantity:1,purchaseCycles:1};
   const old=await (await oldContext.post(base+'/purchase-configuration-handoffs',{data:input})).json();expect(old.code).toBe(0);
   const active=await (await activeContext.post(base+'/purchase-configuration-handoffs',{data:input})).json();expect(active.code).toBe(0);
   for(const result of [old,active])expect(result.data.reference).toMatch(/^[a-f0-9]{64}$/);
   const oldPath=resolve(root,old.data.reference);const activePath=resolve(root,active.data.reference);
   await access(oldPath);await access(activePath);
   // Controlled age only for this test's new object. No mocked storage, job or API response.
   // This proves actual scheduled deletion, not a naturally elapsed 15-minute TTL.
   const aged=new Date(Date.now()-17*60*1000);await utimes(oldPath,aged,aged);
   await expect.poll(async()=>{try{await access(oldPath);return false;}catch(error){if((error as NodeJS.ErrnoException).code!=='ENOENT')throw error;return true;}},{timeout:95000,intervals:[1000]}).toBe(true);
   await access(activePath);
  }finally{await oldContext.dispose();await activeContext.dispose();}
 });
});
