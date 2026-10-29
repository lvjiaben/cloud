import {test,expect} from '../support/source-site';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
import {HomeDetailPage} from '../pages/HomeDetailPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-44 首页快捷图与方案原图',()=>{
 test('TC-44a: 新闻帮助按链接使用中性图标，方案长标题保持单行并可切换',async({page},info)=>{
  const site=new HomeDetailPage(page);const titles=['测试企业云计算行业解决方案一','测试企业网络安全行业解决方案二','测试企业数据存储行业解决方案三'];
  await page.route('**/x/**',async route=>{const r=route.request(),result=previewResponse(new URL(r.url()),r.method());
   if(new URL(r.url()).pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,shortcuts:'测试新闻 | 测试说明 | /news.html?from=home'+String.fromCharCode(10)+'测试帮助 | 测试说明 | /help.html',solutions:titles.map(title=>title+' | 测试说明 | - | -').join(String.fromCharCode(10))}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  await site.open('/index.html');
  const cards=page.locator('[data-home-blocks] > .bannerbelow-list');
  await expect(cards).toHaveCount(2);
  for(const [index,icon] of ['newspaper','book-open'].entries()){
   await expect(cards.nth(index).locator('.bannerbelow-item-icon')).toHaveCSS('background-image',new RegExp('sidebar-'+icon+'[.]svg'));
   await site.hoverShortcut(index);await expect(cards.nth(index).locator('.bannerbelow-item-icon')).toHaveCSS('animation-name','none');
  }
  await site.captureSection('.bannerbelow-layoutcontainer',info.outputPath('shortcuts.png'));

 });
 test('TC-44b: 显式本方图优先，未知及外站快捷不猜图，空资料仍隐藏方案',async({page},info)=>{
  const site=new HomeDetailPage(page);let empty=false;
  await page.route('**/x/**',async route=>{const r=route.request(),result=previewResponse(new URL(r.url()),r.method());
   if(new URL(r.url()).pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,shortcuts:'测试商品 | - | /products.html'+String.fromCharCode(10)+'测试外站新闻 | - | https://example.test/news.html',solutions:empty?'':'测试本方图 | 测试说明 | /static/picture/sidebar-newspaper.svg | /products.html'}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  await site.open('/index.html');await expect(page.locator('.main-option-name')).toHaveCount(7);
  await expect(page.locator('[data-home-blocks] .bannerbelow-icon-container:visible')).toHaveCount(2);await expect(page.locator('[data-home-blocks] .bannerbelow-item-icon:visible')).toHaveCount(0);
  await site.captureSection('.bannerbelow-layoutcontainer',info.outputPath('external-shortcuts.png'));
  empty=true;await site.open('/index.html');await expect(page.locator('.container_bg15')).toBeVisible();
 });
});
