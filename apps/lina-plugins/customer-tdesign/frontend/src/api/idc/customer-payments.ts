import { pluginApiPath, requestClient } from "#/api/request";

import { requestWithHttpStatus } from "./request-with-http-status";

const pluginID = "idc-payment";

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
  invoiceNo?: string;
  orderNo?: string;
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

export function listCustomerPaymentChannels(
  currencyCode?: string,
  silentErrorMessage = false,
) {
  return requestWithHttpStatus<{ channels: CustomerPaymentChannel[] }>(
    requestClient,
    path("payment-channels"),
    {
      method: "GET",
      silentErrorMessage,
      params: { currencyCode: currencyCode || undefined },
    },
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
    path("balance-recharges"),
    input,
  );
}

export function getCustomerPaymentStatus(attemptNo: string) {
  return requestClient.get<CustomerPaymentStatus>(
    path(`payments/${encodeURIComponent(attemptNo)}`),
    { silentErrorMessage: true },
  );
}

/** Create a channel payment against the server-owned order payment context. */
export function createCustomerOrderPayment(input: {
  paymentContextNo: string;
  channelNo: string;
  modeCode: string;
  balanceAmountMinor: string;
}) {
  return requestClient.post<CustomerPaymentCreated>(path("payments"), input);
}
