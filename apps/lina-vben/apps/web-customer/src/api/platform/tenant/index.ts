import type {
  PlatformTenant,
  PlatformTenantListParams,
  TenantImpersonationResult,
} from './model';

import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'tenant-core';

export async function platformTenantList(params?: PlatformTenantListParams) {
  const res = await requestClient.get<{
    list: PlatformTenant[];
    total: number;
  }>(pluginApiPath(pluginID, 'platform/tenants'), { params });
  return { items: res.list, total: res.total };
}

export function platformTenantImpersonate(id: number) {
  return requestClient.post<TenantImpersonationResult>(
    pluginApiPath(pluginID, `platform/tenants/${id}/impersonate`),
  );
}

export function platformTenantEndImpersonate(id: number) {
  return requestClient.post(
    pluginApiPath(pluginID, `platform/tenants/${id}/end-impersonate`),
  );
}
