import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { delete: vi.fn(), get: vi.fn(), post: vi.fn(), put: vi.fn() };
vi.mock('#/api/request', () => ({ pluginApiPath: (id: string, route: string) => `/x/${id}/api/v1/${route}`, requestClient }));

describe('purchase configuration API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('uses the public catalog maximum bounded cursor page', async () => {
    const { listPublicCatalogProducts } = await import('./purchase-configuration');
    requestClient.get.mockResolvedValue({ nextCursor: 'next', products: [] });
    await listPublicCatalogProducts('cursor');
    expect(requestClient.get).toHaveBeenCalledWith('/x/idc-core/api/v1/public/catalog/products', { params: { cursor: 'cursor', limit: 24 } });
  });
});
