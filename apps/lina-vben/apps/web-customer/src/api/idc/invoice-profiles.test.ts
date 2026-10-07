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

describe('customer invoice profile API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists only through the current-customer account route', async () => {
    const { listInvoiceProfiles } = await import('./invoice-profiles');
    await listInvoiceProfiles({ cursor: 'next', limit: 20 });
    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/billing/invoice-profiles',
      { params: { cursor: 'next', limit: 20 } },
    );
  });

  it('keeps bank-account replacement explicit on update', async () => {
    const { updateInvoiceProfile } = await import('./invoice-profiles');
    await updateInvoiceProfile('IVP /1', {
      bankAccount: '6222000012345678',
      bankAccountAction: 'replace',
      bankName: 'Example Bank',
      expectedVersion: 3,
      idempotencyKey: 'invoice-profile-update-0001',
      invoiceTitle: 'Example Co.',
      taxRegistrationNo: 'TAX-1',
      titleTypeCode: 'enterprise',
    });
    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/billing/invoice-profiles/IVP%20%2F1',
      expect.objectContaining({
        bankAccountAction: 'replace',
        expectedVersion: 3,
      }),
    );
  });

  it('uses separate default and deactivation actions', async () => {
    const { deactivateInvoiceProfile, setDefaultInvoiceProfile } =
      await import('./invoice-profiles');
    await setDefaultInvoiceProfile('IVP /1', 2);
    await deactivateInvoiceProfile('IVP /1', 3);
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/billing/invoice-profiles/IVP%20%2F1/default',
      { expectedVersion: 2 },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/billing/invoice-profiles/IVP%20%2F1/deactivation',
      { expectedVersion: 3 },
    );
  });
});
