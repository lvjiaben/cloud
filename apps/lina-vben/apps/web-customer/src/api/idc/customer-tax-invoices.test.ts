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

describe('customer tax invoice API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('loads only the current-customer picker route', async () => {
    const { getCustomerTaxInvoiceOptions } =
      await import('./customer-tax-invoices');
    await getCustomerTaxInvoiceOptions();
    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/tax-invoice-options',
    );
  });

  it('submits only profile, bill allocations, and the retry key', async () => {
    const { createCustomerTaxInvoice } =
      await import('./customer-tax-invoices');
    const input = {
      idempotencyKey: 'tax-invoice-create-0001',
      profileNo: 'profile-1',
      selections: [{ billingInvoiceNo: 'bill-1', requestedGrossMinor: '8800' }],
    };
    await createCustomerTaxInvoice(input);
    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/tax-invoices',
      input,
    );
    expect(input).not.toHaveProperty('invoiceTypeCode');
    expect(input).not.toHaveProperty('taxAmountMinor');
  });

  it('lists only through the current-customer history route', async () => {
    const { listCustomerTaxInvoices } = await import('./customer-tax-invoices');
    await listCustomerTaxInvoices({ cursor: 'next', limit: 20 });
    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-compliance/api/v1/account/compliance/tax-invoices',
      { params: { cursor: 'next', limit: 20 } },
    );
  });
});
