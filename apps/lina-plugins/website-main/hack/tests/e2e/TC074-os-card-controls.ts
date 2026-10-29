import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ConfigurationControlsPage} from '../pages/ConfigurationControlsPage';
import type {Page} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve,extname} from 'node:path';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},headless:true});
const root=prefix+'/catalog/products/os-controls',revision='o'.repeat(43);
const os=[
 {optionToken:'alma-9',labelZhCN:'AlmaLinux 9.2'},
 {optionToken:'centos-9',labelZhCN:'CentOS-9-Stream-x64'},
 {optionToken:'centos-8',labelZhCN:'CentOS-8-Stream-x64'},
 {optionToken:'ubuntu-22',labelZhCN:'Ubuntu-22.04-x64'},
 {optionToken:'ubuntu-24',labelZhCN:'Ubuntu-24.04.1-x64'},
 {optionToken:'windows-2022',labelZhCN:'Windows-2022-Datacenter-cn'},
 {optionToken:'debian-12',labelZhCN:'Debian-12.0_x64'},
 {optionToken:'rocky-9',labelZhCN:'Rocky-linux-9.2-x64'},
 {optionToken:'rocky-9b',labelZhCN:'Rocky Linux 9.3'},
];
const fields=[{fieldToken:'os',type:'single_select',labelZhCN:'操作系统',required:true,visible:true,disabled:false,options:os},{fieldToken:'cores',type:'integer',labelZhCN:'处理器核心数',minimumInteger:'2',maximumInteger:'16',visible:true,disabled:false}];
async function setup(page:Page){
 const business=await controlledBusiness(page),state={evaluated:[] as any[],quoted:[] as string[],quoteFailure:false};
 const publicRoot=resolve(import.meta.dirname,'../../../frontend/public'),assetBase='/x-assets/website-main/v-test/';
 await page.route('**'+assetBase+'**',async route=>{
  const pathname=new URL(route.request().url()).pathname;
  const file=resolve(publicRoot,pathname.slice(assetBase.length));
  const mime:Record<string,string>={'.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf'};
  try{return await route.fulfill({contentType:mime[extname(file)]||'application/octet-stream',body:await readFile(file)});}catch{return await route.fulfill({status:404,body:'Missing versioned test asset'});}
 });
 await page.route('**/configure.html?product=os-controls',async route=>{
  const html=await readFile(resolve(publicRoot,'configure.html'),'utf8');
  await route.fulfill({contentType:'text/html',body:html.replace(/(<head[^>]*>)/i,'$1<base href="'+assetBase+'">')});
 });
 await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'os-controls',nameZhCN:'受控系统卡测试',categoryNo:'cat-a',availability:'available',currencies:['CNY']}}}}));
 await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values:[]}}}));
 await page.route('**'+root+'/purchase-configuration/evaluate',r=>{const values=r.request().postDataJSON().values;state.evaluated.push(values);return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values,valid:values.some((value:any)=>value.fieldToken==='os')}}});});
 await page.route('**'+root+'/purchase-configuration/quote',r=>{const values=r.request().postDataJSON().values,token=values.find((value:any)=>value.fieldToken==='os')?.optionTokens?.[0]||'';state.quoted.push(token);return state.quoteFailure?r.fulfill({status:503,json:{code:50,data:null}}):r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'month',labelZhCN:'月付',minimumPurchaseCycles:1,status:'final',totalAmount:token==='ubuntu-24'?'35900':'49900'}]}}});});
 const site=new ConfigurationControlsPage(page);await site.open('/configure.html?product=os-controls');
 await expect(page.locator('[data-quote] button')).toHaveCount(1);return {site,state,business};
}
test.describe('TC-74 小白云式操作系统卡片与版本下拉（隔离受控数据）',()=>{
 test('TC-74a: 家族分组、首项默认、悬停和版本切换保持数据联动',async({page})=>{
  const {site,state,business}=await setup(page);const group=page.getByRole('group',{name:'操作系统选项',exact:true}),cards=group.locator('.os-card');
  await expect(cards).toHaveCount(6);const centos=cards.filter({has:page.getByText('CentOS',{exact:true})}),ubuntu=cards.filter({has:page.getByText('Ubuntu',{exact:true})}),alma=cards.filter({has:page.getByText('AlmaLinux',{exact:true})});
  await expect(cards.nth(0)).toHaveAttribute('data-os','CentOS');await expect(cards.nth(5)).toHaveAttribute('data-os','AlmaLinux');
  await expect(page.locator('#field-os')).toHaveValue('centos-9');await expect(centos).toHaveClass(/active/);await expect(group.locator('.lina-option-buttons')).toHaveCount(0);
  await expect(centos.locator('img')).toHaveAttribute('src','http://localhost:6680/x-assets/website-main/v-test/static/picture/21.svg');await expect(centos.locator('img')).toHaveAttribute('alt','');await expect.poll(()=>cards.locator('img').evaluateAll(images=>images.every(img=>(img as HTMLImageElement).complete&&(img as HTMLImageElement).naturalWidth>0))).toBe(true);await site.captureControls('tc074-os-cards');
  await page.mouse.move(0,850);await expect(ubuntu.locator('img')).toHaveCSS('opacity','0.4');const idle=await ubuntu.locator('img').evaluate(el=>({filter:getComputedStyle(el).filter,transition:getComputedStyle(el).transition}));expect(idle.filter).toContain('grayscale(1)');expect(idle.transition).toContain('0.3s');
  await ubuntu.locator('.os-header').hover();await expect(ubuntu.locator('img')).toHaveCSS('opacity','1');const hover=await ubuntu.evaluate(el=>({border:getComputedStyle(el).borderColor,name:getComputedStyle(el.querySelector('.os-name')!).color,filter:getComputedStyle(el.querySelector('img')!).filter,transition:getComputedStyle(el.querySelector('img')!).transition}));expect(hover.border).toBe('rgb(22, 93, 255)');expect(hover.name).toBe('rgb(22, 93, 255)');expect(hover.filter).toContain('grayscale(0)');expect(hover.transition).toContain('0.3s');await site.captureControls('tc074-os-hover');
  const cpuBefore=await page.locator('.lina-integer-slider').boundingBox();await ubuntu.locator('summary').click();await expect(ubuntu.locator('details')).toHaveAttribute('open','');await expect(centos).toHaveClass(/active/);await expect(ubuntu).not.toHaveClass(/active/);await site.captureControls('tc074-os-menu');
  const cpuOpen=await page.locator('.lina-integer-slider').boundingBox();expect(Math.abs(cpuBefore!.y-cpuOpen!.y)).toBeLessThan(1);await page.locator('.lina-integer-slider').click();await expect(group.locator('details[open]')).toHaveCount(0);
  await ubuntu.locator('summary').click();await ubuntu.locator('summary').press('Escape');await expect(group.locator('details[open]')).toHaveCount(0);await expect(centos).toHaveClass(/active/);
  await ubuntu.locator('summary').click();await ubuntu.getByRole('button',{name:'Ubuntu-24.04.1-x64',exact:true}).click();await expect(page.locator('#field-os')).toHaveValue('ubuntu-24');await expect(ubuntu).toHaveClass(/active/);await expect(alma).not.toHaveClass(/active/);
  await expect.poll(()=>state.evaluated.at(-1)?.find((value:any)=>value.fieldToken==='os')?.optionTokens?.[0]).toBe('ubuntu-24');await expect.poll(()=>state.quoted.at(-1)).toBe('ubuntu-24');await expect(site.selectedPrice()).toHaveText('¥359.00');await site.captureControls('tc074-os-selected');
  await site.term(0);await expect(page.locator('[data-add]')).toBeEnabled();
  state.quoteFailure=true;const windows=page.locator('.os-card[data-os="Windows"]');await windows.locator('summary').click();await windows.locator('.lina-os-version').first().click();await expect(page.locator('[data-add]')).toBeDisabled();await expect(site.selectedPrice()).toHaveText('待报价');await expect(page.locator('[data-status]')).toContainText('重新读取配置');await site.captureControls('tc074-os-quote-failure','quote');
  state.quoteFailure=false;await site.refreshConfiguration();await expect(page.locator('[data-quote] button')).toHaveCount(1);await expect(page.locator('#field-os')).toHaveValue('windows-2022');await expect(windows).toHaveClass(/active/);await expect(page.locator('[data-add]')).toBeEnabled();
  await page.setViewportSize({width:390,height:844});await site.captureControls('tc074-os-mobile');const mobile=await group.boundingBox();expect(mobile!.x).toBeGreaterThanOrEqual(0);expect(mobile!.x+mobile!.width).toBeLessThanOrEqual(390);
  expect(business.requests.every(request=>request.method==='GET'||(request.method==='POST'&&/\/purchase-configuration\/(evaluate|quote)$/.test(request.path)))).toBe(true);
 });
});

