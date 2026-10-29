import {test,expect} from '@playwright/test';
import {CatalogPage} from '../pages/CatalogPage';
import {ConfigurationControlsPage} from '../pages/ConfigurationControlsPage';

const categoryNo='ctg_0843ae4ede9a24327b46e12f828c3dff';
const path='/products.html?categoryNo='+categoryNo;
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:1000}});

test.describe('TC-86 实际官网最低配置月起价',()=>{
 test('TC-86a: 八款商品展示权威最低月价并对照实际配置报价',async({page})=>{
  test.setTimeout(90000);
  const site=new CatalogPage(page),configuration=new ConfigurationControlsPage(page),errors:string[]=[];
  page.on('pageerror',error=>errors.push(error.message));
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=12&categoryNo='+categoryNo);
  expect(response.ok()).toBe(true);
  const result=await response.json();expect(result.code).toBe(0);expect(result.data.products).toHaveLength(8);
  await site.open(path);
  for(const product of result.data.products){
   const price=product.minimumConfigurationMonthlyPrice;
   expect(price,product.publicSlug+' requires a complete minimum monthly price').toBeTruthy();
   expect(price.currencyCode).toBe('CNY');
   const amount=(Number(price.amountMinor)/100).toFixed(2),card=site.product(product.publicSlug);
   await expect(card.locator('.amount')).toHaveText(amount);
   await expect(card.locator('.period')).toHaveText('起/月');
   await expect(card.locator('.price')).toHaveAttribute('title','最低可购配置的月付价，含适用税费，不含一次性安装费');
   await expect(card.locator('.lina-catalog-setup')).toHaveCount(0);
   await site.revealPrice(product.publicSlug);
   await expect(card.locator('.price')).toBeInViewport();
  }
  await site.captureCatalog('tc086-live-minimum-monthly-second-row');
  for(const product of result.data.products){
   // Choose the actual smallest CPU/memory/disk/bandwidth specs, including later options.
   // The simulation fixtures price their smallest numeric specs at the minimum.
   await configuration.open('/configure.html?product='+product.publicSlug);
   const monthly=page.locator('[data-quote] button').filter({hasText:/^(?:1月|月)[:：]/});
   await expect(monthly).toHaveCount(1);
   const schema=await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+product.publicSlug+'/purchase-configuration').then(r=>r.json());
   for(const field of schema.data.fields){
    if(field.type!=='single_select'||!field.visible||field.disabled||!field.options?.length)continue;
    const options=field.options;
    if(!options.every((o:any)=>/^\d+\s*(?:核|GiB|Mbps)$/.test(o.labelZhCN)))continue;
    const smallest=[...options].sort((a:any,b:any)=>parseInt(a.labelZhCN)-parseInt(b.labelZhCN))[0];
    const button=configuration.options(field.labelZhCN).getByRole('button',{name:smallest.labelZhCN,exact:true});
    if(await button.getAttribute('aria-pressed')==='true')continue;
    const requote=page.waitForResponse(r=>r.url().includes('/purchase-configuration/quote')&&r.request().method()==='POST');
    await configuration.option(field.labelZhCN,smallest.labelZhCN);
    expect((await requote).ok()).toBe(true);
    await expect(monthly).toHaveCount(1);
   }
   const quoted=Number((await monthly.innerText()).split('CNY ')[1]);
   expect(quoted,product.publicSlug+' minimum must be purchasable at the configuration page').toBe(Number(product.minimumConfigurationMonthlyPrice.amountMinor)/100);
   await expect(page.locator('.os-card')).toHaveCount(6);
   await expect(page.locator('[data-quote] button')).toHaveCount(4);
   console.log(product.publicSlug+': ¥'+quoted.toFixed(2)+' 起/月, interactive quote matched');
  }
  expect(errors).toEqual([]);
 });
 test('TC-86b: 手机商品卡保留起价且不横向溢出',async({page})=>{
  await page.setViewportSize({width:390,height:844});
  const site=new CatalogPage(page);await site.open(path);
  await expect(page.locator('[data-products] .amount')).toHaveCount(8);
  await expect(page.locator('[data-products] .period')).toHaveText(Array(8).fill('起/月'));
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  const slug=await page.locator('[data-products] [data-product]').first().getAttribute('data-product');
  await site.revealPrice(slug!);
  await expect(site.product(slug!).locator('.price')).toBeInViewport();
  await site.captureCatalog('tc086-live-minimum-monthly-mobile');
 });
});
