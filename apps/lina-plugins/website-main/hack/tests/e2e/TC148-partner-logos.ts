import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {PartnerLogosPage} from '../pages/PartnerLogosPage';

const names=['remoteL17063681798362.png','local65b51d7304309.jpg','local65b51d681ab80.jpeg','remoteL17063681802376.png','local65b51d6d7b96f.jpeg','remoteL17063681807597.png','remoteL17063681819770.png','remoteL17063681816166.png','remoteL17063681811685.png','remoteL17063681815858.png','local65b51d70398c6.jpg','remoteL17063681823466.png'];
test.use({baseURL:process.env.PARTNER_LOGOS_LIVE?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:!!process.env.PARTNER_LOGOS_LIVE,headless:true});
test.describe('TC-148 原版合作伙伴十二图',()=>{
 for(const mobile of [false,true])test(mobile?'TC-148b: 手机七方案完整显示十二图':'TC-148a: 桌面七方案原图顺序及本地加载',async({page},info)=>{
  test.setTimeout(180000);if(mobile)await page.setViewportSize({width:390,height:844});
  const foreign:string[]=[];page.on('request',r=>{if(/xiaobaiyun\.cn$/.test(new URL(r.url()).hostname))foreign.push(r.url());});
  const site=new PartnerLogosPage(page);
  for(const path of ['web','electricity','safety','game','financial','government','Goingtosea']){
   expect((await site.open('/plan/'+path+'.html'))?.status()).toBe(200);
   await expect(page.locator('.partner-Content .currency-title-a')).toHaveText('合作伙伴');
   await expect(page.locator('.partner-Content .currency-title-a-content')).toHaveText('他们正使用我们的产品和服务');
   const logos=await site.logos();expect(logos.map(x=>x.name)).toEqual(names);
   for(const logo of logos){
    expect(logo.loaded).toBe(true);expect(logo.visible).toBe(true);expect(new URL(logo.src).origin).toBe(new URL(page.url()).origin);
    const response=await page.request.get(logo.src);expect(response.status()).toBe(200);
    expect(await response.body()).toEqual(await readFile(new URL('../../../frontend/public/lina/safety-assets/'+logo.name,import.meta.url)));
   }
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   await site.screenshot(info.outputPath(path+'-'+(mobile?'mobile':'desktop')+'.png'));
  }
  expect(foreign).toEqual([]);
 });
});
