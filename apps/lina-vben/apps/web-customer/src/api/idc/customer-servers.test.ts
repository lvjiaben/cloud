import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { get: vi.fn(), post: vi.fn() };

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer server API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('uses current-customer list and encoded detail routes', async () => {
    const { getCustomerServer, listCustomerServers } =
      await import('./customer-servers');
    await listCustomerServers({ cursor: 'next', limit: 20 });
    await getCustomerServer('service/one');
    expect(requestClient.get).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/servers',
      { params: { cursor: 'next', limit: 20 } },
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/servers/service%2Fone',
    );
  });

  it('keeps authorization and creation on the same frozen intent', async () => {
    const { authorizeServerOperation, createServerOperation } =
      await import('./customer-servers');
    const intent = {
      action: 'reinstall' as const,
      parameters: { os: 101 },
      requestNo: 'server-operation-0001',
    };
    await authorizeServerOperation('service-1', intent);
    await createServerOperation('service-1', intent, { proofToken: 'proof' });
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/servers/service-1/operation-authorizations',
      intent,
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/servers/service-1/operations',
      { ...intent, proofToken: 'proof' },
    );
  });

  it('requires an explicit confirmation to consume confidential output', async () => {
    const { redeemServerOperationOutput } = await import('./customer-servers');
    await redeemServerOperationOutput('service-1', 'operation/1');
    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/servers/service-1/operations/operation%2F1/confidential-output',
      { confirmed: true },
    );
  });

  it('uses separate quote and checkout routes for chargeable accessories', async () => {
    const { checkoutServerAccessoryQuote, createServerAccessoryQuote } =
      await import('./customer-servers');
    await createServerAccessoryQuote(
      'service/1',
      'snapshot_create',
      { diskObjectNo: 'disk-1', name: 'before-upgrade' },
      'request-1',
    );
    await checkoutServerAccessoryQuote('quote/1', 'checkout-1');
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/servers/service%2F1/accessory-quotes',
      {
        action: 'snapshot_create',
        idempotencyKey: 'request-1',
        parameters: { diskObjectNo: 'disk-1', name: 'before-upgrade' },
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/server-accessory-quotes/quote%2F1/checkout',
      { idempotencyKey: 'checkout-1' },
    );
  });
});
