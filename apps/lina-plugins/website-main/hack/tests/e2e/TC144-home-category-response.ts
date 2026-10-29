import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';

test.use({baseURL:'http://localhost:6680',websiteGateway:!process.env.WEBSITE_HOME_CANDIDATE,headless:true});
test.describe('TC-144 首页分类统一即时切换',()=>{
 for(const mobile of [false,true])test(`TC-144${mobile?'b':'a'}: 慢接口不阻塞选中与已读商品，旧响应不覆盖当前分类`,async({page},info)=>{
  await controlledBusiness(page);
  await page.setViewportSize({width:mobile?390:1440,height:900});
  const names=['弹性云','云服务器演示','计算型服务器演示','高防服务器演示'];
  const categories=names.map((name,i)=>({categoryNo:'home-'+i,nameZhCN:name}));
  let slow=false,fail=false,updated=false;
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('**'+prefix+'/catalog/products?*',async route=>{
   const cat=new URL(route.request().url()).searchParams.get('categoryNo');
   if(slow)await new Promise(resolve=>setTimeout(resolve,1200));
   if(fail&&cat==='home-0')return route.fulfill({status:503,json:{code:50}});
   const i=categories.findIndex(c=>c.categoryNo===cat);
   return route.fulfill({json:{code:0,data:{categories,products:i<0||i===3?[]:[{categoryNo:cat,publicSlug:'home-product-'+i,nameZhCN:updated&&i===0?'更新后的分类商品0':'分类商品'+i,availability:'available'}],nextCursor:''}}});
  });
  const site=new WebsitePage(page);await site.open('/');
  const cards=page.locator('[data-products] .systemproduct-name');
  await expect(cards).toHaveText(['分类商品0']);
  slow=true;
  for(let i=1;i<4;i++){
   await site.category(names[i]);
   await expect(page.locator('[data-categories] a.active')).toHaveText(names[i],{timeout:500});
   if(i===3)await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');
   else await expect(cards).toHaveText(['分类商品'+i]);
  }
  slow=true;
  for(let i=0;i<4;i++){
   await site.category(names[i]);
   await expect(page.locator('[data-categories] a.active')).toHaveText(names[i],{timeout:500});
   await expect(page.locator('.systemproduct-left-title')).toHaveText(names[i],{timeout:500});
   await expect(cards).toHaveText(i===3?[]:['分类商品'+i],{timeout:500});
  }
  await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');
  await page.waitForTimeout(1400);
  await expect(cards).toHaveCount(0);
  await expect(page.locator('[data-categories] a.active')).toHaveText(names[3]);
  fail=true;await site.category(names[0]);await expect(cards).toHaveText(['分类商品0'],{timeout:500});
  await expect(page.locator('[data-status]')).toHaveText('商品加载失败，请刷新重试。');
  await page.locator('[data-status]').scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath('category-refresh-failed.png'),animations:'disabled'});
  fail=false;slow=false;updated=true;await site.retry();await expect(page.locator('[data-status]')).toHaveText('');
  await expect(cards).toHaveText(['更新后的分类商品0']);
  await page.locator('[data-categories]').scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath('category-recovered.png'),animations:'disabled'});
  expect(errors).toEqual([]);
 });
});
