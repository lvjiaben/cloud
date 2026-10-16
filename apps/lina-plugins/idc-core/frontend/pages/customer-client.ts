import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export type CustomerType = "enterprise" | "individual";
export type CustomerStatus =
  | "active"
  | "anonymized"
  | "disabled"
  | "pending_activation"
  | "restricted";
export type VerificationStatus =
  | "expired"
  | "pending"
  | "rejected"
  | "revoked"
  | "unverified"
  | "verified";

export interface CustomerLevel {
  code: string;
  levelNo: string;
  nameEnUs: string;
  nameZhCn: string;
  priority: number;
}

export interface CustomerSummary {
 balances?: CustomerBalanceSummary[];
  countryCode: string;
  createdAt: number;
  customerNo: string;
  customerCode: string;
  displayName: string;
  enterpriseContractPolicyCode: "none" | "required" | "";
  enterpriseContractTemplateNo: string;
  level: CustomerLevel | null;
  preferredLocale: string;
  primaryPhone: string;
  primaryEmail: string;
  status: CustomerStatus;
  tagCount: number;
  type: CustomerType;
  updatedAt: number;
  verification: VerificationStatus;
  version: number;
}

export interface CustomerSegment {
  code: string;
  colorToken: string;
  nameEnUs: string;
  nameZhCn: string;
  number: string;
  status: "active" | "archived";
}

export interface CustomerSegmentPreview {
  hasMore: boolean;
  items: CustomerSegment[];
}

export interface CustomerOverviewCounts {
  activeAddresses: number;
  activeBlacklist: number;
  activeContacts: number;
  activeCustomFields: number;
  activeGroups: number;
  activeNotes: number;
  activeTags: number;
  availableAttachments: number;
  openRisks: number;
  plannedFollowUps: number;
}

export interface CustomerOverview {
 statistics?: CustomerOperationsStatistics;
  balances: CustomerBalanceSummary[];
  business: CustomerBusinessSummary;
  counts: CustomerOverviewCounts;
  customer: CustomerSummary;
  groups: CustomerSegmentPreview;
  tags: CustomerSegmentPreview;
}

export interface CustomerBalanceSummary {
  accountTypeCode: string;
  availableBalanceMinor: string;
  balanceMinor: string;
  currencyCode: string;
  heldBalanceMinor: string;
  statusCode: string;
  updatedAt: number;
}

export interface CustomerBusinessSummary {
  audits: number;
  bills: number;
  contracts: number;
  notifications: number;
  orders: number;
  servers: number;
  taxInvoices: number;
  tickets: number;
  transactions: number;
}

export interface CustomerReferrer {
  customerCode: string;
  customerNo: string;
  displayName: string;
}

export interface CustomerListParams {
  page?: number;
  customerCode?: string;
  cursor?: string;
  groupNo?: string;
  limit?: number;
  namePrefix?: string;
  phone?: string;
  status?: CustomerStatus | "";
  tagNo?: string;
  type?: CustomerType | "";
}

export interface CustomerSegmentListParams {
  cursor?: string;
  kind: "group" | "tag";
  limit?: number;
  locale: "en-US" | "zh-CN";
  namePrefix?: string;
}

export interface CustomerSegmentListResult {
  items: CustomerSegment[];
  nextCursor: string;
}

export interface CustomerListResult {
  total: number;
  customers: CustomerSummary[];
  nextCursor: string;
}

export type CustomerTagBatchAction = "assign" | "revoke";

export interface CustomerTagBatchInput {
  action: CustomerTagBatchAction;
  customerNos: string[];
  idempotencyKey: string;
  reason?: string;
  tagNos: string[];
  tagNames?: string[];
}

export interface CustomerTagBatchResult {
  auditNo: string;
  customerCount: number;
  replayed: boolean;
  tagCount: number;
}

export interface CustomerNoteInput {
  category: "billing" | "compliance" | "general" | "risk" | "sales" | "service";
  content: string;
  supersedesNo?: string;
  idempotencyKey: string;
}

export interface CustomerFollowUpInput {
  assignedUserId: number;
  channel:
    | "email"
    | "meeting"
    | "other"
    | "phone"
    | "sms"
    | "ticket"
    | "wechat";
  detail: string;
  plannedAt: number;
  subject: string;
  idempotencyKey: string;
}

export interface CustomerBlacklistInput {
  evidenceReference: string;
  expiresAt: number;
  reasonCode: string;
  reasonDetail: string;
  riskLevel: "critical" | "high" | "low" | "medium";
  scope:
    | "all_business"
    | "purchase"
    | "refund"
    | "registration"
    | "resource_operation";
  startsAt: number;
  idempotencyKey: string;
}

export type CustomerAccountTransitionReason =
  | "account_reactivated"
  | "activation_completed"
  | "manual_disable"
  | "restriction_resolved"
  | "risk_restriction"
  | "verification_required";

