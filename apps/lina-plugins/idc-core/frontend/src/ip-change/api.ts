import type { PluginHostMountContext } from '../host-context';

import { createPluginRequest } from '../runtime/request';

const base = '/x/idc-core/api/v1/admin/resource/ip-change-policies';

export type IPChangePolicy = {
  amountMinor: number;
  currencyCode: string;
  enabled: boolean;
  freeChangesPerWindow: number;
  maxChangesPerDay: number;
  maximumSnapshotAgeSeconds: number;
  policyNo: string;
  publishedAt: number;
  version: number;
  windowDays: number;
};

export type IPChangePolicyInput = Omit<
  IPChangePolicy,
  'policyNo' | 'publishedAt' | 'version'
> & { expectedVersion: number };

export function createIPChangePolicyAPI(
  currentContext: () => PluginHostMountContext,
) {
  const request = createPluginRequest(currentContext);
  return {
    list() {
      return request<{ items: IPChangePolicy[] }>(base).then(
        ({ items }) => items ?? [],
      );
    },
    publish(input: IPChangePolicyInput) {
      return request<{ policy: IPChangePolicy }>(base, {
        body: input,
        method: 'POST',
      }).then(({ policy }) => policy);
    },
  };
}
