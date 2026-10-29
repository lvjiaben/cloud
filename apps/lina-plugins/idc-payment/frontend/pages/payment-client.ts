import { pluginApiPath, requestClient } from "#/api/request";

export interface PaymentHealth { capabilityCode: string; channelNo: string; displayName: string; healthCode: string; latestBusinessDate?: number; providerCode: string; resultUnknown: boolean; unresolvedCount: number }
export interface PaymentBatch { batchNo: string; capabilityCode: string; channelNo: string; differenceCount: number; paymentCount: number; refundCount: number; resultUnknown: boolean; statusCode: string; businessDate: number }
export interface PaymentDifference { attemptNo: string; channelAmountMinor: string; channelStatusCode: string; currencyCode: string; differenceCode: string; externalReference: string; lineNo: string; localAmountMinor: string; localStatusCode: string; refundAttemptNo: string; resolutionCode: string; supplementRequestNo: string; supplementStatusCode: string; supplementVersion: number; version: number }
export interface ExceptionFund { actionCode: string; actionStatus: string; amountMinor: string; attemptNo: string; channelNo: string; createdAt: number; currencyCode: string; exceptionNo: string; orderNo: string; providerCode: string; reasonCode: string; statusCode: string; version: number }
export interface PaymentChannel {
  channelNo: string;
  currencies: string[];
  displayName: string;
  endpointOrigin: string;
  merchantReference: string;
  modes: string[];
  providerCode: "alipay" | "epay" | "unionpay" | "wechatpay";
  statusCode: string;
  verifiedAt: number;
  version: number;
}

export interface PaymentConfigurationField {
  key: string;
  label?: { enUS?: string; zhCN?: string };
  maximumLength?: number;
  minimumLength?: number;
  required?: boolean;
  staticCandidates?: Array<{
    label?: { enUS?: string; zhCN?: string };
    value: string;
  }>;
  type: string;
}

export interface PaymentProviderCandidate {
  currencies: string[];
  displayName: string;
  endpointOrigin: string;
  interfaceConfiguration?: {
    fields?: PaymentConfigurationField[];
    version: string;
  };
  modes: string[];
  pluginId: string;
  providerCode: PaymentChannel["providerCode"];
  releaseVersion: string;
}

export interface PaymentChannelCreateInput {
  apiV3Key?: string;
  appId?: string;
  certificateSerialNo?: string;
  channelNo: string;
  displayName: string;
  enabledModes: string[];
  endpointOrigin: string;
  jsapiAuthorized?: boolean;
  merchantId: string;
  merchantReference?: string;
  privateKey?: string;
  providerCode: PaymentChannel["providerCode"];
  publicKey?: string;
  secret?: string;
  wechatPayPublicKeyId?: string;
}

const api = (path: string) => pluginApiPath("idc-payment", path);
export interface PaymentPublicAddress { notificationOrigin: string; customerReturnOrigin: string; version: number }
export const getPaymentPublicAddresses = () => requestClient.get<{addresses: PaymentPublicAddress}>(api('/admin/payment-public-addresses'));
export const savePaymentPublicAddresses = (input: PaymentPublicAddress) => requestClient.put<{addresses: PaymentPublicAddress}>(api('/admin/payment-public-addresses'), {notificationOrigin: input.notificationOrigin, customerReturnOrigin: input.customerReturnOrigin, expectedVersion: input.version});
export const getPaymentChannelConfigurations = (page = 1, pageSize = 20) =>
  requestClient.get<{
    channels: PaymentChannel[];
    providerCandidates: PaymentProviderCandidate[];
    total: number;
  }>(api("/admin/payment-channels"), { params: { page, pageSize } });
export const createPaymentChannelConfiguration = (input: PaymentChannelCreateInput) =>
  requestClient.post<{ channel: PaymentChannel }>(api("/admin/payment-channels"), input);
export const setPaymentChannelStatus = (
  channel: PaymentChannel,
  statusCode: "disabled" | "enabled",
) =>
  requestClient.put<{ channel: PaymentChannel }>(
    api(`/admin/payment-channels/${encodeURIComponent(channel.channelNo)}/status`),
    { expectedVersion: channel.version, statusCode },
  );
export const savePaymentChannelModes = (channel: PaymentChannel, enabledModes: string[]) =>
  requestClient.put<{channel: PaymentChannel}>(api(`/admin/payment-channels/${encodeURIComponent(channel.channelNo)}/payment-modes`), {enabledModes, expectedVersion: channel.version});
export const verifyPaymentChannelConnection = (channel: PaymentChannel) =>
  requestClient.post<{ channel: PaymentChannel }>(
    api(
      `/admin/payment-channels/${encodeURIComponent(channel.channelNo)}/connection-verifications`,
    ),
    { expectedVersion: channel.version },
  );
export const getPaymentChannels = (page = 1, pageSize = 20) => requestClient.get<{ channels: PaymentHealth[]; total: number }>(api("/admin/payment-reconciliation/channels"), { params: { page, pageSize } });
export const getPaymentBatches = (channelNo = "", page = 1, pageSize = 20) => requestClient.get<{ batches: PaymentBatch[]; total: number }>(api("/admin/payment-reconciliation/batches"), { params: { channelNo, page, pageSize } });
export const getPaymentDifferences = (batchNo: string, page = 1, pageSize = 20) => requestClient.get<{ differences: PaymentDifference[]; total: number }>(api(`/admin/payment-reconciliation/batches/${encodeURIComponent(batchNo)}/differences`), { params: { page, pageSize } });
export const submitPaymentSupplement = (item: PaymentDifference) => requestClient.post(api(`/admin/payment-reconciliation/differences/${encodeURIComponent(item.lineNo)}/supplements`), { expectedVersion: item.version });
export const reviewPaymentSupplement = (item: PaymentDifference, decisionCode: "approve" | "reject", rejectionReasonCode = "") => requestClient.post(api(`/admin/payment-reconciliation/supplements/${encodeURIComponent(item.supplementRequestNo)}/review`), { decisionCode, rejectionReasonCode, expectedVersion: item.supplementVersion });
export const getExceptionFunds = (page = 1, pageSize = 20) => requestClient.get<{ funds: ExceptionFund[]; total: number }>(api("/admin/payment-exception-funds"), { params: { page, pageSize } });
export const allocateExceptionFund = (item: ExceptionFund, targetOrderNo: string) => requestClient.post(api(`/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/allocation`), { targetOrderNo });
export const refundExceptionFund = (item: ExceptionFund) => requestClient.post(api(`/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/original-refund`), {});
export const uploadExceptionFundProof = (item: ExceptionFund, body: FormData) => requestClient.post<{ proofDigest: string; proofReference: string }>(api(`/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/manual-refund-proof`), body);
export const confirmExceptionFundRefund = (item: ExceptionFund, proofReference: string, proofDigest: string) => requestClient.post(api(`/admin/payment-exception-funds/${encodeURIComponent(item.exceptionNo)}/manual-refund-confirmation`), { expectedVersion: item.version, proofDigest, proofReference });
