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

describe('customer contract API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists only the current customer contracts with a bounded cursor', async () => {
    const { listCustomerContracts } = await import('./customer-contracts');
    requestClient.get.mockResolvedValue({ items: [], nextCursor: '' });

    await listCustomerContracts({ cursor: 'next', limit: 20 });

    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/contracts',
      { params: { cursor: 'next', limit: 20 } },
    );
  });

  it('uses separate actions for a fresh link and an authoritative recheck', async () => {
    const {
      createCustomerContractSigningLink,
      recheckCustomerContract,
    } = await import('./customer-contracts');
    requestClient.post.mockResolvedValue({ item: {} });
    requestClient.put.mockResolvedValue({ item: {} });

    await createCustomerContractSigningLink('CTR /1');
    await recheckCustomerContract('CTR /1');

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/contracts/CTR%20%2F1/signing-links',
    );
    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/contracts/CTR%20%2F1/authoritative-state',
    );
  });

  it('loads a customer-owned order contract without accepting a customer number', async () => {
    const { getCustomerOrderContract } = await import('./customer-contracts');
    requestClient.get.mockResolvedValue({ item: {} });

    await getCustomerOrderContract('ORD /1');

    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/orders/ORD%20%2F1/contract',
    );
  });
});
