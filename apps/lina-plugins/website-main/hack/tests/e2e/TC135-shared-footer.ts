import {test,expect} from '../support/source-site';
import {SharedFooterPage} from '../pages/SharedFooterPage';
const live=process.env.BT_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live,headless:true});
test.describe('TC-135 统一页脚与清晰宝塔导航',()=>{
 test('TC-135a: 多页权威Logo与本地备案图标',async({page},info)=>{
  test.setTimeout(120000);
  const brand={siteName:{zhCN:'本方品牌'},footer:{zhCN:'本方公司'},headerLogoUrl:'https://brand.example/logo.svg',footerLogoUrl:'https://brand.example/logo.svg',icpNumber:'本方ICP备案号',publicSecurityNumber:'本方公安备案号',navigation:[]};
  await page.route('https://brand.example/logo.svg',r=>r.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect width="120" height="40" fill="#1677ff"/></svg>'}));
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand}}}));
  const site=new SharedFooterPage(page);
  for(const path of ['/','/bt.html','/plan/game.html','/plan/safety.html','/plan/government.html','/plan/financial.html','/about.html']){
   expect((await site.open(path))?.status()).toBe(200);
   await expect(page.locator('[data-footer-identity-logo] img')).toHaveAttribute('src',brand.footerLogoUrl);
   await expect(page.locator('[data-brand-footer]')).toHaveText('本方公司');
   await site.footer();await expect(page.locator('.sky-footer-container')).toBeVisible();
   for(const kind of ['icp','public-security']){const row=page.locator('[data-filing-kind="'+kind+'"]');await expect(row).toBeVisible();await expect(row.locator('img')).toHaveAttribute('src',/^\/(lina|static)\//);}
   expect(await page.locator('[data-footer-identity-logo] img,[data-filing-kind] img').evaluateAll(async nodes=>{try{await Promise.all(nodes.map(n=>(n as HTMLImageElement).decode()));return true;}catch{return false;}})).toBe(true);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   if(path==='/bt.html')await site.screenshot(info.outputPath('shared-footer.png'));
  }
 });
 test('TC-135b: 桌面手机字号、居中与按钮无缩放',async({page},info)=>{
  const site=new SharedFooterPage(page);
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:1000});await site.open('/bt.html');
   const nav=page.locator('.et-hero-tabs-container');
   await expect(nav).toHaveCSS('justify-content','center');await expect(page.locator('.et-hero-tab a').first()).toHaveCSS('font-size',width===1440?'16px':'14px');
   if(width===1440)expect(await nav.evaluate(e=>{const items=[...e.querySelectorAll('.et-hero-tab')],a=items[0].getBoundingClientRect(),b=items.at(-1)!.getBoundingClientRect(),r=e.getBoundingClientRect();return Math.abs((a.left+b.right)/2-(r.left+r.right)/2)<2;})).toBe(true);
   await site.hoverHero();await expect(page.locator('.pagebanner-button a').first()).toHaveCSS('font-size','14px');await expect(page.locator('.pagebanner-button a').first()).toHaveCSS('transform','none');await expect(page.locator('.pagebanner-button a span').first()).toHaveCSS('will-change','auto');
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   await site.screenshot(info.outputPath('bt-clear-'+width+'.png'));
  }
 });
});
