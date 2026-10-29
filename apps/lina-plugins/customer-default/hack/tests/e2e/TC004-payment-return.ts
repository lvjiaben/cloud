import { test, expect } from '@host-tests/fixtures/auth';
import { LoginPage } from '@host-tests/pages/LoginPage';
import { PaymentReturnPage } from '../pages/PaymentReturnPage';

const origin = process.env.CUSTOMER_E2E_BASE_URL || 'http://127.0.0.1:6032';

test.describe('TC-4 付款回跳与开通进度衔接', () => {
  test('TC-4j: 未付款订单在手机首屏保留付款操作优先于待付款交付',async({page})=>{
    await page.route('**/account/commerce/orders/ORD_payment_visible',r=>r.fulfill({json:{code:0,data:{order:{
      summary:{orderNo:'ORD_payment_visible',currencyCode:'CNY',statusCode:'pending_payment',totalAmountMinor:'1100',createdAt:1792494724000,canCancel:true,canPreparePayment:true},
      paymentPreparation:{contextNo:'PPC_ready_visible',statusCode:'ready',reasonCode:'',identityStatusCode:'verified'},
      items:[],deliveries:[{productNameZhCN:'待付款云服务器',productNameEnUS:'Unpaid server',quantity:1,statusCode:'awaiting_payment',ongoingService:true,updatedAt:1792494724000}],serviceEntitlements:[],reviews:[],
    }}}}));
    await page.setViewportSize({width:414,height:896});
    await page.goto(origin+'/account/orders/ORD_payment_visible');
    const facts=page.getByTestId('commerce-order-facts');
    await expect(facts.getByRole('button',{name:'继续付款',exact:true})).toBeInViewport({ratio:1});
    const progress=page.getByTestId('commerce-deliveries');
    await expect(progress).toContainText('待付款');
    const factsY=await facts.evaluate(el=>el.getBoundingClientRect().top);
    const progressY=await progress.evaluate(el=>el.getBoundingClientRect().top);
    expect(factsY).toBeLessThan(progressY);
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await new PaymentReturnPage(page,origin).screenshot('unpaid-order-payment-first');
  });
  test('TC-4i: 订单首屏显示人工处理，付款账单可读且桌面窄屏双语不溢出', async ({page}) => {
    const orderNo='ORD_visibility_012345678901234567890';
    await page.route('**/account/commerce/orders/'+orderNo, r => r.fulfill({json:{code:0,data:{order:{
      summary:{orderNo,currencyCode:'CNY',statusCode:'paid',totalAmountMinor:'1100',createdAt:1792494724000,canCancel:false,canPreparePayment:false},
      invoice:{invoiceNo:'INV_long_012345678901234567890123456789',currencyCode:'CNY',statusCode:'paid',totalAmountMinor:'1100',paidAmountMinor:'1100',refundableAmountMinor:'0',paymentDueAt:1792496524000},
      paymentPreparation:{contextNo:'PPC_old_ready',statusCode:'ready',reasonCode:'',identityStatusCode:'verified'},
      items:[{orderItemNo:'item_visibility',productNameZhCN:'陕西云服务器',productNameEnUS:'Shaanxi cloud server',quantity:1,purchaseCycles:1,totalAmountMinor:'1100'}],
      deliveries:[{productNameZhCN:'陕西云服务器',productNameEnUS:'Shaanxi cloud server',quantity:1,statusCode:'manual_processing',ongoingService:true,updatedAt:1792494724000}],serviceEntitlements:[],reviews:[],
    }}}}));
    await page.setViewportSize({width:1280,height:800});
    await page.goto(origin+'/account/orders/'+orderNo);
    const progress=page.getByTestId('commerce-deliveries');
    const facts=page.getByTestId('commerce-order-facts');
    const invoice=page.getByTestId('commerce-order-invoice');
    await expect(progress.getByText('待人工处理',{exact:true})).toBeInViewport({ratio:1});
    await expect(progress).toContainText('付款记录已保留，请勿重复付款。');
    await expect(facts).toContainText('已付款');
    await expect(facts).toContainText('CNY 11.00');
    await expect(invoice).toContainText('INV_long_012345678901234567890123456789');
    await expect.poll(()=>facts.evaluate(el=>el.getBoundingClientRect().height)).toBeLessThan(330);
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await new PaymentReturnPage(page,origin).screenshot('order-progress-desktop');
    await page.setViewportSize({width:414,height:896});
    await expect(progress.getByText('待人工处理',{exact:true})).toBeInViewport({ratio:1});
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await new PaymentReturnPage(page,origin).screenshot('order-progress-mobile');
    await new LoginPage(page).switchLanguage('English');
    await page.reload();
    await expect(progress.getByText('Needs staff attention',{exact:true})).toBeInViewport({ratio:1});
    await expect(progress).toContainText('Your payment is recorded');
    await expect(progress).not.toContainText('commerce.');
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await new PaymentReturnPage(page,origin).screenshot('order-progress-en-mobile');
    await invoice.scrollIntoViewIfNeeded();
    await expect(invoice).toContainText('CNY 11.00');
    await new PaymentReturnPage(page,origin).screenshot('order-invoice-en-mobile');
  });
  for (const statusCode of ['manual_processing', 'not_completed'] as const) {
    test(`TC-4${statusCode === 'manual_processing' ? 'f' : 'g'}: ${statusCode === 'manual_processing' ? '人工待处理' : '明确失败'}停止等待且保留付款`, async ({page}) => {
      await page.clock.install();
      let orderReads = 0;
      await page.route('**/account/commerce/orders/ORD_attention_test', r => {
        orderReads += 1;
        return r.fulfill({json:{code:0,data:{order:{summary:{orderNo:'ORD_attention_test',currencyCode:'CNY',statusCode:'paid',totalAmountMinor:'1100',createdAt:Date.now(),canCancel:false,canPreparePayment:false},paymentPreparation:{contextNo:'PPC_settled_fixture',statusCode:'ready',reasonCode:'',identityStatusCode:'verified'},items:[],deliveries:[{productNameZhCN:'测试云服务器',productNameEnUS:'Test cloud server',quantity:1,statusCode,ongoingService:true,updatedAt:Date.now()}],serviceEntitlements:[],reviews:[]}}}});
      });
      await page.goto(origin + '/account/orders/ORD_attention_test');
      await expect(page.getByText('已付款', {exact: true})).toBeVisible();
      await expect(page.getByText('实名和订单状态已通过核验，可以继续选择支付方式。',{exact:true})).toHaveCount(0);
      await expect(page.getByRole('button',{name:'我已完成，重新检查',exact:true})).toHaveCount(0);
      await expect(page.getByText('状态暂不可用', {exact: true})).toHaveCount(0);
      const delivery = page.getByTestId('commerce-deliveries');
      await expect(delivery).toContainText(statusCode === 'manual_processing' ? '待人工处理' : '处理未完成');
      await expect(delivery).toContainText(statusCode === 'manual_processing' ? '付款记录已保留，请勿重复付款。' : '无需重复付款。');
      await expect(delivery).not.toContainText('开通正在处理中，状态会自动更新。');
      await expect(delivery).not.toContainText('履约状态暂不可用');
      await page.clock.fastForward(16000);
      await page.clock.runFor(1000);
      expect(orderReads).toBe(1);
      await new PaymentReturnPage(page, origin).screenshot('delivery-' + statusCode);
    });
  }
  test.beforeEach(async ({page}) => {
    await page.route('**/api/v1/auth/login', r => r.fulfill({json:{code:0,data:{accessToken:'fixture-return-session',refreshToken:'fixture-return-refresh',tenants:[]}}}));
    await page.route('**/api/v1/user/info', r => r.fulfill({json:{code:0,data:{id:702,username:'return-ui',realName:'付款测试客户',roles:['user'],permissions:[],menus:[],homePath:'/account/billing/invoices'}}}));
    await page.route('**/api/v1/plugins/dynamic', r => r.fulfill({json:{code:0,data:{list:[]}}}));
    await page.route('**/customer/external-menus', r => r.fulfill({json:{code:0,data:{items:[]}}}));
    await page.route('**/account/product-directories*', r => r.fulfill({json:{code:0,data:{items:[],total:0,page:1,pageSize:100}}}));
    await page.route('**/account/initial-password/status', r => r.fulfill({json:{code:0,data:{required:false}}}));
    await page.route('**/account/billing/invoices*', r => r.fulfill({json:{code:0,data:{invoices:[],nextCursor:''}}}));
    await page.goto(origin + '/auth/login');
    await new LoginPage(page).login('return-ui','fixture-only-password');
  });

  test('TC-4a: 订单付款确认与充值区分，直接进入本人订单', async ({page}) => {
    await page.route('**/account/payments/PAT_return_test', r => r.fulfill({json:{code:0,data:{attemptNo:'PAT_return_test',purposeCode:'invoice',orderNo:'ORD_return_test',invoiceNo:'INV_return_test',statusCode:'succeeded',currencyCode:'CNY'}}}));
    let orderReads=0;
    await page.route('**/account/commerce/orders/ORD_return_test', r => {
      const completed=++orderReads>1;
      return r.fulfill({json:{code:0,data:{order:{summary:{orderNo:'ORD_return_test',currencyCode:'CNY',statusCode:'paid',totalAmountMinor:'1000',createdAt:Date.now(),canCancel:false,canPreparePayment:false},items:[],deliveries:[{productNameZhCN:'测试云服务器',productNameEnUS:'Test cloud server',quantity:1,statusCode:completed?'completed':'processing',ongoingService:true,updatedAt:Date.now()}],serviceEntitlements:completed?[{entitlementNo:'ent_return_test',orderItemNo:'item_return_test',productNameZhCN:'测试云服务器',productNameEnUS:'Test cloud server',statusCode:'active',startedAt:Date.now(),expiresAt:Date.now()+86400000}]:[],reviews:[]}}}});
    });
    const view = new PaymentReturnPage(page, origin); await view.open();
    await expect(page.getByRole('heading',{name:'付款已确认',exact:true})).toBeVisible();
    await expect(page.getByText('充值已经到账',{exact:true})).toHaveCount(0);
    await expect(page.getByTestId('payment-return-page')).toContainText('商品开通单独处理');
    await view.screenshot('confirmed-order'); await view.viewDelivery();
    await expect(page).toHaveURL(/\/account\/orders\/ORD_return_test$/);
    await expect(page.getByTestId('commerce-deliveries')).toContainText('测试云服务器');
    await expect(page.getByTestId('commerce-deliveries')).toContainText('处理中');
    await expect(page.getByTestId('commerce-deliveries')).toContainText('开通正在处理中，状态会自动更新。');
    await view.screenshot('provisioning-order');
    await expect(page.getByTestId('commerce-deliveries')).toContainText('已完成',{timeout:12000});
    await expect(page.getByTestId('commerce-service-entitlements')).toContainText('服务中');
    expect(orderReads).toBe(2);
    await view.screenshot('provisioned-service');
  });

  test('TC-4b: 渠道尚未确认时自动查询，确认后显示真实结果', async ({page}) => {
    let reads=0;
    await page.route('**/account/payments/PAT_return_test', r => r.fulfill({json:{code:0,data:{attemptNo:'PAT_return_test',purposeCode:'invoice',orderNo:'ORD_return_test',statusCode:++reads>1?'succeeded':'confirming',currencyCode:'CNY'}}}));
    const view = new PaymentReturnPage(page, origin); await view.open();
    await expect(page.getByRole('heading',{name:'正在等待支付平台确认',exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'查看开通进度',exact:true})).toHaveCount(0);
    await expect(page.getByRole('heading',{name:'付款已确认',exact:true})).toBeVisible({timeout:12000});
    expect(reads).toBe(2); await view.screenshot('query-confirmed');
  });

  test('TC-4c: 结果需要核对时不宣称成功，读取失败可以重试', async ({page}) => {
    let unavailable=true;
    await page.route('**/account/payments/PAT_return_test', r => unavailable?r.fulfill({status:503,json:{code:50,message:'Unavailable'}}):r.fulfill({json:{code:0,data:{attemptNo:'PAT_return_test',purposeCode:'invoice',statusCode:'manual_review',currencyCode:'CNY'}}}));
    const view=new PaymentReturnPage(page, origin);await view.open();
    await expect(page.getByText('暂时无法读取付款状态，请稍后刷新。',{exact:true})).toBeVisible();
    await view.screenshot('read-failed');unavailable=false;await view.refresh();
    await expect(page.getByRole('heading',{name:'付款需要核对',exact:true})).toBeVisible();
    await expect(page.getByTestId('payment-return-page')).toContainText('待人工核对');
    await expect(page.getByTestId('payment-return-page')).not.toContainText('finance.status.');
    await expect(page.getByRole('button',{name:'查看开通进度',exact:true})).toHaveCount(0);
    await view.screenshot('manual-review');
  });

  test('TC-4d: 余额充值继续显示到账，信用还款显示还款确认', async ({page}) => {
    let purpose='balance_recharge';
    await page.route('**/account/payments/PAT_return_test', r => r.fulfill({json:{code:0,data:{attemptNo:'PAT_return_test',purposeCode:purpose,statusCode:'succeeded',currencyCode:'CNY'}}}));
    const view = new PaymentReturnPage(page, origin);await view.open();
    await expect(page.getByRole('heading',{name:'充值已经到账',exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'查看开通进度',exact:true})).toHaveCount(0);
    purpose='credit_statement';await view.refresh();
    await expect(page.getByRole('heading',{name:'还款已确认',exact:true})).toBeVisible();
    await view.screenshot('repayment');
  });
  test('TC-4h: 付款准备只在允许付款的订单显示，已付款不显示继续支付提示',async({page})=>{
   let paid=true;
   await page.route('**/account/commerce/orders/ORD_preparation_test',r=>r.fulfill({json:{code:0,data:{order:{summary:{orderNo:'ORD_preparation_test',currencyCode:'CNY',statusCode:paid?'paid':'pending_payment',totalAmountMinor:'1100',createdAt:Date.now(),canCancel:!paid,canPreparePayment:!paid},paymentPreparation:{contextNo:'PPC_read_fixture',statusCode:'ready',reasonCode:'',identityStatusCode:'verified'},items:[],deliveries:[],serviceEntitlements:[],reviews:[]}}}}));
   await page.goto(origin+'/account/orders/ORD_preparation_test');await expect(page.getByText('已付款',{exact:true})).toBeVisible();
   const ready=page.getByText('实名和订单状态已通过核验，可以继续选择支付方式。',{exact:true});
   await expect(ready).toHaveCount(0);await expect(page.getByRole('button',{name:'我已完成，重新检查',exact:true})).toHaveCount(0);
   paid=false;await page.reload();await expect(ready).toBeVisible();await expect(page.getByRole('button',{name:'我已完成，重新检查',exact:true})).toBeVisible();await new PaymentReturnPage(page,origin).screenshot('actionable-payment-preparation');
  });

});
