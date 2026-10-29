import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {mkdir} from 'node:fs/promises';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-43 栏目中性图标整页集成',()=>{
 for(const [file,count]of [['news.html',3],['announcements.html',3],['help.html',3],['content.html?id=controlled-news-1',4]] as const)test('TC-43 '+file,async({page},info)=>{
  await controlledBusiness(page);await page.goto('/'+file);await expect(page.locator('[data-brand-name]').first()).toHaveText('受控测试专用网站');
  const links=page.locator('.create-article a');await expect(links).toHaveCount(count);
  const labels=count===4?['新闻动态','公告','帮助文档','资源下载']:['新闻动态','帮助文档','资源下载'];
  const hrefs=count===4?['/news.html','/announcements.html','/help.html','/downloads.html']:['/news.html','/help.html','/downloads.html'];
  await expect(links).toHaveText(labels);for(let i=0;i<count;i++)await expect(links.nth(i)).toHaveAttribute('href',hrefs[i]);
  await expect.poll(()=>links.locator('img').evaluateAll(ns=>ns.every(n=>(n as HTMLImageElement).complete&&(n as HTMLImageElement).naturalWidth>0))).toBe(true);
  const metrics=await links.evaluateAll(ns=>ns.map(n=>{const r=n.getBoundingClientRect(),img=n.querySelector('img')!,ir=img.getBoundingClientRect(),s=n.querySelector('span')!.getBoundingClientRect();return {label:n.textContent,rect:r.toJSON(),icon:ir.toJSON(),text:s.toJSON(),alt:img.getAttribute('alt')};}));
  for(const m of metrics){expect(m.icon.width).toBe(38);expect(m.icon.height).toBe(38);expect(m.rect.height).toBeGreaterThanOrEqual(57);expect(m.icon.bottom).toBeLessThanOrEqual(m.text.top+1);expect(m.alt).toBe('');}
  for(let i=0;i<metrics.length;i++)for(let j=i+1;j<metrics.length;j++){const a=metrics[i].rect,b=metrics[j].rect;expect(a.right<=b.left+1||b.right<=a.left+1||a.bottom<=b.top+1||b.bottom<=a.top+1).toBe(true);}
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await mkdir('D:/project1/temp/20261019/website-sidebar-icons-release-v022/screenshots',{recursive:true});
  await page.screenshot({path:'D:/project1/temp/20261019/website-sidebar-icons-release-v022/screenshots/'+Date.now()+'-'+file.split('?')[0]+'.png',fullPage:true,animations:'disabled'});
  await info.attach('sidebar-metrics',{body:JSON.stringify(metrics),contentType:'application/json'});
 });
});
