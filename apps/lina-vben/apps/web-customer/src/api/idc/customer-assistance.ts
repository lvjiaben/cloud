import { pluginApiPath, requestClient } from '#/api/request';

export interface CustomerAssistanceExchange {
  accessToken: string;
  expiresAt: number;
  homePath: string;
  sessionNo: string;
}

export function verifyCustomerAssistance(accessToken: string) {
  return requestClient.post<CustomerAssistanceExchange>(
    pluginApiPath(
      'idc-core',
      'public/customer-assistance/access-verifications',
    ),
    { accessToken },
    { silentErrorMessage: true },
  );
}
