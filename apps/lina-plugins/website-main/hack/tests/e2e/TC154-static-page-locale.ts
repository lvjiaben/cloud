import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {LocalePage} from '../pages/LocalePage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-154 后续官网页面完整中英文切换',()=>{
 const pages=[
  ['/plan/game.html','[data-game]','Gaming solutions'],
  ['/plan/financial.html','[data-financial]','Financial solutions'],
  ['/plan/safety.html','[data-safety]','Security solutions'],
  ['/plan/government.html','[data-government]','Public sector solutions'],
  ['/plan/Goingtosea.html','[data-goingsea]','Global expansion'],
  ['/cooperation/partner.html','[data-partner]','Partners'],
  ['/about.html','main.et-main',"Customers' first choice"],
  ['/Assure.html','[data-assure]','Service'],
  ['/bt.html','[data-bt]','Panel'],
  ['/plan/electricity.html','[data-electricity]','E-commerce'],
  ['/plan/web.html','[data-web-solution]','Website'],
  ['/certificate.html','[data-certificate]','Our qualifications'],
  ['/terms.html','.terms-page','Account and information security'],
 ];
 for(const [index,[path,selector,title]] of pages.entries())test(`TC-154${String.fromCharCode(97+index)}: ${path}`,async({page},info)=>{
  const {requests,state}=await controlledBusiness(page);const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  state.brand.siteName={zhCN:'双语测试网站',enUS:'Bilingual Test Website'};
  state.brand.contactPhone='400-000-1234';
  const site=new LocalePage(page);await site.open(path);
  await expect(page).toHaveTitle(/双语测试网站/);
  const chinese=await page.locator(selector).textContent();
  await site.toggle();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await expect(page.locator(selector)).toContainText(title);
  await expect.poll(()=>site.untranslated(selector)).toEqual([]);
  await expect(page.locator('.navbar-Login')).toHaveText('Sign up');
  await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('Products & Services');
  await expect(page).toHaveTitle(/Bilingual Test Website/);
  expect(await page.title()).not.toMatch(/[\u3400-\u9fff]/);
  await site.capture(info.outputPath('english-page.png'));
  await site.reload();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
  await expect.poll(()=>site.untranslated(selector)).toEqual([]);
  await site.toggle();await expect(page.locator('html')).toHaveAttribute('lang','zh-CN');
  await expect(page.locator(selector)).toHaveText(chinese||'');
  await expect(page).toHaveTitle(/双语测试网站/);
  expect(errors).toEqual([]);expect(requests.filter(r=>r.method==='POST')).toEqual([]);
 });
 test('TC-154n: 手机英文方案、资质与条款正文保持可读且切换可达',async({page},info)=>{
  await controlledBusiness(page);const site=new LocalePage(page);
  for(const [path,selector,title] of [pages[0],pages[11],pages[12]]){
   await site.open(path);
   if(path===pages[0][0])await site.toggle();
   await expect(page.locator(selector)).toContainText(title);
   await site.useMobileMenu();
   await expect(page.locator('[data-public-locale-switch]')).toBeVisible();
   await expect.poll(()=>site.overflow()).toBeLessThanOrEqual(390);
   await expect.poll(()=>site.untranslated(selector)).toEqual([]);
   if(path===pages[0][0])await site.capture(info.outputPath('english-mobile-navigation.png'));
   await site.closeMobileMenu();
   if(path==='/terms.html')expect(await site.termsLinksFit()).toBe(true);
   await site.capture(info.outputPath(path.replaceAll('/','_')+'-mobile.png'));
  }
 });
});
