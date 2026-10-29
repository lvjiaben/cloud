import {test,expect} from '../support/source-site';
import {BrandSyncPage} from '../pages/BrandSyncPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',reducedMotion:'reduce'});

test.describe('TC-124 品牌资料快速展示',()=>{
 test('TC-124a: 再次进入无需等待品牌接口，核对后替换和清空旧值',async({page},info)=>{
  let calls=0,hold=false,release!:()=>void;
  let brand={...previewBrand,siteName:{zhCN:'快速网站'},footer:{zhCN:'快速公司'},contactPhone:'01012345678',contactEmail:'fast@example.invalid',copyright:'测试版权',headerLogoUrl:'/brand-fast.svg',footerLogoUrl:'/brand-fast.svg'};
  const gate=new Promise<void>(resolve=>release=resolve);
  await page.route('**/brand-fast.svg',route=>route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="134" height="43"><rect width="134" height="43" fill="blue"/></svg>'}));
  await page.route('**/x/**',async route=>{
   const request=route.request(),url=new URL(request.url()),response=previewResponse(url,request.method());
   if(url.pathname.endsWith('/brand')){calls++;if(hold)await gate;response.data={brand};}
   await route.fulfill({status:response.status,json:{code:0,data:response.data}});
  });
  const site=new BrandSyncPage(page);
  try{
   await site.open('/');await expect(page.locator('[data-brand-footer]')).toHaveText('快速公司');
   // Initial pageshow must not schedule a second copy of the initial request.
   await page.waitForLoadState('networkidle');expect(calls).toBe(1);
   hold=true;
   await site.open('/index.html');
   await expect(page.locator('[data-brand-footer]')).toHaveText('快速公司',{timeout:1000});
   await expect(page.locator('.horizontal-menu-navbar-logo img')).toHaveAttribute('src','http://localhost:6680/brand-fast.svg');
   await expect(page.locator('[data-footer-contact=phone] a')).toHaveText('01012345678');
   await site.captureSection('.sky-footer-left',info.outputPath('cached-footer-before-response.png'));
   brand={...brand,siteName:{zhCN:'更新网站'},footer:{zhCN:'更新公司'},contactPhone:'',contactEmail:'',copyright:'',headerLogoUrl:'',footerLogoUrl:''};
   release();
   await expect(page.locator('[data-brand-footer]')).toHaveText('更新公司');
   await expect(page.locator('.horizontal-menu-navbar-logo [data-brand-name]')).toHaveText('更新网站');
   await expect(page.locator('[data-footer-contact=phone]')).toBeHidden();
   await expect(page.locator('[data-footer-contact=email]')).toBeHidden();
   await expect(page.locator('[data-footer-identity-logo]')).toHaveCount(0);
   await site.captureSection('.sky-footer-left',info.outputPath('authoritative-cleared-footer.png'));
  }finally{release();}
 });
 test('TC-124b: 短期资料在接口失败时保留，手动重试恢复且停用响应清除旧资料',async({page},info)=>{
  let failure=false,inactive=false;
  const brand={...previewBrand,siteName:{zhCN:'恢复网站'},footer:{zhCN:'保留公司'},contactPhone:'01012345678'};
  await page.route('**/x/**',async route=>{
   const request=route.request(),url=new URL(request.url()),response=previewResponse(url,request.method());
   if(url.pathname.endsWith('/brand')){
    if(failure)return route.fulfill({status:503,json:{code:503}});
    response.data=inactive?{}:{brand};
   }
   await route.fulfill({status:response.status,json:{code:0,data:response.data}});
  });
  const site=new BrandSyncPage(page);
  await site.open('/');await expect(page.locator('[data-brand-footer]')).toHaveText('保留公司');
  failure=true;await site.open('/index.html');
  await expect(page.locator('.lina-brand-error')).toContainText('网站资料暂时无法加载，请重试。');
  await expect(page.locator('[data-brand-footer]')).toHaveText('保留公司');
  await site.captureSection('.sky-footer-left',info.outputPath('cached-footer-during-failure.png'));
  failure=false;await site.retryBrand();await expect(page.locator('.lina-brand-error')).toHaveCount(0);
  inactive=true;await site.refreshBrand();
  await expect(page.locator('[data-brand-footer]')).toBeHidden();
  await expect.poll(()=>page.evaluate(()=>localStorage.getItem('lina.public-brand-identity.v1'))).toBeNull();
  await site.captureSection('.nav-layoutcontainer',info.outputPath('inactive-brand-cleared.png'));
 });
});
