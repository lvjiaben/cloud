import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ArticleFeedbackPage} from '../pages/ArticleFeedbackPage';
import type {Page} from '@playwright/test';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
const id='controlled-feedback',endpoint=prefix+'/content/items/'+id;
const path='/content.html?id='+id+'&query=%E6%B5%8B%E8%AF%95%2B%E6%9F%A5%E8%AF%A2&currency=CNY';
const item={contentNo:id,type:'knowledge',title:'受控正文复制竞态验收',category:{nameZhCN:'测试分类'},publishedAt:1792200000000,body:'<h2>准备工作</h2><p>仅浏览器测试资料，不发布后台。</p>'+Array.from({length:24},(_,i)=>'<p>用于目录滚动的受控段落 '+i+'</p>').join('')+'<h3>处理步骤</h3><p>受控正文已恢复。</p>'};
const denied='未能复制链接，请复制浏览器地址栏中的网址。',copied='链接已复制，可以粘贴分享。',failure='内容加载失败，请重试。';
function gate(){let release!:(value:boolean)=>void;const promise=new Promise<boolean>(resolve=>release=resolve);return {promise,release};}
async function setup(page:Page,delayed=false){
 const fixture=await controlledBusiness(page),responses=[gate(),gate()];let requests=0;
 await page.route('**'+endpoint,async route=>{
  const attempt=requests++,ok=delayed?await responses[attempt].promise:true;
  await route.fulfill({status:ok?200:503,json:ok?{code:0,data:{item}}:{code:50,data:null}});
 });
 const site=new ArticleFeedbackPage(page);await site.controlClipboard();await site.open(path);
 return {site,fixture,responses,requests:()=>requests};
}

test.describe('TC-38 正文目录与复制异步反馈（浏览器隔离数据）',()=>{
 test('TC-38a: 新复制优先，旧成功或拒绝均不得覆盖最新结果',async({page})=>{
  const {site,fixture}=await setup(page);await expect(site.body).toContainText('受控正文已恢复。');await site.capture('initial');
  await site.copy();await site.copy();await expect.poll(()=>site.clipboardValues()).toEqual([page.url(),page.url()]);
  await site.settleCopy(1,false);await expect(site.status).toHaveText(denied);
  await site.settleCopy(0,true);await expect(site.status).toHaveText(denied);await expect(site.status).toHaveAttribute('data-content-action-feedback','');await expect(site.status).toBeInViewport();await site.capture('new-denial-survives-old-success');
  await site.copy();await site.copy();await site.settleCopy(3,true);await expect(site.status).toHaveText(copied);
  await site.settleCopy(2,false);await expect(site.status).toHaveText(copied);await expect(site.status).toBeInViewport();await site.capture('new-copy-success');
  await site.copy();await site.copy();await site.settleCopy(4,false);await expect(site.status).toHaveText(copied);
  await site.settleCopy(5,false);await expect(site.status).toHaveText(denied);
  expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
 });
 test('TC-38b: 加载失败后旧复制成功不得覆盖错误与可操作重试',async({page})=>{
  const {site,fixture,responses,requests}=await setup(page,true);
  try{
   await expect.poll(requests).toBe(1);await expect(site.status).toHaveText('正在加载内容…');await site.copy();responses[0].release(false);
   await expect(site.status).toHaveText(failure);await site.settleCopy(0,true);await expect(site.status).toHaveText(failure);await expect(site.status).not.toHaveAttribute('data-content-action-feedback');await expect(site.body).toBeEmpty();await site.capture('failure-retained',true);
   await site.retryContent();await expect.poll(requests).toBe(2);await expect(site.status).toHaveText('正在加载内容…');responses[1].release(true);
   await expect(site.body).toContainText('受控正文已恢复。');await expect(site.retry).toBeHidden();await expect(site.status).toBeEmpty();await site.capture('retry-recovered');
   expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
  }finally{responses.forEach(r=>r.release(true));}
 });
 test('TC-38c: 重试加载时旧复制拒绝不得写回过期反馈',async({page})=>{
  const {site,responses,requests}=await setup(page,true);
  try{
   await expect.poll(requests).toBe(1);await site.copy();responses[0].release(false);await expect(site.status).toHaveText(failure);
   await site.retryContent();await expect.poll(requests).toBe(2);await expect(site.status).toHaveText('正在加载内容…');await site.settleCopy(0,false);
   await expect(site.status).toHaveText('正在加载内容…');await expect(site.status).not.toHaveAttribute('data-content-action-feedback');await site.capture('retry-loading-retained',true);
   responses[1].release(true);await expect(site.body).toContainText('受控正文已恢复。');await expect(site.status).toBeEmpty();await expect(site.retry).toBeHidden();
  }finally{responses.forEach(r=>r.release(true));}
 });
 test('TC-38d: 目录保留内容ID、原查询与编码值，并定位实际正文',async({page})=>{
  const {site}=await setup(page);await expect(site.directory.getByRole('link')).toHaveText(['准备工作','处理步骤']);
  const before=new URL(page.url());await expect(site.directory.getByRole('link',{name:'处理步骤'})).toHaveAttribute('href',before.pathname+before.search+'#content-heading-1');
  await site.heading('处理步骤');const after=new URL(page.url());expect(after.search).toBe(before.search);expect(after.searchParams.get('id')).toBe(id);expect(after.searchParams.get('query')).toBe('测试+查询');expect(after.hash).toBe('#content-heading-1');
  await expect(page.locator('#content-heading-1')).toHaveText('处理步骤');await expect(page.locator('#content-heading-1')).toBeInViewport();await expect(site.directory.locator('.current-catalog')).toHaveText('处理步骤');
  await site.capture('directory-query-retained');await site.copy();expect(await site.clipboardValues()).toEqual([page.url()]);await site.settleCopy(0,true);await expect(site.status).toHaveText(copied);
 });
});
