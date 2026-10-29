import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-26 受控非空目录与内容（不代表后台成功）',()=>{
 test('TC-26a: 分类、名称筛选、分页、可售与缺货人工确认入口',async({page})=>{
  const fixture=await controlledBusiness(page),site=new WebsitePage(page);
  await site.open('/products.html');
  await expect(page.locator('[data-products]')).toContainText('测试专用甲');
  const unavailable=page.locator('[data-products] > *').filter({hasText:'测试专用缺货'});
  await expect(unavailable).toContainText('暂不可订购');await expect(unavailable.locator('.buy-now-button')).toHaveCount(0);
  await page.locator('[data-more]').click();
  const manual=page.locator('[data-products] > *').filter({hasText:'测试专用人工确认'});
  await expect(manual).toContainText('需人工确认');await expect(manual.locator('.buy-now-button')).toHaveCount(0);
  await expect(page.locator('[data-more]')).toBeHidden();
  await page.getByRole('link',{name:'测试分类甲',exact:true}).click();
  await expect(page.locator('[data-products]')).not.toContainText('测试专用缺货');
  await site.searchProducts('测试专用甲');
  await expect(page.locator('[data-products] > *')).toHaveCount(1);
  await expect(page.locator('.buy-now-button')).toHaveAttribute('href','/configure.html?product=controlled-alpha');
  expect(fixture.requests.some(r=>r.path.includes('cursor=controlled-next'))).toBe(true);
  expect(fixture.requests.some(r=>r.path.includes('categoryNo=cat-a')&&r.path.includes('namePrefix='))).toBe(true);
  await site.capture('controlled-catalog-filter');
  fixture.state.empty=true;await site.searchProducts('无匹配');await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');
  expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
 });
 for(const [path,type] of [['news','news'],['announcements','announcement'],['help','knowledge'],['downloads','download']]){
  test('TC-26 '+type+': 非空分页搜索详情、空态及失败恢复',async({page})=>{
   const fixture=await controlledBusiness(page),site=new WebsitePage(page);
   await site.open('/'+path+'.html');await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');
   if(type!=='download'){
    const title=page.locator('[data-content-list] .news-title h1').first();
    await expect(title).toHaveText('测试专用'+type+'1');await expect(title).toHaveCSS('font-size','16px');await expect(title).toHaveCSS('font-weight','700');
    const time=page.locator('[data-content-list] .news-articleInformation .news-create-time').first();await expect(time).toHaveCSS('font-size','12px');await expect(time).toHaveCSS('padding-left','18px');
   }
   await page.locator('[data-more]').click();await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'2');
   await expect(page.locator('[data-more]')).toBeHidden();
   await site.searchContent('测试专用');await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');
   await site.capture('controlled-'+type+'-search');
   await page.locator('[data-content-list] a').first().click();await expect(page.locator('[data-content-body]')).toContainText('测试正文 可见文字');
   await site.open('/'+path+'.html');await site.searchContent('无匹配');await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');
   fixture.state.contentFailure=true;await site.searchContent('测试');await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');
   await expect(page.locator('[data-content-list]')).toBeEmpty();
   fixture.state.contentFailure=false;await site.retry();await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');
   expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
  });
 }
 test('TC-26f: 有效下载票据产生浏览器附件且字节一致（模拟传输）',async({page})=>{
  await controlledBusiness(page);const site=new WebsitePage(page),bytes=Buffer.from('受控测试附件，非正式下载');
  await page.route('**'+prefix+'/content/items/controlled-file',r=>r.fulfill({json:{code:0,data:{item:{contentNo:'controlled-file',title:'受控附件',body:'<p>受控下载</p>',file:{fileName:'controlled.txt'}}}}}));
  let tickets=0;
  await page.route('**'+prefix+'/content/items/controlled-file/download-tickets',r=>{tickets++;return r.fulfill({json:{code:0,data:{token:'controlled-ticket',expiresAt:Date.now()+60000}}});});
  await page.route('**'+prefix+'/content/downloads/controlled-ticket',r=>r.fulfill({headers:{'content-type':'application/octet-stream','content-disposition':'attachment; filename="controlled.txt"'},body:bytes}));
  await site.open('/content.html?id=controlled-file');await site.capture('controlled-download-ready');
  const pending=page.waitForEvent('download');await site.download();const download=await pending;
  expect(download.suggestedFilename()).toBe('controlled.txt');
  expect(createHash('sha256').update(await readFile((await download.path())!)).digest('hex')).toBe(createHash('sha256').update(bytes).digest('hex'));
  expect(tickets).toBe(1);
 });
 for(const [path,type] of [['news','news'],['announcements','announcement'],['help','knowledge']]){
  test('TC-26 '+type+': 加载更多主题与下载一致，分页错误恢复及无游标隐藏',async({page},info)=>{
   const fixture=await controlledBusiness(page),site=new WebsitePage(page);
   await site.open('/downloads.html');await expect(page.locator('[data-more]')).toBeVisible();
   const style=()=>page.locator('[data-more]').evaluate(el=>{const c=getComputedStyle(el);return {background:c.backgroundColor,color:c.color,radius:c.borderRadius,font:c.fontSize,lineHeight:c.lineHeight,padding:c.padding,height:Math.round(el.getBoundingClientRect().height)};});
   const downloadTheme=await style();
   await site.open('/'+path+'.html');await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');
   await expect(page.locator('[data-more]')).toBeVisible();await expect(page.locator('[data-more]')).toHaveText('加载更多');
   await site.captureContent(info.outputPath(path+'-more-visible.png'),0);
   expect(await style()).toEqual(downloadTheme);
   fixture.state.contentFailure=true;await site.moreContent();await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');
   await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');await expect(page.locator('[data-more]')).toBeHidden();
   await site.captureContent(info.outputPath(path+'-more-error.png'),0);
   fixture.state.contentFailure=false;await site.retry();await expect(page.locator('[data-status]')).toHaveText('');
   await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'2');await expect(page.locator('[data-content-list] > li')).toHaveCount(2);await expect(page.locator('[data-more]')).toBeHidden();
   expect(fixture.requests.filter(r=>r.path.includes('cursor=controlled-next')).length).toBe(2);
   await site.searchContent('测试');await expect(page.locator('[data-content-list]')).toContainText('测试专用'+type+'1');await expect(page.locator('[data-more]')).toBeHidden();
   await site.searchContent('无匹配');await expect(page.locator('[data-content-list]')).toBeEmpty();await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');await expect(page.locator('[data-more]')).toBeHidden();
   await site.captureContent(info.outputPath(path+'-more-empty.png'),0);
   expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
  });
 }

});
