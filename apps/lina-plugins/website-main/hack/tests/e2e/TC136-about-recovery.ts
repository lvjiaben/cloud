import {test,expect} from '../support/source-site';
import {AboutRecoveryPage} from '../pages/AboutRecoveryPage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
test.describe('TC-136 公司资料加载恢复',()=>{
 test('TC-136c 实际接口页面读取',async({page},info)=>{
  test.setTimeout(90000);
  const site=new AboutRecoveryPage(page);await site.open();
  await expect(page).toHaveTitle(/^公司简介/,{timeout:40000});
  await expect(page.locator('[data-about-introduction]')).toBeEmpty();
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
  await page.screenshot({path:info.outputPath('about-live.png')});
 });
 test('TC-136a 首次临时失败自动恢复，后台失败保留已有资料',async({page},info)=>{
  let calls=0,fail=false;
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>{
   calls++;return r.fulfill(calls===1||fail?{status:503,json:{code:50,data:null}}:{json:{code:0,data:{brand:{siteName:{zhCN:'网站名称'},footer:{zhCN:'公司资料恢复验收'},navigation:[],footerNavigation:[]}}}});
  });
  const site=new AboutRecoveryPage(page);await site.open();
  await expect(site.body()).toHaveText('公司资料恢复验收');expect(calls).toBe(2);
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
  fail=true;await site.refresh();await expect.poll(()=>calls).toBe(3);
  await expect(site.body()).toHaveText('公司资料恢复验收');
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
  await page.screenshot({path:info.outputPath('about-recovered.png')});
 });
 test('TC-136b 持续失败次数有限，手动重试可恢复',async({page},info)=>{
  let calls=0,fail=true;
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>{calls++;return r.fulfill(fail?{status:503,json:{code:50,data:null}}:{json:{code:0,data:{brand:{siteName:{zhCN:'网站名称'},footer:{zhCN:'公司资料重试验收'},navigation:[],footerNavigation:[]}}}});});
  const site=new AboutRecoveryPage(page);await site.open();
  await expect(page.locator('[data-brand-retry]')).toBeVisible();expect(calls).toBe(2);
  await expect(page.locator('.lina-brand-error p')).toHaveText('网站资料暂时无法加载，请重试。');
  await page.screenshot({path:info.outputPath('about-unavailable.png')});
  fail=false;await site.retry();await expect(site.body()).toHaveText('公司资料重试验收');
  expect(calls).toBe(3);await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
 });
});
