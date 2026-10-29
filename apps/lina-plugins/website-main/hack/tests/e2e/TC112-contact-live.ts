import {test,expect} from '@playwright/test';
import {ContactPage} from '../pages/ContactPage';
test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-112 联系页实际部署',()=>{
 test('TC-112a: 实际资料、图片、缺失接口404与空白地图',async({page},info)=>{
  const errors:string[]=[],foreign:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(!r.url().startsWith('http://127.0.0.1:6680'))foreign.push(r.url());});
  await page.route('**/*',route=>['GET','HEAD'].includes(route.request().method())?route.continue():route.abort());
  const site=new ContactPage(page);await site.open();await expect(site.card('phone').locator('a')).toHaveAttribute('href',/^tel:/,{timeout:20000});
  await expect(page.locator('[data-contact-card="ticket"] a')).toHaveCount(2);for(const a of await page.locator('[data-contact-card="ticket"] a').all())await expect(a).toHaveAttribute('href','http://127.0.0.1:6032/account/support');
  await expect(page.locator('[data-contact-faq] a').first()).toHaveAttribute('href',/content[.]html/);
  await page.evaluate(()=>scrollTo({top:0,behavior:'instant'}));await site.screenshot(info.outputPath('contact-live-desktop.png'));
  expect((await page.request.get('/404.html?feature=contact-online')).status()).toBe(404);await expect(page.locator('[data-contact-map] *')).toHaveCount(0);
  await site.language();await expect(page.locator('#presales h2')).toHaveText('Pre-sales consultation');await page.reload();await expect(page.locator('#presales h2')).toHaveText('Pre-sales consultation');
  await page.setViewportSize({width:390,height:844});await site.screenshot(info.outputPath('contact-live-mobile.png'));expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)).toBe(false);expect(foreign).toEqual([]);expect(errors).toEqual([]);
 });
});
