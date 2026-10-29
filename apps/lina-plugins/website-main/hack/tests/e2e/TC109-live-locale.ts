import {test,expect} from '@playwright/test';
test('TC-109 实际官网语言按钮与跨页保持',async({page},info)=>{
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 await page.route('**/*',route=>{const request=route.request();return ['GET','HEAD'].includes(request.method())?route.continue():route.abort('blockedbyclient');});
 await page.goto('http://127.0.0.1:6680/');
 const toggle=page.locator('[data-public-locale-switch]');await expect(toggle).toHaveText('English');
 await expect(page.locator('.horizontal-menu-navbar-menu>li')).not.toHaveCount(0);
 const signup=await page.locator('.navbar-Login').boundingBox(),button=await toggle.boundingBox();expect(button!.x).toBeGreaterThanOrEqual(signup!.x+signup!.width);
 await page.screenshot({path:info.outputPath('live-chinese-button.png')});
 await toggle.click();await expect(page.locator('html')).toHaveAttribute('lang','en-US');await expect(page.locator('.navbar-Login')).toHaveText('Sign up');
 await page.screenshot({path:info.outputPath('live-english-button.png')});
 await page.goto('http://127.0.0.1:6680/products.html');await expect(toggle).toHaveText('中文');await page.reload();await expect(page.locator('html')).toHaveAttribute('lang','en-US');
 await toggle.click();await expect(page.locator('.navbar-Login')).toHaveText('免费注册');expect(errors).toEqual([]);
});
