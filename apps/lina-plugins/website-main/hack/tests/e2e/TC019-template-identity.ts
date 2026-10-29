import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

// Read-only verification of the installed identity, current merchant data and
// existing customer entry. Does not create carts, accounts, content or orders.
test.describe('TC-19 已安装官网模板身份切换',()=>{
 test('TC-19a: 新身份十页资源生效且保留商家品牌与用户中心入口',async({page,request},info)=>{
  test.setTimeout(90000);
  const brandResponse=await request.get('/x/idc-portal/api/v1/public/brand');
  expect(brandResponse.ok()).toBeTruthy();
  const result=await brandResponse.json();expect(result.code).toBe(0);
  const brand=result.data.brand;
  const version=process.env.WEBSITE_EXPECTED_VERSION||'v0.1.10';
  expect(version).toMatch(/^v\d+\.\d+\.\d+$/);
  const assetBase='/x-assets/website-main/'+version+'/';
  expect(brand.templateEntry.assetBase).toBe(assetBase);
  expect(brand.templateEntry.url).toBe(assetBase+'index.html');
  const site=new WebsitePage(page),errors:string[]=[],missing:string[]=[],writes:string[]=[];
  page.on('pageerror',error=>errors.push(error.message));
  page.on('response',response=>{if(response.status()>=400&&!new URL(response.url()).pathname.startsWith('/x/'))missing.push(response.status()+' '+response.url());});
  page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.method()+' '+request.url());});
  const records:unknown[]=[];
  for(const name of ['index','products','configure','news','announcements','help','downloads','content','contact','about']){
   const response=await page.goto('/'+name+'.html');expect(response?.ok(),name).toBeTruthy();
   await expect(page.locator('base')).toHaveAttribute('href',assetBase);
   await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN||brand.siteName.enUS);
   await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|正在读取/);
   records.push({name,screenshot:await site.capture('website-main-'+name)});
  }
  await site.open('/products.html');
  const customer=page.getByRole('link',{name:'控制台',exact:true});
  await expect(customer).toHaveAttribute('href',/^https?:\/\/(?:localhost|127\.0\.0\.1):6032\/account$/);
  expect(errors).toEqual([]);expect(missing).toEqual([]);expect(writes).toEqual([]);
  await info.attach('identity-pages',{body:JSON.stringify({entry:brand.templateEntry,records,errors,missing,writes},null,2),contentType:'application/json'});
 });
});
