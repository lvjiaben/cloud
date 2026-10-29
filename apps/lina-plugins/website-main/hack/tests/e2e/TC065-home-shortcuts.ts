import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-65 首页四张业务快捷卡',()=>{
 test('TC-65a: 四卡绑定本方活动、商品和客户中心入口',async({page},info)=>{
  await page.route('**/x/**',async route=>{
   const request=route.request(),url=new URL(request.url()),response=previewResponse(url,request.method());
   await route.fulfill({status:response.status,json:{code:response.status===200?0:1,data:response.data}});
  });
  const site=new HomeDetailPage(page);await site.open('/');
  const cards=page.locator('[data-home-blocks] > .bannerbelow-list');
  await expect(cards).toHaveCount(4);
  await page.screenshot({path:info.outputPath('home-shortcuts-home.png'),animations:'disabled'});
  await expect(cards.nth(0)).toHaveAttribute('href','http://localhost:6680/newestactivity.html');
  await expect(cards.nth(0)).toHaveAttribute('data-home-target','/newestactivity.html');
  await expect(cards.nth(1)).toHaveAttribute('href',/\/configure\.html\?product=.+/);
  await expect(cards.nth(1)).toHaveAttribute('href',(await page.locator('.swiper-text:visible .Button').getAttribute('href'))!);
  await expect(cards.nth(2)).toHaveAttribute('href','http://localhost:6032/account/referrals');
  await expect(cards.nth(3)).toHaveAttribute('href','http://localhost:6032/account/servers');
  await expect(cards.nth(0)).not.toHaveAttribute('aria-disabled','true');
  await Promise.all([page.waitForURL(/\/newestactivity\.html$/),cards.nth(0).click()]);
  const activityTitle=page.locator('.globalBanner .pagebanner-title');
  const activityDescription=page.locator('.globalBanner .pagebanner-description');
  await expect(activityTitle).toBeVisible();
  await expect(activityTitle).toHaveText('最新活动');
  await expect(activityDescription).toBeVisible();
  await expect(activityDescription).toHaveText('涵盖免费试用、产品促销、开发者资源等，助您降本增效');
  await expect(page.locator('[data-activity-filter]')).toHaveText(['全部活动','正在进行','活动预告','已结束']);
  await expect(page.locator('[data-activity-empty]')).toBeVisible();
  await page.screenshot({path:info.outputPath('home-shortcuts-activity-initial.png'),animations:'disabled'});
  await page.locator('[data-activity-filter="ongoing"]').click();
  await expect(page.locator('[data-activity-filter="ongoing"]')).toHaveAttribute('aria-selected','true');
  await expect(page.locator('[data-activity-empty]')).toHaveText('更多优惠活动，敬请期待！');
  await site.captureSection('[data-activity-page]',info.outputPath('home-shortcuts-activity.png'));
 });
});
