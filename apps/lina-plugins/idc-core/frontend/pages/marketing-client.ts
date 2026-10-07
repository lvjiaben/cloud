import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export interface MarketingOverviewSummary {
  activeSegmentMembers: number;
  assetActive: number;
  assetConsumedUses: number;
  assetIssued: number;
  assetReservedUses: number;
  assetTotal: number;
  campaignActive: number;
  campaignConsumedUnits: number;
  campaignPaused: number;
  campaignReservedUnits: number;
  campaignScheduled: number;
  campaignTotal: number;
  consentOptedIn: number;
  consentOptedOut: number;
  pointsAccountCount: number;
  pointsAvailable: number;
  pointsDebt: number;
  pointsFrozen: number;
  segmentActive: number;
  segmentDynamic: number;
  segmentTotal: number;
}

export interface MarketingCurrencySummary {
  availableMinor: number;
  consumedMinor: number;
  currencyCode: string;
  expiredMinor: number;
  frozenMinor: number;
  grantCount: number;
  pendingMinor: number;
  reversedMinor: number;
}

export interface MarketingCampaignSummary {
  campaignNo: string;
  endsAt: number;
  globalConsumedCount: number;
  globalQuotaLimit: null | number;
  globalReservedCount: number;
  isNewcomerExclusive: boolean;
  nameEnUs: string;
  nameZhCn: string;
  priority: number;
  startsAt: number;
  status: string;
  targetSegmentNo: string;
  targetSnapshotNo: string;
  type: string;
  updatedAt: number;
}

export interface MarketingGrantSummary {
  availableMinor: number;
  campaignNo: string;
  consumedMinor: number;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  expiredMinor: number;
  expiresAt: null | number;
  frozenMinor: number;
  grantNo: string;
  grantType: string;
  originalMinor: number;
  pendingMinor: number;
  reversedMinor: number;
  status: string;
  updatedAt: number;
  withdrawable: boolean;
}

export interface MarketingOverview {
  currencies: MarketingCurrencySummary[];
  generatedAt: number;
  recentCampaigns: MarketingCampaignSummary[];
  recentGrants: MarketingGrantSummary[];
  summary: MarketingOverviewSummary;
}

export function marketingOverview() {
  return requestClient.get<MarketingOverview>(
    pluginApiPath(pluginID, "admin/marketing/overview"),
  );
}

export interface MarketingSegmentSummaryItem {
  activeMemberCount: number;
  conditionCount: number;
  createdAt: number;
  latestSnapshotAt: null | number;
  latestSnapshotMembers: number;
  latestSnapshotNo: string;
  latestSnapshotStatus: string;
  mode: string;
  nameEnUs: string;
  nameZhCn: string;
  refreshIntervalMinutes: null | number;
  segmentCode: string;
  segmentNo: string;
  status: string;
  updatedAt: number;
  version: number;
}

export interface MarketingSegmentConditionValue {
  integer: null | number;
  text: string;
}

export interface MarketingSegmentCondition {
  conditionNo: string;
  currencyCode: string;
  fact: string;
  operator: string;
  orGroupNo: number;
  sortOrder: number;
  valueBoolean: boolean | null;
  valueIntegerLower: null | number;
  valueIntegerUpper: null | number;
  valueKind: string;
  valueText: string;
  values: MarketingSegmentConditionValue[];
}

export interface MarketingSegmentSnapshot {
  completedAt: null | number;
  memberCount: number;
  segmentVersion: number;
  snapshotNo: string;
  startedAt: number;
  status: string;
}

export interface MarketingSegmentDetail {
  conditions: MarketingSegmentCondition[];
  description: { enUs: string; zhCn: string };
  segment: MarketingSegmentSummaryItem;
  snapshots: MarketingSegmentSnapshot[];
}

