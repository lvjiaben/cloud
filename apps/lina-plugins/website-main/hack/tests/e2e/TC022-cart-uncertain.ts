import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-22 加车结果未知保护',()=>{
 test('TC-22a: 断线后重新报价及刷新均不重复提交，确认检查后才能继续',async({page,request})=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await (await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  // Only these controlled transport failures are intercepted. No cart write reaches a backend.
  let submissions=0;
  await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.abort('connectionfailed'));
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',r=>{submissions++;return r.abort('connectionfailed');});
  const site=new WebsitePage(page);const url='/configure.html?product='+encodeURIComponent(product.publicSlug);
  await site.open(url);await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await site.submit();
  await expect(page.locator('[data-status]')).toContainText('尚未提交商品');expect(submissions).toBe(0);
  await page.unroute('**/x/idc-core/api/v1/public/commerce/carts');
  // This response represents only transport acknowledgement, never a completed transaction.
  await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{}}}));
  await site.refreshConfiguration();await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await site.submit();
  await expect(page.locator('[data-cart-recovery]')).toContainText('上次加车结果尚未确认');expect(submissions).toBe(1);
  await site.capture('website-main-cart-uncertain');
  await site.refreshConfiguration();await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeDisabled();
  await site.open(url);await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeDisabled();expect(submissions).toBe(1);
  await expect(page.locator('[data-cart-recovery] a')).toHaveAttribute('href','http://localhost:6032/account/cart');
  page.once('dialog',d=>d.dismiss());await page.getByRole('button',{name:'已检查购物车，重新选购'}).click();await expect(page.locator('[data-add]')).toBeDisabled();
  page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'已检查购物车，重新选购'}).click();await expect(page.locator('[data-add]')).toBeEnabled();expect(submissions).toBe(1);
  await site.submit();await expect(page.locator('[data-cart-recovery]')).toBeVisible();expect(submissions).toBe(2);
 });
 test('TC-22b: 正在提交时重复表单事件只发送一次商品请求',async({page,request})=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await(await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  let submissions=0,release!:()=>void;const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{}}}));
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',async r=>{submissions++;await held;await r.abort('connectionfailed');});
  const site=new WebsitePage(page);
  try{
   await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeEnabled();
   await site.submit();await expect.poll(()=>submissions).toBe(1);
   await site.repeatCartSubmit();await site.repeatCartSubmit();expect(submissions).toBe(1);await expect(page.locator('[data-add]')).toBeDisabled();
  }finally{release();}
  await expect(page.locator('[data-cart-recovery]')).toBeVisible();expect(submissions).toBe(1);await site.capture('website-main-cart-repeat');
 });
 for(const operation of ['getItem','setItem'] as const)test('TC-22'+(operation==='getItem'?'c':'d')+': 浏览器 '+operation+' 失败不发送商品请求',async({page,request})=>{
  await page.addInitScript(operation=>{const original=Storage.prototype[operation];Storage.prototype[operation]=function(key:string,...rest:string[]){if(key.startsWith('lina-pending-cart:'))throw new DOMException('Storage unavailable','SecurityError');return original.apply(this,[key,...rest] as never);};},operation);
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await(await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  let submissions=0;await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{}}}));
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',r=>{submissions++;return r.abort('connectionfailed');});
  const site=new WebsitePage(page);await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();
  if(operation==='getItem'){await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-cart-recovery]')).toBeVisible();}
  else{await expect(page.locator('[data-add]')).toBeEnabled();await site.submit();await expect(page.locator('[data-status]')).toContainText('尚未提交商品');await expect(page.locator('[data-add]')).toBeDisabled();}
  expect(submissions).toBe(0);await site.capture('website-main-cart-storage-'+operation);
 });
 for(const failure of ['business-rejection','missing-reference'] as const)test('TC-22'+(failure==='business-rejection'?'e':'f')+': '+failure+' 不跳转且保留防重复保护',async({page,request})=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await(await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  let submissions=0;const submitted:Record<string,unknown>[]=[];
  // Fault injection only. Neither response creates a real cart or item.
  await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{}}}));
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',r=>{submissions++;submitted.push(r.request().postDataJSON());return r.fulfill({json:failure==='business-rejection'?{code:50,message:'受控业务拒绝'}:{code:0,data:{cart:{}}}});});
  const site=new WebsitePage(page);await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();
  await site.setPurchaseCount('quantity','2');await site.setPurchaseCount('cycles','3');await site.submit();
  await expect(page.locator('[data-status]')).toContainText('未能确认加车结果');await expect(page.locator('[data-cart-recovery]')).toBeVisible();await expect(page.locator('[data-add]')).toBeDisabled();expect(submissions).toBe(1);
  await expect(page).toHaveURL(/configure\.html\?product=/);await expect(page.locator('[name=quantity]')).toHaveValue('2');await expect(page.locator('[name=cycles]')).toHaveValue('3');
  await site.repeatCartSubmit();expect(submissions).toBe(1);
  await site.refreshConfiguration();await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeDisabled();expect(submissions).toBe(1);
  await expect(page.locator('[name=quantity]')).toHaveValue('2');await expect(page.locator('[name=cycles]')).toHaveValue('3');
  expect(submitted[0]).toMatchObject({quantity:2,purchaseCycles:3,publicSlug:product.publicSlug});await site.capture('website-main-cart-'+failure);
 });
 test('TC-22g: 购物车初始化期间重复提交不新增请求，失败后允许重新报价',async({page,request})=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await(await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  let carts=0,items=0,release!:()=>void;const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/x/idc-core/api/v1/public/commerce/carts',async r=>{carts++;await held;await r.abort('connectionfailed');});
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',r=>{items++;return r.abort('blockedbyclient');});
  const site=new WebsitePage(page);
  try{
   await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await site.submit();
   await expect.poll(()=>carts).toBe(1);await site.repeatCartSubmit();await site.repeatCartSubmit();
   await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-refresh]')).toBeDisabled();expect(carts).toBe(1);expect(items).toBe(0);
  }finally{release();}
  await expect(page.locator('[data-status]')).toContainText('尚未提交商品');await expect(page.locator('[data-cart-recovery]')).toHaveCount(0);
  await site.refreshConfiguration();await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeEnabled();
  expect(carts).toBe(1);expect(items).toBe(0);await site.capture('website-main-cart-init-recovered');
 });
 test('TC-22h: 无法清除待确认记录时保持锁定，刷新后也不能重复加车',async({page,request})=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  const data=(await(await request.get(origin+'/x/idc-core/api/v1/public/catalog/products?limit=12')).json()).data;
  const product=data.products.find((p:{availability:string})=>p.availability==='available');expect(product).toBeTruthy();
  await page.addInitScript(()=>{const original=Storage.prototype.removeItem;Storage.prototype.removeItem=function(key:string){if(key.startsWith('lina-pending-cart:'))throw new DOMException('Storage unavailable','SecurityError');return original.call(this,key);};});
  let items=0;await page.route('**/x/idc-core/api/v1/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{}}}));
  await page.route('**/x/idc-core/api/v1/public/commerce/cart-items',r=>{items++;return r.abort('connectionfailed');});
  const site=new WebsitePage(page);const url='/configure.html?product='+encodeURIComponent(product.publicSlug);
  await site.open(url);await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await site.submit();await expect(page.locator('[data-cart-recovery]')).toBeVisible();
  page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'已检查购物车，重新选购'}).click();
  await expect(page.locator('[data-status]')).toHaveText('无法更新浏览器中的加车记录，仍禁止重复提交。请先查看用户中心购物车。');
  await site.capture('website-main-cart-remove-feedback');
  await expect(page.locator('[data-cart-recovery]')).toBeVisible();await expect(page.locator('[data-add]')).toBeDisabled();await site.repeatCartSubmit();expect(items).toBe(1);
  await site.open(url);await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeDisabled();
  await expect(page.locator('[data-cart-recovery]')).toContainText('上次加车结果尚未确认');await site.repeatCartSubmit();expect(items).toBe(1);await site.capture('website-main-cart-remove-denied');
 });
});
