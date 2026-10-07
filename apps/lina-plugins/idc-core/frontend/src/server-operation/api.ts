import type { PluginHostMountContext } from '../host-context';

import { createPluginRequest } from '../runtime/request';

const base = '/x/idc-core/api/v1/admin/resource/server-operation-policy';

export type ServerOperationPolicy = {
  allowCustomerTransfer: boolean;
  requireHighRiskReauth: boolean;
  updatedAt: string;
  version: number;
};

export function createServerOperationPolicyAPI(
  currentContext: () => PluginHostMountContext,
) {
  const request = createPluginRequest(currentContext);
  return {
    get() {
      return request<{ policy: ServerOperationPolicy }>(base).then(
        ({ policy }) => policy,
      );
    },
    update(requireHighRiskReauth: boolean, allowCustomerTransfer: boolean, expectedVersion: number) {
      return request<{ policy: ServerOperationPolicy }>(base, {
        body: { allowCustomerTransfer, expectedVersion, requireHighRiskReauth },
        method: 'PUT',
      }).then(({ policy }) => policy);
    },
  };
}
