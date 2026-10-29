import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-16 真实商品修改同步',()=>{
 test('TC-16a: 后台修改销售文案在原列表及配置页刷新可见，结束恢复测试商品',async({page})=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','只允许本地隔离测试');
  const backend=process.env.WEBSITE_API_ORIGIN!;expect(['localhost','127.0.0.1']).toContain(new URL(backend).hostname);
  expect(['16630','16634'],'只允许既有隔离后台，禁止当前6030业务库').toContain(new URL(backend).port);
  const state=JSON.parse(await readFile(process.env.WEBSITE_QA_CATALOG_FILE!,'utf8'));
  const productNo=state.product.productNo;
  const login=await (await page.request.post(backend+'/api/v1/auth/login',{data:{username:process.env.WEBSITE_QA_ADMIN_USER,password:process.env.WEBSITE_QA_ADMIN_PASSWORD,clientType:'web'}})).json();expect(login.code).toBe(0);
  const headers={Authorization:'Bearer '+login.data.accessToken};const url=backend+'/x/idc-core/api/v1/admin/catalog/products/'+productNo;
  const detail=await (await page.request.get(url,{headers})).json();expect(detail.code).toBe(0);
  const product=detail.data.product;expect(product.publicSlug).toBe('isolated-integration-only');
  const original={nameZhCn:product.nameZhCn,nameEnUs:product.nameEnUs,summaryZhCn:product.summaryZhCn,summaryEnUs:product.summaryEnUs};
  let version=product.descriptionVersion;
  async function update(data:typeof original){const result=await (await page.request.put(url+'/description',{headers,data:{...data,version}})).json();expect(result.code,result.message).toBe(0);version=result.data.version;}
  const title=original.nameZhCn+' 更新验收';
  try{
   await update({...original,nameZhCn:title});const changedAt=Date.now();
   const site=new WebsitePage(page);await site.open('/products.html');
   await expect(page.locator('[data-products]')).toContainText(title);
   await site.open('/configure.html?product='+product.publicSlug);
   await expect(page.locator('.allocation-header-title h1')).toHaveText(title);
   expect(Date.now()-changedAt).toBeLessThan(10000);
   await expect(page.locator('[data-quote] button').first()).toHaveText('1月：CNY 1.00');
   await site.capture('real-product-update',false);
  }finally{await update(original);}
 });
});