export interface MarketingCampaignManagementItem {
  isNewcomerExclusive: boolean;
  benefitCount: number;
  campaignCode: string;
  campaignNo: string;
  createdAt: number;
  endsAt: number;
  globalConsumedCount: number;
  globalQuotaLimit: null | number;
  globalReservedCount: number;
  nameEnUs: string;
  nameZhCn: string;
  perCustomerQuotaLimit: null | number;
  priority: number;
  reservationCount: number;
  scopeCount: number;
  stackPolicy: string;
  startsAt: number;
  status: string;
  targetSegmentNo: string;
  targetSnapshotNo: string;
  type: string;
  updatedAt: number;
}

export interface MarketingCampaignScope {
  kind: string;
  matchMode: string;
  referenceCode: string;
}

export interface MarketingCampaignBenefit {
  amountMinor: null | number;
  benefitNo: string;
  currencyCode: string;
  cycleCount: null | number;
  cycleScope: string;
  maximumDiscountMinor: null | number;
  minimumSpendMinor: null | number;
  pointsQuantity: null | number;
  purchasePhase: string;
  rateBasisPoints: null | number;
  status: string;
  type: string;
  version: number;
}

export interface MarketingCampaignDetail {
  benefits: MarketingCampaignBenefit[];
  campaign: MarketingCampaignManagementItem;
  description: { enUs: string; zhCn: string };
  scopes: MarketingCampaignScope[];
  stackGroup: string;
  version: number;
}

export interface MarketingSegmentPage {
  total: number;
  nextCursor: string;
  segments: MarketingSegmentSummaryItem[];
}

export interface MarketingCampaignPage {
  total: number;
  campaigns: MarketingCampaignManagementItem[];
  nextCursor: string;
}

export function marketingSegments(params: {
  page?: number;
  cursor?: string;
  limit?: number;
  mode?: string;
  namePrefix?: string;
  status?: string;
}) {
  return requestPreservingHttpStatus<MarketingSegmentPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/segments"),
    { params },
  );
}

export function marketingSegmentDetail(segmentNo: string) {
  return requestClient.get<MarketingSegmentDetail>(
    pluginApiPath(
      pluginID,
      `admin/marketing/segments/${encodeURIComponent(segmentNo)}`,
    ),
  );
}

export function marketingCampaigns(params: {
  page?: number;
  cursor?: string;
  limit?: number;
  namePrefix?: string;
  status?: string;
  targetSegmentNo?: string;
  type?: string;
}) {
  return requestPreservingHttpStatus<MarketingCampaignPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/campaigns"),
    { params },
  );
}

export function marketingCampaignDetail(campaignNo: string) {
  return requestClient.get<MarketingCampaignDetail>(
    pluginApiPath(
      pluginID,
      `admin/marketing/campaigns/${encodeURIComponent(campaignNo)}`,
    ),
  );
}

