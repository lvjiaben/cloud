import {test,expect} from '../support/source-site';
import {SharedFooterPage} from '../pages/SharedFooterPage';
test.use({baseURL:process.env.TELECOM_LIVE?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:!!process.env.TELECOM_LIVE,headless:true});
test.describe('TC-150 电信许可证专属页脚',()=>{
 for(const mobile of [false,true])test(mobile?'TC-150b: 手机许可证与清空':'TC-150a: 桌面多页许可证、图标及安全链接',async({page},info)=>{
  test.setTimeout(120000);if(mobile)await page.setViewportSize({width:390,height:844});
  const brand={siteName:{zhCN:'许可证隔离验收'},footer:{zhCN:'本方公司'},telecomLicenseNumber:'电信增值业务许可证：测试编号',telecomLicenseUrl:'https://example.test/license',publicSecurityNumber:'公网安备：本方测试编号',publicSecurityUrl:'https://example.test/police?recordcode=own-test',icpNumber:'网站备案号：本方测试编号',icpUrl:'https://example.test/icp',navigation:[]};
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand}}}));
  const site=new SharedFooterPage(page);
  for(const path of ['/','/plan/safety.html','/about.html']){
   expect((await site.open(path))?.status()).toBe(200);await site.footer();
   const row=page.locator('[data-filing-kind=telecom-license]');await expect(row).toBeVisible();await expect(row.locator('.filing-title')).toHaveText(brand.telecomLicenseNumber);
   await expect(row).toHaveAttribute('href',brand.telecomLicenseUrl);await expect(row).toHaveAttribute('target','_blank');await expect(row).toHaveAttribute('rel','noopener noreferrer');
   await expect(row.locator('img')).toHaveAttribute('src','/static/filing/gongan.png');expect(await row.locator('img').evaluate(async(e:HTMLImageElement)=>{await e.decode();return e.naturalWidth>0;})).toBe(true);
   expect(await row.evaluate(e=>{const r=e.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth+1;})).toBe(true);await site.screenshot(info.outputPath(path.replaceAll('/','-')+(mobile?'mobile':'desktop')+'.png'));
  }
  const row=page.locator('[data-filing-kind=telecom-license]');
  await page.context().route('https://example.test/**',r=>r.fulfill({contentType:'text/html',body:'<title>Own lookup</title>Configured lookup destination'}));
  for(const [kind,icon,number,url]of [['telecom-license','gongan.png',brand.telecomLicenseNumber,brand.telecomLicenseUrl],['public-security','gongan.png',brand.publicSecurityNumber,brand.publicSecurityUrl],['icp','local657930f11d1cd.png',brand.icpNumber,brand.icpUrl]]){
   const item=page.locator('[data-filing-kind="'+kind+'"]');await expect(item.locator('.filing-title')).toHaveText(number);await expect(item.locator('img')).toHaveAttribute('src','/static/filing/'+icon);
   expect(await item.locator('img').evaluate(async(e:HTMLImageElement)=>{await e.decode();return e.naturalWidth>0;})).toBe(true);
   const popupPromise=page.waitForEvent('popup');await item.click();const popup=await popupPromise;await expect(popup).toHaveURL(url);await expect(popup.locator('body')).toContainText('Configured lookup destination');await popup.close();
  }
  for(const link of ['', 'javascript:alert(1)','//evil.test/','https://user:pass@example.test/']){brand.telecomLicenseUrl=link;await site.open('/about.html');await site.footer();await expect(row).toBeVisible();await expect(row).not.toHaveAttribute('href',/.+/);}
  brand.telecomLicenseNumber='';brand.telecomLicenseUrl='https://example.test/license';await site.open('/about.html');await expect(row).toBeHidden();await expect(row.locator('img')).toHaveCount(0);
  brand.publicSecurityNumber='';brand.icpNumber='';await site.open('/about.html');for(const kind of ['telecom-license','public-security','icp']){const item=page.locator('[data-filing-kind="'+kind+'"]');await expect(item).toBeHidden();await expect(item.locator('.filing-title')).toHaveText('');await expect(item).not.toHaveAttribute('href',/.+/);}
 });
});
