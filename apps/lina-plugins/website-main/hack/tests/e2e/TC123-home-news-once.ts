import {test,expect} from '@playwright/test';
import {HomeDetailPage} from '../pages/HomeDetailPage';
test.use({baseURL:'http://127.0.0.1:6680',headless:true});
test.describe('TC-123 首页内容只初始化一次',()=>{
 test('TC-123a: 新闻公告各请求一次、每篇只展示一次',async({page},info)=>{
  const calls:string[]=[],errors:string[]=[];
  page.on('request',r=>{const u=new URL(r.url());if(u.pathname.endsWith('/public/content/items'))calls.push(u.searchParams.get('type')||'');});
  page.on('pageerror',e=>errors.push(e.message));
  const site=new HomeDetailPage(page);await site.open('/');await page.waitForLoadState('networkidle');
  expect(calls.filter(t=>t==='news')).toHaveLength(1);
  expect(calls.filter(t=>t==='announcement')).toHaveLength(1);
  const hrefs=await page.locator('[data-home-news]>a').evaluateAll(links=>links.map(a=>a.getAttribute('href')));
  expect(hrefs.length).toBeGreaterThan(0);expect(new Set(hrefs).size).toBe(hrefs.length);
  const data=await Promise.all(['news','announcement'].map(async type=>{
   const response=await page.request.get('/x/idc-core/api/v1/public/content/items?type='+type+'&limit=6');
   expect(response.ok()).toBe(true);return (await response.json()).data.items;
  }));
  const expected=data.flat().sort((a,b)=>b.publishedAt-a.publishedAt).slice(0,6);
  await expect(page.locator('[data-home-news] .news-article-item-title p')).toHaveText(expected.map(item=>item.title));
  await site.captureSection('.container_bg6',info.outputPath('home-news-once.png'));
  expect(errors).toEqual([]);
 });
});
