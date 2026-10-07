import type { Page, Route } from '@playwright/test';

export type PaymentLifecycleMockState = {
  allocationBodies: Record<string, unknown>[];
  confirmationBodies: Record<string, unknown>[];
  createBodies: Record<string, unknown>[];
  refundRequests: number;
};

const api = '**/x/idc-payment/api/v1/**';
const fundNo = 'pex_late_success_e2e_0001';

function ok(route: Route, data: unknown) {
  return route.fulfill({
    body: JSON.stringify({ code: 0, data }),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installPaymentLifecycleMock(page: Page, options: { resultUnknown?: boolean } = {}) {
  const state: PaymentLifecycleMockState = {
    allocationBodies: [], confirmationBodies: [], createBodies: [], refundRequests: 0,
  };
  let actionCode = options.resultUnknown ? 'original_refund' : '';
  let actionStatus = options.resultUnknown ? 'result_unknown' : '';
  let visible = true;
  const fund = () => ({
    actionCode, actionNo: actionCode ? 'pfa_late_success_e2e_0001' : '', actionStatus,
    amountMinor: '900719925474099301', attemptNo: 'PAT_cancelled_late_0001',
    channelNo: 'channel-epay', createdAt: Date.now(), currencyCode: 'CNY',
    exceptionNo: fundNo, orderNo: 'ORD_cancelled_0001', proofReference: '',
    providerCode: 'epay', reasonCode: 'late_success', statusCode: 'pending',
    targetOrderNo: '', version: actionStatus === 'manual_pending' ? 2 : 1,
  });

  await page.route(api, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname.endsWith('/account/payment-channels')) {
      return ok(route, { channels: [
        { channelNo: 'channel-alipay', currencies: ['CNY'], displayName: '支付宝', modes: ['alipay_page', 'alipay_wap'], provider: 'alipay' },
        { channelNo: 'channel-wechat', currencies: ['CNY'], displayName: '微信支付', modes: ['wechat_native', 'wechat_h5'], provider: 'wechatpay' },
      ] });
    }
    if (url.pathname.endsWith('/account/balance-recharges') && request.method() === 'POST') {
      const body = request.postDataJSON() as Record<string, unknown>;
      state.createBodies.push(body);
      const mode = String(body.modeCode);
      const redirectMethod = mode === 'wechat_native' ? 'QR' : mode === 'wechat_h5' || mode === 'alipay_wap' ? 'GET' : 'POST';
      return ok(route, { attemptNo: `PAT_${mode}_e2e_0001`, expiresAt: Date.now() + 60_000, redirectFields: { out_trade_no: `PAT_${mode}_e2e_0001` }, redirectMethod, redirectUrl: mode === 'wechat_native' ? 'weixin://wxpay/bizpayurl?pr=e2e' : `https://payments.example.test/${mode}`, statusCode: 'submitted' });
    }
    if (url.pathname.endsWith('/admin/payment-exception-funds') && request.method() === 'GET') {
      return ok(route, { funds: visible ? [fund()] : [], total: visible ? 1 : 0 });
    }
    if (url.pathname.endsWith(`/${fundNo}/allocation`)) {
      state.allocationBodies.push(request.postDataJSON() as Record<string, unknown>);
      visible = false;
      return ok(route, { fund: { ...fund(), statusCode: 'allocated', actionCode: 'allocate', actionStatus: 'completed' } });
    }
    if (url.pathname.endsWith(`/${fundNo}/original-refund`)) {
      state.refundRequests++;
      actionCode = 'original_refund';
      actionStatus = 'manual_pending';
      return ok(route, { fund: fund() });
    }
    if (url.pathname.endsWith(`/${fundNo}/manual-refund-proof`)) {
      return ok(route, { fileName: 'refund-proof.pdf', proofDigest: 'f'.repeat(64), proofReference: 'host-file:123', sizeBytes: '20' });
    }
    if (url.pathname.endsWith(`/${fundNo}/manual-refund-confirmation`)) {
      state.confirmationBodies.push(request.postDataJSON() as Record<string, unknown>);
      visible = false;
      return ok(route, { fund: { ...fund(), statusCode: 'refunded', actionStatus: 'completed' } });
    }
    return route.fallback();
  });
  return { fundNo, state };
}
