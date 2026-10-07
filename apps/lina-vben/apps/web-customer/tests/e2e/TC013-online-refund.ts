import { RefundsPage } from '../pages/RefundsPage';
import {
  balanceRefundNo,
  installCustomerRefundMock,
  originalRefundNo,
  refundInvoiceItemNo,
  refundOrderNo,
} from '../support/customer-refund-api-mock';
import { expect, test } from '../fixtures/customer';

test.describe('TC013 客户在线退款', () => {
  test('TC013a: 客户按权威预览提交退款且请求不携带客户编号', async ({
    page,
  }) => {
    const state = await installCustomerRefundMock(page);
    const refunds = new RefundsPage(page);
    await refunds.login();
    await refunds.open(refundOrderNo);
    await refunds.selectWholeItem(refundInvoiceItemNo, '5000');
    await refunds.openPreview();
    expect(state.previewBodies).toEqual([
      {
        orderNo: refundOrderNo,
        selections: [
          {
            amountMinor: '5000',
            entitlementNos: [],
            invoiceItemNo: refundInvoiceItemNo,
            quantity: 1,
          },
        ],
      },
    ]);
    await refunds.chooseReason('配置选错');
    await refunds.confirmImpact();
    await refunds.submit();
    await expect.poll(() => state.submitBodies.length).toBe(1);
    expect(state.submitBodies[0]).not.toHaveProperty('customerNo');
    expect(state.submitBodies[0]?.previewDigest).toBe('a'.repeat(64));
    expect(String(state.submitBodies[0]?.idempotencyKey)).toMatch(
      /^customer-refund-/u,
    );
    await expect(refunds.refund('RFD_CREATED_0001')).toContainText('等待审核');
    await refunds.screenshot('customer-refund-submitted-zh');
  });

  test('TC013b: 英文窄屏预览明确显示长期服务权益终止影响', async ({
    page,
  }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await installCustomerRefundMock(page, { locale: 'en-US' });
    const refunds = new RefundsPage(page);
    await refunds.login();
    await refunds.open(refundOrderNo);
    await refunds.selectEntitlement(
      refundInvoiceItemNo,
      'ENT_REFUND_0001',
      '5000',
    );
    await refunds.openPreview();
    const drawer = page
      .getByRole('dialog')
      .filter({ hasText: 'Confirm refund amount' });
    await expect(drawer).toContainText('terminates the selected ongoing entitlement');
	await expect(drawer).toContainText(
		'Ongoing service “Production cloud service” will be terminated',
	);
	await expect(drawer).not.toContainText('ENT_REFUND_0001');
    await expect(drawer).not.toContainText('refunds.preview');
    const drawerBox = await drawer.boundingBox();
    expect(drawerBox?.width || Number.POSITIVE_INFINITY).toBeLessThanOrEqual(390);
    expect(
      await drawer.evaluate((element) => element.scrollWidth <= element.clientWidth),
    ).toBe(true);
    expect(await refunds.hasHorizontalOverflow()).toBe(false);
    await refunds.screenshot('customer-refund-impact-en-mobile');
  });

  test('TC013e: 退款数量为二时仍可只选择一个需要终止的长期权益', async ({
    page,
  }) => {
    const state = await installCustomerRefundMock(page);
    const refunds = new RefundsPage(page);
    await refunds.login();
    await refunds.open(refundOrderNo);
    await refunds.selectEntitlement(
      refundInvoiceItemNo,
      'ENT_REFUND_0001',
      '5000',
    );
    await refunds.setQuantity(refundInvoiceItemNo, 2);
    await refunds.openPreview();

    expect(state.previewBodies[0]).toMatchObject({
      selections: [
        {
          entitlementNos: ['ENT_REFUND_0001'],
          invoiceItemNo: refundInvoiceItemNo,
          quantity: 2,
        },
      ],
    });
    const drawer = page
      .getByRole('dialog')
      .filter({ hasText: '确认退款金额和服务影响' });
    await expect(drawer).toContainText(/退款数量\s*:\s*2/u);
    await expect(drawer).toContainText('长期服务“生产云服务”将被终止');
    await expect(drawer).not.toContainText('ENT_REFUND_0001');
  });

  test('TC013c: 原路退款结果明确显示原付款来源和已退款', async ({ page }) => {
    await installCustomerRefundMock(page);
    const refunds = new RefundsPage(page);
    await refunds.login();
    await refunds.open();
    await expect(refunds.refund(originalRefundNo)).toContainText('原付款来源');
    await expect(refunds.refund(originalRefundNo)).toContainText('已退款');
    await refunds.screenshot('customer-refund-original-source-zh');
  });

  test('TC013d: 混合来源拒绝时整笔等待，同意后才统一放行', async ({ page }) => {
    const state = await installCustomerRefundMock(page);
    const refunds = new RefundsPage(page);
    await refunds.login();
    await refunds.open();
    const card = refunds.refund(balanceRefundNo);
    await expect(card).toContainText('整笔退款目前全部暂停');

    await card.getByTestId('refund-balance-keep-pending').click();
    const pendingDialog = page.getByRole('dialog');
    await expect(pendingDialog).toContainText('任何一部分');
    await pendingDialog
      .getByRole('button', { name: '暂不同意，整笔继续等待' })
      .click();
    await expect.poll(() => state.consentBodies.length).toBe(1);
    expect(state.consentBodies[0]).toMatchObject({
      consent: false,
      expectedVersion: 2,
    });
    await expect(card).toContainText('等待确认退款去向');

    await card.getByTestId('refund-balance-accept').click();
    const acceptDialog = page.getByRole('dialog');
    await expect(acceptDialog).toContainText('所有部分才会一起开始执行');
    await acceptDialog
      .getByRole('button', { name: '同意不能原路退的部分进入余额' })
      .click();
    await expect.poll(() => state.consentBodies.length).toBe(2);
    expect(state.consentBodies[1]).toMatchObject({
      consent: true,
      expectedVersion: 3,
    });
    expect(state.consentBodies[1]).not.toHaveProperty('customerNo');
    await expect(refunds.refund(balanceRefundNo)).toContainText(
      '部分原路退回，部分进入同币种余额',
    );
    await expect(refunds.refund(balanceRefundNo)).toContainText('已批准');
    await expect(acceptDialog).toBeHidden();
    await refunds.screenshot('customer-refund-mixed-consent-zh');
  });
});
