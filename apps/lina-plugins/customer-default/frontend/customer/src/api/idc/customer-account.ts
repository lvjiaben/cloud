import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';

export type CustomerType = 'enterprise' | 'individual';
export type CustomerStatus =
  | 'active'
  | 'disabled'
  | 'pending_activation'
  | 'restricted';

export interface CustomerProfile {
  countryCode: string;
  customerCode: number;
  displayName: string;
  identifierMasked: string;
  legalNameMasked: string;
  preferredLocale: string;
  status: CustomerStatus;
  type: CustomerType;
  verification: string;
  version: number;
}

export interface ClosureFinding {
  code:
    | 'active_contracts'
    | 'active_services'
    | 'in_progress_operations'
    | 'legal_retention'
    | 'unsettled_funds'
    | 'unsettled_invoices';
  count: number;
}

export interface CustomerContact {
  contactNo: string;
  createdAt: number;
  displayName: string;
  email: string;
  emailVerified: boolean;
  phone: string;
  phoneCountryCode: string;
  phoneVerified: boolean;
  preferredLocale: string;
  status: 'active' | 'archived';
  updatedAt: number;
  version: number;
}

export interface CustomerContactSaveInput {
  displayName: string;
  email: string;
  idempotencyKey: string;
  phone: string;
  phoneCountryCode: string;
  preferredLocale: string;
}

export type CustomerAddressType =
  | 'billing'
  | 'contact'
  | 'registered'
  | 'service';

export interface CustomerAddress {
  addressLine1: string;
  addressLine2: string;
  addressNo: string;
  city: string;
  countryCode: string;
  createdAt: number;
  default: boolean;
  district: string;
  label: string;
  postalCode: string;
  province: string;
  recipientName: string;
  recipientPhone: string;
  status: 'active' | 'archived';
  type: CustomerAddressType;
  updatedAt: number;
  version: number;
}

export interface CustomerAddressSaveInput {
  addressLine1: string;
  addressLine2: string;
  city: string;
  countryCode: string;
  default: boolean;
  district: string;
  idempotencyKey: string;
  label: string;
  postalCode: string;
  province: string;
  recipientName: string;
  recipientPhone: string;
  type: CustomerAddressType;
}

export interface ClosureRequest {
  findings: ClosureFinding[];
  requestNo: string;
  requestedAt: number;
  status: 'blocked' | 'pending_review';
}

export interface DataExportStatus {
  expiresAt: number;
  requestNo: string;
  requestedAt: number;
  status: string;
}

function path(pathName: string) {
  return pluginApiPath(pluginID, pathName);
}

export function getCustomerProfile() {
  return requestClient.get<{ item: CustomerProfile }>(
    path('account/customer/profile'),
  );
}

export function updateCustomerProfile(input: {
  countryCode: string;
  displayName: string;
  expectedVersion: number;
  idempotencyKey: string;
  preferredLocale: string;
}) {
  return requestClient.put<{ item: CustomerProfile }>(
    path('account/customer/profile'),
    input,
  );
}

export function listCustomerContacts() {
  return requestClient.get<{ items: CustomerContact[] }>(
    path('account/customer/contacts'),
  );
}

export function createCustomerContact(input: CustomerContactSaveInput) {
  return requestClient.post<{ item: CustomerContact }>(
    path('account/customer/contacts'),
    input,
  );
}

export function updateCustomerContact(
  contactNo: string,
  input: CustomerContactSaveInput & { expectedVersion: number },
) {
  return requestClient.put<{ item: CustomerContact }>(
    path(`account/customer/contacts/${encodeURIComponent(contactNo)}`),
    input,
  );
}

export function archiveCustomerContact(
  contactNo: string,
  input: { expectedVersion: number; idempotencyKey: string },
) {
  return requestClient.post<{ item: CustomerContact }>(
    path(
      `account/customer/contacts/${encodeURIComponent(contactNo)}/archive`,
    ),
    input,
  );
}

export function listCustomerAddresses() {
  return requestClient.get<{ items: CustomerAddress[] }>(
    path('account/customer/addresses'),
  );
}

export function createCustomerAddress(input: CustomerAddressSaveInput) {
  return requestClient.post<{ item: CustomerAddress }>(
    path('account/customer/addresses'),
    input,
  );
}

export function updateCustomerAddress(
  addressNo: string,
  input: CustomerAddressSaveInput & { expectedVersion: number },
) {
  return requestClient.put<{ item: CustomerAddress }>(
    path(`account/customer/addresses/${encodeURIComponent(addressNo)}`),
    input,
  );
}

export function archiveCustomerAddress(
  addressNo: string,
  input: { expectedVersion: number; idempotencyKey: string },
) {
  return requestClient.post<{ item: CustomerAddress }>(
    path(
      `account/customer/addresses/${encodeURIComponent(addressNo)}/archive`,
    ),
    input,
  );
}

export function requestCustomerClosure(input: {
  confirmed: true;
  expectedVersion: number;
  idempotencyKey: string;
}) {
  return requestClient.post<{ item: ClosureRequest }>(
    path('account/customer/closure-requests'),
    input,
  );
}

export function createDataExportProof(input: {
  confirmed: true;
  expectedVersion: number;
  idempotencyKey: string;
}) {
  return requestClient.post<{ expiresAt: number; proofToken: string }>(
    path('account/customer/data-exports/proof'),
    input,
  );
}

export function requestDataExport(input: {
  confirmed: true;
  expectedVersion: number;
  idempotencyKey: string;
  proofToken: string;
}) {
  return requestClient.post<{
    requestNo: string;
    requestedAt: number;
    taskNo: string;
  }>(path('account/customer/data-exports'), input);
}

export function getDataExportStatus(requestNo: string) {
  return requestClient.get<DataExportStatus>(
    path(`account/customer/data-exports/${encodeURIComponent(requestNo)}`),
  );
}

export function createDataExportTicket(requestNo: string) {
  return requestClient.post<{ expiresAt: number; token: string }>(
    path(
      `account/customer/data-exports/${encodeURIComponent(requestNo)}/download-ticket`,
    ),
  );
}

export function dataExportDownloadURL(token: string) {
  return path(
    `account/customer/data-export-downloads/${encodeURIComponent(token)}`,
  );
}

export function createCustomerIdempotencyKey(prefix: string) {
  const random =
    typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : Array.from(crypto.getRandomValues(new Uint8Array(24)), (item) =>
          item.toString(16).padStart(2, '0'),
        ).join('');
  return `${prefix}-${random}`;
}
