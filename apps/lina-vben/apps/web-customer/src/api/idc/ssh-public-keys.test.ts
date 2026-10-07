import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = { delete: vi.fn(), get: vi.fn(), post: vi.fn() };

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('customer SSH public-key API', () => {
  beforeEach(() => vi.clearAllMocks());

  it('uses ownership-only list and create routes', async () => {
    const { createSSHPublicKeyApi, listSSHPublicKeysApi } =
      await import('./ssh-public-keys');
    await listSSHPublicKeysApi();
    await createSSHPublicKeyApi({
      authorizedKey: 'ssh-ed25519 AAAA...',
      name: 'Work laptop',
      requestNo: 'ssh-key-request-0001',
    });
    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/ssh-public-keys',
    );
    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/ssh-public-keys',
      {
        authorizedKey: 'ssh-ed25519 AAAA...',
        name: 'Work laptop',
        requestNo: 'ssh-key-request-0001',
      },
    );
  });

  it('encodes the owned key number and sends the optimistic version when revoking', async () => {
    const { revokeSSHPublicKeyApi } = await import('./ssh-public-keys');
    await revokeSSHPublicKeyApi('SSH/key 1', {
      expectedVersion: 3,
      requestNo: 'ssh-revoke-request-0001',
    });
    expect(requestClient.delete).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/ssh-public-keys/SSH%2Fkey%201',
      {
        data: {
          expectedVersion: 3,
          requestNo: 'ssh-revoke-request-0001',
        },
      },
    );
  });
});
