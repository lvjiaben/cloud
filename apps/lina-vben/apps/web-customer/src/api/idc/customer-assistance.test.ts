import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { post: vi.fn() };

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer assistance API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps the short-lived token out of the request URL', async () => {
    const { verifyCustomerAssistance } = await import(
      './customer-assistance'
    );
    requestClient.post.mockResolvedValue({
      accessToken: 'delegated-token',
      homePath: '/account',
    });

    await verifyCustomerAssistance('safe-token-value');

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/public/customer-assistance/access-verifications',
      { accessToken: 'safe-token-value' },
      { silentErrorMessage: true },
    );
    expect(requestClient.post.mock.calls[0]?.[0]).not.toContain(
      'safe-token-value',
    );
  });
});
