import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-payment';

export interface CustomerPaymentChannel {
  channelNo: string;
  currencies: string[];
  displayName: string;
  modes: string[];
  provider: string;
}

export interface CustomerPaymentCreated {
  attemptNo: string;
  expiresAt: number;
  redirectFields: Record<string, string>;
  redirectMethod: string;
  redirectUrl: string;
  statusCode: string;
}

export interface CustomerPaymentStatus {
  attemptNo: string;
  balanceAmountMinor: string;
  channelAmountMinor: string;
  channelNo: string;
  confirmedAt?: number;
  currencyCode: string;
  expiresAt: number;
  modeCode: string;
  purposeCode: string;
  purposeReference: string;
  queryQueued: boolean;
  statusCode: string;
  totalAmountMinor: string;
}

function path(pathName: string) {
  return pluginApiPath(pluginID, `account/${pathName}`);
}

export function listCustomerPaymentChannels(currencyCode?: string) {
  return requestClient.get<{ channels: CustomerPaymentChannel[] }>(
    path('payment-channels'),
    { params: { currencyCode: currencyCode || undefined } },
  );
}

export function createCustomerBalanceRecharge(input: {
  amountMinor: string;
  channelNo: string;
  currencyCode: string;
  modeCode: string;
  openId?: string;
}) {
  return requestClient.post<CustomerPaymentCreated>(
    path('balance-recharges'),
    input,
  );
}

export function getCustomerPaymentStatus(attemptNo: string) {
  return requestClient.get<CustomerPaymentStatus>(
    path(`payments/${encodeURIComponent(attemptNo)}`),
  );
}
