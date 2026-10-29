import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {SettlementPage} from '../pages/SettlementPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-69 固定底部结算栏',()=>{
 test('TC-69a: 页面首中尾固定、明细向上展开、窄屏与报价失败',async({page},info)=>{
  await controlledBusiness(page);const root=prefix+'/catalog/products/controlled-alpha',revision='c'.repeat(43);
  await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-alpha',nameZhCN:'固定结算测试',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:[],values:[]}}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:[],values:[],valid:true}}}));
  let failure=false;
  await page.route('**'+root+'/purchase-configuration/quote',r=>failure?r.fulfill({status:503,json:{code:503,message:'报价服务暂不可用'}}):r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'月付',minimumPurchaseCycles:1,status:'final',subtotalAmount:'6900',setupAmount:'0',taxAmount:'0',totalAmount:'6900'}]}}}));
  const site=new SettlementPage(page);await site.open('/configure.html?product=controlled-alpha');await site.chooseTerm();
  await expect(page.locator('[data-total-amount]')).toHaveText('¥69.00');
  for(const fraction of [0,.5,1]){await site.scrollTo(fraction);await expect.poll(async()=>{const g=await site.geometry();return Math.abs(g.bottom-g.viewport);}).toBeLessThan(2);await expect(page.locator('[data-add]')).toBeInViewport();}
  await site.toggleDetails();await expect(page.locator('[data-breakdown]')).toContainText('应付合计');
  const detail=await page.locator('[data-breakdown]').boundingBox(),bar=await page.locator('.allocation-footer').boundingBox();expect(detail!.y+detail!.height).toBeLessThanOrEqual(bar!.y+1);
  await site.captureContent(info.outputPath(Date.now()+'-desktop-settlement.png'),0);await site.toggleDetails();
  await site.setViewport(390,844);await site.scrollTo(.5);await expect.poll(async()=>{const g=await site.geometry();return Math.abs(g.bottom-g.viewport);}).toBeLessThan(2);const mobile=await site.geometry();expect(mobile.left).toBe(0);expect(mobile.right).toBeLessThanOrEqual(mobile.width);await expect(page.locator('[data-add]')).toBeInViewport();
  await site.toggleDetails();await expect(page.locator('[data-breakdown]')).toBeInViewport();await site.captureContent(info.outputPath(Date.now()+'-mobile-settlement.png'),0);
  failure=true;await site.refreshConfiguration();await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-total-summary]')).toBeVisible();await expect(page.locator('[data-total-amount]')).toHaveText('待报价');await expect(page.locator('[data-breakdown-toggle]')).toBeDisabled();await expect(page.locator('[data-breakdown]')).toBeHidden();
 });
 test('TC-69b: 任意新增服务器共用费用栏，未选配置仍可见，报价随商品和规格变化',async({page},info)=>{
  await controlledBusiness(page);
  const revision='d'.repeat(43),fields=[{fieldToken:'spec',type:'single_select',visible:true,required:true,labelZhCN:'规格',options:[{optionToken:'small',labelZhCN:'标准'},{optionToken:'large',labelZhCN:'增强'}]}];
  const slugs=['new-server-'+Date.now(),'another-server-'+Date.now()];
  for(const [index,slug] of slugs.entries()){
   const root=prefix+'/catalog/products/'+slug;let selected=false,enhanced=false;
   await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:slug,nameZhCN:'任意新增服务器'+index,availability:'available',currencies:['CNY']}}}}));
   await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values:[]}}}));
   await page.route('**'+root+'/purchase-configuration/evaluate',r=>{const values=r.request().postDataJSON().values||[];selected=!!values[0]?.optionTokens?.length;enhanced=values[0]?.optionTokens?.[0]==='large';return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values,valid:selected}}});});
   await page.route('**'+root+'/purchase-configuration/quote',r=>{const amount=String((index+1)*10000+(enhanced?5000:0));return r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'月付',minimumPurchaseCycles:1,status:'final',subtotalAmount:amount,setupAmount:'0',taxAmount:'0',totalAmount:amount}]}}});});
   const site=new SettlementPage(page);await site.open('/configure.html?product='+slug);
   await expect(page.locator('#field-spec')).toHaveValue('small');await expect(page.locator('[data-total-summary]')).toBeVisible();await expect(page.locator('[data-total-amount]')).toHaveText('¥'+((index+1)*100).toFixed(2));await expect(page.locator('[data-add]')).toBeDisabled();
   await site.scrollTo(.5);const geometry=await site.geometry();expect(Math.abs(geometry.bottom-geometry.viewport)).toBeLessThan(2);
   await site.selectConfiguration('small');await site.chooseTerm();await expect(page.locator('[data-total-amount]')).toHaveText('¥'+((index+1)*100).toFixed(2));await expect(page.locator('[data-add]')).toBeEnabled();
   await site.selectConfiguration('large');await site.chooseTerm();await expect(page.locator('[data-total-amount]')).toHaveText('¥'+((index+1)*100+50).toFixed(2));await site.toggleDetails();await expect(page.locator('[data-breakdown]')).toContainText('应付合计');
   await site.selectConfiguration('small');await expect(page.locator('[data-total-amount]')).toHaveText('¥'+((index+1)*100).toFixed(2));await expect(page.locator('[data-breakdown]')).toBeHidden();await expect(page.locator('[data-add]')).toBeDisabled();
   await site.captureContent(info.outputPath(Date.now()+'-new-server-pending.png'),0);
  }
 });
 test('TC-69c: 窄屏和桌面规格滚动都限制在配置区，左侧分类及页面底部保持固定',async({page},info)=>{
  await controlledBusiness(page);
  const slug='scroll-boundary-'+Date.now(),root=prefix+'/catalog/products/'+slug,revision='e'.repeat(43);
  const fields=Array.from({length:18},(_,index)=>({fieldToken:index===0?'spec':'field-'+index,type:'single_select',visible:true,required:true,labelZhCN:'规格项目'+index,options:[{optionToken:'first',labelZhCN:'首项'},{optionToken:'second',labelZhCN:'次项'}]}));
  await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:slug,nameZhCN:'滚动边界测试',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values:[]}}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values:r.request().postDataJSON().values,valid:true}}}));
  await page.route('**'+root+'/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'月付',minimumPurchaseCycles:1,status:'final',subtotalAmount:'6900',setupAmount:'0',taxAmount:'0',totalAmount:'6900'}]}}}));
  await page.goto('/configure.html?product='+slug);
  for(const width of [1615,1440,1920,642]){
   await page.setViewportSize({width,height:900});
   const main=page.locator('.allocation-mian'),side=page.locator('.shopping-cart-side'),menu=page.locator('.shopping-cart-side .sky-cart-menu'),footer=page.locator('.allocation-footer');
   await expect.poll(()=>main.evaluate(el=>el.scrollHeight>el.clientHeight)).toBe(true);
   const before=await side.boundingBox();
   await page.evaluate(()=>{const e=document.querySelector('.allocation-mian')!;e.scrollTop=e.scrollHeight;window.scrollTo(0,document.documentElement.scrollHeight);});
   await expect.poll(()=>main.evaluate(el=>el.scrollTop)).toBeGreaterThan(0);
   await expect.poll(()=>page.evaluate(()=>document.scrollingElement?.scrollTop??0)).toBe(0);
   const after=await side.boundingBox(),bar=await footer.boundingBox();
   expect(after!.y).toBe(before!.y);expect(Math.abs(bar!.y+bar!.height-900)).toBeLessThan(2);expect(await menu.evaluate(el=>getComputedStyle(el).overflowY)).toBe('auto');
   await expect.poll(async()=>{const g=await siteGeometry();return Math.abs(g.left-(width<768?0:g.sidebarRight));}).toBeLessThan(1);
   const geometry=await siteGeometry();expect(Math.abs(geometry.right-geometry.width)).toBeLessThan(1);expect(geometry.position).toBe('fixed');
   await page.screenshot({path:info.outputPath('scroll-'+width+'-bottom.png'),fullPage:false,animations:'disabled'});
  }
  async function siteGeometry(){return new SettlementPage(page).geometry();}
 });
});
