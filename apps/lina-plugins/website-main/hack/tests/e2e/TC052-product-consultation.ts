import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewBrand,previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-52 商品介绍与定制咨询',()=>{
 test('TC-52a: 固定介绍忽略旧配置，咨询及入场动画保持可用',async({page},info)=>{
  let populated=false;await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand'))r.data={brand:{...previewBrand,templateConfig:populated?{productintrotitle:'本方介绍',productintrodescription:'本方配置说明'}:{}}};await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.motion('no-preference');await site.open('/');
  const intro=page.locator('[data-home-product-intro]');await site.focusSection('[data-home-product-intro]');await expect(intro.locator('.systemproduct-left-top')).toBeVisible();await expect(intro.locator('.systemproduct-left-title')).toHaveText('全部商品');await expect(intro.locator('.systemproduct-left-desc')).toHaveText('');await expect(intro.locator('.systemproduct-right-bottom')).toBeVisible();await expect(intro).toHaveClass(/lina-product-intro-entered/);await expect(intro).toHaveCSS('animation-name','slideInLeft');
  await expect(intro.locator('.systemproduct-right-bottom-desc')).toHaveText('更多定制机型，为您提供个性化高性能服务器解决方案。');await expect(intro.locator('a')).toHaveAttribute('href','/contact.html');await site.captureSection('[data-home-product-intro]',info.outputPath('empty-intro-consultation.png'));
  populated=true;await site.motion('reduce');await site.open('/');await site.focusSection('[data-home-product-intro]');await expect(intro.locator('.systemproduct-left-title')).toHaveText('全部商品');await expect(intro.locator('.systemproduct-left-desc')).toHaveText('');await expect(intro).toHaveCSS('animation-name','none');const left=await intro.boundingBox(),products=await page.locator('[data-products]').boundingBox();expect(products!.x).toBeGreaterThanOrEqual(left!.x+left!.width);expect(products!.width).toBeGreaterThan(600);await site.captureSection('[data-home-product-intro]',info.outputPath('populated-intro-consultation.png'));await site.consultation();await expect(page).toHaveURL(/contact.html$/);
 });
 test('TC-52b: 四张固定快捷卡没有未经确认的业务入口',async({page},info)=>{
  await page.route('**/x/**',async route=>{const q=route.request(),r=previewResponse(new URL(q.url()),q.method());await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');const cards=page.locator('[data-home-blocks] > .bannerbelow-list');
  await expect(cards.locator('.bannerbelow-title')).toHaveText(['最新活动','产品特惠、轻松上云','推广赚佣 轻松赚钱','弹性计算 管理简单']);
  await expect(cards.locator('.bannerbelow-description')).toHaveText(['产品促销、助您降本增效','4核2G西信高防云','每笔推荐均可赚钱（现金返佣）','云控制台、弹性配置，灵活管理']);
  for(let i=0;i<4;i++){await expect(cards.nth(i)).not.toHaveAttribute('href');await expect(cards.nth(i)).toHaveAttribute('aria-disabled','true');await expect(cards.nth(i)).toHaveAttribute('title','尚未关联本方业务入口');await site.clickShortcut(i);await expect(page).toHaveURL('http://localhost:6680/');}
  await site.captureSection('.bannerbelow-layoutcontainer',info.outputPath('fixed-shortcuts.png'));
 });
});
