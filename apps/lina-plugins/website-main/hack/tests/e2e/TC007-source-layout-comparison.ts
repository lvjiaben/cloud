import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {mountOriginalSite,originalOrigin} from '../support/original-site';

test.use({baseURL:'http://localhost:6680'});

// Source is served with scripts stripped and a restrictive CSP; source business
// endpoints and authorization scripts are never executed. Images/data may be absent.
const pages=[
 ['index.html','index.html','.systemproduct-layoutcontainer'],
 ['products.html','cart-2_7.html','.shopping-cart-page'],
 ['configure.html','cart-configureproduct_128.html','.allocation-card'],
 ['news.html','news.html','.et-main'],
 ['announcements.html','news.html','.et-main'],
 ['help.html','help.html','.et-main'],
 ['downloads.html','downloads.html','.table-responsive'],
 ['content.html','109.html','.et-main.articleContent .news-txt-container'],
 ['contact.html','contact.html','.et-main'],
 ['about.html','Introduction.html','.et-main'],
];
test.describe('TC-7 离线原页面同视口对比证据',()=>{
 for(const width of [1440,390])test('TC-7'+(width===1440?'a':'b')+': '+width+' 像素原容器保留及成对截图',async({page},info)=>{
  test.setTimeout(90000);const site=new WebsitePage(page),records:unknown[]=[];
  let configurationPath:string|null=null;
  await mountOriginalSite(page);
  await site.setViewport(width,width===1440?900:844);
  for(const [target,source,selector]of pages){
   const response=await site.open(originalOrigin+'/'+source);expect(response?.ok()).toBeTruthy();
   expect(response!.headers()['content-security-policy']).toContain("script-src 'none'");
   await expect(page.locator(selector).first()).toBeAttached();
   const original=await site.layout();expect(original.width).toBe(width);
   const originalShot=await site.capture('source-'+width+'-'+target,false);
   await site.inspectFullPage();const originalFullShot=await site.capture('source-full-'+width+'-'+target);
   // A source-site product identifier is never reused for our configuration page.
   // Resolve the actual path from our existing backend-rendered catalog instead.
   const targetPath=target==='configure.html'?configurationPath:'/'+target;
   expect(targetPath,'existing available product required for valid configuration comparison').toBeTruthy();
   await site.open(targetPath!);await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|正在读取/);
   await expect(page.locator('[data-brand-name]').first()).not.toHaveText('');
   if(target==='products.html')configurationPath=await site.availableConfigurationPath();
   if(target==='configure.html')await expect(page.locator('[data-quote] button').first()).toBeVisible();
   await expect(page.locator(selector).first()).toBeAttached();
   const migrated=await site.layout();expect(migrated.width).toBe(width);
   expect(migrated.scrollWidth,target+' horizontal overflow').toBeLessThanOrEqual(width);
   const migratedShot=await site.capture('migrated-'+width+'-'+target,false);
   await site.inspectFullPage();
   records.push({target,targetPath,source,selector,original,migrated,originalShot,originalFullShot,migratedShot,migratedFullShot:await site.capture('migrated-full-'+width+'-'+target)});
  }
  await info.attach('source-comparison',{body:JSON.stringify({width,records,limitation:'原容器、同视口及滚动整页截图；配置使用本方既有商品，内容页无有效正文。原页脚本不执行，动画与完整像素保真仍须分别验收。'},null,2),contentType:'application/json'});
 });
});
