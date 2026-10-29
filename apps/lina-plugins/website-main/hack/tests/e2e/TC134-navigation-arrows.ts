import {test,expect} from '../support/source-site';
import {NavigationArrowsPage} from '../pages/NavigationArrowsPage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
test.describe('TC-134 公共导航箭头',()=>{
 for(const mobile of [false,true])test(mobile?'TC-134b 手机全局箭头':'TC-134a 桌面全局箭头',async({page},info)=>{
  test.setTimeout(180000);
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new NavigationArrowsPage(page);
  for(const path of ['/', '/products.html','/plan/game.html','/plan/financial.html','/plan/safety.html','/plan/Goingtosea.html','/plan/government.html','/plan/web.html','/plan/electricity.html','/cooperation/partner.html','/Assure.html','/bt.html','/contact.html','/about.html','/help.html','/news.html','/downloads.html','/aff.html','/webmaster.html']){
   await site.open(path);
   await expect(site.arrows()).toHaveCount(5);
   expect(await site.styles()).toEqual(Array.from({length:5},()=>({content:'""',width:'5px',height:'5px',right:'1px',bottom:'1px',transform:'matrix(0.707107, 0.707107, -0.707107, 0.707107, 0, 0)'})));
   await site.toggle('生态合作');
   await expect(site.menu('生态合作')).toHaveAttribute('aria-expanded','true');
   await expect.poll(()=>site.rotation('生态合作')).toBe('matrix(-0.707107, -0.707107, 0.707107, -0.707107, 0, 0)');
   await expect(page.locator('.lina-open .lina-nav-panel')).toBeVisible();
   if(path==='/cooperation/partner.html')await page.screenshot({path:info.outputPath(mobile?'arrows-mobile.png':'arrows-desktop.png')});
   await site.close();
   await expect(site.menu('生态合作')).toHaveAttribute('aria-expanded','false');
  }
 });
});
