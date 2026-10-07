import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-compliance';

export type CustomerContractStatus = 'completed' | 'failed' | 'processing';

export interface CustomerContractItem {
  amountMinor: string;
  canOpenSigning: boolean;
  canRecheck: boolean;
  contractNo: string;
  createdAt: number;
  currencyCode: string;
  languageCode: 'en-US' | 'zh-CN';
  orderNo: string;
  signingExpiresAt: number;
  statusCode: CustomerContractStatus;
  templateName: string;
  version: number;
}

export interface CustomerContractPage {
  items: CustomerContractItem[];
  nextCursor: string;
}

export interface CustomerSigningLinkItem {
  contract: CustomerContractItem;
  expiresAt: number;
  url: string;
}

function path(pathName = 'account/compliance/contracts') {
  return pluginApiPath(pluginID, pathName);
}

export function listCustomerContracts(params: {
  cursor?: string;
  limit?: number;
} = {}) {
  return requestClient.get<CustomerContractPage>(path(), { params });
}

export function getCustomerOrderContract(orderNo: string) {
  return requestClient.get<{ item: CustomerContractItem }>(
    path(`account/compliance/orders/${encodeURIComponent(orderNo)}/contract`),
  );
}

export function createCustomerContractSigningLink(contractNo: string) {
  return requestClient.post<{ item: CustomerSigningLinkItem }>(
    path(
      `account/compliance/contracts/${encodeURIComponent(contractNo)}/signing-links`,
    ),
  );
}

export function recheckCustomerContract(contractNo: string) {
  return requestClient.put<{ item: CustomerContractItem }>(
    path(
      `account/compliance/contracts/${encodeURIComponent(contractNo)}/authoritative-state`,
    ),
  );
}
