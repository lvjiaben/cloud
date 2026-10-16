import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
};

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('personal verification customer APIs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps all identity operations inside the compliance account boundary', async () => {
    const {
      createPersonalVerificationConsent,
      getPersonalVerification,
      refreshPersonalVerification,
      startPersonalVerification,
    } = await import('./personal-verification');
    const root = '/x/idc-compliance/api/v1/account/compliance';

    await getPersonalVerification();
    await createPersonalVerificationConsent({
      idempotencyKey: 'personal-consent-test-0001',
      locale: 'zh-CN',
      policyVersion: 'personal-spi-v1',
    });
    await startPersonalVerification({
      certName: 'Test User',
      certNo: '11010519491231002X',
      consentNo: 'consent_test',
    });
    await refreshPersonalVerification('attempt/test');

    expect(requestClient.get).toHaveBeenCalledWith(
      `${root}/personal-verification`,
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      `${root}/personal-verification-consents`,
      {
        accepted: true,
        idempotencyKey: 'personal-consent-test-0001',
        locale: 'zh-CN',
        policyVersion: 'personal-spi-v1',
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${root}/personal-verifications`,
      {
        certName: 'Test User',
        certNo: '11010519491231002X',
        certType: 'IDENTITY_CARD',
        consentNo: 'consent_test',
      },
    );
    expect(requestClient.put).toHaveBeenCalledWith(
      `${root}/personal-verifications/attempt%2Ftest/authoritative-state`,
    );
  });
});
