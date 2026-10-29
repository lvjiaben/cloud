import {test,expect} from '../support/source-site';
import {ContactPage} from '../pages/ContactPage';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
test.use({baseURL:'http://localhost:6680'});
test.describe('TC-111 联系页视觉与本方接口',()=>{
 test('TC-111c: 实际联系页使用已视审的无水印横幅',async({page},info)=>{
  await page.unrouteAll({behavior:'wait'});
  await page.goto('http://127.0.0.1:6680/contact.html');
  const banner=page.locator('.problem-row-item-title');
  await expect(banner).toBeVisible();
  await expect(banner).toHaveCSS('background-image',/remoteL17075769161931[.]png/);
  const approved=[
   ['remoteL17075769161931.png','58a90be6a4f5011e9f72a583f36dd6b65936746dc596294ee33e83b30ef3881d'],
   ['remoteL17075769177597.png','8e276c3b37c2f62a6d735c909197b39144307f3408f00bdd42d542b9e565e6df'],
  ];
  // Digest locks the full-resolution assets visually reviewed during the brand audit.
  for(const [name,digest] of approved){
   const response=await page.request.get('http://127.0.0.1:6680/static/image/'+name);
   expect(response.status()).toBe(200);
   expect(createHash('sha256').update(await response.body()).digest('hex')).toBe(digest);
   const source=await readFile(new URL('../../../frontend/public/static/image/'+name,import.meta.url));
   expect(createHash('sha256').update(source).digest('hex')).toBe(digest);
  }
  await banner.scrollIntoViewIfNeeded();
  await banner.screenshot({path:info.outputPath('contact-faq-clean.png')});
 });
 test('TC-111a: 真实公开资料、分区、悬停、404及语言',async({page},info)=>{
  const errors:string[]=[],foreign:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(!r.url().startsWith('http://localhost:6680'))foreign.push(r.url());});
  test.setTimeout(60000);const site=new ContactPage(page);await site.open();
  await expect(page.locator('.globalBanner')).toBeVisible();await expect(page.locator('#presales [data-contact-card]')).toHaveCount(5);await expect(page.locator('#aftersales [data-contact-card]')).toHaveCount(7);
  const brand=(await (await page.request.get('http://127.0.0.1:6030/x/idc-portal/api/v1/public/brand')).json()).data.brand;
  await expect(site.card('phone').locator('.label')).toHaveText(brand.contactPhone,{timeout:20000});await expect(site.card('phone').locator('a')).toHaveAttribute('href','tel:'+encodeURIComponent(brand.contactPhone));
  await expect(site.card('qq').locator('a')).toHaveAttribute('href',new RegExp('uin='+brand.businessQq));await expect(site.card('online').locator('a')).toHaveAttribute('href','/404.html?feature=contact-online');
  await expect(page.locator('[data-contact-faq] a').first()).toHaveAttribute('href',/content[.]html[?]id=cnt_/);
  await page.evaluate(()=>scrollTo(0,0));await site.reveal('phone');await expect.poll(()=>site.card('phone').locator('.consultationButton').evaluate(e=>e.getBoundingClientRect().height)).toBeGreaterThan(100);await site.screenshot(info.outputPath('contact-hover.png'));
  expect(await page.locator('main img').evaluateAll(imgs=>imgs.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.address();await expect(page.locator('[data-contact-map]')).toBeVisible();await expect(page.locator('[data-contact-map] *')).toHaveCount(0);await expect(page.locator('[data-contact-address]')).toContainText(brand.companyAddress);await site.screenshot(info.outputPath('contact-map-empty.png'));
  await site.language();await expect(page.locator('#presales h2')).toHaveText('Pre-sales consultation');await expect(page.locator('#aftersales h2')).toHaveText('After-sales support');await expect(site.card('phone').locator('.label')).toHaveText(brand.contactPhone,{timeout:20000});
  expect(foreign).toEqual([]);expect(errors).toEqual([]);
 });
 test('TC-111b: 手机、空资料与读取失败恢复',async({page},info)=>{
  await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});
  await page.route('**/public/brand',route=>route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'测试网站'},enterpriseWechatQrCodeUrl:'/static/picture/telephone.svg',navigation:[]}}}}));
  await page.route('**/content/items?type=knowledge&limit=5',route=>route.fulfill({status:503,json:{code:1}}));
  test.setTimeout(60000);const site=new ContactPage(page);await site.open();await expect(site.card('phone').locator('a')).toHaveAttribute('href','/404.html?feature=contact-phone');await expect(page.locator('[data-contact-faq]')).toContainText('帮助文档暂时不可用。');
  await expect(site.card('wechat').locator('.consultation-ewm img')).toHaveAttribute('src',/static[/]picture[/]telephone[.]svg/);expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)).toBe(false);await site.screenshot(info.outputPath('contact-mobile.png'));
  await page.route('**/content/items?type=knowledge&limit=5',route=>route.fulfill({json:{code:0,data:{items:[]}}}));await page.locator('[data-contact-faq] button').click();await expect(page.locator('[data-contact-faq] a')).toHaveAttribute('href','/404.html?feature=contact-help');
  await site.card('online').focus();await expect(site.card('online').locator('.consultationButton')).toHaveCSS('height',await site.card('online').evaluate(e=>getComputedStyle(e).height));
 });
});
