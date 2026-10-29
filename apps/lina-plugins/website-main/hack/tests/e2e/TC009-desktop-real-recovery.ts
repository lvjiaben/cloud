import {test,expect,type Route} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-9 桌面真实接口搜索与失败恢复',()=>{
 test('TC-9a: 公司资料请求失败显示明确状态，重试恢复真实品牌与空资料',async({page})=>{
  const site=new WebsitePage(page);const path='**/x/idc-portal/api/v1/public/brand';
  const fail=(route:Route)=>route.abort();
  await page.route(path,fail);await site.open('/about.html');
  await expect(page.locator('[data-status]')).toHaveText('网站资料暂时无法加载，请重试。');
  await expect(page.locator('.lina-brand-error')).toHaveCount(1);await site.capture('desktop-brand-failure');
  await page.unroute(path,fail);const response=page.waitForResponse(r=>r.url().endsWith('/public/brand'));
  await site.retryBrand();const result=await response;expect(result.ok()).toBeTruthy();const data=(await result.json()).data.brand;
  await expect(page.locator('.lina-brand-error')).toHaveCount(0);
  await expect(page.locator('[data-brand-name]').first()).toHaveText(data.siteName.zhCN||data.siteName.enUS);
  await expect(page.locator('[data-status]')).toHaveText('公司介绍、资质和服务承诺尚无公开资料。');await site.capture('desktop-brand-recovered');
 });
 test('TC-9b: 四类内容真实搜索无结果，清空恢复权威列表',async({page},info)=>{
  const site=new WebsitePage(page);
  const evidence:unknown[]=[];
  for(const [path,type]of [['news','news'],['announcements','announcement'],['help','knowledge'],['downloads','download']]){
   await site.open('/'+path+'.html');
   const response=page.waitForResponse(r=>{const u=new URL(r.url());return u.pathname.endsWith('/public/content/items')&&u.searchParams.get('query')==='桌面搜索检查'&&u.searchParams.get('type')===type;});
   await site.searchContent('桌面搜索检查');const result=await response;expect(result.ok()).toBeTruthy();const body=await result.json();
   expect(body.code).toBe(0);expect(body.data?.items||[]).toHaveLength(0);
   await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');
   await expect(page.locator('[data-content-list]')).toBeEmpty();await expect(page).toHaveURL(/query=/);
   const cleared=page.waitForResponse(r=>{const u=new URL(r.url());return u.pathname.endsWith('/public/content/items')&&!u.searchParams.has('query')&&u.searchParams.get('type')===type;});
   await site.searchContent('');const restored=await cleared;expect((await restored.json()).code).toBe(0);
   await expect(page).toHaveURL(new RegExp('/'+path+'\\.html$'));await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|加载失败/);
   evidence.push({type,url:result.url(),status:result.status(),body,recoveryStatus:restored.status()});
  }
  await site.capture('desktop-download-search-cleared');
  await info.attach('real-content-search-blocker',{body:JSON.stringify(evidence,null,2),contentType:'application/json'});
 });
});
