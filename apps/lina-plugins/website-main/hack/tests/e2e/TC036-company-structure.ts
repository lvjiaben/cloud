import {test,expect} from '../support/source-site';
import {ContactMediaPage} from '../pages/ContactMediaPage';
import {resolve} from 'node:path';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
const directory=resolve(import.meta.dirname,'../../../../../../temp/20261018/assistant-content-pages');
const brand={siteName:{zhCN:'受控验收 · 公司页面'},navigation:[],footerNavigation:[],footer:{zhCN:'仅浏览器模拟，不是经营资料'}};

test.describe('TC-36 公司页面结构保留与资料边界',()=>{
 for(const [kind,index] of [['about',0],['contact',1]] as const){
  for(const [state,offset] of [['filled',0],['empty',1],['failure',2]] as const){
   test('TC-36'+String.fromCharCode(97+index*3+offset)+': '+kind+' '+state,async({page},info)=>{
    const site=new ContactMediaPage(page),requests:string[]=[];
    let failed=state==='failure';
    await page.route('**/x/**',async route=>{
     requests.push(route.request().method()+' '+new URL(route.request().url()).pathname);
     expect(route.request().method()).toBe('GET');
     expect(new URL(route.request().url()).pathname).toBe('/x/idc-portal/api/v1/public/brand');
     await route.fulfill(failed?{status:503,json:{code:50,data:null}}:{json:{code:0,data:{brand:{...brand,...(state==='filled'?{contactPhone:'000-0000-0000',contactEmail:'audit@example.invalid'}:{})}}}});
    });
    await site.open('/'+kind+'.html');
    const expected=kind==='about'?3:4;
    await expect(page.locator('.et-main .et-slide')).toHaveCount(expected);
    await expect(page.locator('.et-hero-tab')).toHaveCount(expected);
    await expect(page.locator('.et-container')).toBeHidden();
    await expect(page.locator('.globalBanner')).toBeHidden();
    for(let i=1;i<expected;i++)await expect(page.locator('.et-slide').nth(i)).toBeHidden();
    await expect(page.locator('.et-main')).not.toContainText(/白猿|小白云|400-8876675|1531335070|7X24|全球加速节点|无锡|NGINX/);
    await expect(page.locator('.et-main [id],.et-main [srcset],.et-main [style],.et-main [onclick],.globalBanner [href],.et-container [href]')).toHaveCount(0);
    expect(await site.mediaViolations(kind)).toEqual([]);
    if(kind==='about'){
     await expect(page.locator('.companyIntroduction-left')).toHaveCount(1);
     await expect(page.locator('.companyIntroduction-left')).toBeHidden();
     await expect(page.locator('.companyIntroduction-slogn li')).toHaveCount(4);
     await expect(page.locator('.culture-Content')).toHaveCount(1);
     await expect(page.locator('.programme-content-item')).toHaveCount(4);
     await expect(page.locator('.et-main a[href]')).toHaveCount(0);
    }else{
     await expect(page.locator('.currency-content-item')).toHaveCount(12);
     await expect(page.locator('.companyAddress')).toHaveCount(1);
     await expect(page.locator('.frequentlyAskedQuestions .problem-news-title')).toHaveCount(5);
     await expect(page.locator('[data-content-contact]')).toHaveCount(2);
     await expect(page.locator('.et-slide:not(:first-child) a[href]')).toHaveCount(0);
    }
    if(failed){
     await expect(page.locator('[data-status]')).toHaveText('网站资料暂时无法加载，请重试。');
     if(kind==='about')await expect(page.locator('[data-content-body]')).toBeEmpty();
     else {await expect(page.locator('[data-content-contact]:visible')).toHaveCount(0);await expect(page.locator('.et-main img:visible')).toHaveCount(0);}
     await page.screenshot({path:resolve(directory,Date.now()+'-'+kind+'-failure.png'),fullPage:true});
     failed=false;await site.retryBrand();
    }
    await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN);
    const title=page.locator(kind==='about'?'.companyIntroduction-right-title':'.et-main .currency-title-a').first();
    const header=await page.locator('.nav-layoutcontainer').boundingBox();
    expect((await title.boundingBox())!.y).toBeGreaterThanOrEqual(header!.y+header!.height);
    await expect(page.locator('.et-main img:visible')).toHaveCount(kind==='contact'&&state==='filled'?2:0);
    if(kind==='about'){
     await expect(page.locator('[data-content-body]')).toHaveText(brand.siteName.zhCN);
     await expect(page.locator('[data-status]')).toHaveText('公司介绍、资质和服务承诺尚无公开资料。');
    }else if(state==='filled'){
     await expect(page.locator('[data-content-contact=phone]')).toBeVisible();
     await expect(page.locator('[data-content-contact=phone] .label')).toHaveText('000-0000-0000');
     await expect(page.locator('[data-content-contact=phone] a')).toHaveAttribute('href','tel:000-0000-0000');
     await expect(page.locator('[data-content-contact=email] a')).toHaveAttribute('href','mailto:audit@example.invalid');
     await expect(page.locator('.currency-content-item:visible')).toHaveCount(2);
     await expect.poll(()=>site.imageStates()).toEqual([{src:'static/picture/telephone.svg',alt:'',ariaHidden:'true',decoded:true},{src:'static/picture/mailbox.svg',alt:'',ariaHidden:'true',decoded:true}]);
    }else{
     await expect(page.locator('[data-content-contact]:visible')).toHaveCount(0);
     await expect(page.locator('[data-status]')).toHaveText('暂未公布联系方式。');
    }
    await expect(page.locator('body')).not.toContainText(/plugin\.[\w.]+/);
    expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
    const screenshot=resolve(directory,Date.now()+'-'+kind+'-'+state+'.png');
    await page.screenshot({path:screenshot,fullPage:true,animations:'disabled'});
    await info.attach('company-page-evidence',{body:JSON.stringify({screenshot,requests,source:'当前本地源码，接口仅浏览器受控响应，未安装'}),contentType:'application/json'});
    expect(requests).toHaveLength(state==='failure'?(kind==='about'?3:2):1);
   });
  }
 }
 test('TC-36g: 中性联系图精确允许来源',async({page})=>{
  const site=new ContactMediaPage(page);
  await page.route('**/x/**',route=>{expect(route.request().method()).toBe('GET');expect(new URL(route.request().url()).pathname).toBe('/x/idc-portal/api/v1/public/brand');return route.fulfill({json:{code:0,data:{brand}}});});
  await site.open('/contact.html');
  await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN);
  await expect(page.locator('[data-status]')).toHaveText('暂未公布联系方式。');
  expect(await site.mediaViolations('contact')).toEqual([]);
  await expect(page.locator('[data-content-contact]:visible')).toHaveCount(0);
 });
 test('TC-36h: 拒绝其它来源、错卡与额外图片',async({page})=>{
  const site=new ContactMediaPage(page);
  await page.route('**/x/**',route=>{expect(route.request().method()).toBe('GET');expect(new URL(route.request().url()).pathname).toBe('/x/idc-portal/api/v1/public/brand');return route.fulfill({json:{code:0,data:{brand}}});});
  await site.open('/contact.html');
  await expect(page.locator('[data-brand-name]').first()).toHaveText(brand.siteName.zhCN);
  await expect(page.locator('[data-status]')).toHaveText('暂未公布联系方式。');
  for(const probe of ['https://www.xiaobaiyun.cn/static/picture/telephone.svg','/static/picture/telephone.svg','static/picture/telephone.svg?brand=old','data:image/svg+xml,<svg/>','static/picture/customer.svg','wrong-card','extra']){
   expect(await site.probeRejectedMedia(probe),probe).not.toEqual([]);
   expect(await site.mediaViolations('contact'),'恢复隔离DOM').toEqual([]);
  }
 });

});
