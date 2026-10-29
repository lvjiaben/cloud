import {test,expect} from '../support/source-site';
import {NavigationLayerPage} from '../pages/NavigationLayerPage';
test.use({baseURL:'http://localhost:6680',websiteGateway:true,headless:true});
test.describe('TC-149 全站导航高于正文栏目导航',()=>{
 for(const width of [1440,1024])test(`TC-149${width===1440?'a':'b'}: ${width}宽度展开菜单覆盖栏目导航，关闭后锚点仍可用`,async({page},info)=>{
  await page.setViewportSize({width,height:900});
  const site=new NavigationLayerPage(page);await site.open('/plan/web.html');
  await expect(page.locator('[data-web-solution] .et-container')).toBeVisible();
  await site.positionSectionNavigation();await site.clickTitle('解决方案');
  await expect(site.panel()).toBeVisible();
  await expect.poll(()=>site.overlap()).toEqual({intersects:true,menuOnTop:true});
  await page.screenshot({path:info.outputPath('menu-over-section-nav.png')});
  await site.closePanel();await expect(site.panel()).toHaveCount(0);
  await site.selectProblem();await expect(page).toHaveURL(/#web-problems$/);
  await expect(page.locator('.et-hero-tab a[href="#web-problems"]')).toHaveAttribute('aria-current','location');
  await page.screenshot({path:info.outputPath('section-nav-restored.png')});
 });
});


test.describe('TC-149 栏目吸顶强制收回公共菜单',()=>{
 for(const [index,path,width] of [[0,'/plan/web.html',1440],[1,'/plan/web.html',390],[2,'/Assure.html',1440],[3,'/Assure.html',390]] as const)test('TC-149'+String.fromCharCode(99+index)+': '+path+' '+width+'吸顶关闭并可恢复',async({page},info)=>{
  await page.setViewportSize({width,height:900});
  const site=new NavigationLayerPage(page);await site.open(path);
  if(width<768)await page.locator('.lina-menu-toggle').click();
  await site.clickTitle('解决方案');await expect(site.panel()).toBeVisible();
  await page.locator('.et-container').evaluate(el=>scrollTo({top:scrollY+el.getBoundingClientRect().top+20,behavior:'instant'}));
  await expect(page.locator('.et-container')).toHaveClass(/is-stuck/);
  await page.locator('.horizontal-menu-navbar-menu > li > [aria-expanded]').first().evaluate(el=>el.dispatchEvent(new FocusEvent('focus')));
  await expect(site.panel()).toHaveCount(0);
  await expect(page.locator('.nav-layoutcontainer')).not.toHaveClass(/lina-mobile-open/);
  await expect(page.locator('.horizontal-menu-navbar-menu [aria-expanded="true"]')).toHaveCount(0);
  await expect(page.locator('.lina-nav-panel:visible')).toHaveCount(0);
  await expect.poll(()=>page.locator('.nav-layoutcontainer').evaluate(el=>el.getBoundingClientRect().bottom)).toBeLessThanOrEqual(0);
  await page.screenshot({path:info.outputPath('sticky-menu-collapsed.png')});
  await page.evaluate(()=>scrollTo({top:0,behavior:'instant'}));
  await expect(page.locator('.et-container')).not.toHaveClass(/is-stuck/);
  if(width<768)await page.locator('.lina-menu-toggle').click();
  await site.clickTitle('解决方案');await expect(site.panel()).toBeVisible();
 });
});
