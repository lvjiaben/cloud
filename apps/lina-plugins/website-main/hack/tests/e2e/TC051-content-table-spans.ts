import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-51 正文合并单元格与安全清洗',()=>{
 test('TC-51a: 合并表格对齐、rowspan零语义及非法属性拒绝',async({page},info)=>{
  const invalid=['-1','1.5','100000','2x',' 2','01'];
  const body='<table><thead><tr><th colspan="2">标题</th></tr></thead><tbody><tr><td rowspan="2">合并行</td><td>A</td></tr><tr><td>B</td></tr></tbody></table>'+
   '<table><tbody><tr><td rowspan="0">跨到组尾</td><td>C</td></tr><tr><td>D</td></tr></tbody></table>'+
   '<table><tbody><tr>'+invalid.map(value=>`<td colspan="${value}" rowspan="${value}" onclick="window.bad=true" style="display:none">非法${value}</td>`).join('')+'<td colspan="0">零列</td></tr></tbody></table><script>window.bad=true</script>';
  await page.route('**/x/**',async route=>{const request=route.request(),url=new URL(request.url()),r=previewResponse(url,request.method());if(url.pathname.endsWith('/content/items/span-test')){r.status=200;r.data={item:{contentNo:'span-test',type:'news',title:'模拟合并表格验收',body}};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/content.html?id=span-test');
  const article=page.locator('[data-content-body]'),tables=article.locator('table');await expect(tables).toHaveCount(3);
  await expect(tables.first().locator('th')).toHaveAttribute('colspan','2');await expect(tables.first().locator('td').first()).toHaveAttribute('rowspan','2');
  const cells=await tables.first().locator('th,td').evaluateAll(ns=>ns.map(n=>{const r=n.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height};}));
  expect(cells[2].x).toBe(cells[3].x);expect(cells[2].x).toBeGreaterThan(cells[1].x);expect(cells[1].height).toBeGreaterThan(cells[2].height);expect(cells[0].width).toBeGreaterThan(cells[1].width);
  await expect(tables.nth(1).locator('td').first()).toHaveAttribute('rowspan','0');
  const zero=await tables.nth(1).locator('td').evaluateAll(ns=>ns.map(n=>n.getBoundingClientRect().height));expect(zero[0]).toBeGreaterThan(zero[1]);
  await expect(tables.nth(2).locator('[colspan],[rowspan],[onclick],[style]')).toHaveCount(0);expect(await page.evaluate(()=>Boolean((window as any).bad))).toBe(false);await expect(article.locator('script')).toHaveCount(0);
  await site.captureSection('[data-content-body]',info.outputPath('merged-table-safe.png'));
 });
});
