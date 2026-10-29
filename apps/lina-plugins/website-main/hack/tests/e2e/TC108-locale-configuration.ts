import {test,expect} from '../support/source-site';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test('TC-108 真实配置报价切换语言保留选择与价格，不刷新文档或重复报价',async({page},info)=>{
 test.setTimeout(60000);
 const errors:string[]=[],writes:string[]=[],documents:string[]=[],prices:string[]=[];
 page.on('pageerror',error=>errors.push(error.message));
 page.on('request',request=>{if(request.isNavigationRequest())documents.push(request.url());if(request.method()==='POST'){if(/\/purchase-configuration\/(evaluate|quote)$/.test(request.url()))prices.push(request.url());else writes.push(request.url());}});
 await page.goto('/products.html?product=ecs-zhenjiang-configurable-simulation&action=configureproduct');
 await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
 const secondTerm=page.locator('[data-quote] button').nth(1);await secondTerm.click();
 const values=()=>page.locator('#addCartForm').evaluate(form=>({inputs:[...form.querySelectorAll<HTMLInputElement|HTMLSelectElement>('input,select,textarea')].map(input=>({name:input.name,value:input.value,checked:'checked' in input?input.checked:undefined})),term:form.querySelector('[data-billing-term-token][aria-pressed=true]')?.getAttribute('data-billing-term-token'),total:document.querySelector('[data-total-amount]')?.textContent}));
 const before=await values(),count=prices.length;
 const toggle=page.locator('[data-public-locale-switch]');await toggle.focus();await page.keyboard.press('Enter');
 await expect(page.locator('html')).toHaveAttribute('lang','en-US');await expect(page.locator('[data-fields]')).toContainText('Operating system');
 await expect(page.locator('[data-total-summary]')).toContainText('Total cost');
 expect(await values()).toEqual(before);expect(prices).toHaveLength(count);expect(documents).toHaveLength(1);
 await page.screenshot({path:info.outputPath('english-configuration.png'),animations:'disabled'});
 await page.setViewportSize({width:390,height:844});await page.locator('.lina-menu-toggle').click();await expect(toggle).toBeVisible();
 await page.screenshot({path:info.outputPath('english-configuration-mobile.png'),animations:'disabled'});
 expect(await page.locator('body').evaluate(el=>el.scrollWidth)).toBeLessThanOrEqual(390);
 await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','zh-CN');expect(await values()).toEqual(before);
 expect(writes).toEqual([]);expect(errors).toEqual([]);
});
