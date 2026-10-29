import {test,expect} from '@playwright/test';
import {NavigationPage} from '../pages/NavigationPage';
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-68 正式官网真实鼠标悬停',()=>{
 test('TC-68a: 当前插件资源、五组标题移入、跨面板与移出收起',async({page},info)=>{
  const site=new NavigationPage(page);await site.open('/');
  await expect(page.locator('base')).toHaveAttribute('href','/x-assets/website-main/v0.1.39/');
  for(const label of ['产品与服务','解决方案','生态合作','支持与服务','了解我们']){
   await site.leaveNavigation();
   const title=page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:label});await title.hover();
   await expect(title).toHaveAttribute('aria-expanded','true');
   const panel=page.locator('.lina-open .lina-nav-panel');await expect(panel).toBeVisible();
   await expect(panel.locator('h2')).toHaveText(label);await expect(panel.locator('a[href]').first()).toBeVisible();
   await site.enterPanel();await expect(panel).toBeVisible();
   await page.screenshot({path:info.outputPath(Date.now()+'-'+label+'.png')});
   await site.leaveNavigation();await expect(panel).toHaveCount(0);await expect(title).toHaveAttribute('aria-expanded','false');
  }
 });
});

