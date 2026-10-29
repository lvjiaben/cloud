import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {mountOriginalSite,originalOrigin} from '../support/original-site';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
const endpoint=prefix+'/content/items/article-layout';
const item={contentNo:'article-layout',type:'knowledge',title:'受控正文布局验收',category:{nameZhCN:'受控分类'},publishedAt:1792200000000,summary:'受控摘要',body:'<h2>准备工作</h2><p>本段只在浏览器测试中出现。</p>'+Array.from({length:35},(_,i)=>'<p>受控长文段落 '+i+'</p>').join('')+'<h3>处理步骤</h3><pre><code>'+('long-code-'.repeat(80))+'</code></pre><table><tr>'+Array.from({length:20},(_,i)=>'<th>受控列'+i+'</th>').join('')+'</tr></table><script>throw new Error("source script")</script>'};
test.describe('TC-31 直接迁移原正文页，不发布原站文章',()=>{
 test('TC-31a: 原详情容器、真实字段投影、目录定位和列表归属',async({page},info)=>{
  await controlledBusiness(page);await mountOriginalSite(page);const site=new WebsitePage(page);
  await site.open(originalOrigin+'/109.html');
  const original=page.locator('.news-txt-title h1');
  const style=await original.evaluate(el=>({fontSize:getComputedStyle(el).fontSize,lineHeight:getComputedStyle(el).lineHeight}));
  const sourceScreenshot=await site.capture('original-article-1440');
  await page.route('**'+endpoint,r=>r.fulfill({json:{code:0,data:{item}}}));
  await site.open('/content.html?id=article-layout');
  await expect(page.locator('.et-main.articleContent .news-txt-title h1')).toHaveText(item.title);
  expect(await page.locator('.news-txt-title h1').evaluate(el=>({fontSize:getComputedStyle(el).fontSize,lineHeight:getComputedStyle(el).lineHeight}))).toEqual(style);
  await expect(page.locator('[data-content-parent]')).toHaveAttribute('href','/help.html');
  await expect(page.locator('[data-content-category]')).toHaveText(['受控分类','受控分类']);
  await expect(page.locator('[data-content-time]')).toContainText('发布时间：');
  await expect(page.locator('.news-txt-title-info-author,.news-txt-title-info-read,.news-txtpaging')).toHaveCount(0);
  await expect(page.locator('[data-content-directory] a')).toHaveText(['准备工作','处理步骤']);
  await expect(page.locator('[data-content-body] script')).toHaveCount(0);
  expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
  const migratedScreenshot=await site.capture('migrated-article-1440');
  await site.openArticleHeading('处理步骤');
  await expect(page).toHaveURL(/#content-heading-1$/);
  const heading=page.locator('#content-heading-1');
  await expect.poll(()=>heading.evaluate(el=>Math.round(el.getBoundingClientRect().top))).toBeGreaterThanOrEqual(62);
  // A heading near the end may stop above the footer before reaching the top.
  await expect.poll(()=>heading.evaluate(el=>el.getBoundingClientRect().bottom)).toBeLessThan(900);
  expect(await page.evaluate(()=>scrollY)).toBeGreaterThan(0);
  await site.capture('article-heading-navigation',false);
  await site.articleTool('top');await expect.poll(()=>page.evaluate(()=>scrollY)).toBe(0);
  await site.articleTool('back');await expect(page).toHaveURL(/\/help.html$/);
  await expect(page.locator('[data-content-list]')).toContainText('测试专用knowledge1');
  await info.attach('article-source-layout',{body:JSON.stringify({source:'109.html',style,sourceScreenshot,migratedScreenshot,limitation:'正文资料受控，不代表已发布真实文章；原作者、阅读量和前后篇关系不复用。'}),contentType:'application/json'});
 });
 test('TC-31b: 分享成功和浏览器拒绝时分别显示真实结果',async({page})=>{
  await controlledBusiness(page);const site=new WebsitePage(page);
  await page.addInitScript(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async(value:string)=>{if(location.search.includes('denied'))throw new Error('denied');(window as any).__copiedArticleURL=value;}}});});
  await page.route('**'+endpoint+'*',r=>r.fulfill({json:{code:0,data:{item}}}));
  await site.open('/content.html?id=article-layout');await expect(page.locator('.news-txt-title h1')).toHaveText(item.title);
  await site.articleTool('copy');await expect(page.locator('[data-status]')).toHaveText('链接已复制，可以粘贴分享。');
  expect(await page.evaluate(()=>(window as any).__copiedArticleURL)).toBe(page.url());
  await site.open('/content.html?id=article-layout&denied=1');await expect(page.locator('.news-txt-title h1')).toHaveText(item.title);
  await site.articleTool('copy');await expect(page.locator('[data-status]')).toHaveText('未能复制链接，请复制浏览器地址栏中的网址。');
  await site.capture('article-share-denied');
 });
 test('TC-31c: 正文未发布或接口失败时保持空正文，重试后恢复原详情结构',async({page})=>{
  await controlledBusiness(page);const site=new WebsitePage(page);let fail=true;
  await page.route('**'+endpoint,r=>r.fulfill({json:fail?{code:50,data:null}:{code:0,data:{item:{...item,publishedAt:0,category:null,body:'<p>恢复后的受控正文</p>'}}}}));
  await site.open('/content.html?id=article-layout');await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');
  await expect(page.locator('[data-content-body]')).toBeEmpty();await expect(page.locator('body')).not.toContainText(/小白云|身份验证错误|3383|小久/);
  await site.capture('article-unpublished-failure');
  fail=false;await site.retry();await expect(page.locator('[data-content-body]')).toHaveText('恢复后的受控正文');
  await expect(page.locator('.news-txt-title-info')).toBeHidden();await expect(page.locator('[data-content-directory]')).toHaveText('全文');
  await expect(page.locator('[data-retry]')).toBeHidden();await site.capture('article-recovered');
 });
 test('TC-31d: 桌面328px正文宽表可读且图片代码不撑破，普通两列表格无回归',async({page},info)=>{
  await controlledBusiness(page);const site=new WebsitePage(page);
  const normal='<table data-normal><thead><tr><th>名称</th><th>说明</th></tr></thead><tbody><tr><td>普通字段</td><td>普通两列表格说明</td></tr></tbody></table>';
  const wide='<table><thead><tr>'+Array.from({length:12},(_,i)=>'<th>列'+i+'</th>').join('')+'</tr></thead><tbody><tr>'+Array.from({length:12},(_,i)=>'<td>'+('WIDE_CELL_'+i+'_').repeat(5)+'</td>').join('')+'</tr></tbody></table>';
  await page.route('**/layout-probe.svg',r=>r.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="1800" height="600"><rect width="1800" height="600" fill="#dce8f4"/><text x="40" y="150" font-size="90">ISOLATED 1800 x 600</text></svg>'}));
  await page.route('**'+endpoint,r=>r.fulfill({json:{code:0,data:{item:{...item,body:'<h2>仅浏览器布局探针</h2><img src="/layout-probe.svg" alt="尺寸探针"><pre><code>'+('LONG_CODE_'.repeat(100))+'</code></pre>'+normal+wide}}}}));
  await site.open('/content.html?id=article-layout');await expect(page.locator('[data-content-body] table')).toHaveCount(2);
  for(const width of [328,964]){
   await site.constrainArticle(width);
   await expect.poll(()=>page.locator('[data-content-body] img').evaluate((e:HTMLImageElement)=>e.naturalWidth)).toBe(1800);
   await site.captureContent(info.outputPath('article-'+width+'.png'),120);
   const metrics=await page.locator('[data-content-body]').evaluate(el=>{
    const [normal,wide]=[...el.querySelectorAll('table')];const img=el.querySelector('img')!,pre=el.querySelector('pre')!;
    return {width:el.clientWidth,scroll:el.scrollWidth,imgWidth:img.getBoundingClientRect().width,imgRatio:img.getBoundingClientRect().width/img.getBoundingClientRect().height,
     preWidth:pre.getBoundingClientRect().width,preScroll:pre.scrollWidth,preOverflow:getComputedStyle(pre).overflowX,
     normalColumns:normal.rows[0].cells.length,normalRowHeight:normal.rows[1].getBoundingClientRect().height,normalScroll:normal.scrollWidth,normalWidth:normal.clientWidth,
     wideWidth:wide.clientWidth,wideScroll:wide.scrollWidth,wideOverflow:getComputedStyle(wide).overflowX,cellWidths:[...wide.rows[1].cells].map(c=>c.getBoundingClientRect().width),wideRowHeight:wide.rows[1].getBoundingClientRect().height};
   });
   await info.attach('metrics-'+width,{body:JSON.stringify(metrics),contentType:'application/json'});
   expect(metrics.width).toBe(width);expect(metrics.scroll).toBe(width);expect(metrics.imgWidth).toBeLessThanOrEqual(width);expect(metrics.imgRatio).toBeCloseTo(3,1);
   expect(metrics.preWidth).toBeLessThanOrEqual(width);expect(metrics.preScroll).toBeGreaterThan(width);expect(metrics.preOverflow).toBe('auto');
   expect(metrics.normalColumns).toBe(2);expect(metrics.normalRowHeight).toBeLessThan(120);expect(metrics.normalScroll).toBeLessThanOrEqual(metrics.normalWidth+1);
   expect(Math.min(...metrics.cellWidths)).toBeGreaterThanOrEqual(112);expect(metrics.wideRowHeight).toBeLessThan(500);expect(metrics.wideOverflow).toBe('auto');
   if(width===328){expect(metrics.wideScroll).toBeGreaterThan(metrics.wideWidth);expect(await site.scrollArticleTable()).toBeGreaterThan(0);await site.captureContent(info.outputPath('article-328-scrolled.png'),120);}
  }
 });

});
