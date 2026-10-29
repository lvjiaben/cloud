import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {ProblemChecksPage} from '../pages/ProblemChecksPage';

test.use({baseURL:process.env.PROBLEM_CHECKS_LIVE?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:!!process.env.PROBLEM_CHECKS_LIVE,headless:true});
const paths=['safety','web','game','electricity','financial','government','Goingtosea'];
test.describe('TC-147 面临的问题共用原版对号',()=>{
 for(const mobile of [false,true])test(mobile?'TC-147b: 手机全部方案原版对号':'TC-147a: 桌面全部方案原版对号',async({page},info)=>{
  test.setTimeout(120000);
  if(mobile)await page.setViewportSize({width:390,height:844});
  const foreign:string[]=[];page.on('request',r=>{if(new URL(r.url()).hostname==='www.xiaobaiyun.cn')foreign.push(r.url());});
  const site=new ProblemChecksPage(page);
  for(const path of paths){
   expect((await site.open('/plan/'+path+'.html'))?.status()).toBe(200);
   const checks=await site.checks();expect(checks.length).toBeGreaterThan(0);
   for(const check of checks){
    expect(check.text).toBeTruthy();expect(check.content).toBe('""');
    const imageURL=new URL(check.image.slice(5,-2));
    expect(imageURL.origin).toBe(new URL(page.url()).origin);expect(imageURL.pathname).toMatch(/\/a_select\.png$/);
    expect(check).toMatchObject({width:mobile?'16px':'22px',height:mobile?'16px':'22px',left:'0px',position:'absolute',size:'100% 100%'});
   }
   const iconURL=checks[0].image.slice(5,-2);
   expect(await page.evaluate(async src=>{const image=new Image();image.src=src;await image.decode();return image.naturalWidth>0&&image.naturalHeight>0;},iconURL)).toBe(true);
   const icon=await page.request.get(iconURL);expect(icon.status()).toBe(200);
   expect(await icon.body()).toEqual(await readFile(new URL('../../../frontend/public/static/picture/a_select.png',import.meta.url)));
   await site.screenshot(info.outputPath(path+'-'+(mobile?'mobile':'desktop')+'.png'));
  }
  const image=await page.request.get('/static/picture/a_select.png');expect(image.status()).toBe(200);
  expect(await image.body()).toEqual(await readFile(new URL('../../../frontend/public/static/picture/a_select.png',import.meta.url)));
  expect(foreign).toEqual([]);
 });
});
