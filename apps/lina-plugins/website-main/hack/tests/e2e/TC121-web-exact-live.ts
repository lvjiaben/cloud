import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {WebSolutionPage} from '../pages/WebSolutionPage';
test.use({baseURL:'http://127.0.0.1:6680',headless:true});
test.describe('TC-121 网站解决方案实际原文一致性',()=>{
 test('TC-121a: 实际6680原文逐字段、无附加文案、锚点及手机布局',async({page},info)=>{
  const site=new WebSolutionPage(page),errors:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  await site.open('#web-advantages');
  const expected=JSON.parse(await readFile(new URL('../support/web-exact-reference.json',import.meta.url),'utf8'));
  const actual=await page.locator('[data-web-solution]').evaluate(root=>{
   const fields:string[]=[];
   function collect(node:Node){
    if(node.nodeType===3){const s=node.nodeValue!.trim();if(s)fields.push(s);}
    else if(node instanceof Element&&node.matches('[data-web-products],script,style'))return;
    else for(const child of node.childNodes)collect(child);
   }
   collect(root);return fields;
  });
  expect(actual).toEqual(expected);
  await site.screenshot(info.outputPath('web-exact-desktop.png'));
  await site.section('architecture');await expect(site.nav('architecture')).toHaveAttribute('aria-current','location');
  await site.screenshot(info.outputPath('web-exact-architecture.png'));
  await page.setViewportSize({width:390,height:844});await site.open('#web-advantages');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await site.screenshot(info.outputPath('web-exact-mobile.png'));
  expect(errors).toEqual([]);
 });
});
