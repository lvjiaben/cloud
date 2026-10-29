import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-94 购买配置仅保留固定费用合计',()=>{
 test.describe.configure({timeout:60000});
 test.beforeEach(async({page})=>{
  if(process.env.WEBSITE_FOOTER_CANDIDATE){
   for(const [file,type] of [['site.js','application/javascript'],['site.css','text/css']]){
    const body=await readFile(resolve(process.env.WEBSITE_FOOTER_CANDIDATE,'candidate-ordinary-'+file),'utf8');
    await page.route('**/lina/'+file,route=>route.fulfill({contentType:type,body}));
   }
  }
 });
 for(const [name,path,width] of [
  ['固定套餐','/products.html?product=ecs-zhenjiang-balanced-draft-20261020&action=configureproduct',1440],
  ['选配套餐','/products.html?product=ecs-zhenjiang-configurable-simulation&action=configureproduct',1440],
  ['旧入口手机','/configure.html?product=ecs-shanghai-configurable-simulation',390],
 ] as const){
  test('TC-94: '+name+'无公共页脚或重复报价',async({page},info)=>{
   await page.setViewportSize({width,height:900});
   await page.goto(path);
   await expect(page.locator('body')).toHaveAttribute('data-page','configure');
   await expect(page.locator('[data-total-amount]')).toContainText(/CNY\s*[\d,.]+/,{timeout:25000});
   await expect(page.locator('.sky-footer-container')).toBeHidden();
   await expect(page.locator('[data-selected-price]')).toHaveCount(0);
   await expect(page.locator('[data-status]')).not.toContainText('报价按单件及所选结算周期显示');
   const footer=page.locator('.allocation-footer');
   await page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));
   await expect(footer).toBeVisible();
   const box=await footer.boundingBox();expect(box).not.toBeNull();
   expect(Math.abs(box!.y+box!.height-900)).toBeLessThanOrEqual(2);
   const terms=page.locator('[data-quote] button');
   expect(await terms.count()).toBeGreaterThan(1);
   await terms.last().click();
   await expect(terms.last()).toHaveAttribute('aria-pressed','true');
   const termPrice=(await terms.last().innerText()).match(/CNY\s*[\d,.]+/)![0];
   await expect(page.locator('[data-total-amount]')).toHaveText(termPrice);
   await page.screenshot({path:info.outputPath('configure-bottom.png'),animations:'disabled'});
   await page.locator('.nav-top-page .return').click();
   await expect(page.locator('body')).toHaveAttribute('data-page','products');
   await expect(page.locator('[data-products] .product-card').first()).toBeVisible({timeout:25000});
   await page.goBack();
   await expect(page.locator('body')).toHaveAttribute('data-page','configure');
   await expect(page.locator('.sky-footer-container')).toBeHidden();
   await page.locator('.horizontal-menu-navbar-logo a').click();
   await expect(page.locator('body')).toHaveAttribute('data-page','home');
   await expect(page.locator('.sky-footer-container')).toBeVisible();
  });
 }
 test('TC-94: 报价失败仍显示错误且禁止加车',async({page},info)=>{
  await page.route('**/purchase-configuration/quote',route=>route.fulfill({status:503,json:{code:1,message:'unavailable'}}));
  await page.goto('/products.html?product=ecs-zhenjiang-balanced-draft-20261020&action=configureproduct');
  await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。',{timeout:25000});
  await expect(page.locator('[data-add]')).toBeDisabled();
  await expect(page.locator('[data-total-amount]')).toHaveText('待报价');
  await expect(page.locator('.sky-footer-container')).toBeHidden();
  await page.locator('[data-status]').scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath('configure-quote-failed.png'),animations:'disabled'});
  await page.unroute('**/purchase-configuration/quote');
  await page.locator('[data-refresh]').click();
  await expect(page.locator('[data-total-amount]')).toContainText(/CNY\s*[\d,.]+/,{timeout:25000});
  await expect(page.locator('[data-status]')).toHaveText('');
 });
});
