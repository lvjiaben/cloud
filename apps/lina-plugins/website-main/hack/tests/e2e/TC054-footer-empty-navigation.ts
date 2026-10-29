import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-54 页脚默认导航与退役配置隔离',()=>{
 test('TC-54a: 重复装配恢复默认四列且忽略旧备案配置',async({page},info)=>{
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand'))r.data={brand:{...previewBrand,footerNavigation:[]}};await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');const root=page.locator('[data-footer-navigation]');await expect(root.locator('.sky-footer-nav-heading')).toHaveCount(4);const original=await root.innerHTML();
  await site.renderFooter([{group:true,label:{zhCN:'动态栏目'},children:[{label:{zhCN:'本方新闻'},href:'/news.html'}]}]);await expect(root).toContainText('动态栏目');await expect(root.locator('a')).toHaveAttribute('href',/news.html$/);
  for(const navigation of [[],undefined]){await site.renderFooter(navigation);await expect(root.locator('.sky-footer-nav-heading')).toHaveText(['服务指南','代理系统','帮助中心','关于我们']);expect(await root.innerHTML()).toBe(original);await expect(root).not.toContainText('动态栏目');await expect(page.locator('[data-filing-kind=icp]')).toBeHidden();await expect(page.locator('[data-filing-kind=icp] .filing-title')).toBeEmpty();await expect(page.locator('[data-footer-logo]')).toHaveCount(0);}
  await site.captureSection('.sky-footer-container',info.outputPath('restored-default-footer.png'));
 });
});
