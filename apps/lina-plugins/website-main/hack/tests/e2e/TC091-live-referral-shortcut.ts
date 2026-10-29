import {test,expect} from '@playwright/test';
import {HomeDetailPage} from '../pages/HomeDetailPage';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-91 实际官网推广入口',()=>{
 test('TC-91a: 推广卡跳转到本方用户中心',async({page},info)=>{
  const site=new HomeDetailPage(page);
  await site.open('/');
  const card=page.locator('[data-home-blocks] > .bannerbelow-list').nth(2);
  await expect(card).toContainText('推广赚佣 轻松赚钱');
  await expect(card).toContainText('每笔推荐均可赚钱（现金返佣）');
  await expect(card).toHaveAttribute('href','http://127.0.0.1:6032/account/referrals');
  await expect(card).not.toHaveAttribute('aria-disabled','true');
  await site.captureSection('[data-home-blocks]',info.outputPath('referral-shortcut.png'));
  const navigation=page.waitForResponse(response=>response.request().isNavigationRequest()&&response.url()==='http://127.0.0.1:6032/account/referrals');
  await site.clickShortcut(2);
  expect((await navigation).status()).toBe(200);
  await expect(page).toHaveURL(/^http:\/\/127\.0\.0\.1:6032\//);
  await expect(page.locator('body')).not.toBeEmpty();
  await expect(page.locator('.loading').first()).toBeHidden({timeout:20000});
  await page.screenshot({path:info.outputPath('referral-destination.png'),animations:'disabled'});
 });
});
