import {test,expect} from '../support/source-site';
import {ProductMenuLayoutPage} from '../pages/ProductMenuLayoutPage';

test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});

test.describe('TC-151 商品下拉列表利用完整宽度',()=>{
 test.setTimeout(90000);
 for(const published of [false,true])test(`TC-151${published?'b':'a'}: ${published?'正式插件':'首页'}宽屏四列及窄屏布局`,async({page},info)=>{
  const response=await page.request.get('/x/idc-portal/api/v1/public/brand');
  expect(response.ok()).toBe(true);
  const brand=(await response.json()).data.brand;
  const site=new ProductMenuLayoutPage(page);
  // Preserve the existing two-column layout at and below 1100px.
  for(const [width,columns] of [[1460,4],[1100,2],[390,2]]){
   await site.openProducts(published?brand.templateEntry.url:'/',width);
   await expect(site.products().first()).toBeVisible();
   await expect.poll(async()=> (await site.products().count())).toBeGreaterThanOrEqual(4);
   const layout=await site.geometry();
   const row=layout.items.filter(item=>Math.abs(item.y-layout.items[0].y)<2);
   expect(row.length,`${width}px每排商品数`).toBe(columns);
   if(width>1100)expect(layout.available-layout.width,'取消右侧300px预留').toBeLessThan(60);
   expect(Math.max(...row.map(item=>item.width))-Math.min(...row.map(item=>item.width))).toBeLessThan(2);
   if(width>=768){
    expect(Math.max(...layout.items.map(item=>item.right))).toBeLessThanOrEqual(width);
    expect(layout.scroll).toBeLessThanOrEqual(layout.viewport);
   }
   await site.capture(info.outputPath(`${Date.now()}-${published?'published':'root'}-${width}.png`));
  }
 });
});
