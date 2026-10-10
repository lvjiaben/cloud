// Sales catalog filters use the public owner API independently of purchased-service menus.
import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect,test} from '@host-tests/fixtures/auth';
import {LoginPage} from '@host-tests/pages/LoginPage';
test('TC-24: 购买销售分类筛选、URL恢复和失败重试',async({page})=>{
 const categories=[{categoryNo:'ctg_cloud',nameZhCN:'云计算',nameEnUS:'Cloud computing'},{categoryNo:'ctg_ssl',nameZhCN:'安全证书',nameEnUS:'Certificates'}];
 const product=(categoryNo:string,name:string)=>({categoryNo,categoryNameZhCN:name,categoryNameEnUS:name,publicSlug:categoryNo,nameZhCN:name+'商品',nameEnUS:name,summaryZhCN:'销售分类测试',summaryEnUS:'Sales category test',currencies:['CNY'],availability:'available'});
 const requests:Array<{category:string;cursor:string}>=[];let fail=false;
 await page.route('**/api/v1/auth/login',route=>route.fulfill({json:{code:0,data:{accessToken:'ui-catalog-only',refreshToken:'ui-catalog-refresh',tenants:[]}}}));
 await page.route('**/api/v1/user/info',route=>route.fulfill({json:{code:0,data:{id:702,username:'catalog-customer',realName:'销售分类测试客户',roles:['user'],permissions:[],menus:[],homePath:'/account/products'}}}));
 await page.route('**/api/v1/plugins/dynamic',route=>route.fulfill({json:{code:0,data:{list:[]}}}));
 await page.route('**/customer/external-menus',route=>route.fulfill({json:{code:0,data:{items:[]}}}));
 await page.route('**/account/product-directories*',route=>route.fulfill({json:{code:0,data:{items:[],total:0,page:1,pageSize:100}}}));
 await page.route('**/account/initial-password/status',route=>route.fulfill({json:{code:0,data:{required:false}}}));
 await page.route(/\/public\/catalog\/products(?:\?|$)/u,async route=>{
  const query=new URL(route.request().url()).searchParams,category=query.get('categoryNo')||'',cursor=query.get('cursor')||'';requests.push({category,cursor});
  if(fail){await route.fulfill({status:503,json:{code:50,message:'测试暂时失败'}});return;}
  await route.fulfill({json:{code:0,data:{categories,products:cursor?[]:category?[product(category,category==='ctg_cloud'?'云计算':'安全证书')]:[product('ctg_cloud','云计算'),product('ctg_ssl','安全证书')],nextCursor:category||cursor?'':'opaque-all-next'}}});
 });
 await page.goto('http://127.0.0.1:6032/auth/login');await new LoginPage(page).login('catalog-customer','test-only-password');
 const nav=page.getByTestId('purchase-sales-categories'),list=page.getByTestId('customer-product-catalog');
 await expect(nav.getByRole('button',{name:'全部商品',exact:true})).toHaveAttribute('aria-current','page');await expect(list).toContainText('云计算商品');await expect(list).toContainText('安全证书商品');
 await page.getByRole('button',{name:'加载更多',exact:true}).click();await expect.poll(()=>requests.at(-1)?.cursor).toBe('opaque-all-next');
 await nav.getByRole('button',{name:'云计算',exact:true}).click();await expect(page).toHaveURL(/categoryNo=ctg_cloud/u);await expect(list).toContainText('云计算商品');await expect(list).not.toContainText('安全证书商品');expect(requests.at(-1)).toEqual({category:'ctg_cloud',cursor:''});
 await page.reload();await expect(nav.getByRole('button',{name:'云计算',exact:true})).toHaveAttribute('aria-current','page');await expect(list).toContainText('云计算商品');
 const dir=resolve('../../temp/20261009/purchase-categories');await mkdir(dir,{recursive:true});await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(animation=>animation.effect?.getTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>undefined)));});await page.screenshot({path:resolve(dir,`${Date.now()}-selected.png`),fullPage:true});
 fail=true;await nav.getByRole('button',{name:'安全证书',exact:true}).click();await expect(page.getByText('商品列表加载失败，请稍后重试',{exact:true})).toBeVisible();await expect(list).toHaveCount(0);await expect(nav.getByRole('button',{name:'安全证书',exact:true})).toHaveAttribute('aria-current','page');
 await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(animation=>animation.effect?.getTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>undefined)));});await page.screenshot({path:resolve(dir,`${Date.now()}-error.png`),fullPage:true});
 fail=false;await page.getByRole('button',{name:/^重\s*试$/u}).click();await expect(list).toContainText('安全证书商品');expect(requests.at(-1)).toEqual({category:'ctg_ssl',cursor:''});
 await page.setViewportSize({width:390,height:844});await expect(nav).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(animation=>animation.effect?.getTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>undefined)));});await page.screenshot({path:resolve(dir,`${Date.now()}-mobile.png`),fullPage:true});
 await nav.getByRole('button',{name:'全部商品',exact:true}).click();await expect(page).not.toHaveURL(/categoryNo=/u);await expect(list).toContainText('云计算商品');await expect(list).toContainText('安全证书商品');
});
