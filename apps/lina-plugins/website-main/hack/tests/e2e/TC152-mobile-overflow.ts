import {test,expect} from '../support/source-site';

test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});

test.describe('TC-152 手机首页横向溢出',()=>{
 test('TC-152a: 首页及方案页保持视口宽度',async({page},info)=>{
  test.setTimeout(90000);
  for(const path of ['/','/plan/web.html','/plan/safety.html','/cooperation/partner.html']){
   await page.setViewportSize({width:390,height:844});
   await page.goto(path);await page.waitForLoadState('domcontentloaded');
   await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
   await page.screenshot({path:info.outputPath(`${path.replaceAll('/','-')||'home'}-mobile.png`),fullPage:false});
  }
 });
});
