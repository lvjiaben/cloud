import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = {
  get: vi.fn(),
  post: vi.fn(),
};

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('enterprise contract signer API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps signer maintenance inside the compliance account boundary', async () => {
    const {
      createContractSigner,
      disableContractSigner,
      listContractSigners,
    } = await import('./contract-signers');
    const input = {
      certName: '测试经办人',
      certNo: '110105199001011234',
      certType: 'IDENTITY_CARD' as const,
      consentAccepted: true as const,
      consentPolicyVersion: '1' as const,
      idempotencyKey: 'signer-create-0001',
      phone: '13800138000',
    };

    await listContractSigners();
    await createContractSigner(input);
    await disableContractSigner('csgn_test', {
      expectedVersion: 2,
      idempotencyKey: 'signer-disable-0001',
    });

    const root =
      '/x/idc-compliance/api/v1/account/compliance/contract-signers';
    expect(requestClient.get).toHaveBeenCalledWith(root);
    expect(requestClient.post).toHaveBeenNthCalledWith(1, root, input);
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${root}/csgn_test/disablements`,
      { expectedVersion: 2, idempotencyKey: 'signer-disable-0001' },
    );
  });

  it('uses explicit identity start and authoritative check commands', async () => {
    const { checkContractSignerIdentity, startContractSignerIdentity } =
      await import('./contract-signers');

    await startContractSignerIdentity('csgn_test');
    await checkContractSignerIdentity('csgn_test', 'csia_test');

    const root =
      '/x/idc-compliance/api/v1/account/compliance/contract-signers/csgn_test/identity-verifications';
    expect(requestClient.post).toHaveBeenNthCalledWith(1, root);
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${root}/csia_test/authoritative-checks`,
    );
  });

  it('sends phone codes only through the frozen human-check and verification flow', async () => {
    const {
      completeContractSignerPhoneChallenge,
      createContractSignerPhoneChallenge,
      sendContractSignerPhoneCode,
      verifyContractSignerPhone,
    } = await import('./contract-signers');
    const root =
      '/x/idc-compliance/api/v1/account/compliance/contract-signers/csgn_test';

    await createContractSignerPhoneChallenge('csgn_test', {
      browserNonce: 'browser-nonce',
      locale: 'zh-CN',
      presentationKind: 'accessible',
    });
    await completeContractSignerPhoneChallenge('csgn_test', {
      answer: '5',
      browserNonce: 'browser-nonce',
      challengeNo: 'challenge-1',
      locale: 'zh-CN',
    });
    await sendContractSignerPhoneCode('csgn_test', {
      browserNonce: 'browser-nonce',
      idempotencyKey: 'signer-code-send-0001',
      locale: 'zh-CN',
      proofToken: 'proof-1',
    });
    await verifyContractSignerPhone('csgn_test', {
      browserNonce: 'browser-nonce',
      code: '123456',
      locale: 'zh-CN',
      verificationNo: 'verification-1',
    });

    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      `${root}/phone-human-challenges`,
      {
        browserNonce: 'browser-nonce',
        locale: 'zh-CN',
        presentationKind: 'accessible',
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${root}/phone-human-challenge-completions`,
      {
        answer: '5',
        browserNonce: 'browser-nonce',
        challengeNo: 'challenge-1',
        locale: 'zh-CN',
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      3,
      `${root}/phone-verification-codes`,
      {
        browserNonce: 'browser-nonce',
        idempotencyKey: 'signer-code-send-0001',
        locale: 'zh-CN',
        proofToken: 'proof-1',
      },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      4,
      `${root}/phone-verifications`,
      {
        browserNonce: 'browser-nonce',
        code: '123456',
        locale: 'zh-CN',
        verificationNo: 'verification-1',
      },
    );
  });
});
