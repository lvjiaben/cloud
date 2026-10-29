import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-6 十页真实资源与响应式审计',()=>{
 for(const width of [1440,390])test('TC-6'+(width===1440?'a':'b')+': '+width+' 像素页面无原站请求、脚本异常或资源缺失',async({page},info)=>{
  test.setTimeout(90000);
  const site=new WebsitePage(page);await site.setViewport(width,width===1440?900:844);
  const remote:string[]=[],errors:string[]=[],missing:string[]=[],observations:unknown[]=[];
  page.on('request',r=>{if(!['localhost','127.0.0.1'].includes(new URL(r.url()).hostname))remote.push(r.url());});
  page.on('pageerror',e=>errors.push(e.message));
  page.on('response',r=>{if(r.status()>=400&&!new URL(r.url()).pathname.startsWith('/x/'))missing.push(r.status()+' '+r.url());});
  for(const name of ['index','products','configure','news','announcements','help','downloads','content','contact','about']){
   await site.open('/'+name+'.html');
   await expect(page.locator('[data-brand-name]').first()).toHaveText('本地开发联调');
   await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|正在读取/);
   await expect(page.locator('body')).not.toContainText(/\u5c0f\u767d\u4e91|白猿|成都西信|As9929|\u0078\u0069\u0061\u006f\u0062\u0061\u0069\u0079\u0075\u006e\.cn/i);
   await expect.poll(async()=>{const layout=await site.layout();return layout.scrollWidth<=layout.width;}).toBeTruthy();
   const layout=await site.layout();expect(layout.width).toBe(width);
   if(name==='about'){
    const title=await page.locator('.companyIntroduction-right-title').boundingBox();
    const header=await page.locator('.horizontal-menu-navbar').boundingBox();
    expect(title).toBeTruthy();expect(header).toBeTruthy();expect(title!.y).toBeGreaterThanOrEqual(header!.y+header!.height);
   }
   if(name==='downloads'){
    await expect(page.locator('table.table [data-content-list]')).toHaveCount(1);
    await expect(page.locator('table th')).toHaveText(['资源名称','发布时间']);
    await expect(page.locator('.nav-layoutcontainer')).toHaveCSS('position','fixed');
    const panels=await page.locator('.sky-content>.d-flex>.card,.sky-content>.d-flex>.w-100').evaluateAll(elements=>elements.map(e=>e.getBoundingClientRect().toJSON()));
    expect(panels).toHaveLength(2);
    if(width===390)expect(panels[1].top).toBeGreaterThanOrEqual(panels[0].bottom);
   }
   observations.push({page:name,width,layout,screenshot:await site.capture('audit-'+width+'-'+name)});
  }
  await info.attach('page-resource-audit',{body:JSON.stringify({remote,errors,missing,observations},null,2),contentType:'application/json'});
  expect(remote).toEqual([]);expect(errors).toEqual([]);expect(missing).toEqual([]);
 });
});
