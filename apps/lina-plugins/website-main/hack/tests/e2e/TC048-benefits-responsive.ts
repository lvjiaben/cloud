import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',reducedMotion:'reduce'});
test.describe('TC-48 四卡图片与窄屏可读性',()=>{
 test('TC-48a: 桌面保持两列，手机单列，四幅图片加载且文字不逐字挤压',async({page},info)=>{
  await page.route('**/x/**',async r=>{const q=r.request(),v=previewResponse(new URL(q.url()),q.method());await r.fulfill({status:v.status,json:{code:0,data:v.data}});});
  const site=new HomeDetailPage(page);await site.open('/');
  for(const width of [1440,390,320]){await page.setViewportSize({width,height:900});await site.captureSection('.chooseUs-container',info.outputPath(width+'-benefits.png'));
   const rows=await page.locator('.chooseUs-itme').evaluateAll(ns=>ns.map(n=>({x:n.getBoundingClientRect().x,y:n.getBoundingClientRect().y,w:n.getBoundingClientRect().width,text:n.querySelector('.chooseUs-itme-main')!.clientWidth,loaded:(n.querySelector('img') as HTMLImageElement).naturalWidth===200})));
   expect(rows).toHaveLength(4);expect(rows.every(n=>n.loaded&&n.text>150&&n.x>=0&&n.x+n.w<=width)).toBe(true);
   if(width<768){expect(rows.every(n=>n.x===rows[0].x)).toBe(true);expect(rows[1].y).toBeGreaterThan(rows[0].y);}else expect(rows[0].y).toBe(rows[1].y);
   await expect(page.locator('.chooseUs-itme-tag')).toHaveCount(15);
  }
 });
});