export function updateMarketingCampaignNewcomer(input: {
  campaignNo: string;
  enabled: boolean;
  expectedVersion: number;
  requestNo: string;
}) {
  return requestClient.put<{
    campaignNo: string;
    enabled: boolean;
    updatedAt: number;
    version: number;
  }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/campaigns/${encodeURIComponent(input.campaignNo)}/newcomer-setting`,
    ),
    input,
  );
}

export interface MarketingPromotionAssetItem {
  assetNo: string;
  benefitNo: string;
  benefitType: string;
  campaignNo: string;
  claimedAt: null | number;
  consumedUses: number;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  faceAmountMinor: null | number;
  rateBasisPoints: null | number;
  reservedUses: number;
  status: string;
  type: string;
  updatedAt: number;
  usageLimit: number;
  validFrom: number;
  validUntil: number;
}

export interface MarketingAssetReservationItem {
  createdAt: number;
  discountAmountMinor: number;
  expiresAt: number;
  reservationNo: string;
  reservedUses: number;
  status: string;
}

export interface MarketingAssetPage {
  total: number;
  assets: MarketingPromotionAssetItem[];
  nextCursor: string;
}

export interface MarketingAssetDetail {
  asset: MarketingPromotionAssetItem;
  reservations: MarketingAssetReservationItem[];
}

export interface MarketingPointsAccountItem {
  availablePoints: number;
  createdAt: number;
  customerNo: string;
  debtPoints: number;
  entryCount: number;
  frozenPoints: number;
  lotCount: number;
  updatedAt: number;
  version: number;
}

export interface MarketingPointsLotItem {
  availablePoints: number;
  campaignNo: string;
  consumedPoints: number;
  createdAt: number;
  expiredPoints: number;
  expiresAt: null | number;
  frozenPoints: number;
  lotNo: string;
  originalPoints: number;
  reversalShortfallPoints: number;
  reversedPoints: number;
  sourceReference: string;
  sourceType: string;
  status: string;
}

export interface MarketingPointsEntryItem {
  availableDelta: number;
  createdAt: number;
  debtDelta: number;
  entryNo: string;
  frozenDelta: number;
  lotNo: string;
  reversalOfEntryNo: string;
  sourceReference: string;
  sourceType: string;
  type: string;
}

export interface MarketingPointsAccountPage {
  total: number;
  accounts: MarketingPointsAccountItem[];
  nextCursor: string;
}

export interface MarketingPointsAccountDetail {
  account: MarketingPointsAccountItem;
  entries: MarketingPointsEntryItem[];
  lots: MarketingPointsLotItem[];
}

export interface MarketingValueGrantItem {
  availableAmountMinor: number;
  benefitNo: string;
  campaignNo: string;
  consumedAmountMinor: number;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  customerQuotaReserved: boolean;
  expiredAmountMinor: number;
  expiresAt: null | number;
  financialPostingReference: string;
  frozenAmountMinor: number;
  globalQuotaReserved: boolean;
  grantNo: string;
  originalAmountMinor: number;
  pendingAmountMinor: number;
  reversedAmountMinor: number;
  sourceReference: string;
  status: string;
  type: string;
  updatedAt: number;
  withdrawable: boolean;
}

export interface MarketingValueEntryItem {
  availableDeltaMinor: number;
  consumedDeltaMinor: number;
  createdAt: number;
  entryNo: string;
  expiredDeltaMinor: number;
  frozenDeltaMinor: number;
  pendingDeltaMinor: number;
  reversalOfEntryNo: string;
  reversedDeltaMinor: number;
  sourceReference: string;
  sourceType: string;
  type: string;
}

export interface MarketingValueGrantPage {
  total: number;
  grants: MarketingValueGrantItem[];
  nextCursor: string;
}

export interface MarketingValueGrantDetail {
  entries: MarketingValueEntryItem[];
  grant: MarketingValueGrantItem;
}

export function marketingAssets(params: {
  page?: number;
  campaignNo?: string;
  currency?: string;
  cursor?: string;
  customerNo?: string;
  limit?: number;
  status?: string;
  type?: string;
}) {
  return requestPreservingHttpStatus<MarketingAssetPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/assets"),
    { params },
  );
}

export function marketingAssetDetail(assetNo: string) {
  return requestClient.get<MarketingAssetDetail>(
    pluginApiPath(
      pluginID,
      `admin/marketing/assets/${encodeURIComponent(assetNo)}`,
    ),
  );
}

export function marketingPointsAccounts(params: {
  page?: number;
  cursor?: string;
  customerNo?: string;
  limit?: number;
}) {
  return requestPreservingHttpStatus<MarketingPointsAccountPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/points/accounts"),
    { params },
  );
}

export function marketingPointsAccountDetail(customerNo: string) {
  return requestClient.get<MarketingPointsAccountDetail>(
    pluginApiPath(
      pluginID,
      `admin/marketing/points/accounts/${encodeURIComponent(customerNo)}`,
    ),
  );
}

export function marketingValueGrants(params: {
  page?: number;
  campaignNo?: string;
  currency?: string;
  cursor?: string;
  customerNo?: string;
  limit?: number;
  status?: string;
  type?: string;
}) {
  return requestPreservingHttpStatus<MarketingValueGrantPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/grants"),
    { params },
  );
}

export function marketingValueGrantDetail(grantNo: string) {
  return requestClient.get<MarketingValueGrantDetail>(
    pluginApiPath(
      pluginID,
      `admin/marketing/grants/${encodeURIComponent(grantNo)}`,
    ),
  );
}

export interface MarketingManagedAssetMutation {
  assetNo: string;
  customerNo: string;
  reused: boolean;
  status: string;
  updatedAt: number;
}

export interface MarketingManagedPointsMutation {
  availablePoints: number;
  customerNo: string;
  debtPoints: number;
  frozenPoints: number;
  lotNo: string;
  reused: boolean;
}

export interface MarketingValueGrantFinancialMutation {
  adjustmentNo: string;
  adjustmentType: "marketing_grant_post" | "marketing_grant_reverse";
  amountMinor: string;
  currencyCode: string;
  customerNo: string;
  grantNo: string;
  ledgerTransactionNo: string;
  reused: boolean;
  status: "pending_review" | "posted" | "rejected";
}

export interface MarketingValueGrantFinancialAdjustment extends MarketingValueGrantFinancialMutation {
  createdAt: number;
  requestedBy: number;
}

export interface MarketingValueGrantFinancialAdjustmentPage {
  total: number;
  adjustments: MarketingValueGrantFinancialAdjustment[];
  nextCursor: string;
}

export function marketingAssetIssue(data: {
  benefitNo: string;
  campaignNo: string;
  customerNo: string;
  reasonCode: string;
  requestNo: string;
}) {
  return requestClient.post<{ mutation: MarketingManagedAssetMutation }>(
    pluginApiPath(pluginID, "admin/marketing/assets/issue"),
    data,
  );
}

export function marketingCouponBatchGenerate(data: {
  benefitNo: string;
  campaignNo: string;
  codeLength: number;
  count: number;
  requestNo: string;
}) {
  return requestClient.download<Blob>(
    pluginApiPath(pluginID, "admin/marketing/coupon-codes/batches"),
    { data, method: "POST", responseReturn: "body" },
  );
}

export function marketingAssetRevoke(
  assetNo: string,
  data: {
    expectedUpdatedAt: number;
    reasonCode: string;
    requestNo: string;
  },
) {
  return requestClient.post<{ mutation: MarketingManagedAssetMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/assets/${encodeURIComponent(assetNo)}/revoke`,
    ),
    data,
  );
}

