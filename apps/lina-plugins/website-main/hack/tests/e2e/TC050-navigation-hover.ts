import {test,expect} from '../support/source-site';import {NavigationPage} from '../pages/NavigationPage';import {previewBrand,previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-50 后台导航悬停',()=>{
 test('TC-50a: 四配置组切换、跨面板、离开、关闭、键盘及十页一致',async({page},info)=>{
  test.setTimeout(90000);
  let mode='groups';const labels=['解决方案','生态合作','支持与服务','了解我们'];
  await page.route('**/x/**',async r=>{const q=r.request(),u=new URL(q.url()),v=previewResponse(u,q.method());if(u.pathname.endsWith('/brand'))v.data={brand:{...previewBrand,navigation:mode==='empty'?[]:mode==='custom'?[{label:{zhCN:'管理员自定义'},group:true,children:[{label:{zhCN:'本方新闻'},href:'/news.html'}]}]:[...labels.map(label=>({label:{zhCN:label},group:true,children:[{label:{zhCN:'模拟本方栏目'},group:true,children:[{label:{zhCN:'本方帮助'},href:'/help.html'},{label:{zhCN:'未配置目标'.repeat(20)}}]}]})),{label:{zhCN:'应用市场'},href:'/downloads.html'}]}};await r.fulfill({status:v.status,json:{code:0,data:v.data}});});
  const site=new NavigationPage(page);
  for(const path of ['index','products','configure','news','announcements','help','downloads','content','about','contact']){await site.open('/'+path+'.html');for(let i=0;i<4;i++){await site.hoverGroup(i);await expect(page.locator('.lina-open')).toHaveCount(1);await expect(page.locator('.lina-open h2')).toHaveText(labels[i]);await site.enterPanel();await expect(page.locator('.lina-open')).toHaveCount(1);await expect(page.locator('.lina-open a[href]')).toHaveAttribute('href',/help.html$/);}await site.closePanel();await expect(page.locator('.lina-open')).toHaveCount(0);}
  await site.hoverGroup(0);const overflow=await page.locator('.lina-open .lina-nav-panel').evaluate(root=>({width:root.clientWidth,scroll:root.scrollWidth}));expect(overflow.scroll).toBeLessThanOrEqual(overflow.width+1);await site.captureSection('.nav-layoutcontainer',info.outputPath('configured-panel.png'));await site.leaveNavigation();await expect(page.locator('.lina-open')).toHaveCount(0);
  await site.focusGroup(2);await expect(page.locator('.lina-open h2')).toHaveText('支持与服务');await site.escape();await expect(page.locator('.lina-open')).toHaveCount(0);
  await site.market();await expect(page).toHaveURL(/downloads.html$/);
  mode='custom';await site.open('/');await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('管理员自定义');await expect(page.locator('.horizontal-menu-navbar-menu')).not.toContainText('解决方案');
  mode='empty';await site.open('/');await expect(page.locator('.lina-nav-panel')).toHaveCount(0);await expect(page.locator('.horizontal-menu-navbar-menu > li')).toHaveCount(0);await expect(page.getByRole('link',{name:'控制台',exact:true})).toBeVisible();
 });
});
