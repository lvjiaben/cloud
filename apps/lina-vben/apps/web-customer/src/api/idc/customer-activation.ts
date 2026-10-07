import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';

export interface CustomerActivationResult {
  customerNo: string;
  customerType: 'enterprise' | 'individual';
  reused: boolean;
  status: 'active';
}

export function redeemCustomerActivation(token: string) {
  return requestClient.post<CustomerActivationResult>(
    pluginApiPath(pluginID, 'account/activation-invitations/redeem'),
    { token },
  );
}

export function getInitialPasswordStatus() {
  return requestClient.get<{ required: boolean }>(
    pluginApiPath(pluginID, 'account/initial-password/status'),
  );
}

export function changeInitialPassword(newPassword: string) {
  return requestClient.put<{ required: false }>(
    pluginApiPath(pluginID, 'account/initial-password'),
    { newPassword },
  );
}
