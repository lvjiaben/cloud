import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-107 官网全局中英文切换',()=>{
 test('原框架译文、双语资料、跨页与刷新、单语正文保留及错误恢复',async({page},info)=>{
  test.setTimeout(120000);
  const {state,requests}=await controlledBusiness(page);
  state.brand.siteName={zhCN:'双语测试网站',enUS:'Bilingual Test Website'};
  state.brand.seoTitle=state.brand.siteName;
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  await page.goto('/');
  const toggle=page.locator('[data-public-locale-switch]');await expect(toggle).toHaveText('English');
  await expect(page.locator('[data-brand-name]')).toHaveText('双语测试网站');
  await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await expect(page.locator('[data-brand-name]')).toHaveText('Bilingual Test Website');
  await expect(page.locator('.sk-title').first()).toHaveText('Cloud products and services for your business');
  await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('Products & Services');
  await expect(page.locator('.navbar-Login')).toHaveText('Sign up');
  const registration=await page.locator('.navbar-Login').boundingBox(),switchBounds=await toggle.boundingBox();expect(switchBounds!.x).toBeGreaterThanOrEqual(registration!.x+registration!.width);expect(Math.abs(switchBounds!.y+switchBounds!.height/2-registration!.y-registration!.height/2)).toBeLessThan(2);
  const searchBounds=await page.locator('.input-inner-wrapper > .input-label').boundingBox(),iconBounds=await page.locator('.input-label .input-suffix').boundingBox();
  expect(iconBounds!.x+iconBounds!.width).toBeLessThanOrEqual(searchBounds!.x+searchBounds!.width);
  await page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'Partners'}).click();
  const ad=page.locator('.lina-open .lina-nav-promotion');await expect(ad).toContainText('Referral partnership');await expect(ad).toBeVisible();
  await page.screenshot({path:info.outputPath('english-desktop-navigation.png'),animations:'disabled'});
  const partnerCard=page.locator('.lina-nav-panel[data-menu="生态合作"] a[href$="/cooperation/partner.html"] .Submenu-title');
  await expect(partnerCard).toHaveText('Partners');
  await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','zh-CN');
  const ecosystemTitle=page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'生态合作'});
  await expect(ecosystemTitle).toHaveText('生态合作');
  await expect(page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'合作伙伴'})).toHaveCount(0);
  await ecosystemTitle.click();await expect(partnerCard).toHaveText('合作伙伴');
  await expect(partnerCard.locator('..').locator('..')).toHaveAttribute('href','http://localhost:6680/cooperation/partner.html');
  await page.screenshot({path:info.outputPath('restored-chinese-ecosystem.png'),animations:'disabled'});
  await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  // A fresh English module import must retain each original Chinese source too.
  await page.reload();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'Partners'}).click();
  await expect(partnerCard).toHaveText('Partners');
  await toggle.click();await expect(ecosystemTitle).toHaveText('生态合作');
  await ecosystemTitle.click();await expect(partnerCard).toHaveText('合作伙伴');
  await page.screenshot({path:info.outputPath('english-reload-restored-chinese-ecosystem.png'),animations:'disabled'});
  await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await page.keyboard.press('Escape');
  const urls=['/products.html','/news.html','/announcements.html','/help.html','/downloads.html','/newestactivity.html','/contact.html','/about.html'];
  for(const url of urls){
   await page.goto(url);await expect(page.locator('html')).toHaveAttribute('lang','en-US');await expect(toggle).toHaveText('中文');
   await expect(page.locator('.navbar-Login')).toHaveText('Sign up');
   const bounds=await toggle.boundingBox();expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(1440);
  }
  await page.reload();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await page.goto('/404.html');await expect(page.locator('html')).toHaveAttribute('lang','en-US');await expect(page.getByRole('heading')).toHaveText('Page not found or not yet available');
  await page.goto('/content.html?id=controlled-news-1');await expect(page.locator('[data-content-body]')).toContainText('测试正文');
  state.contentFailure=true;await page.goto('/news.html');
  await expect(page.locator('[data-status]')).toHaveText('Content is temporarily unavailable. Try again.');
  await expect(page.locator('[data-retry]')).toHaveText('Try again');
  state.contentFailure=false;await page.locator('[data-retry]').click();await expect(page.locator('[data-content-list]')).toContainText('测试专用news1');
  expect(requests.some(request=>request.method==='POST')).toBe(false);
  await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','zh-CN');await expect(page.locator('.navbar-Login')).toHaveText('免费注册');
  expect(errors).toEqual([]);
 });
 test('手机英文按钮、导航和异步商品英文资料',async({page},info)=>{
  const {state}=await controlledBusiness(page);state.brand.siteName={zhCN:'中文测试品牌',enUS:'English Brand'};
  await page.route('**/x/idc-core/api/v1/public/catalog/products?*',route=>route.fulfill({json:{code:0,data:{categories:[{categoryNo:'cat-a',nameZhCN:'中文分类',nameEnUS:'Cloud category'}],products:[{publicSlug:'controlled-alpha',categoryNo:'cat-a',nameZhCN:'中文服务器',nameEnUS:'English Server',summaryZhCN:'中文规格',summaryEnUS:'4 vCPU; Memory: 8 GB',availability:'available',currencies:['CNY']}],nextCursor:''}}}));
  await page.goto('/');await page.locator('[data-public-locale-switch]').click();
  await expect(page.locator('.swipervertical-title').first()).toHaveText('English Server');
  await page.setViewportSize({width:390,height:844});await page.locator('.lina-menu-toggle').click();
  const toggle=page.locator('[data-public-locale-switch]');await expect(toggle).toBeVisible();
  await page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'Partners'}).click();
  await page.screenshot({path:info.outputPath('english-mobile-navigation.png'),animations:'disabled'});
  expect(await page.locator('body').evaluate(el=>el.scrollWidth)).toBeLessThanOrEqual(390);
  await toggle.click();await expect(page.locator('.swipervertical-title').first()).toHaveText('中文服务器');
 });
});
