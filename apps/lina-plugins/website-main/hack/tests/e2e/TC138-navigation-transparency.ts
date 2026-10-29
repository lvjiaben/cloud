import {test,expect} from '../support/source-site';
import {NavigationTransparencyPage} from '../pages/NavigationTransparencyPage';
const live=process.env.NAV_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live,headless:true});

test.describe('TC-138 公共导航透明背景',()=>{
 for(const width of [1440,390])test(`TC-138${width===1440?'a':'b'}: ${width}px跨页面透明与菜单可读性`,async({page},info)=>{
  test.setTimeout(180000);
  await page.setViewportSize({width,height:1000});
  // Keep this visual regression independent of unrelated catalog/backend latency.
  await page.route('**/x/**',route=>route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'本方官网'},navigation:[]}}}}));
  const site=new NavigationTransparencyPage(page);
  for(const path of ['/','/bt.html','/Assure.html','/plan/game.html','/plan/safety.html','/plan/government.html','/plan/financial.html','/about.html','/contact.html','/products.html','/help.html','/cooperation/partner.html','/news.html','/downloads.html','/aff.html','/webmaster.html','/plan/Goingtosea.html','/plan/web.html','/plan/electricity.html']){
   await site.open(path);
   await expect(page.locator('.headerBg')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
   await expect(page.locator('.headerBg')).toHaveCSS('backdrop-filter','blur(12px)');
   await site.toggle('生态合作');
   const panel=page.locator('.lina-open .lina-nav-panel');
   await expect(panel).toBeVisible();
   expect(await panel.evaluate(e=>getComputedStyle(e).backgroundColor)).not.toBe('rgba(0, 0, 0, 0)');
   if(path==='/bt.html')await page.screenshot({path:info.outputPath(`bt-menu-${width}.png`)});
   await site.close();
   await expect(site.menu('生态合作')).toHaveAttribute('aria-expanded','false');
   await expect(panel).toHaveCount(0);
   await site.scroll(420);
   await expect(page.locator('.headerBg')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
   await site.scroll(0);
   if(path==='/bt.html')await page.screenshot({path:info.outputPath(`bt-transparent-${width}.png`)});
  }
 });
});
