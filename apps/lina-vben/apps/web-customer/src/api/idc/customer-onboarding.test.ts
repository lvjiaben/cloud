import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { get: vi.fn(), post: vi.fn() };
vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer onboarding APIs', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps public verification in notify and authenticated redemption in core', async () => {
    const api = await import('./customer-onboarding');
    const binding = {
      browserNonce: 'browser-nonce-0001',
      customerType: 'individual' as const,
      destination: 'buyer@example.com',
      destinationKind: 'email' as const,
      locale: 'zh-CN' as const,
      returnPath: '/account',
      username: 'buyer01',
    };
    await api.createOnboardingHumanChallenge(binding);
    await api.getCustomerOnboardingStatus();
    await api.redeemCustomerOnboarding({
      browserNonce: binding.browserNonce,
      deviceId: 'device-00112233445566778899aabbccddeeff0011',
      hostRegistrationUserId: 18,
      token: 'opaque-token',
    });
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-notify/api/v1/public/account/onboarding/human-challenges',
      { ...binding, presentationKind: 'accessible' },
    );
    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/onboarding/status',
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/onboarding/redeem',
      {
        browserNonce: binding.browserNonce,
        deviceId: 'device-00112233445566778899aabbccddeeff0011',
        hostRegistrationUserId: 18,
        token: 'opaque-token',
      },
    );
  });
});
