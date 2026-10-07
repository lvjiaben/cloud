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
  reason: string;
  tagNos: string[];
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
