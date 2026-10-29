import {test,expect,type APIRequestContext} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

// Existing published data only. Never creates fixtures, carts, accounts or orders.
// Evaluation and quote POSTs are the existing read-only pricing operations.
async function readBackend(api:APIRequestContext,path:string){
 const origin=process.env.WEBSITE_API_ORIGIN;
 expect(origin,'explicit local backend required').toBeTruthy();
 expect(['localhost','127.0.0.1']).toContain(new URL(origin!).hostname);
 const response=await api.get(new URL(path,origin).href);
 expect(response.ok(),path).toBeTruthy();const result=await response.json();
 expect(result.code,path).toBe(0);return result.data;
}

test.describe('TC-18 普通迁移页面复用现有后台只读验收',()=>{
 test('TC-18a: 后台商品及分类进入原卡片，当前配置和报价直接来自既有接口',async({page,request},info)=>{
  const base='/x/idc-core/api/v1/public';
  const data=await readBackend(request,base+'/catalog/products?limit=12');
  const {brand}=await readBackend(request,'/x/idc-portal/api/v1/public/brand');
  const site=new WebsitePage(page);const writes:string[]=[],errors:string[]=[],stylesheets:{url:string;status:number}[]=[];
  page.on('response',r=>{if(r.request().resourceType()==='stylesheet')stylesheets.push({url:r.url(),status:r.status()});});
  page.on('request',r=>{if(!['GET','HEAD'].includes(r.method())&&!/\/purchase-configuration\/(evaluate|quote)$/.test(new URL(r.url()).pathname))writes.push(r.method()+' '+r.url());});
  page.on('pageerror',error=>errors.push(error.message));
  await site.open('/products.html');
  await expect(page.locator('[data-status]')).not.toHaveText(/正在加载/);
  await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN||brand.siteName.enUS);
  await expect(page.locator('[data-products] > *')).toHaveCount(data.products.length);
  await expect(page.locator('[data-categories] a')).toHaveText(['全部商品',...data.categories.map((c:{nameZhCN:string;nameEnUS:string})=>c.nameZhCN||c.nameEnUS)]);
  for(const product of data.products){
   const card=page.locator('[data-products] > *').filter({has:page.getByRole('heading',{name:product.nameZhCN||product.nameEnUS,exact:true})});
   await expect(card).toHaveCount(1);
   await expect(card.locator('.product-description')).toHaveText(product.summaryZhCN||product.summaryEnUS||'');
   if(product.availability==='available')await expect(card.locator('.buy-now-button')).toHaveAttribute('href','/configure.html?product='+encodeURIComponent(product.publicSlug));
   else await expect(card.locator('.buy-now-button')).toHaveCount(0);
  }
  await site.capture('existing-backend-catalog');
  const product=data.products.find((p:{availability:string})=>p.availability==='available');
  expect(product,'existing available local product is required; do not create one here').toBeTruthy();
  const productPath=base+'/catalog/products/'+encodeURIComponent(product.publicSlug);
  const current=await readBackend(request,productPath+'/purchase-configuration');
  const quoted=page.waitForResponse(r=>new URL(r.url()).pathname===productPath+'/purchase-configuration/quote');
  await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));
  const response=await quoted;expect(response.ok()).toBeTruthy();const quote=(await response.json()).data;
  await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN||brand.siteName.enUS);
  expect(quote.configurationRevision).toBe(current.configurationRevision);
  await expect(page.locator('.allocation-header-title h1')).toHaveText(product.nameZhCN||product.nameEnUS);
  await expect(page.locator('[data-product-summary]')).toHaveText(product.summaryZhCN||product.summaryEnUS||'');
  await expect(page.getByRole('combobox',{name:'币种'})).toHaveClass('form-control');
  for(const name of ['quantity','cycles']){
   await expect(page.locator('[name='+name+']')).toHaveClass('form-control');
   expect((await page.locator('[name='+name+']').boundingBox())!.height).toBeGreaterThanOrEqual(36);
  }
  await expect(page.locator('[data-add]')).toHaveClass(/btn-primary/);
  await expect(page.locator('[data-refresh]')).toHaveClass(/btn-light/);
  await expect(page.locator('[data-add]')).toBeDisabled();
  const final=quote.terms.findIndex((term:{status:string})=>term.status==='final');
  expect(final,'a final server quote is required').toBeGreaterThanOrEqual(0);
  const term=quote.terms[final],digits=new Intl.NumberFormat('zh-CN',{style:'currency',currency:quote.currencyCode}).resolvedOptions().maximumFractionDigits!;
  const amount=Number(term.totalAmount);expect(Number.isSafeInteger(amount)).toBeTruthy();
  await expect(page.locator('[data-quote] button').nth(final)).toHaveText((term.labelZhCN||term.nameZhCN||term.labelEnUS||term.nameEnUS)+'：'+quote.currencyCode+' '+(amount/10**digits).toFixed(digits));
  await site.selectTerm(final);
  await expect(page.locator('[data-add]')).toBeEnabled();
  for(const selector of ['[aria-label="币种"]','[name=quantity]','[name=cycles]']){
   await expect(page.locator(selector)).toHaveCSS('border-top-style','solid');
   await expect(page.locator(selector)).toHaveCSS('border-top-width','1px');
   await expect(page.locator(selector)).toHaveCSS('background-color','rgb(255, 255, 255)');
  }
  await expect(page.locator('[data-add]')).toHaveCSS('color','rgb(255, 255, 255)');
  const controls=await page.locator('[data-purchase-controls] input,[data-purchase-controls] select,[data-add],[data-refresh]').evaluateAll(elements=>elements.map(element=>{
   const style=getComputedStyle(element);const box=element.getBoundingClientRect();
   return {name:element.getAttribute('name')||element.getAttribute('aria-label')||element.textContent,height:box.height,border:style.borderTop,color:style.color,background:style.backgroundColor};
  }));
  const primary=controls.find(control=>control.name==='加入购物车')!;
  const secondary=controls.find(control=>control.name==='重新读取配置')!;
  const rgb=primary.background.match(/\d+/g)!.map(Number);
  expect(rgb[2]).toBeGreaterThan(rgb[0]+60);
  expect(primary.background).not.toBe(secondary.background);
  expect(primary.height).toBeGreaterThanOrEqual(36);
  expect(stylesheets.length).toBeGreaterThan(8);
  for(const sheet of stylesheets)expect(sheet.status,sheet.url).toBe(200);
  for(const file of ['bootstrap.min-v7.9.5.css','config.css','lina/site.css'])expect(stylesheets.some(sheet=>sheet.url.endsWith('/'+file))).toBeTruthy();
  await site.capture('existing-backend-server-quote');
  await info.attach('computed-source-styles',{body:JSON.stringify({stylesheets,controls},null,2),contentType:'application/json'});
  await info.attach('existing-product-contract',{body:JSON.stringify({slug:product.publicSlug,configurationRevision:current.configurationRevision,fieldTypes:current.fields.map((f:{type:string})=>f.type),currency:quote.currencyCode,termStatus:term.status,totalAmount:term.totalAmount,complexConfigurationVerified:current.fields.length>0},null,2),contentType:'application/json'});
  expect(writes).toEqual([]);expect(errors).toEqual([]);
 });

 test('TC-18b: 四类内容及网站名与现有后台一致，空数据明确显示',async({page,request},info)=>{
  const site=new WebsitePage(page);const observations:unknown[]=[];
  const {brand}=await readBackend(request,'/x/idc-portal/api/v1/public/brand');
  for(const [path,type]of [['news','news'],['announcements','announcement'],['help','knowledge'],['downloads','download']]){
   const data=await readBackend(request,'/x/idc-core/api/v1/public/content/items?type='+type+'&limit=20');
   await site.open('/'+path+'.html');
   await expect(page.locator('[data-status]')).not.toHaveText(/正在加载/);
   await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN||brand.siteName.enUS);
   await expect(page.locator('[data-content-list] > *')).toHaveCount(data.items.length);
   if(!data.items.length)await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');
   for(const item of data.items){
    const link=page.locator('[data-content-list] a[href="/content.html?id='+encodeURIComponent(item.contentNo)+'"]');
    await expect(link).toHaveCount(1);await expect(link).toContainText(item.title);
   }
   observations.push({page:path,type,count:data.items.length,screenshot:await site.capture('existing-backend-'+type)});
  }
  await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN||brand.siteName.enUS);
  await info.attach('existing-content-contract',{body:JSON.stringify(observations,null,2),contentType:'application/json'});
 });
});
