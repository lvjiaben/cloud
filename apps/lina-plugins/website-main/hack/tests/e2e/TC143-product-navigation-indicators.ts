import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ActivationIndicatorPage} from '../pages/ActivationIndicatorPage';

test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
const names=['弹性云','云服务器演示','计算型服务器演示','高防服务器演示'];
const categories=names.map((name,i)=>({categoryNo:'indicator-'+i,nameZhCN:name}));
const screenshotName=(label:string)=>`${new Date().toISOString().replace(/[-:]/g,'').replace('T','-').slice(0,15)}-${Date.now()}-${label}.png`;
test.describe('TC-143 产品分类指示线与异步刷新',()=>{
 test('TC-143d: 当前选中插件在减少动态下仍播放分类下划线',async({page},info)=>{
  await page.emulateMedia({reducedMotion:'reduce'});
  const response=await page.request.get('/x/idc-portal/api/v1/public/brand');expect(response.ok()).toBe(true);
  const brand=(await response.json()).data.brand;expect(brand.templateEntry.assetBase).toMatch(/^\/x-assets\/website-main\/v[^/]+\/$/);
  const site=new ActivationIndicatorPage(page);await site.open(brand.templateEntry.url);await site.clickTitle('产品与服务');
  const first=site.category(names[0]);await expect(first).toHaveAttribute('aria-pressed','true');
  const opening=await site.sampleReplay(first,'::after',true);expect(opening,'实际安装插件的默认分类应播放动画').not.toBeNull();expect(opening!.middle).toBeGreaterThan(0);expect(opening!.middle).toBeLessThan(1);await site.captureIndicator(info.outputPath(screenshotName('published-opening-midpoint')));await site.resumeReplay(first);
  for(const name of names){await site.enterPanel();const button=site.category(name);await site.hover(button);const replay=await site.sampleReplay(button,'::after',true);expect(replay).not.toBeNull();expect(replay!.duration).toBe(500);expect(replay!.middle).toBeGreaterThan(0);expect(replay!.middle).toBeLessThan(1);expect(await site.categoryLineAnimations(button)).toHaveLength(1);await expect(button).toHaveCSS('border-bottom-color','rgba(0, 0, 0, 0)');await site.resumeReplay(button);}
 });
 test('TC-143c: 实际首页默认分类重复移入均展开，选中保留及减少动态',async({page},info)=>{
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  const site=new ActivationIndicatorPage(page);await site.open('/');await site.clickTitle('产品与服务');
  const selected=site.category('弹性云');await expect(selected).toHaveAttribute('aria-pressed','true');
  const opening=await site.sampleReplay(selected,'::after',true);expect(opening,'打开菜单时默认分类也应展开').not.toBeNull();expect(opening!.middle).toBeGreaterThan(0);expect(opening!.middle).toBeLessThan(1);await site.captureIndicator(info.outputPath(screenshotName('category-opening-midpoint')));await site.resumeReplay(selected);await site.enterPanel();
  for(let pass=0;pass<2;pass++){
   await expect.poll(async()=>Math.round((await site.measure(selected)).width)).toBe(Math.round((await site.measure(selected)).full));
   await site.hover(selected);const sample=await site.sampleReplay(selected,'::after',true);expect(sample).not.toBeNull();expect(sample!.duration).toBe(500);expect(sample!.middle).toBeGreaterThan(0);expect(sample!.middle).toBeLessThan(1);
   await expect(selected).toHaveCSS('border-bottom-color','rgba(0, 0, 0, 0)');await site.captureIndicator(info.outputPath(screenshotName('default-category-midpoint-'+pass)));await site.resumeReplay(selected);await site.moveOffCategory();
   await expect(selected).toHaveAttribute('aria-pressed','true');
  }
  await site.keyboard('Tab');await site.focus(selected);await expect(selected).toBeFocused();const focused=await site.sampleReplay(selected);expect(focused).not.toBeNull();expect(focused!.duration).toBe(500);
  await site.reduce();await site.clickTitle('产品与服务');await site.enterPanel();await site.hover(selected);
  const reduced=await site.sampleReplay(selected,'::after',true);expect(reduced,'用户减少动态开启时仍应播放指定分类线动画').not.toBeNull();expect(reduced!.duration).toBe(500);expect(reduced!.middle).toBeGreaterThan(0);expect(reduced!.middle).toBeLessThan(1);expect((await site.lineState()).transition).toBe('0s');await site.captureIndicator(info.outputPath(screenshotName('default-category-reduced')));await site.resumeReplay(selected);expect(errors).toEqual([]);
 });
 for(const mobile of [false,true])test(`TC-143${mobile?'b':'a'}: 四个分类${mobile?'点击':'悬停'}动画和接口刷新保留按钮`,async({page},info)=>{
  const liveBrand=await page.request.get('/x/idc-portal/api/v1/public/brand');expect(liveBrand.ok()).toBe(true);const liveBrandData=await liveBrand.json();const assetBase=liveBrandData.data?.brand?.templateEntry?.assetBase;expect(assetBase).toMatch(/^\/x-assets\/website-main\/v[^/]+\/$/);const fixture=await controlledBusiness(page);fixture.state.brand.siteName={};await page.setViewportSize({width:mobile?390:1440,height:900});
  let fail=false;
  await page.route('**'+prefix+'/catalog/products?*',async route=>{
   const category=new URL(route.request().url()).searchParams.get('categoryNo');
   if(fail&&category==='indicator-2')return route.fulfill({status:503,json:{code:50}});
   const i=categories.findIndex(c=>c.categoryNo===category);
   return route.fulfill({json:{code:0,data:{categories,products:i<0?[]:[{categoryNo:category,publicSlug:'test-indicator-'+i,nameZhCN:'分类测试商品'+i,summaryZhCN:'仅自动化测试使用'}],nextCursor:''}}});
  });
  const site=new ActivationIndicatorPage(page);await site.open('/');if(mobile)await site.toggleMenu();await site.clickTitle('产品与服务');await expect(site.category(names[0])).toBeVisible();await site.captureIndicator(info.outputPath(screenshotName('category-menu-open')));
  for(const i of [1,0,2,3]){
   const button=site.category(names[i]),handle=await button.elementHandle();
   if(mobile)await site.click(button);else{await site.enterPanel();await site.hover(button);}
   await expect(button).toHaveAttribute('aria-pressed','true');const transition=await site.measure(button);expect(transition.animation).toMatch(/^lina-category-/);expect(transition.duration).toBe('0s');const replay=await site.sampleReplay(button,'::after',i===1);expect(replay).not.toBeNull();expect(replay!.duration).toBe(500);expect(replay!.middle).toBeGreaterThan(0);expect(replay!.middle).toBeLessThan(1);const lineAnimations=await site.categoryLineAnimations(button);expect(lineAnimations).toHaveLength(1);await expect(button).toHaveCSS('border-bottom-color','rgba(0, 0, 0, 0)');if(i===1){await site.captureIndicator(info.outputPath(screenshotName('category-animation-midpoint')));await site.resumeReplay(button);}
   await expect(button).toHaveAttribute('aria-pressed','true');await expect(site.panel().locator('.Submenu-middle-content .Submenu-title')).toHaveText(['分类测试商品'+i]);expect(await handle!.evaluate(e=>e.isConnected)).toBe(true);
   await expect.poll(async()=>Math.round((await site.measure(button)).width)).toBe(Math.round((await site.measure(button)).full));
  }
  await site.captureIndicator(info.outputPath(screenshotName('four-categories')));
  const selected=site.category(names[3]);if(!mobile)await site.moveOffCategory();const selectedLine=await site.measure(selected);expect(Math.round(selectedLine.width)).toBe(Math.round(selectedLine.full));expect(selectedLine.color).toBe(await selected.evaluate(e=>getComputedStyle(e).color));
  await page.emulateMedia({reducedMotion:'reduce'});await expect.poll(async()=>Math.round((await site.measure(selected)).width)).toBe(Math.round((await site.measure(selected)).full));expect(await selected.evaluate(e=>getComputedStyle(e,'::after').transitionDuration)).toBe('0s');await site.captureIndicator(info.outputPath(screenshotName('reduced-motion-selected-line')));await page.emulateMedia({reducedMotion:'no-preference'});
  await expect.poll(async()=>Math.round((await site.measure(site.category(names[0]))).width)).toBe(0);await site.captureIndicator(info.outputPath(screenshotName('selected-line-after-pointer-leave')));
  fail=true;if(mobile)await site.click(site.category(names[2]));else await site.hover(site.category(names[2]));await expect(site.panel()).toContainText('商品加载失败，请重试。');await site.captureIndicator(info.outputPath(screenshotName('category-load-failed')));fail=false;await site.retryNavigation();await expect(site.panel().locator('.Submenu-middle-content .Submenu-title')).toHaveText(['分类测试商品2']);await expect(site.category(names[2])).toHaveAttribute('aria-pressed','true');
 });
});