export interface CustomerAccountTransitionResult {
  customerNo: string;
  fromStatus: CustomerStatus;
  occurredAt: number;
  reason: CustomerAccountTransitionReason;
  subjectVersion: number;
  toStatus: CustomerStatus;
}

export type CustomerActivityKind = "blacklist" | "follow_up" | "note";

export interface CustomerActivityItem {
  category: string;
  createdAt: number;
  detail: string;
  kind: CustomerActivityKind;
  number: string;
  plannedAt: number;
  riskLevel: string;
  status: string;
  title: string;
  updatedAt: number;
}

export interface CustomerActivityResult {
  items: CustomerActivityItem[];
  nextCursor: string;
}

export interface CustomerAssistanceSession {
  customerNo: string;
  endedAt?: number;
  expiresAt: number;
  launchUrl?: string;
  sessionNo: string;
  startedAt: number;
  status: "active" | "ended" | "expired" | "superseded";
}

export interface CustomerActivationInvitation {
  customerNo: string;
  customerType: CustomerType;
  destinationHint: string;
  displayName: string;
  existingIdentityBound: boolean;
  expiresAt: number;
  invitationNo: string;
  reused: boolean;
  status: "active" | "consumed" | "expired" | "superseded";
}

export interface CustomerActivationCreateInput {
  bindExistingIdentity: boolean;
  countryCode: string;
  customerType: CustomerType;
  displayName: string;
  email?: string;
  idempotencyKey: string;
  phone?: string;
  phoneCountryCode?: string;
  preferredLocale: "en-US" | "zh-CN";
  reason: string;
}

export interface CustomerDirectCreateInput {
  countryCode: string;
  customerType: CustomerType;
  displayName: string;
  email?: string;
  idempotencyKey: string;
  initialPassword: string;
  phone?: string;
  phoneCountryCode?: string;
  preferredLocale: "en-US" | "zh-CN";
  reason: string;
}

export function customerActivationCreate(data: CustomerActivationCreateInput) {
  return requestClient.post<CustomerActivationInvitation>(
    pluginApiPath(pluginID, "admin/customers/activation-invitations"),
    data,
  );
}

export function customerActivationResend(
  customerNo: string,
  data: { idempotencyKey: string; reason: string },
) {
  return requestClient.post<CustomerActivationInvitation>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/activation-invitations/resend`,
    ),
    data,
  );
}

export function customerDirectCreate(data: CustomerDirectCreateInput) {
  return requestClient.post<{
    customerNo: string;
    reused: boolean;
    status: "active";
    userId: number;
  }>(pluginApiPath(pluginID, "admin/customers/direct-accounts"), data);
}

export function customerPasswordReset(
  customerNo: string,
  data: { idempotencyKey: string; newPassword: string; reason: string },
) {
  return requestClient.put(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/password`,
    ),
    data,
  );
}

export type CustomerAssistanceEndResult = Pick<
  CustomerAssistanceSession,
  "customerNo" | "endedAt" | "sessionNo" | "status"
>;

export function customerAssistanceStart(
  customerNo: string,
  data: { idempotencyKey: string; reason: string; ticketNo?: string },
) {
  return requestClient.post<CustomerAssistanceSession>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/assistance-sessions`,
    ),
    data,
  );
}

export function customerAssistanceEnd(sessionNo: string) {
  return requestClient.post<CustomerAssistanceEndResult>(
    pluginApiPath(
      pluginID,
      `admin/customer-assistance/${encodeURIComponent(sessionNo)}/end`,
    ),
  );
}

export function customerList(params?: CustomerListParams) {
  return requestPreservingHttpStatus<CustomerListResult>(requestClient,
    pluginApiPath(pluginID, "admin/customers"),
    { method: "GET", params },
  );
}

export function customerOverview(customerNo: string) {
  return requestClient.get<CustomerOverview>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}`,
    ),
  );
}

export function customerAccountTransition(
  customerNo: string,
  data: {
    expectedVersion: number;
    idempotencyKey: string;
    reason: CustomerAccountTransitionReason;
    toStatus: Exclude<CustomerStatus, "anonymized" | "pending_activation">;
  },
) {
  return requestClient.put<CustomerAccountTransitionResult>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/account-status`,
    ),
    data,
  );
}

export function customerReferrer(customerNo: string) {
  return requestClient.get<CustomerReferrer>(
    pluginApiPath(
      pluginID,
      `admin/sales-channel/customers/${encodeURIComponent(customerNo)}/referrer`,
    ),
  );
}

export function customerSegments(params: CustomerSegmentListParams) {
  return requestClient.get<CustomerSegmentListResult>(
    pluginApiPath(pluginID, "admin/customer-segments"),
    { params },
  );
}

export function customerAssignments(
  customerNo: string,
  params: Omit<CustomerSegmentListParams, "namePrefix">,
) {
  return requestClient.get<CustomerSegmentListResult>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/segments`,
    ),
    { params },
  );
}

