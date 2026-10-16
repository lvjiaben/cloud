import { describe, expect, it, vi } from 'vitest';

const requestClient = { post: vi.fn(), put: vi.fn() };

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer payment recovery API', () => {
  it('merges the browser-held guest cart without accepting a token argument', async () => {
    const { mergeGuestCart } = await import('./customer-commerce');

    await mergeGuestCart();

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/commerce/guest-cart-merge',
    );
  });

  it('resumes only the customer-owned opaque payment context', async () => {
    const { resumePaymentPreparation } = await import('./customer-commerce');

    await resumePaymentPreparation('payment/context');

    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/commerce/payment-preparations/payment%2Fcontext',
    );
  });
});
