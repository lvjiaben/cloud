import { pluginApiPath, requestClient } from '#/api/request';

const base = pluginApiPath('idc-core', 'account/ssh-public-keys');

export interface SSHPublicKeyItem {
  algorithm: string;
  createdAt: number;
  fingerprint: string;
  keyNo: string;
  name: string;
  revokedAt: null | number;
  status: 'active' | 'revoked';
  version: number;
}

export async function listSSHPublicKeysApi() {
  return requestClient.get<{ items: SSHPublicKeyItem[] }>(base);
}

export async function createSSHPublicKeyApi(input: {
  authorizedKey: string;
  name: string;
  requestNo: string;
}) {
  return requestClient.post<{ item: SSHPublicKeyItem }>(base, input);
}

export async function revokeSSHPublicKeyApi(
  keyNo: string,
  input: { expectedVersion: number; requestNo: string },
) {
  return requestClient.delete<{ item: SSHPublicKeyItem }>(
    `${base}/${encodeURIComponent(keyNo)}`,
    { data: input },
  );
}