export function marketingPointsCredit(
  customerNo: string,
  data: {
    points: number;
    reasonCode: string;
    requestNo: string;
  },
) {
  return requestClient.post<{ mutation: MarketingManagedPointsMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/points/accounts/${encodeURIComponent(customerNo)}/credits`,
    ),
    data,
  );
}

export function marketingPointsReverse(
  customerNo: string,
  data: {
    awardSourceReference: string;
    awardSourceType: string;
    points: number;
    reasonCode: string;
    requestNo: string;
  },
) {
  return requestClient.post<{ mutation: MarketingManagedPointsMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/points/accounts/${encodeURIComponent(customerNo)}/reversals`,
    ),
    data,
  );
}

export function marketingValueGrantPostRequest(
  grantNo: string,
  data: { reasonCode: string; requestNo: string },
) {
  return requestClient.post<{ mutation: MarketingValueGrantFinancialMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/grants/${encodeURIComponent(grantNo)}/posting-requests`,
    ),
    data,
  );
}

export function marketingValueGrantReverseRequest(
  grantNo: string,
  data: { amountMinor: string; reasonCode: string; requestNo: string },
) {
  return requestClient.post<{ mutation: MarketingValueGrantFinancialMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/grants/${encodeURIComponent(grantNo)}/reversal-requests`,
    ),
    data,
  );
}

