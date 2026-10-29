import {test,expect} from '@playwright/test';
import {AffiliatePage} from '../pages/AffiliatePage';
test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-114 联盟推广页实际部署',()=>{
 test('TC-114a: 实际页面、资源、导航和客户中心登录衔接',async({page},info)=>{
  const errors:string[]=[],privateCalls:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{const u=new URL(r.url());if(u.pathname.includes('/account/sales-channel'))privateCalls.push(r.url());if(u.origin!=='http://127.0.0.1:6680')foreign.push(r.url());});
  await page.route('**/*',r=>['GET','HEAD'].includes(r.request().method())?r.continue():r.abort());
  const site=new AffiliatePage(page);await site.open();
  await expect(page.locator('[data-aff-customer]').first()).toHaveAttribute('href','http://127.0.0.1:6032/account/referrals');
  await expect.poll(()=>page.locator('main img').evaluateAll(images=>images.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.screenshot(info.outputPath('affiliate-live-desktop.png'));await site.section('process');await expect(site.nav('process')).toHaveAttribute('aria-current','location');await site.screenshot(info.outputPath('affiliate-live-process.png'));
  await page.reload();await page.waitForLoadState('networkidle');await expect(site.nav('process')).toHaveAttribute('aria-current','location');
  await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});await site.section('methods');await site.screenshot(info.outputPath('affiliate-live-mobile.png'));
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);expect(privateCalls).toEqual([]);expect(foreign).toEqual([]);expect(errors).toEqual([]);
  await page.locator('#methods [data-aff-customer]').click();await page.waitForURL(/127\.0\.0\.1:6032\/(auth\/login|account\/referrals)/);await page.waitForLoadState('networkidle');
  const target=new URL(page.url());if(target.pathname==='/auth/login'){expect(decodeURIComponent(target.searchParams.get('redirect')||'')).toBe('/account/referrals');await expect(page.locator('input[type=password]')).toBeVisible();}
  else expect(target.pathname).toBe('/account/referrals');
  await page.screenshot({path:info.outputPath('affiliate-customer-target.png'),fullPage:false});
 });
});
