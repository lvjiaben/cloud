import {test,expect} from '../support/source-site';
import {writeFile} from 'node:fs/promises';

test.use({baseURL:process.env.WEBSITE_TEST_URL||'http://127.0.0.1:6680',websiteGateway:true,reducedMotion:'reduce'});

test.describe('TC-126 首次访问品牌首屏',()=>{
 test('TC-126a: 品牌接口慢时先显示顶部Logo及整块页脚资料',async({page},info)=>{
  let release!:()=>void,brandPending=false,brandFinishedAt=0,logoRequestedAt=0,logoFailure='';
  const gate=new Promise<void>(resolve=>release=resolve),startedAt=Date.now();
  page.on('request',request=>{if(request.url().includes('/v3/catalog/album/'))logoRequestedAt=Date.now();});
  page.on('requestfailed',request=>{if(request.url().includes('/v3/catalog/album/'))logoFailure=request.failure()?.errorText||'unknown';});
  page.on('requestfinished',request=>{if(request.url().endsWith('/x/idc-portal/api/v1/public/brand'))brandFinishedAt=Date.now();});
  await page.addInitScript(()=>localStorage.clear());
  await page.route('https://i.iheart.com/**',route=>route.fulfill({status:200,contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="134" height="43"><rect width="134" height="43" fill="#1677ff"/></svg>'}));
  await page.route('**/x/idc-portal/api/v1/public/brand',async route=>{
   brandPending=true;await gate;await route.continue();
  });
  try{
   await page.goto('/',{waitUntil:'domcontentloaded'});
   const hint=await page.evaluate(()=>window.__LINA_PUBLIC_IDENTITY__);
   expect(hint?.brand?.footer?.zhCN).toBeTruthy();expect(Date.now()-hint.savedAt).toBeLessThanOrEqual(30000);
   await expect(page.locator('[data-brand-footer]')).toHaveText(hint.brand.footer.zhCN,{timeout:1500});
   const header=page.locator('.horizontal-menu-navbar-logo [data-brand-name]');
   await expect(header).toHaveAttribute('aria-label',hint.brand.siteName.zhCN);
   await expect(header).toHaveAttribute('data-header-logo-source',hint.brand.headerLogoUrl);
   const headerImage=header.locator('img');
   await expect(headerImage).toBeVisible();await expect.poll(()=>headerImage.evaluate(image=>image.naturalWidth)).toBe(134);
   await expect(page.locator('.sky-footer-copy')).toHaveText(hint.brand.copyright);
   await expect(page.locator('.sky-footer-left [data-footer-contact="phone"] a')).toHaveText(hint.brand.contactPhone);
   await expect(page.locator('.sky-footer-left [data-footer-contact="email"] a')).toHaveText(hint.brand.contactEmail);
   await expect(page.locator('.sky-footer-left [data-footer-detail="qq"] a')).toHaveText(hint.brand.businessQq);
   await expect(page.locator('.sky-footer-left [data-footer-detail="address"] a')).toHaveText(hint.brand.companyAddress);
   const identityVisibleMs=Date.now()-startedAt;
   expect(brandPending).toBe(true);expect(brandFinishedAt).toBe(0);
   expect(logoRequestedAt).toBeGreaterThan(0);expect(logoRequestedAt-startedAt).toBeLessThan(brandFinishedAt||3000);
   await writeFile(info.outputPath('brand-first-paint-timings.json'),JSON.stringify({identityVisibleMs,logoRequestStartedMs:logoRequestedAt-startedAt,brandResponseHeld:brandFinishedAt===0,logoRequestFailure:logoFailure||null},null,2));
   await page.screenshot({path:info.outputPath('brand-before-authority.png'),fullPage:false});
   await page.locator('.sky-footer-left').scrollIntoViewIfNeeded();
   const footerImage=page.locator('[data-footer-identity-logo] img');
   await expect(footerImage).toBeVisible();await expect.poll(()=>footerImage.evaluate(image=>image.naturalWidth)).toBe(134);
   await page.screenshot({path:info.outputPath('footer-before-authority.png'),fullPage:false});
   const brandResponse=page.waitForResponse(response=>response.url().endsWith('/x/idc-portal/api/v1/public/brand'));
   release();
   expect((await brandResponse).ok()).toBe(true);
   await expect(page.locator('[data-brand-footer]')).toHaveText(hint.brand.footer.zhCN);
  }finally{release();}
 });
});
