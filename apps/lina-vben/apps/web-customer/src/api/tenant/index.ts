import type { LoginTenant, TenantTokenResult } from './model';

import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'tenant-core';

export async function authLoginTenants(userId: number) {
  const res = await requestClient.get<{ list: LoginTenant[] }>(
    pluginApiPath(pluginID, 'auth/login-tenants'),
    { params: { userId } },
  );
  return res.list;
}

export function authSelectTenant(preToken: string, tenantId: number) {
  return requestClient.post<TenantTokenResult>(
    pluginApiPath(pluginID, 'auth/select-tenant'),
    {
      preToken,
      tenantId,
    },
  );
}

export function authSwitchTenant(targetTenantId: number) {
  return requestClient.post<TenantTokenResult>(
    pluginApiPath(pluginID, 'auth/switch-tenant'),
    {
      tenantId: targetTenantId,
    },
  );
}
