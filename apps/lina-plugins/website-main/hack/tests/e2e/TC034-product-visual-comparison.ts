import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {mountOriginalSite,originalOrigin} from '../support/original-site';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-34 商品与有效配置整页对照',()=>{
 test('TC-34a: 原页只读对照与当前真实商品配置、报价布局',async({page},info)=>{
  const site=new WebsitePage(page),evidence:unknown[]=[];await mountOriginalSite(page);
  const writes:string[]=[];page.on('request',r=>{if(!['GET','HEAD'].includes(r.method())&&!/\/purchase-configuration\/(evaluate|quote)$/.test(new URL(r.url()).pathname))writes.push(r.method()+' '+r.url());});
  let configure:string|null=null;
  for(const [name,source,selector] of [['products','cart-2_7.html','.shopping-cart-page'],['configure','cart-configureproduct_128.html','.allocation-card']]){
   const response=await site.open(originalOrigin+'/'+source);expect(response?.ok()).toBe(true);expect(response!.headers()['content-security-policy']).toContain("script-src 'none'");
   await site.inspectFullPage();const original=await site.capture('product-comparison-source-'+name);
   await site.open(name==='products'?'/products.html':configure!);await expect(page.locator('[data-brand-name]').first()).not.toHaveText('');
   await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|正在读取|正在检查/);
   if(name==='products'){configure=await site.availableConfigurationPath();expect(configure).toBeTruthy();}
   else await expect(page.locator('[data-quote] button').first()).toContainText('CNY');
   await expect(page.locator(selector).first()).toBeVisible();await site.inspectFullPage();expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
   const migrated=await site.capture('product-comparison-current-'+name);evidence.push({name,source,original,migrated,configure});
  }
  expect(writes).toEqual([]);await info.attach('product-visual-pairs',{body:JSON.stringify(evidence,null,2),contentType:'application/json'});
 });
});
