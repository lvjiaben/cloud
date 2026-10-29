import {test,expect} from '../support/live-commerce';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';

test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-98 商品卡不等待分类资料',()=>{
 test('TC-98a: 分类请求挂起时商品卡及购买入口仍立即可用',async({page})=>{
  const site=new CommerceNavigationPage(page);
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=1');
  expect(response.ok()).toBe(true);
  const {data}=await response.json(),product=data.products[0];expect(product).toBeTruthy();
  let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/catalog/products?limit=1',async route=>{await gate;await route.fallback();});
  try{
   await site.open('/products.html?product='+encodeURIComponent(product.publicSlug));
   await expect(page.locator('[data-products] h5')).toHaveText(product.nameZhCN||product.nameEnUS,{timeout:2000});
   await expect(page.locator('[data-products] .buy-now-button')).toHaveAttribute('href','/configure.html?product='+encodeURIComponent(product.publicSlug));
   console.log('TC098 pending categories: '+await site.capture('tc098-pending-categories',false));
  }finally{release();}
  await expect(page.locator('[data-catalog-group]')).not.toHaveCount(0);
 });
 test('TC-98b: 分类失败保留商品及价格，重试仅补读分类',async({page})=>{
  const site=new CommerceNavigationPage(page),response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=1');
  const {data}=await response.json(),product=data.products[0];expect(product).toBeTruthy();
  let failed=true,details=0;
  page.on('request',request=>{if(new URL(request.url()).pathname.endsWith('/catalog/products/'+product.publicSlug))details++;});
  await page.route('**/catalog/products?limit=1',route=>failed?route.fulfill({status:503,json:{code:1}}):route.fallback());
  await site.open('/products.html?product='+encodeURIComponent(product.publicSlug));
  await expect(page.locator('[data-products] h5')).toHaveText(product.nameZhCN||product.nameEnUS);
  const price=await page.locator('[data-products] .price').textContent();
  await expect(page.locator('[data-categories]')).toContainText('商品分类加载失败，请重试。');
  console.log('TC098 failure: '+await site.capture('tc098-category-failure',false));
  const detailsBeforeRetry=details;failed=false;await site.retryCategories();
  await expect(page.locator('[data-catalog-group]')).not.toHaveCount(0);
  await expect(page.locator('[data-products] .price')).toHaveText(price);expect(details).toBe(detailsBeforeRetry);
 });
 test('TC-98c: 同页切换服务器复用分类，商品及价格仍实时读取',async({page})=>{
  const site=new CommerceNavigationPage(page);
  await site.open('/products.html');
  const alternative=page.locator('[data-catalog-product]').first();await expect(alternative).toBeVisible();
  const slug=(await alternative.getAttribute('data-catalog-product'))!;
  const expected=await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+encodeURIComponent(slug));
  const {product}=(await expected.json()).data;
  let metadataReads=0,detailReads=0;
  page.on('request',request=>{const url=new URL(request.url());if(url.pathname.endsWith('/catalog/products')&&url.searchParams.get('limit')==='1')metadataReads++;if(url.pathname.endsWith('/catalog/products/'+slug))detailReads++;});
  await site.chooseServer(slug);
  await expect(page.locator('[data-products] h5')).toHaveText(product.nameZhCN||product.nameEnUS);
  await expect(page.locator('[data-products] .product-card')).toHaveCount(1);
  if(product.minimumConfigurationMonthlyPrice?.currencyCode==='CNY')await expect(page.locator('[data-products] .amount')).toHaveText((Number(product.minimumConfigurationMonthlyPrice.amountMinor)/100).toFixed(2));
  expect(metadataReads).toBe(0);expect(detailReads).toBe(1);
  await site.buy();await expect(page.locator('[data-quote] button')).not.toHaveCount(0,{timeout:15000});
  await expect(page.locator('.allocation-header-title h1')).toHaveText(product.nameZhCN||product.nameEnUS);
 });
 test('TC-98d: 已进入购买配置后迟到的分类失败不覆盖新视图',async({page})=>{
  const site=new CommerceNavigationPage(page),response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=1');
  const {data}=await response.json(),product=data.products[0];expect(product).toBeTruthy();
  let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  let calls=0,finish!:()=>void;
  const finished=new Promise<void>(resolve=>{finish=resolve;});
  await page.route('**/catalog/products?limit=1',async route=>{
   if(++calls===1){await gate;await route.fulfill({status:503,json:{code:1}});finish();}else await route.fallback();
  });
  try{
   await site.open('/products.html?product='+encodeURIComponent(product.publicSlug));
   await expect(page.locator('[data-products] h5')).toHaveText(product.nameZhCN||product.nameEnUS,{timeout:2000});
   await site.buy();await expect(page.locator('[data-quote] button')).not.toHaveCount(0,{timeout:15000});
  }finally{release();}
  await finished;
  console.log('TC098 late response: '+await site.capture('tc098-late-response',false));
  await expect(page.locator('[data-catalog-group]')).not.toHaveCount(0);
  await expect(page.locator('[data-categories]')).not.toContainText('商品分类加载失败');
  await expect(page.locator('body')).toHaveAttribute('data-page','configure');
  await expect(page.locator('.allocation-header-title h1')).toHaveText(product.nameZhCN||product.nameEnUS);
 });
});
