import {test,expect} from '@playwright/test';
import {BrandSyncPage} from '../pages/BrandSyncPage';

test.use({baseURL:'http://127.0.0.1:6680',headless:true});

test.describe('TC-122 顶部Logo实际部署',()=>{
 test('TC-122a: 实际配置在首页、商品页和联系页显示并保持图片比例',async({page},info)=>{
  test.setTimeout(120000);
  const response=await page.request.get('/x/idc-portal/api/v1/public/brand');
  expect(response.ok()).toBe(true);
  const {data:{brand}}=await response.json();
  expect(brand.headerLogoUrl).toBeTruthy();
  const site=new BrandSyncPage(page);
  for(const path of ['/','/products.html','/contact.html']){
   await site.open(path);
   const logo=page.locator('.horizontal-menu-navbar-logo [data-brand-name] img');
   await expect(logo).toHaveAttribute('src',new URL(brand.headerLogoUrl,page.url()).href,{timeout:20000});
   await expect.poll(()=>logo.evaluate((image:HTMLImageElement)=>image.naturalWidth),{timeout:15000}).toBeGreaterThan(0);
   const dimensions=await logo.evaluate((image:HTMLImageElement)=>({width:image.width,height:image.height,ratio:image.naturalWidth/image.naturalHeight}));
   expect(dimensions.height).toBe(43);
   expect(dimensions.width/dimensions.height).toBeCloseTo(dimensions.ratio,1);
   await site.captureSection('.nav-layoutcontainer',info.outputPath(path==='/'?'home-logo.png':path.slice(1)+'.png'));
  }
  await site.setViewport(390,844);
  const logo=page.locator('.horizontal-menu-navbar-logo img');
  await expect(logo).toBeVisible();
  expect((await logo.boundingBox())!.height).toBe(34);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await site.captureSection('.nav-layoutcontainer',info.outputPath('mobile-logo.png'));
 });
});