export function marketingValueGrantAdjustments(params: {
  page?: number;
  cursor?: string;
  limit?: number;
  status?: "pending_review" | "posted" | "rejected";
}) {
  return requestPreservingHttpStatus<MarketingValueGrantFinancialAdjustmentPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/marketing/grant-adjustments"),
    { params },
  );
}

export function marketingValueGrantAdjustmentReview(
  adjustmentNo: string,
  data: { decision: "approve" | "reject"; reasonCode: string },
) {
  return requestClient.post<{ mutation: MarketingValueGrantFinancialMutation }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/grant-adjustments/${encodeURIComponent(adjustmentNo)}/reviews`,
    ),
    data,
  );
}

export interface MarketingPerformanceItem {
  campaignNo: string;
  nameZhCn: string;
  nameEnUs: string;
  currencyCode: string;
  exposure: number;
  claim: number;
  use: number;
  order: number;
  receiptMinor: string;
  refundMinor: string;
  costMinor: string;
  costComplete: boolean;
  netMinor: string;
}

export interface MarketingAbuseCaseItem {
  caseNo: string;
  campaignNo: string;
  customerNo: string;
  signalCode: string;
  statusCode: "confirmed_abuse" | "false_positive" | "pending_review";
  relatedAccountCount: number;
  evidenceSummaryCode: string;
  windowStartedAt: number;
  windowEndedAt: number;
  observedCount: number;
  blockedUntil: number | null;
  reviewReason: string;
  reviewedBy: number;
  reviewedAt: number | null;
  version: number;
  createdAt: number;
  updatedAt: number;
}

export interface MarketingMissingCostItem {
  orderItemNo: string;
  orderNo: string;
  versionNo: string;
  currencyCode: string;
  periodUnitCode: string;
  periodCount: number;
  quantity: number;
  purchaseCycles: number;
  productNameZhCn: string;
  productNameEnUs: string;
  statusCode: "corrected" | "missing";
  unitCostMinor: null | string;
  totalCostMinor: null | string;
  correctionReason: string;
  version: number;
  createdAt: number;
  updatedAt: number;
}

export function marketingPerformance(params: {
  endedAt: number;
  startedAt: number;
  page?: number;
  limit?: number;
}) {
  return requestPreservingHttpStatus<{
    endedAt: number;
    rows: MarketingPerformanceItem[];
    startedAt: number;
    total: number;
  }>(requestClient, pluginApiPath(pluginID, "admin/marketing/performance"), {
    method: "GET",
    params,
  });
}

export function marketingAbuseCases(params: {
  limit?: number;
  page?: number;
  status?: string;
}) {
  return requestPreservingHttpStatus<{
    cases: MarketingAbuseCaseItem[];
    total: number;
  }>(requestClient, pluginApiPath(pluginID, "admin/marketing/abuse-cases"), {
    method: "GET",
    params,
  });
}

export function marketingAbuseReview(
  caseNo: string,
  data: {
    decision: "confirmed_abuse" | "false_positive";
    expectedVersion: number;
    reason: string;
  },
) {
  return requestClient.post<{ case: MarketingAbuseCaseItem }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/abuse-cases/${encodeURIComponent(caseNo)}/reviews`,
    ),
    data,
  );
}

export function marketingMissingCosts(params: {
  limit?: number;
  page?: number;
}) {
  return requestPreservingHttpStatus<{
    items: MarketingMissingCostItem[];
    total: number;
  }>(requestClient, pluginApiPath(pluginID, "admin/marketing/missing-costs"), {
    method: "GET",
    params,
  });
}

export function marketingCorrectMissingCost(
  orderItemNo: string,
  data: {
    expectedVersion: number;
    reason: string;
    requestNo: string;
    unitCostMinor: string;
  },
) {
  return requestClient.post<{ item: MarketingMissingCostItem }>(
    pluginApiPath(
      pluginID,
      `admin/marketing/missing-costs/${encodeURIComponent(orderItemNo)}/corrections`,
    ),
    data,
  );
}
