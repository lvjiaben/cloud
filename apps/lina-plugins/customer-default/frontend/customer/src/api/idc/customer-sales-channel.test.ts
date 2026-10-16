import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { get: vi.fn(), post: vi.fn() };

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer sales-channel API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps referral and commission reads scoped to the customer account', async () => {
    const {
      getCustomerCommissionOverview,
      getCustomerReferralOverview,
      listCustomerCommissionSettlements,
    } = await import('./customer-sales-channel');
    await getCustomerReferralOverview();
    await getCustomerCommissionOverview({ currencyCode: 'CNY' });
    await listCustomerCommissionSettlements({ cursor: 'next' });
    expect(requestClient.get).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/sales-channel/referral',
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/sales-channel/commission/overview',
      { params: { limit: 20, currencyCode: 'CNY' } },
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      3,
      '/x/idc-core/api/v1/account/sales-channel/commission/settlements',
      { params: { limit: 20, cursor: 'next' } },
    );
  });

  it('creates a referral link through the customer-scoped plugin path', async () => {
    const { createCustomerReferralLink } = await import('./customer-sales-channel');
    await createCustomerReferralLink({
      policyCode: 'first_touch', attributionWindowSeconds: 2592000,
      idempotencyKey: 'referral-link-20260830-001',
    });
    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/sales-channel/referral-links',
      { policyCode: 'first_touch', attributionWindowSeconds: 2592000,
        idempotencyKey: 'referral-link-20260830-001' },
    );
  });
});