export function customerActivities(
  customerNo: string,
  params: { cursor?: string; kind: CustomerActivityKind; limit?: number },
) {
  return requestClient.get<CustomerActivityResult>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/activities`,
    ),
    { params },
  );
}

export function customerTagBatch(data: CustomerTagBatchInput) {
  return requestClient.post<CustomerTagBatchResult>(
    pluginApiPath(pluginID, "admin/customers/tags/batch"),
    data,
  );
}

export function customerNoteCreate(
  customerNo: string,
  data: CustomerNoteInput,
) {
  return requestClient.post(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/notes`,
    ),
    data,
  );
}

export function customerFollowUpPlan(
  customerNo: string,
  data: CustomerFollowUpInput,
) {
  return requestClient.post(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/follow-ups`,
    ),
    data,
  );
}

export function customerFollowUpTransition(
  customerNo: string,
  followUpNo: string,
  data: {
    action: "cancel" | "complete";
    idempotencyKey: string;
    outcome?: string;
  },
) {
  return requestClient.post(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/follow-ups/${encodeURIComponent(followUpNo)}/transition`,
    ),
    data,
  );
}

export function customerBlacklistCreate(
  customerNo: string,
  data: CustomerBlacklistInput,
) {
  return requestClient.post(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/blacklist`,
    ),
    data,
  );
}

export function customerBlacklistDecision(
  customerNo: string,
  entryNo: string,
  data: { action: "approve" | "reject" | "revoke"; idempotencyKey: string },
) {
  return requestClient.post(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/blacklist/${encodeURIComponent(entryNo)}/decision`,
    ),
    data,
  );
}

export function customerSegmentAdd(customerNo: string, data: { kind: "group" | "tag"; segmentNo?: string; name?: string; idempotencyKey: string }) {
  return requestClient.post<{ segmentNo: string }>(pluginApiPath(pluginID, `admin/customers/${encodeURIComponent(customerNo)}/segments`), data);
}
export function customerSegmentRemove(customerNo: string, segmentNo: string, data: { kind: "group" | "tag"; idempotencyKey: string }) {
  return requestClient.delete(pluginApiPath(pluginID, `admin/customers/${encodeURIComponent(customerNo)}/segments/${encodeURIComponent(segmentNo)}`), { data });
}
export interface CustomerAuditRecord { auditNo: string; occurredAt: number; operationCode: string; result: string }
export function customerAuditRecords(customerNo: string) {
  return requestClient.get<{ items: CustomerAuditRecord[] }>(pluginApiPath(pluginID, `admin/customers/${encodeURIComponent(customerNo)}/audit-records`));
}

/** Apply one audited manual identity command, then refresh the authoritative overview. */
export function customerVerificationUpdate(customerNo: string, data: {action: "approve" | "clear"; expectedVersion: number; idempotencyKey: string}) {
 return requestClient.put<void>(pluginApiPath(pluginID, `admin/customers/${encodeURIComponent(customerNo)}/verification`),data);
}

export interface CustomerOperationsStatistics {
 currencies: {currencyCode:string;paidMinor:string;refundedMinor:string;unpaidMinor:string;unpaidCount:number}[];
 tickets:{total:number;processing:number;replied:number;closed:number};
 products:number;expiring:number;expired:number;
 productCategories: {productType:string;pluginId:string;total:number}[];
}
export interface CustomerProduct {deliveryKind:"server"|"entitlement"|"pending";number:string;orderNo:string;productNo:string;nameZhCn:string;nameEnUs:string;information:string;informationEnUs?:string;amountMinor:string;currencyCode:string;productType:string;pluginId:string;periodUnit:string;periodCount:number;createdAt:number;expiresAt:number|null;status:string}
export function customerProducts(customerNo:string,page=1,pageSize=10){return requestClient.get<{items:CustomerProduct[];total:number}>(pluginApiPath(pluginID,`/admin/customers/${encodeURIComponent(customerNo)}/products`),{params:{page,pageSize}})}
export interface CustomerAPIOverview {authorized:boolean;appId:string;displayName:string;status:string;tokenId:number;canReset:boolean;scopes:string[];sourceCidrs:string[];expiresAt:number|null}
export interface CustomerAPICall {id:number;method:string;operation:string;succeeded:boolean;durationMillis:number;createdAt:number}
const apiCustomerPath=(customerNo:string)=>pluginApiPath("idc-channel",`/admin/customers/${encodeURIComponent(customerNo)}/open-platform`);
export function customerAPIOverview(customerNo:string){return requestClient.get<CustomerAPIOverview>(apiCustomerPath(customerNo))}
export function customerAPICalls(customerNo:string,page=1,pageSize=10){return requestClient.get<{list:CustomerAPICall[];total:number}>(`${apiCustomerPath(customerNo)}/calls`,{params:{page,pageSize}})}
export function customerAPIReset(customerNo:string,tokenId:number){return requestClient.post<{appId:string;secret:string}>(`${apiCustomerPath(customerNo)}/secrets`,{tokenId})}
