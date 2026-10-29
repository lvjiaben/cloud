import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/WebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032',apiOrigin:'http://127.0.0.1:6030'});

// Controlled failure inputs only. No content is published and no backend writes occur.
const detail='/x/idc-core/api/v1/public/content/items/controlled-contract-input';
const item={contentNo:'controlled-contract-input',title:'受控正文测试',body:'<p>正文保留</p><script>throw new Error("must not run")</script><iframe src="https://example.invalid"></iframe><a href="javascript:alert(1)">不可执行链接</a><img src="javascript:alert(1)"><p onclick="alert(1)">无事件正文</p>',file:{fileName:'受控输入.txt'}};

test.describe('TC-5 受控内容异常契约',()=>{
 test('TC-5a: 正文保留展示元素并移除脚本、内嵌页面和执行地址',async({page})=>{
  const site=new WebsitePage(page);const errors:string[]=[],remote:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).hostname==='example.invalid')remote.push(r.url());});
  await page.route('**'+detail,route=>route.fulfill({json:{code:0,data:{item}}}));
  await site.open('/content.html?id=controlled-contract-input');
  const body=page.locator('[data-content-body]');await expect(body).toContainText('正文保留');
  await expect(page.locator('.news-txt-title [data-page-title]')).toHaveText(item.title);
  await expect(page.locator('.news-txt-title')).toBeVisible();
  await expect(body.locator('script,iframe,[onclick],[href^="javascript:"],[src^="javascript:"]')).toHaveCount(0);
  expect(errors).toEqual([]);expect(remote).toEqual([]);await site.capture('controlled-content-sanitized');
 });
 test('TC-5b: 空票据及过期票据不发起下载，按钮恢复可重试',async({page})=>{
  const site=new WebsitePage(page);let downloadRequests=0;
  page.on('request',r=>{if(r.url().includes('/content/downloads/'))downloadRequests++;});
  await page.route('**'+detail,route=>route.fulfill({json:{code:0,data:{item}}}));
  for(const ticket of [{token:'',expiresAt:Date.now()+60000},{token:'expired-controlled-input',expiresAt:1}]){
   await page.route('**'+detail+'/download-tickets',route=>route.fulfill({json:{code:0,data:ticket}}));
   await site.open('/content.html?id=controlled-contract-input');await site.download();
   await expect(page.locator('[data-status]')).toHaveText('下载暂不可用，请重试。');
   await expect(page.getByRole('button',{name:/^下载 /})).toBeEnabled();expect(downloadRequests).toBe(0);
  }
  await site.capture('controlled-download-ticket-invalid');
 });
 test('TC-5c: 票据请求期间禁止重复下载，服务失败不报告下载成功',async({page})=>{
  const site=new WebsitePage(page);let calls=0;let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**'+detail,route=>route.fulfill({json:{code:0,data:{item}}}));
  await page.route('**'+detail+'/download-tickets',async route=>{calls++;await gate;await route.fulfill({status:503,json:{code:503}});});
  try{
   await site.open('/content.html?id=controlled-contract-input');await site.download();
   const button=page.getByRole('button',{name:/^下载 /});await expect(button).toBeDisabled();
   await expect(page.locator('[data-status]')).toHaveText('正在准备下载…');expect(calls).toBe(1);
   release();await expect(page.locator('[data-status]')).toHaveText('下载暂不可用，请重试。');
   await expect(button).toBeEnabled();expect(calls).toBe(1);await site.capture('controlled-download-service-failure');
  }finally{release();}
 });
 test('TC-5d: 迟到的网站资料不能覆盖已加载正文的标题和摘要',async({page})=>{
  const site=new WebsitePage(page);let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/x/idc-portal/api/v1/public/brand',async route=>{await gate;await route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'受控网站资料'},seoTitle:{zhCN:'不能覆盖正文的标题'},seoDescription:{zhCN:'不能覆盖正文的摘要'}}}}});});
  await page.route('**'+detail,route=>route.fulfill({json:{code:0,data:{item:{...item,seoTitle:'正文独立标题',summary:'正文独立摘要'}}}}));
  try{
   await site.open('/content.html?id=controlled-contract-input');
   await expect(page).toHaveTitle('正文独立标题');await expect(page.locator('meta[name=description]')).toHaveAttribute('content','正文独立摘要');
   release();await expect(page.locator('[data-brand-name]').first()).toHaveText('受控网站资料');
   await expect(page).toHaveTitle('正文独立标题');await expect(page.locator('meta[name=description]')).toHaveAttribute('content','正文独立摘要');
   await site.capture('desktop-content-metadata-race');
  }finally{release();}
 });
});

