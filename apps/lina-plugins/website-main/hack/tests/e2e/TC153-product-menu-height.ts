import {test,expect} from '../support/source-site';
import {ProductMenuLayoutPage} from '../pages/ProductMenuLayoutPage';

test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});

test.describe('TC-153 产品菜单按内容高度展开',()=>{
 for(const published of [false,true])test(`TC-153${published?'b':'a'}: ${published?'插件入口':'产品页'}少量商品收起留白，多排和长名称自然撑高，超屏可滚动`,async({page},info)=>{
  test.setTimeout(90000);
  let count=1,long=false;
  await page.route('**/x/idc-core/api/v1/public/catalog/products?*',async route=>{
   const response=await route.fetch();const body=await response.json();
   const data=body.data??body;
   const category=new URL(route.request().url()).searchParams.get('categoryNo');
   if(category&&data.products?.length){
    const sample=data.products[0];
    data.products=Array.from({length:count},(_,i)=>({...sample,categoryNo:category,
     publicSlug:`menu-height-${i}`,nameZhCN:long?'弹性云服务器商品完整名称用于验证自动换行'.repeat(6):`弹性云${i+1}`,
     summaryZhCN:'适用于企业应用',showNewBadge:false}));
   }
   await route.fulfill({response,json:body});
  });
  const site=new ProductMenuLayoutPage(page);
  const brandResponse=await page.request.get('/x/idc-portal/api/v1/public/brand');
  expect(brandResponse.ok()).toBe(true);
  const brand=(await brandResponse.json()).data.brand;
  const heights:number[]=[];
  for(const state of [{count:1,long:false},{count:12,long:false},{count:12,long:true}]){
   count=state.count;long=state.long;
   await site.openProducts(published?brand.templateEntry.url:'/products.html',1460);
   await expect(site.products()).toHaveCount(count);
   const geometry=await site.panelGeometry();heights.push(geometry.height);
   expect(geometry.bottom).toBeLessThanOrEqual(1000);
   if(!long)expect(geometry.bottom-geometry.contentBottom).toBeLessThan(100);
   else expect(geometry.scrollHeight).toBeGreaterThan(geometry.clientHeight);
   await site.capture(info.outputPath(`${Date.now()}-${count}-${long}.png`));
  }
  expect(heights[0]).toBeLessThan(500);
  expect(heights[1]).toBeGreaterThan(heights[0]);
  expect(heights[2]).toBeGreaterThan(heights[1]);
 });
});
