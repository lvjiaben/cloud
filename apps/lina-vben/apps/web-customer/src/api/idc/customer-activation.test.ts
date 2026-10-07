import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = {
  post: vi.fn(),
};

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer activation API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('redeems only through the customer-facing core plugin boundary', async () => {
    const { redeemCustomerActivation } = await import('./customer-activation');

    await redeemCustomerActivation('single-use-activation-token');

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/activation-invitations/redeem',
      { token: 'single-use-activation-token' },
    );
  });
});
