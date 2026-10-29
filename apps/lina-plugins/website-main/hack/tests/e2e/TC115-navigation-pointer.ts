import {test,expect} from '../support/source-site';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000}});
test.describe('TC115 共用导航鼠标路径',()=>{
 for(const path of ['/downloads.html','/webmaster.html','/contact.html','/'])test(path+' 标题进入面板并点击',async({page},info)=>{
  await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{name:'测试官网',navigation:[]}}}}));
  await page.goto(path);
  const title=page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'支持与服务'});
  await title.hover();const panel=page.locator('.lina-open .lina-nav-panel');await expect(panel).toBeVisible();
  await page.waitForTimeout(180);
  const target=panel.locator('a').filter({hasText:'下载中心'});const box=await target.boundingBox();expect(box).not.toBeNull();
  const start=await title.boundingBox();
  for(let step=1;step<=30;step++){
   await page.mouse.move(start!.x+start!.width/2+(box!.x+box!.width/2-start!.x-start!.width/2)*step/30,start!.y+start!.height/2+(box!.y+box!.height/2-start!.y-start!.height/2)*step/30);
   await page.waitForTimeout(10);
  }
  await expect(panel).toBeVisible();await expect(target).toBeVisible();
  await page.screenshot({path:info.outputPath('navigation-pointer.png')});
  await target.click();await expect(page).toHaveURL('http://localhost:6680/downloads.html');
  await title.hover();await expect(panel).toBeVisible();await page.mouse.move(10,900);await expect(panel).toBeHidden();
 });
});
