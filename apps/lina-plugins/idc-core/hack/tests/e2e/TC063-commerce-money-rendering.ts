// Exercise the published admin bundle: source-only tests miss exponent lowering.
import {expect, test} from '../support/catalog-auth';
import {CommerceOrdersPage} from '../pages/CommerceOrdersPage';
import {commerceAdminOrderNo,installCommerceOrderMock} from '../support/commerce-order-mock';

test.describe('TC-63 发布构建的精确订单金额',()=>{
 for(const [suffix,currency,expected] of [['a','CNY','9007199254740993.01'],['b','JPY','900719925474099301'],['c','KWD','900719925474099.301']] as const){
  test(`TC-63${suffix}: ${currency}大金额在列表、详情与商品项目保持精确`,async({adminPage})=>{
   const errors:string[]=[];adminPage.on('pageerror',error=>errors.push(error.message));
   await installCommerceOrderMock(adminPage,{currencyCode:currency});
   const orders=new CommerceOrdersPage(adminPage);await orders.goto();
   await orders.scrollTableToEnd();
   await expect(orders.row(commerceAdminOrderNo)).toContainText(`${currency} ${expected}`);
   const amountCell=orders.row(commerceAdminOrderNo).locator('.vxe-body--column').filter({hasText:`${currency} ${expected}`}).locator('.vxe-cell');
   await expect.poll(()=>amountCell.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
   await orders.openDetail(commerceAdminOrderNo);
   const drawer=adminPage.getByRole('dialog');
   await expect(drawer.getByRole('table').first()).toContainText(`${currency} ${expected}`);
   const items=drawer.getByTestId('commerce-order-items');
   await expect(items.locator('.detail-card')).toHaveCount(2);
   await expect(items).toContainText('通配符 SSL 证书');
   await expect(items).toContainText(`${currency} ${expected}`);
   expect(errors.filter(error=>/BigInt|convert.*number/iu.test(error))).toEqual([]);
   await orders.screenshot('exact-order-amount-'+currency);
  });
 }
});
