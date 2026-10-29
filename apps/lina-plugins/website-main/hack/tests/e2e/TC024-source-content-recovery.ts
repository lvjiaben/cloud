import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});

test.describe('TC-24 当前源码内容搜索与恢复',()=>{
 for(const [path,type]of [['news','news'],['announcements','announcement'],['help','knowledge'],['downloads','download']])test('TC-24 '+type+': 真实搜索业务码与页面结果一致，清空后恢复',async({page,request},info)=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();expect(['localhost','127.0.0.1']).toContain(new URL(origin!).hostname);
  const endpoint='/x/idc-core/api/v1/public/content/items';
  const baseline=await(await request.get(origin+endpoint+'?type='+type+'&limit=20')).json();expect(baseline.code).toBe(0);
  const site=new WebsitePage(page);await site.open('/'+path+'.html');await expect(page.locator('[data-content-list] > *')).toHaveCount(baseline.data.items.length);await expect(page.locator('[data-status]')).not.toContainText('正在加载');
  const keyword='readonly-contract-20261018';
  const pending=page.waitForResponse(r=>{const u=new URL(r.url());return u.pathname===endpoint&&u.searchParams.get('query')===keyword;});
  await site.searchContent(keyword);const response=await pending;const body=await response.json();
  expect(new URL(page.url()).searchParams.get('query')).toBe(keyword);await expect(page.locator('[data-content-search] input')).toHaveValue(keyword);
  if(String(body.code)==='0'){
   await expect(page.locator('[data-content-list] > *')).toHaveCount(body.data.items.length);
   for(const item of body.data.items)await expect(page.locator('[data-content-list] a').filter({hasText:item.title})).toHaveCount(1);
   if(!body.data.items.length)await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');
  }else{
   await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');await expect(page.locator('[data-content-list]')).toBeEmpty();await expect(page.locator('[data-more]')).toBeHidden();
   const retry=page.waitForResponse(r=>{const u=new URL(r.url());return u.pathname===endpoint&&u.searchParams.get('query')===keyword;});await site.retry();expect((await(await retry).json()).code).not.toBe(0);await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');
  }
  const searchScreenshot=await site.capture('source-content-'+type+'-search');
  await site.searchContent('');await expect(page.locator('[data-status]')).not.toContainText('正在加载');await expect(page.locator('[data-status]')).not.toContainText('失败');await expect(page.locator('[data-content-list] > *')).toHaveCount(baseline.data.items.length);await expect(page.locator('[data-retry]')).toBeHidden();expect(new URL(page.url()).searchParams.has('query')).toBe(false);
  const recoveredScreenshot=await site.capture('source-content-'+type+'-recovered');
  await info.attach('real-content-search',{body:JSON.stringify({type,code:body.code,searchSucceeded:String(body.code)==='0',baselineCount:baseline.data.items.length,searchScreenshot,recoveredScreenshot},null,2),contentType:'application/json'});
 });
});
