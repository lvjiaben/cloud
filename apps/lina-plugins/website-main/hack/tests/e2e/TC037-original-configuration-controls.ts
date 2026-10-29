import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ConfigurationControlsPage} from '../pages/ConfigurationControlsPage';
import type {Page} from '@playwright/test';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
const root=prefix+'/catalog/products/controlled-controls',revision='c'.repeat(43);
const long='扩展规格与较长的选项名称用于验证自动换行及完整可读性，所有文字仅为受控测试标签，不包含真实商品承诺；标签继续延长用于覆盖超过单行宽度时的自然换行显示';
const definitions=[
 {fieldToken:'spec',type:'single_select',labelZhCN:'规格',required:true,options:[{optionToken:'opaque-small',labelZhCN:'标准规格'},{optionToken:'opaque-large',labelZhCN:long}]},
 {fieldToken:'extra',type:'multi_select',labelZhCN:'可选配置',options:[{optionToken:'opaque-one',labelZhCN:'选项甲'},{optionToken:'opaque-two',labelZhCN:'选项乙'}]},
 {fieldToken:'count',type:'integer',labelZhCN:'整数配置',minimumInteger:'1',maximumInteger:'8'},
 {fieldToken:'exact',type:'decimal',labelZhCN:'精确小数',minimumDecimal:'0.1',maximumDecimal:'8.5'},
 {fieldToken:'unbounded',type:'integer',labelZhCN:'无上限整数'},
 {fieldToken:'unsafe',type:'integer',labelZhCN:'大整数',minimumInteger:'1',maximumInteger:'9007199254740993'},
 {fieldToken:'blank',type:'integer',labelZhCN:'未选整数',minimumInteger:'1',maximumInteger:'8'},
 {fieldToken:'tree',type:'tree_select',labelZhCN:'层级规格',options:[{optionToken:'parent',labelZhCN:'父选项',children:[{optionToken:'child',labelZhCN:'子选项'}]}]},
 {fieldToken:'disabled',type:'single_select',labelZhCN:'禁用配置',disabled:true,options:[{optionToken:'blocked',labelZhCN:'不可选项'}]},
 {fieldToken:'disabled-int',type:'integer',labelZhCN:'禁用整数',disabled:true,minimumInteger:'1',maximumInteger:'8'}
].map(f=>({...f,visible:true}));
const initial=[{fieldToken:'spec',type:'single_select',optionTokens:['opaque-small']},{fieldToken:'count',type:'integer',integerValue:'3'},{fieldToken:'disabled-int',type:'integer',integerValue:'2'}];
async function setup(page:Page,modelFailure=false){
 const fixture=await controlledBusiness(page),state={quoteFailure:false,modelFailure,latest:[] as any[],evaluations:0,quotes:0};
 await page.route('**'+prefix+'/catalog/products?*',r=>{
  const url=new URL(r.request().url());if(!url.searchParams.get('categoryNo'))return r.fallback();
  expect(url.searchParams.get('categoryNo')).toBe('cat-a');expect(url.searchParams.get('limit')).toBe('12');
  return state.modelFailure?r.fulfill({status:503,json:{code:50,data:null}}):r.fulfill({json:{code:0,data:{products:[{publicSlug:'controlled-sibling',nameZhCN:'同分类第二型号',categoryNo:'cat-a',availability:'available'},{publicSlug:'controlled-sold-out',nameZhCN:'同分类缺货型号',categoryNo:'cat-a',availability:'unavailable'},{publicSlug:'other',nameZhCN:'不应出现的其他分类型号',categoryNo:'cat-b',availability:'available'}],nextCursor:''}}});
 });
 await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-controls',nameZhCN:'受控配置控件测试',categoryNo:'cat-a',availability:'available',currencies:['CNY']}}}}));
 await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:definitions,values:initial}}}));
 await page.route('**'+root+'/purchase-configuration/evaluate',r=>{state.evaluations++;state.latest=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:definitions,values:state.latest,valid:state.latest.some(v=>v.fieldToken==='spec'&&v.optionTokens?.length)}}});});
 await page.route('**'+root+'/purchase-configuration/quote',r=>{state.quotes++;return state.quoteFailure?r.fulfill({status:503,json:{code:50,data:null}}):r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'governed-month',labelZhCN:'测试月付',minimumPurchaseCycles:2,status:'final',totalAmount:'1234'},{billingTermToken:'governed-long',labelZhCN:'测试较长计费周期标签保留服务端返回文字',minimumPurchaseCycles:3,status:'final',totalAmount:'3456'}]}}});});
 const site=new ConfigurationControlsPage(page);await site.open('/configure.html?product=controlled-controls');await expect(page.locator('[data-quote] button')).toHaveCount(2);
 return {site,state,fixture};
}
test.describe('TC-37 原配置按钮与整数滑块（隔离受控数据）',()=>{
 test('TC-37a: 规格键盘选中、精确令牌、多选及必填空值保留',async({page})=>{
  const {site,state}=await setup(page);await expect(site.options('规格').getByRole('button')).toHaveCount(2);await site.captureControls('tc037-initial');
  await site.option('规格',long,'Space');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='spec')?.optionTokens).toEqual(['opaque-large']);await expect(site.options('规格').getByRole('button',{name:long,exact:true})).toHaveAttribute('aria-pressed','true');
  await expect(site.field('spec')).toHaveValue('opaque-large');await expect(page.locator('#field-spec-choice-1')).toBeFocused();
  expect(await site.options('规格').getByRole('button',{name:long,exact:true}).evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);expect((await site.options('规格').getByRole('button',{name:long,exact:true}).boundingBox())!.height).toBeGreaterThan(40);await site.captureControls('tc037-long-selected');
  await expect.poll(()=>state.latest.find(v=>v.fieldToken==='extra')?.optionTokens).toEqual(['opaque-one']);await site.option('可选配置','选项乙');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='extra')?.optionTokens).toEqual(['opaque-one','opaque-two']);
  await site.option('可选配置','选项甲');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='extra')?.optionTokens).toEqual(['opaque-two']);
  await expect(site.options('规格').getByRole('button',{name:'请选择',exact:true})).toHaveCount(0);await expect(site.field('spec').locator('option[value=""]')).toHaveCount(0);
  await expect(site.options('禁用配置').getByRole('button',{name:'不可选项'})).toBeDisabled();await expect(site.field('tree')).toBeVisible();await expect(site.field('tree').locator('option')).toHaveCount(2);
  await site.option('规格','标准规格');await expect(page.locator('[data-quote] button')).toHaveCount(2);expect(state.latest.some(v=>v.fieldToken==='disabled')).toBe(false);
 });
 test('TC-37b: 整数滑块双向同步、端点与不猜步长的回退',async({page})=>{
  const {site,state,fixture}=await setup(page);await expect(page.getByRole('slider')).toHaveCount(3);await expect(site.slider('禁用整数')).toBeDisabled();await expect(page.getByRole('button',{name:'禁用整数增加',exact:true})).toBeDisabled();
  await expect(site.field('exact')).toHaveAttribute('step','any');for(const token of ['exact','unbounded','unsafe'])await expect(site.field(token).locator('..').getByRole('slider')).toHaveCount(0);
  await site.slide('整数配置','ArrowRight');await expect(site.field('count')).toHaveValue('4');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('4');await expect(site.slider('整数配置')).toBeFocused();
  await site.value('count','6');await expect(site.slider('整数配置')).toHaveValue('6');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('6');await site.adjust('整数配置','增加');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('7');
  await site.slide('整数配置','End');await expect(site.field('count')).toHaveValue('8');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('8');await site.adjust('整数配置','增加');await expect(site.field('count')).toHaveValue('8');
  await site.slide('整数配置','Home');await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('1');await site.adjust('整数配置','减少');await expect(site.field('count')).toHaveValue('1');
  await site.value('count','9');expect(await site.field('count').evaluate((el:HTMLInputElement)=>el.validity.rangeOverflow)).toBe(true);await expect.poll(()=>state.latest.find(v=>v.fieldToken==='count')?.integerValue).toBe('9');await site.chooseTerm();await site.repeatCartSubmit();expect(fixture.requests.filter(r=>r.method!=='GET')).toEqual([]);await site.captureControls('tc037-integer-overflow');
  await site.value('count','5');await expect(page.locator('[data-quote] button')).toHaveCount(2);await expect(site.slider('整数配置')).toHaveValue('5');
 });
 test('TC-37c: 改值清旧报价、报价失败恢复与权威计费周期',async({page})=>{
  const {site,state,fixture}=await setup(page);await site.term(1);await expect(page.locator('[data-quote] button').nth(1)).toHaveAttribute('aria-pressed','true');await expect(page.locator('[name=cycles]')).toHaveValue('3');await expect(page.locator('[name=quantity]')).toHaveValue('1');await expect(page.locator('[data-add]')).toBeEnabled();
  state.quoteFailure=true;await site.option('规格',long);await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-quote]')).toBeEmpty();await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。');await expect(page.locator('[data-total-amount]')).toHaveText('待报价');await site.captureControls('tc037-quote-failure','quote');
  state.quoteFailure=false;await site.refreshConfiguration();await expect(page.locator('[data-quote] button')).toHaveCount(2);await expect(page.locator('[data-add]')).toBeDisabled();await site.term(0);await expect(page.locator('[data-quote] button').first()).toHaveAttribute('aria-pressed','true');await expect(page.locator('[data-quote] button').nth(1)).toHaveAttribute('aria-pressed','false');await expect(page.locator('[data-quote] button').first()).toHaveText('测试月付：CNY 12.34');await expect(page.locator('[data-add]')).toBeEnabled();await site.captureControls('tc037-billing-recovered','quote');
  expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
 });
 test('TC-37d: 二级服务器列表失败独立恢复，当前配置仍可报价',async({page})=>{
  const {site,state}=await setup(page,true);
  await expect(site.models()).toHaveAttribute('aria-label','当前商品型号');
  await expect(site.models().getByRole('link')).toHaveCount(1);
  await expect(site.models().getByRole('link')).toHaveAttribute('aria-current','page');
  const menu=page.locator('.shopping-cart-side [data-categories]');
  await expect(menu.locator('.lina-server-menu-feedback').filter({hasText:'服务器列表加载失败，请重试。'})).toHaveCount(1);
  await site.term(0);await expect(page.locator('[data-add]')).toBeEnabled();
  await expect(site.selectedPrice()).toHaveText('¥12.34');await site.captureControls('tc037-model-failure','quote');
  state.modelFailure=false;await site.retryModels();
  await expect(menu.locator('[data-catalog-product=controlled-sibling]')).toHaveText('同分类第二型号');
  await expect(menu.locator('[data-catalog-product=controlled-sold-out]')).toHaveText('同分类缺货型号');
  await expect(menu).not.toContainText('不应出现的其他分类型号');
  await expect(site.models().getByRole('link')).toHaveCount(1);
  await site.option('规格',long);await expect(site.selectedPrice()).toHaveText('待报价');await expect(page.locator('[data-add]')).toBeDisabled();
  await expect(page.locator('[data-quote] button')).toHaveCount(2);await site.term(1);await expect(site.selectedPrice()).toHaveText('¥34.56');
  await site.captureControls('tc037-model-recovered','quote');
 });
});
