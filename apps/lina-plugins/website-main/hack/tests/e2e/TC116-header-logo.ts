// Public API fixtures exercise current production pages without changing real brand settings.
import {test,expect} from '../support/source-site';
import {BrandSyncPage} from '../pages/BrandSyncPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-116 独立顶部Logo',()=>{
 for(const path of ['/','/products.html','/contact.html']){
  test(`TC-116 ${path}: 图片、实时更新、独立页脚、清空与加载失败回退`,async({page},info)=>{
   let headerLogoUrl='/test-header.svg',siteName='隔离网站名称';
   await page.route('**/x/**',async route=>{
    const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());
    if(u.pathname.endsWith('/brand'))r.data={brand:{...previewBrand,siteName:{zhCN:siteName},headerLogoUrl,footerLogoUrl:'/test-footer.svg',logoUrl:'/retired.png'}};
    await route.fulfill({status:r.status,json:{code:0,data:r.data}});
   });
   await page.route('**/test-*.svg',route=>route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="134" height="43"><rect width="134" height="43" fill="#1677ff"/><text x="8" y="28" fill="white" font-size="20">TEST LOGO</text></svg>'}));
   const site=new BrandSyncPage(page);await site.open(path);
   const slot=page.locator('.horizontal-menu-navbar-logo [data-brand-name]'),image=slot.locator('img');
   const footerImage=page.locator('[data-footer-identity-logo] img');
   await expect(image).toHaveAttribute('src','http://localhost:6680/test-header.svg');
   await expect(image).toHaveAttribute('alt',siteName);
   await expect(footerImage).toHaveAttribute('src','http://localhost:6680/test-footer.svg');
   await expect(slot).toHaveAttribute('href','/index.html');
   await expect(image).toBeVisible();expect((await image.boundingBox())!.height).toBe(43);
   expect((await image.boundingBox())!.width).toBe(134);
   await site.captureSection('.nav-layoutcontainer',info.outputPath('header-logo.png'));
   await site.setViewport(390,844);
   await expect(image).toBeVisible();
   const mobile=await image.boundingBox();
   expect(mobile!.height).toBe(34);expect(mobile!.width/mobile!.height).toBeCloseTo(134/43,1);
   expect(mobile!.x).toBeGreaterThanOrEqual(0);expect(mobile!.x+mobile!.width).toBeLessThanOrEqual(390);
   await site.captureSection('.nav-layoutcontainer',info.outputPath('header-logo-mobile.png'));
   await site.setViewport(1440,900);
   await image.evaluate(n=>n.setAttribute('data-preserved','yes'));await site.refreshBrand();
   await expect(image).toHaveAttribute('data-preserved','yes');
   headerLogoUrl='/test-new.svg';siteName='更新网站名称';await site.refreshBrand();
   await expect(image).toHaveAttribute('src','http://localhost:6680/test-new.svg');await expect(image).toHaveAttribute('alt',siteName);
   await expect(footerImage).toHaveAttribute('src','http://localhost:6680/test-footer.svg');
   await site.captureSection('.nav-layoutcontainer',info.outputPath('header-logo-updated.png'));
   headerLogoUrl='';await site.refreshBrand();await expect(image).toHaveCount(0);await expect(slot).toHaveText(siteName);
   await expect(footerImage).toHaveAttribute('src','http://localhost:6680/test-footer.svg');
   await site.captureSection('.nav-layoutcontainer',info.outputPath('header-logo-empty-fallback.png'));
   headerLogoUrl='/missing-header.png';await site.refreshBrand();await expect(slot).toHaveText(siteName);await expect(image).toHaveCount(0);
   await expect(footerImage).toHaveAttribute('src','http://localhost:6680/test-footer.svg');
   await site.captureSection('.nav-layoutcontainer',info.outputPath('header-logo-load-failure.png'));
   headerLogoUrl='javascript:alert(1)';await site.refreshBrand();await expect(slot).toHaveText(siteName);await expect(image).toHaveCount(0);
   await expect(page.locator('img[src*="retired"]')).toHaveCount(0);
  });
 }
});
