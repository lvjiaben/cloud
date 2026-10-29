import {test,expect} from '../support/live-commerce';
import {HomeDetailPage} from '../pages/HomeDetailPage';
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:950}});
test.describe('TC-97 首页商品入口先查看商品卡',()=>{
 for(const kind of ['hero','shortcut','catalog','navigation'])test('TC-97 '+kind,async({page})=>{
  const site=new HomeDetailPage(page);await site.open('/');
  await expect(page.locator('.swiper-text:visible .Button')).toHaveAttribute('href',/product=.+/);
  await site.openProductEntry(kind);await expect(page.locator('body')).toHaveAttribute('data-page','products');
  const slug=new URL(page.url()).searchParams.get('product');expect(slug).toBeTruthy();expect(new URL(page.url()).searchParams.has('action')).toBe(false);
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+encodeURIComponent(slug!)),product=(await response.json()).data.product;
  await expect(page.locator('[data-products] .product-card')).toHaveCount(1);await expect(page.locator('[data-products] h5')).toHaveText(product.nameZhCN||product.nameEnUS);await expect(page.locator('#addCartForm')).toHaveCount(0);
  console.log('TC097 card '+kind+': '+await site.capture('tc097-'+kind+'-card',false));
  await site.buyProductCard();await expect(page.locator('.allocation-header-title h1')).toHaveText(product.nameZhCN||product.nameEnUS);await expect(page.locator('[data-quote] button')).not.toHaveCount(0);expect(new URL(page.url()).searchParams.get('product')).toBe(slug);
 });
});
