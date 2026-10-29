import {test,expect} from '@playwright/test';
import {HomeDetailPage} from '../pages/HomeDetailPage';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-93 首页商品入口先展示商品卡',()=>{
 test('TC-93a: 特惠卡与当前轮播目标一致，先进入对应商品卡再配置',async({page})=>{
  const site=new HomeDetailPage(page);
  await site.open('/');
  const hero=page.locator('.swiper-text:visible'),shortcut=page.locator('[data-home-blocks] > .bannerbelow-list').nth(1);
  await expect(hero.locator('.Button')).toHaveAttribute('href',/product=.+/);
  const firstTarget=await hero.locator('.Button').getAttribute('href');
  await expect(shortcut).toHaveAttribute('href',firstTarget!);
  await site.hero(1);
  await expect(hero.locator('.Button')).toHaveAttribute('href',/product=.+/);
  const target=await hero.locator('.Button').getAttribute('href'),name=await hero.locator('.Title').innerText();
  expect(target).not.toBe(firstTarget);
  await expect(shortcut).toHaveAttribute('href',target!);
  await expect(shortcut).not.toHaveAttribute('aria-disabled','true');
  await site.clickShortcut(1);
  await expect(page).toHaveURL(/\/products\.html\?product=/);
  expect(new URL(page.url()).searchParams.get('product')).toBe(new URL(target!,'http://127.0.0.1:6680').searchParams.get('product'));
  await expect(page.locator('body')).toHaveAttribute('data-page','products');
  await expect(page.locator('[data-products] .product-card')).toHaveCount(1);
  await expect(page.locator('[data-products] h5')).toHaveText(name);
  await expect(page.locator('#addCartForm')).toHaveCount(0);
  await expect(page.locator('[data-categories]').getByRole('link',{name:'全部商品',exact:true})).toHaveCount(0);
  console.log('TC093 screenshot: '+await site.capture('tc093-product-shortcut-card',false));
  await site.open(await page.locator('[data-products] .buy-now-button').getAttribute('href') as string);
  await expect(page.locator('.allocation-header-title h1')).toHaveText(name);
  await expect(page.locator('[data-quote] button')).not.toHaveCount(0);
 });
 test('TC-93b: 无商品、不可购买或读取失败时不生成错误购买链接',async({page})=>{
  let mode='empty';
  await page.route('**/catalog/products?limit=4',async route=>{
   if(mode==='failed')return route.fulfill({status:503,json:{code:1}});
   const products=mode==='empty'?[]:[{publicSlug:'unavailable-shortcut',nameZhCN:'暂不可订购商品',availability:'unavailable'}];
   await route.fulfill({json:{code:0,data:{products}}});
  });
  const site=new HomeDetailPage(page),shortcut=page.locator('[data-home-blocks] > .bannerbelow-list').nth(1);
  for(const value of ['empty','unavailable','failed']){
   mode=value;await site.open('/');
   if(value==='unavailable')await expect(page.locator('.swiper-text:visible .Title')).toHaveText('暂不可订购商品');
   else await expect(page.locator('.lina-server-status')).toContainText(value==='empty'?'暂无已发布的服务器':'服务器加载失败');
   await expect(shortcut).not.toHaveAttribute('href');
   await expect(shortcut).toHaveAttribute('aria-disabled','true');
   console.log('TC093 screenshot: '+await site.capture('tc093-shortcut-'+value,false));
  }
 });
});
