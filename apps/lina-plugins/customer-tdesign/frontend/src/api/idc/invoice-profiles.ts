import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-core";

export type InvoiceProfileTitleType = "enterprise" | "individual";
export type InvoiceProfileStatus = "active" | "archived";
export type BankAccountAction = "clear" | "keep" | "replace";

export interface InvoiceProfileItem {
  bankAccountConfigured: boolean;
  bankAccountMasked?: string;
  bankName?: string;
  createdAt: number;
  invoiceTitle: string;
  isDefault: boolean;
  profileNo: string;
  registeredAddress?: string;
  registeredPhone?: string;
  statusCode: InvoiceProfileStatus;
  taxRegistrationNo?: string;
  titleTypeCode: InvoiceProfileTitleType;
  updatedAt: number;
  version: number;
}

export interface InvoiceProfilePage {
  items: InvoiceProfileItem[];
  nextCursor: string;
}

export interface InvoiceProfileCreateInput {
  bankAccount?: string;
  bankName?: string;
  idempotencyKey: string;
  invoiceTitle: string;
  isDefault: boolean;
  registeredAddress?: string;
  registeredPhone?: string;
  taxRegistrationNo?: string;
  titleTypeCode: InvoiceProfileTitleType;
}

export interface InvoiceProfileUpdateInput
  extends Omit<InvoiceProfileCreateInput, "isDefault"> {
  bankAccountAction: BankAccountAction;
  expectedVersion: number;
}

function path(pathName = "account/billing/invoice-profiles") {
  return pluginApiPath(pluginID, pathName);
}

export function listInvoiceProfiles(
  params: {
    cursor?: string;
    limit?: number;
  } = {},
) {
  return requestClient.get<InvoiceProfilePage>(path(), { params });
}

export function createInvoiceProfile(input: InvoiceProfileCreateInput) {
  return requestClient.post<{ item: InvoiceProfileItem }>(path(), input);
}

export function updateInvoiceProfile(
  profileNo: string,
  input: InvoiceProfileUpdateInput,
) {
  return requestClient.put<{ item: InvoiceProfileItem }>(
    path(`account/billing/invoice-profiles/${encodeURIComponent(profileNo)}`),
    input,
  );
}

export function setDefaultInvoiceProfile(
  profileNo: string,
  expectedVersion: number,
) {
  return requestClient.post<{ item: InvoiceProfileItem }>(
    path(
      `account/billing/invoice-profiles/${encodeURIComponent(profileNo)}/default`,
    ),
    { expectedVersion },
  );
}

export function deactivateInvoiceProfile(
  profileNo: string,
  expectedVersion: number,
) {
  return requestClient.post<{ item: InvoiceProfileItem }>(
    path(
      `account/billing/invoice-profiles/${encodeURIComponent(profileNo)}/deactivation`,
    ),
    { expectedVersion },
  );
}
