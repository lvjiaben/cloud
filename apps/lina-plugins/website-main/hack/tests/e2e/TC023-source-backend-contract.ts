import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
const base='/x/idc-core/api/v1/public';

test.describe('TC-23 当前源码与真实后台只读契约',()=>{
 test('TC-23a: 原商品卡片、分类搜索和单件服务端报价一致',async({page,request},info)=>{
  const origin=process.env.WEBSITE_API_ORIGIN;expect(origin).toBeTruthy();
  expect(['localhost','127.0.0.1']).toContain(new URL(origin!).hostname);
  const read=async(path:string)=>{const response=await request.get(origin+path);expect(response.ok()).toBeTruthy();const body=await response.json();expect(body.code).toBe(0);return body.data;};
  const data=await read(base+'/catalog/products?limit=12');
  const product=data.products.find((p:{availability:string})=>p.availability==='available');
  expect(product,'需要现有已发布商品；本用例不创建业务数据').toBeTruthy();
  const site=new WebsitePage(page),writes:string[]=[];
  page.on('request',r=>{if(!['GET','HEAD'].includes(r.method())&&!/\/purchase-configuration\/(evaluate|quote)$/.test(new URL(r.url()).pathname))writes.push(r.method()+' '+r.url());});
  await site.open('/products.html');
  await expect(page.locator('[data-products] > *')).toHaveCount(data.products.length);
  await expect(page.locator('[data-categories] a')).toHaveText(['全部商品',...data.categories.map((c:any)=>c.nameZhCN||c.nameEnUS)]);
  for(const p of data.products){const card=page.locator('[data-products] > *').filter({has:page.getByRole('heading',{name:p.nameZhCN||p.nameEnUS,exact:true})});await expect(card.locator('.product-description')).toHaveText(p.summaryZhCN||p.summaryEnUS||'');}
  const query=new URLSearchParams({limit:'12',categoryNo:product.categoryNo,namePrefix:product.nameZhCN||product.nameEnUS});
  const filtered=await read(base+'/catalog/products?'+query);
  await site.open('/products.html?categoryNo='+encodeURIComponent(product.categoryNo));
  const search=page.waitForResponse(r=>new URL(r.url()).searchParams.get('namePrefix')===query.get('namePrefix'));
  await site.searchProducts(query.get('namePrefix')!);await search;
  await expect(page.locator('[data-products] > *')).toHaveCount(filtered.products.length);
  for(const p of filtered.products)await expect(page.locator('[data-products] h5').filter({hasText:p.nameZhCN||p.nameEnUS})).toHaveCount(1);
  expect(new URL(page.url()).searchParams.get('categoryNo')).toBe(product.categoryNo);
  const catalogScreenshot=await site.capture('source-contract-filter');
  const path=base+'/catalog/products/'+encodeURIComponent(product.publicSlug);
  const current=await read(path+'/purchase-configuration');
  const quoted=page.waitForResponse(r=>new URL(r.url()).pathname===path+'/purchase-configuration/quote');
  await site.open('/configure.html?product='+encodeURIComponent(product.publicSlug));
  const response=await quoted;expect(response.ok()).toBeTruthy();const result=await response.json();expect(result.code).toBe(0);const quote=result.data;
  expect(quote.configurationRevision).toBe(current.configurationRevision);
  await expect(page.locator('.allocation-header-title h1')).toHaveText(product.nameZhCN||product.nameEnUS);
  const index=quote.terms.findIndex((t:any)=>t.status==='final');expect(index).toBeGreaterThanOrEqual(0);
  const term=quote.terms[index],digits=new Intl.NumberFormat('zh-CN',{style:'currency',currency:quote.currencyCode}).resolvedOptions().maximumFractionDigits!;
  const amount=BigInt(term.totalAmount),factor=10n**BigInt(digits);
  const price=quote.currencyCode+' '+amount/factor+(digits?'.'+String(amount%factor).padStart(digits,'0'):'');
  const termText=(term.labelZhCN||term.nameZhCN||term.labelEnUS||term.nameEnUS)+'：'+price;
  await expect(page.locator('[data-quote] button').nth(index)).toHaveText(termText);
  await expect(page.locator('[data-add]')).toBeDisabled();await site.selectTerm(index);await expect(page.locator('[data-add]')).toBeEnabled();
  await site.setPurchaseCount('quantity','0');await expect(page.locator('[data-add]')).toBeDisabled();
  await site.setPurchaseCount('quantity','2');await site.setPurchaseCount('cycles',String(Math.max(3,term.minimumPurchaseCycles||1)));await expect(page.locator('[data-add]')).toBeEnabled();
  // Existing quote contract is per item/term; counts are submitted only at cart handoff.
  await expect(page.locator('[data-quote] button').nth(index)).toHaveText(termText);
  await expect(page.locator('[data-status]')).toHaveText('报价按单件及所选结算周期显示，结算金额以用户中心为准。');
  await site.refreshConfiguration();await expect(page.locator('[data-quote] button').nth(index)).toHaveText(termText);
  await expect(page.locator('[name=quantity]')).toHaveValue('2');await expect(page.locator('[name=cycles]')).toHaveValue(String(Math.max(3,term.minimumPurchaseCycles||1)));
  await expect(page.locator('[data-add]')).toBeDisabled();await site.selectTerm(index);await expect(page.locator('[data-add]')).toBeEnabled();
  await site.setPurchaseCount('quantity','0');await site.refreshConfiguration();await expect(page.locator('[data-quote] button').nth(index)).toHaveText(termText);await site.selectTerm(index);
  await expect(page.locator('[name=quantity]')).toHaveValue('0');await expect(page.locator('[data-add]')).toBeDisabled();await site.setPurchaseCount('quantity','2');await expect(page.locator('[data-add]')).toBeEnabled();
  const quoteScreenshot=await site.capture('source-contract-quote');
  // A failed configuration read must not discard inputs on the next retry.
  const configurationRoute='**'+path+'/purchase-configuration';
  await page.route(configurationRoute,route=>route.abort('failed'));
  await site.refreshConfiguration();
  await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');
  await expect(page.locator('[data-add]')).toBeDisabled();
  const failedReadScreenshot=await site.capture('source-contract-read-failed');
  await site.refreshConfiguration();
  await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');
  await expect(page.locator('[data-add]')).toBeDisabled();
  await page.unroute(configurationRoute);
  await site.refreshConfiguration();
  await expect(page.locator('[data-quote] button').nth(index)).toHaveText(termText);
  await expect(page.locator('[name=quantity]')).toHaveValue('2');
  await expect(page.locator('[name=cycles]')).toHaveValue(String(Math.max(3,term.minimumPurchaseCycles||1)));
  await expect(page.locator('[data-add]')).toBeDisabled();
  await site.selectTerm(index);await expect(page.locator('[data-add]')).toBeEnabled();
  const recoveredReadScreenshot=await site.capture('source-contract-read-recovered');
  await info.attach('configuration-read-recovery',{body:JSON.stringify({failedReadScreenshot,recoveredReadScreenshot}),contentType:'application/json'});
  await info.attach('real-readonly-contract',{body:JSON.stringify({product:product.publicSlug,configurationRevision:current.configurationRevision,fieldTypes:current.fields.map((f:any)=>f.type),complexConfigurationVerified:current.fields.length>0,currency:quote.currencyCode,totalAmount:term.totalAmount,quantity:2,purchaseCycles:Math.max(3,term.minimumPurchaseCycles||1),catalogScreenshot,quoteScreenshot},null,2),contentType:'application/json'});
  expect(writes).toEqual([]);
 });
});
