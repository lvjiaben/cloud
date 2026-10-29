import {test,expect} from '../support/source-site';
import {NavigationArrowsPage} from '../pages/NavigationArrowsPage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
test.describe('TC-139 工信部许可证查询入口',()=>{
 for(const mobile of [false,true])test(mobile?'TC-139b 手机说明入口':'TC-139a 桌面标题入口',async({page,context},info)=>{
  test.setTimeout(90000);
  // Controlled brand data isolates this link regression from intermittent API failures.
  const brandData={code:0,data:{brand:{siteName:{zhCN:'导航入口验收'},navigation:[{label:'了解我们',group:true,children:[{label:'增值许可证查询',href:'/404.html'}]}]}}};
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:brandData}));
  const target='https://dxyw.miit.gov.cn/dxxzsp/xkz/xkzgl/resource/qiyesearch.jsp?num=B1-xxx&type=xuke';
  if(mobile)await page.setViewportSize({width:390,height:844});
  // Only prove the outgoing URL; do not depend on or write to the ministry site.
  const requests:string[]=[];
  await context.route('https://dxyw.miit.gov.cn/**',r=>{requests.push(r.request().url());return r.fulfill({contentType:'text/html',body:'<title>Navigation target intercepted</title>'});});
  const site=new NavigationArrowsPage(page);await site.open('/');await site.toggle('了解我们');
  const entry=page.locator('[data-menu="了解我们"] a.Submenu-item').filter({hasText:'增值许可证查询'});
  await expect(entry).toHaveAttribute('href',target);await expect(entry).toHaveAttribute('target','_blank');
  await expect(entry).toHaveAttribute('rel','noopener noreferrer');
  await expect(entry.locator('.Submenu-desc')).toHaveText('查找公开的增值业务许可证资料。');
  await entry.scrollIntoViewIfNeeded();await page.screenshot({path:info.outputPath('license-menu.png')});
  const opened=page.waitForEvent('popup');await entry.locator(mobile?'.Submenu-desc':'.Submenu-title').click();
  const popup=await opened;await popup.waitForLoadState();expect(popup.url()).toBe(target);expect(requests).toContain(target);await popup.close();
 });
});
