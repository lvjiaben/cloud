<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/marketing",
  title: "Marketing Operations",
};
</script>

<script setup lang="ts">
import type { TableColumnsType } from "ant-design-vue";

import type {
  MarketingCampaignSummary,
  MarketingCampaignDetail,
  MarketingCampaignManagementItem,
  MarketingGrantSummary,
  MarketingOverview,
  MarketingAssetDetail,
  MarketingPointsAccountDetail,
  MarketingPointsAccountItem,
  MarketingPointsLotItem,
  MarketingPromotionAssetItem,
  MarketingSegmentDetail,
  MarketingSegmentSummaryItem,
  MarketingValueGrantDetail,
  MarketingValueGrantFinancialAdjustment,
  MarketingValueGrantItem,
} from "./marketing-client";

import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { createNumberedPage } from "./numbered-page";

import { useAccess } from "./plugin-access";
import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  Input,
  Select,
  Skeleton,
  Space,
  Switch,
  Tabs,
  TabPane,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import MarketingActionModal from "./components/marketing-action-modal.vue";
import MarketingCouponBatchModal from "./components/marketing-coupon-batch-modal.vue";
import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import MarketingInsightsPanel from "./components/marketing-insights-panel.vue";
import VbenDataGrid from "./components/vben-data-grid.vue";

import {
  marketingCampaignDetail,
  marketingCampaigns,
  marketingAssetDetail,
  marketingAssets,
  marketingOverview,
  marketingPointsAccountDetail,
  marketingPointsAccounts,
  marketingSegmentDetail,
  marketingSegments,
  marketingValueGrantDetail,
  marketingValueGrantAdjustments,
  marketingValueGrants,
  updateMarketingCampaignNewcomer,
} from "./marketing-client";

const data = ref<MarketingOverview>();
const activeTab = ref("overview");
const loadedTabs = new Set<string>(["overview"]);
const marketingTabs = computed(() => [
  {
    key: "overview",
    label: $t("plugin.idc-core.marketing.tabs.overview"),
  },
  {
    key: "campaigns",
    label: $t("plugin.idc-core.marketing.tabs.campaigns"),
  },
  {
    key: "assets",
    label: $t("plugin.idc-core.marketing.tabs.assets"),
  },
  {
    key: "recent",
    label: $t("plugin.idc-core.marketing.tabs.recent"),
  },
]);
const errorMessage = ref("");
const loading = ref(false);
const segmentLoading = ref(false);
const campaignLoading = ref(false);
const directoryError = ref("");
const segmentRows = ref<MarketingSegmentSummaryItem[]>([]);
const segmentPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let segmentSequence = 0;
const segmentPagination = createNumberedPage<MarketingSegmentSummaryItem>(
  (state) => {
    segmentRows.value = state.items;
    segmentPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  clearMarketingDetails,
);

const segmentNamePrefix = ref("");
const segmentMode = ref("");
const segmentStatus = ref("");
const campaignRows = ref<MarketingCampaignManagementItem[]>([]);
const campaignPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let campaignSequence = 0;
const campaignPagination = createNumberedPage<MarketingCampaignManagementItem>(
  (state) => {
    campaignRows.value = state.items;
    campaignPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  clearMarketingDetails,
);

const campaignNamePrefix = ref("");
const campaignType = ref("");
const campaignStatus = ref("");
const detailLoading = ref(false);
const segmentDetail = ref<MarketingSegmentDetail>();
const campaignDetail = ref<MarketingCampaignDetail>();
const assetDirectoryError = ref("");
const assetLoading = ref(false);
const assetRows = ref<MarketingPromotionAssetItem[]>([]);
const assetCustomerNo = ref("");

const assetPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let assetSequence = 0;
const assetPagination = createNumberedPage<MarketingPromotionAssetItem>(
  (state) => {
    assetRows.value = state.items;
    assetPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  clearMarketingDetails,
);

const assetDetail = ref<MarketingAssetDetail>();
const pointsLoading = ref(false);
const pointsRows = ref<MarketingPointsAccountItem[]>([]);
const pointsCustomerNo = ref("");

const pointsPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let pointsSequence = 0;
const pointsPagination = createNumberedPage<MarketingPointsAccountItem>(
  (state) => {
    pointsRows.value = state.items;
    pointsPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  clearMarketingDetails,
);

const pointsDetail = ref<MarketingPointsAccountDetail>();
const valueGrantLoading = ref(false);
const valueGrantRows = ref<MarketingValueGrantItem[]>([]);
const valueGrantCustomerNo = ref("");

const valueGrantPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let valueGrantSequence = 0;
const valueGrantPagination = createNumberedPage<MarketingValueGrantItem>(
  (state) => {
    valueGrantRows.value = state.items;
    valueGrantPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  clearMarketingDetails,
);

const valueGrantDetail = ref<MarketingValueGrantDetail>();
const adjustmentLoading = ref(false);
const adjustmentRows = ref<MarketingValueGrantFinancialAdjustment[]>([]);

const adjustmentPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let adjustmentSequence = 0;
const adjustmentPagination =
  createNumberedPage<MarketingValueGrantFinancialAdjustment>((state) => {
    adjustmentRows.value = state.items;
    adjustmentPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  }, clearMarketingDetails);

const { hasAccessByCodes } = useAccess();
const canIssueAsset = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:asset:issue"]),
);
const canRevokeAsset = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:asset:revoke"]),
);
const canCreditPoints = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:points:credit"]),
);
const canReversePoints = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:points:reverse"]),
);
const canPostGrant = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:grant:post"]),
);
const canReverseGrant = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:grant:reverse"]),
);
const canReviewGrant = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:grant:review"]),
);
const canSetCampaignNewcomer = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:campaign:newcomer:update"]),
);
const newcomerSaving = ref(false);
const [MarketingActionModalRef, marketingActionModalApi] = useVbenModal({
  connectedComponent: MarketingActionModal,
});
const [MarketingCouponBatchModalRef, marketingCouponBatchModalApi] =
  useVbenModal({ connectedComponent: MarketingCouponBatchModal });
const canGenerateCouponBatch = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:coupon-code:generate"]),
);

type MarketingActionKind =
  | "asset_issue"
  | "asset_revoke"
  | "grant_post"
  | "grant_reverse"
  | "grant_review"
  | "points_credit"
  | "points_reverse";

interface MarketingActionPayload {
  asset?: MarketingPromotionAssetItem;
  benefitNo?: string;
  campaignNo?: string;
  lot?: MarketingPointsLotItem;
  pointsAccount?: MarketingPointsAccountItem;
  adjustment?: MarketingValueGrantFinancialAdjustment;
  grant?: MarketingValueGrantItem;
}

const pendingMarketingAction = ref<{
  action: MarketingActionKind;
  payload: MarketingActionPayload;
}>();

const campaignColumns = computed<TableColumnsType<MarketingCampaignSummary>>(
  () => [
    {
      key: "campaign",
      title: $t("plugin.idc-core.marketing.campaign.table.campaign"),
    },
    {
      dataIndex: "type",
      key: "type",
      title: $t("plugin.idc-core.marketing.campaign.table.type"),
      width: 142,
    },
    {
      dataIndex: "status",
      key: "status",
      title: $t("plugin.idc-core.marketing.campaign.table.status"),
      width: 124,
    },
    {
      key: "quota",
      title: $t("plugin.idc-core.marketing.campaign.table.quota"),
      width: 176,
    },
    {
      key: "window",
      title: $t("plugin.idc-core.marketing.campaign.table.window"),
      width: 224,
    },
  ],
);

const grantColumns = computed<TableColumnsType<MarketingGrantSummary>>(() => [
  {
    key: "grant",
    title: $t("plugin.idc-core.marketing.grant.table.grant"),
  },
  {
    dataIndex: "customerNo",
    key: "customer",
    title: $t("plugin.idc-core.marketing.grant.table.customer"),
    width: 190,
  },
  {
    key: "balance",
    title: $t("plugin.idc-core.marketing.grant.table.balance"),
    width: 210,
  },
  {
    dataIndex: "status",
    key: "status",
    title: $t("plugin.idc-core.marketing.grant.table.status"),
    width: 124,
  },
  {
    dataIndex: "updatedAt",
    key: "updated",
    title: $t("plugin.idc-core.marketing.grant.table.updated"),
    width: 174,
  },
]);

const segmentColumns = computed<TableColumnsType<MarketingSegmentSummaryItem>>(
  () => [
    {
      key: "segment",
      title: $t("plugin.idc-core.marketing.directory.segment"),
    },
    {
      dataIndex: "mode",
      key: "mode",
      title: $t("plugin.idc-core.marketing.directory.mode"),
      width: 104,
    },
    {
      dataIndex: "status",
      key: "status",
      title: $t("plugin.idc-core.marketing.directory.status"),
      width: 108,
    },
    {
      key: "members",
      title: $t("plugin.idc-core.marketing.directory.members"),
      width: 118,
    },
    {
      key: "rules",
      title: $t("plugin.idc-core.marketing.directory.rules"),
      width: 96,
    },
    { key: "action", title: "", width: 84 },
  ],
);

const campaignManagementColumns = computed<
  TableColumnsType<MarketingCampaignManagementItem>
>(() => [
  {
    key: "campaign",
    title: $t("plugin.idc-core.marketing.directory.campaign"),
  },
  {
    dataIndex: "type",
    key: "type",
    title: $t("plugin.idc-core.marketing.directory.type"),
    width: 128,
  },
  {
    dataIndex: "status",
    key: "status",
    title: $t("plugin.idc-core.marketing.directory.status"),
    width: 108,
  },
  {
    key: "quota",
    title: $t("plugin.idc-core.marketing.campaign.table.quota"),
    width: 144,
  },
  {
    key: "rules",
    title: $t("plugin.idc-core.marketing.directory.rules"),
    width: 96,
  },
  { key: "action", title: "", width: 84 },
]);

const assetColumns = computed<TableColumnsType<MarketingPromotionAssetItem>>(
  () => [
    {
      key: "asset",
      title: $t("plugin.idc-core.marketing.assets.asset"),
    },
    {
      dataIndex: "customerNo",
      key: "customer",
      title: $t("plugin.idc-core.marketing.assets.customer"),
      width: 150,
    },
    {
      key: "value",
      title: $t("plugin.idc-core.marketing.assets.value"),
      width: 130,
    },
    {
      dataIndex: "status",
      key: "status",
      title: $t("plugin.idc-core.marketing.directory.status"),
      width: 104,
    },
    { key: "action", title: "", width: 158 },
  ],
);

const pointsColumns = computed<TableColumnsType<MarketingPointsAccountItem>>(
  () => [
    {
      dataIndex: "customerNo",
      key: "customer",
      title: $t("plugin.idc-core.marketing.assets.customer"),
    },
    {
      dataIndex: "availablePoints",
      key: "available",
      title: $t("plugin.idc-core.marketing.assets.available"),
      width: 110,
    },
    {
      key: "frozenDebt",
      title: $t("plugin.idc-core.marketing.assets.frozenDebt"),
      width: 130,
    },
    {
      key: "history",
      title: $t("plugin.idc-core.marketing.assets.history"),
      width: 96,
    },
    { key: "action", title: "", width: 156 },
  ],
);

const valueGrantColumns = computed<TableColumnsType<MarketingValueGrantItem>>(
  () => [
    {
      key: "grant",
      title: $t("plugin.idc-core.marketing.assets.grant"),
    },
    {
      dataIndex: "customerNo",
      key: "customer",
      title: $t("plugin.idc-core.marketing.assets.customer"),
      width: 150,
    },
    {
      key: "balance",
      title: $t("plugin.idc-core.marketing.assets.available"),
      width: 130,
    },
    {
      dataIndex: "status",
      key: "status",
      title: $t("plugin.idc-core.marketing.directory.status"),
      width: 104,
    },
    { key: "action", title: "", width: 220 },
  ],
);

const adjustmentColumns = computed<
  TableColumnsType<MarketingValueGrantFinancialAdjustment>
>(() => [
  {
    key: "adjustment",
    title: $t("plugin.idc-core.marketing.actions.adjustment"),
  },
  {
    key: "amount",
    title: $t("plugin.idc-core.marketing.actions.amount"),
    width: 140,
  },
  {
    key: "requester",
    title: $t("plugin.idc-core.marketing.actions.requester"),
    width: 156,
  },
  { key: "action", title: "", width: 92 },
]);

const summary = computed(() => data.value?.summary);

function localizedCampaignName(item: MarketingCampaignSummary) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCn || item.nameEnUs || item.campaignNo
    : item.nameEnUs || item.nameZhCn || item.campaignNo;
}

function localizedName(
  item: { nameEnUs: string; nameZhCn: string },
  fallback: string,
) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCn || item.nameEnUs || fallback
    : item.nameEnUs || item.nameZhCn || fallback;
}

function formatCount(value: number | undefined) {
  return new Intl.NumberFormat(preferences.app.locale || "zh-CN").format(
    value || 0,
  );
}

function formatMinor(value: number, currencyCode: string) {
  return `${formatCount(value)} ${currencyCode}`;
}

function statusColor(status: string) {
  if (["active", "available", "complete"].includes(status)) return "green";
  if (["scheduled", "issued", "pending"].includes(status)) return "blue";
  if (["paused", "frozen"].includes(status)) return "orange";
  if (["reversed", "revoked", "failed"].includes(status)) return "red";
  return "default";
}

async function loadOverview() {
  loading.value = true;
  errorMessage.value = "";
  try {
    data.value = await marketingOverview();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
  } finally {
    loading.value = false;
  }
}

async function loadSegments(
  pageNumber = segmentPager.value.currentPage,
  pageSize = segmentPager.value.pageSize,
) {
  const sequence = ++segmentSequence;
  segmentLoading.value = true;
  directoryError.value = "";
  await segmentPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingSegments({
          page: currentPage,
          limit,
          mode: segmentMode.value,
          namePrefix: segmentNamePrefix.value,
          status: segmentStatus.value,
        });
        return { items: page.segments, total: page.total };
      } catch (error) {
        if (sequence === segmentSequence)
          directoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === segmentSequence) segmentLoading.value = false;
      }
    },
  );
}

async function loadCampaigns(
  pageNumber = campaignPager.value.currentPage,
  pageSize = campaignPager.value.pageSize,
) {
  const sequence = ++campaignSequence;
  campaignLoading.value = true;
  directoryError.value = "";
  await campaignPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingCampaigns({
          page: currentPage,
          limit,
          namePrefix: campaignNamePrefix.value,
          status: campaignStatus.value,
          type: campaignType.value,
        });
        return { items: page.campaigns, total: page.total };
      } catch (error) {
        if (sequence === campaignSequence)
          directoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === campaignSequence) campaignLoading.value = false;
      }
    },
  );
}

function resetSegmentSearch() {
  void loadSegments(1);
}

function resetCampaignSearch() {
  void loadCampaigns(1);
}

async function openSegmentDetail(segmentNo: string) {
  detailLoading.value = true;
  try {
    segmentDetail.value = await marketingSegmentDetail(segmentNo);
  } catch (error) {
    directoryError.value =
      error instanceof Error ? error.message : String(error);
  } finally {
    detailLoading.value = false;
  }
}

async function openCampaignDetail(campaignNo: string) {
  detailLoading.value = true;
  try {
    campaignDetail.value = await marketingCampaignDetail(campaignNo);
  } catch (error) {
    directoryError.value =
      error instanceof Error ? error.message : String(error);
  } finally {
    detailLoading.value = false;
  }
}

async function saveCampaignNewcomerSetting() {
  const detail = campaignDetail.value;
  if (!detail || newcomerSaving.value) return;
  newcomerSaving.value = true;
  directoryError.value = "";
  try {
    const result = await updateMarketingCampaignNewcomer({
      campaignNo: detail.campaign.campaignNo,
      enabled: detail.campaign.isNewcomerExclusive,
      expectedVersion: detail.version,
      requestNo: `newcomer-${crypto.randomUUID()}`,
    });
    detail.version = result.version;
    detail.campaign.updatedAt = result.updatedAt;
    const row = campaignRows.value.find(
      (item) => item.campaignNo === result.campaignNo,
    );
    if (row) {
      row.isNewcomerExclusive = result.enabled;
      row.updatedAt = result.updatedAt;
    }
  } catch (error) {
    directoryError.value =
      error instanceof Error ? error.message : String(error);
    await openCampaignDetail(detail.campaign.campaignNo);
  } finally {
    newcomerSaving.value = false;
  }
}

async function loadAssets(
  pageNumber = assetPager.value.currentPage,
  pageSize = assetPager.value.pageSize,
) {
  const sequence = ++assetSequence;
  assetLoading.value = true;
  assetDirectoryError.value = "";
  await assetPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingAssets({
          page: currentPage,
          customerNo: assetCustomerNo.value,
          limit,
        });
        return { items: page.assets, total: page.total };
      } catch (error) {
        if (sequence === assetSequence)
          assetDirectoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === assetSequence) assetLoading.value = false;
      }
    },
  );
}

async function loadPointsAccounts(
  pageNumber = pointsPager.value.currentPage,
  pageSize = pointsPager.value.pageSize,
) {
  const sequence = ++pointsSequence;
  pointsLoading.value = true;
  assetDirectoryError.value = "";
  await pointsPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingPointsAccounts({
          page: currentPage,
          customerNo: pointsCustomerNo.value,
          limit,
        });
        return { items: page.accounts, total: page.total };
      } catch (error) {
        if (sequence === pointsSequence)
          assetDirectoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === pointsSequence) pointsLoading.value = false;
      }
    },
  );
}

async function loadValueGrants(
  pageNumber = valueGrantPager.value.currentPage,
  pageSize = valueGrantPager.value.pageSize,
) {
  const sequence = ++valueGrantSequence;
  valueGrantLoading.value = true;
  assetDirectoryError.value = "";
  await valueGrantPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingValueGrants({
          page: currentPage,
          customerNo: valueGrantCustomerNo.value,
          limit,
        });
        return { items: page.grants, total: page.total };
      } catch (error) {
        if (sequence === valueGrantSequence)
          assetDirectoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === valueGrantSequence) valueGrantLoading.value = false;
      }
    },
  );
}

async function loadGrantAdjustments(
  pageNumber = adjustmentPager.value.currentPage,
  pageSize = adjustmentPager.value.pageSize,
) {
  const sequence = ++adjustmentSequence;
  if (!canReviewGrant.value) {
    adjustmentPagination.reset();
    return;
  }
  adjustmentLoading.value = true;
  assetDirectoryError.value = "";
  await adjustmentPagination.query(
    { currentPage: pageNumber, pageSize },
    async (currentPage, limit) => {
      try {
        const page = await marketingValueGrantAdjustments({
          page: currentPage,
          limit,
          status: "pending_review",
        });
        return { items: page.adjustments, total: page.total };
      } catch (error) {
        if (sequence === adjustmentSequence)
          assetDirectoryError.value =
            error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        if (sequence === adjustmentSequence) adjustmentLoading.value = false;
      }
    },
  );
}

function resetAssetSearch() {
  void loadAssets(1);
}
function resetPointsSearch() {
  void loadPointsAccounts(1);
}
function resetValueGrantSearch() {
  void loadValueGrants(1);
}

async function openAssetDetail(assetNo: string) {
  detailLoading.value = true;
  try {
    assetDetail.value = await marketingAssetDetail(assetNo);
  } catch (error) {
    assetDirectoryError.value =
      error instanceof Error ? error.message : String(error);
  } finally {
    detailLoading.value = false;
  }
}
async function openPointsDetail(customerNo: string) {
  detailLoading.value = true;
  try {
    pointsDetail.value = await marketingPointsAccountDetail(customerNo);
  } catch (error) {
    assetDirectoryError.value =
      error instanceof Error ? error.message : String(error);
  } finally {
    detailLoading.value = false;
  }
}
async function openValueGrantDetail(grantNo: string) {
  detailLoading.value = true;
  try {
    valueGrantDetail.value = await marketingValueGrantDetail(grantNo);
  } catch (error) {
    assetDirectoryError.value =
      error instanceof Error ? error.message : String(error);
  } finally {
    detailLoading.value = false;
  }
}

function openMarketingAction(
  action: MarketingActionKind,
  payload: MarketingActionPayload = {},
) {
  if (
    segmentDetail.value ||
    campaignDetail.value ||
    assetDetail.value ||
    pointsDetail.value ||
    valueGrantDetail.value
  ) {
    pendingMarketingAction.value = { action, payload };
    segmentDetail.value = undefined;
    campaignDetail.value = undefined;
    assetDetail.value = undefined;
    pointsDetail.value = undefined;
    valueGrantDetail.value = undefined;
    return;
  }
  marketingActionModalApi.setData({ action, ...payload });
  marketingActionModalApi.open();
}

function handleDetailOpenChange(open: boolean) {
  if (open || !pendingMarketingAction.value) return;
  const pending = pendingMarketingAction.value;
  pendingMarketingAction.value = undefined;
  marketingActionModalApi.setData({
    action: pending.action,
    ...pending.payload,
  });
  marketingActionModalApi.open();
}

async function handleMarketingActionSuccess(action: MarketingActionKind) {
  if (action.startsWith("asset_")) {
    assetDetail.value = undefined;
    await Promise.all([loadAssets(), loadOverview()]);
    return;
  }
  if (action.startsWith("grant_")) {
    valueGrantDetail.value = undefined;
    await Promise.all([
      loadValueGrants(),
      loadGrantAdjustments(),
      loadOverview(),
    ]);
    return;
  }
  const customerNo = pointsDetail.value?.account.customerNo;
  await Promise.all([loadPointsAccounts(), loadOverview()]);
  if (customerNo) await openPointsDetail(customerNo);
}

function assetValue(item: MarketingPromotionAssetItem) {
  if (item.faceAmountMinor !== null)
    return formatMinor(item.faceAmountMinor, item.currencyCode);
  return item.rateBasisPoints === null
    ? "—"
    : `${item.rateBasisPoints / 100}% ${item.currencyCode}`;
}

function conditionValue(
  condition: MarketingSegmentDetail["conditions"][number],
) {
  if (condition.values.length) {
    return condition.values.map((item) => item.text || item.integer).join(", ");
  }
  if (condition.valueBoolean !== null) return String(condition.valueBoolean);
  if (condition.valueIntegerLower !== null) {
    const value =
      condition.valueIntegerUpper === null
        ? String(condition.valueIntegerLower)
        : `${condition.valueIntegerLower} – ${condition.valueIntegerUpper}`;
    return condition.currencyCode
      ? `${value} ${condition.currencyCode}`
      : value;
  }
  return condition.valueText || "—";
}

function benefitValue(benefit: MarketingCampaignDetail["benefits"][number]) {
  if (benefit.pointsQuantity !== null)
    return `${formatCount(benefit.pointsQuantity)} pts`;
  if (benefit.amountMinor !== null)
    return formatMinor(benefit.amountMinor, benefit.currencyCode);
  if (benefit.rateBasisPoints !== null)
    return `${benefit.rateBasisPoints / 100}% ${benefit.currencyCode}`;
  return "—";
}

async function handleTabChange(rawKey: string | number) {
  const key = String(rawKey);
  if (loadedTabs.has(key)) return;
  loadedTabs.add(key);
  if (key === "campaigns") {
    await Promise.all([loadSegments(), loadCampaigns()]);
    return;
  }
  if (key === "assets") {
    await Promise.all([
      loadAssets(),
      loadPointsAccounts(),
      loadValueGrants(),
      loadGrantAdjustments(),
    ]);
  }
}

function clearMarketingDetails() {
  segmentDetail.value = undefined;
  campaignDetail.value = undefined;
  assetDetail.value = undefined;
  pointsDetail.value = undefined;
  valueGrantDetail.value = undefined;
  pendingMarketingAction.value = undefined;
  marketingActionModalApi.close();
  marketingCouponBatchModalApi.close();
}

onBeforeUnmount(() => {
  segmentSequence += 1;
  segmentPagination.invalidate();
  campaignSequence += 1;
  campaignPagination.invalidate();
  assetSequence += 1;
  assetPagination.invalidate();
  pointsSequence += 1;
  pointsPagination.invalidate();
  valueGrantSequence += 1;
  valueGrantPagination.invalidate();
  adjustmentSequence += 1;
  adjustmentPagination.invalidate();
});

onMounted(() => void loadOverview());
</script>

<template>
  <Page auto-content-height>
    <div class="marketing-shell" data-testid="marketing-overview-page">
      <header class="marketing-page-header">
        <div>
          <h1>{{ $t("plugin.idc-core.marketing.title") }}</h1>
          <p>{{ $t("plugin.idc-core.marketing.pageDescription") }}</p>
        </div>
        <Space>
          <span v-if="data" class="snapshot-time">
            {{ $t("plugin.idc-core.marketing.generatedAt") }}
            {{ formatTimestamp(data.generatedAt) }}
          </span>
          <Button
            data-testid="marketing-refresh"
            :loading="loading"
            @click="loadOverview"
          >
            {{ $t("plugin.idc-core.marketing.refresh") }}
          </Button>
        </Space>
      </header>

      <Alert
        v-if="errorMessage"
        :description="errorMessage"
        :message="$t('plugin.idc-core.marketing.loadFailed')"
        show-icon
        type="error"
      />

      <Tabs
        v-model:active-key="activeTab"
        class="marketing-tabs"
        @change="handleTabChange"
      >
        <TabPane v-for="tab in marketingTabs" :key="tab.key" :tab="tab.label" />
      </Tabs>

      <div v-show="activeTab === 'overview'">
        <MarketingInsightsPanel />
      </div>

      <Skeleton v-if="loading && !data" active :paragraph="{ rows: 10 }" />

      <template v-else-if="data && summary">
        <section v-show="activeTab === 'overview'" class="signal-grid">
          <article class="signal-card signal-card--violet">
            <div class="signal-label">
              {{ $t("plugin.idc-core.marketing.metrics.audience") }}
            </div>
            <strong>{{ formatCount(summary.activeSegmentMembers) }}</strong>
            <p>
              {{ formatCount(summary.segmentActive) }} /
              {{ formatCount(summary.segmentTotal) }}
              {{ $t("plugin.idc-core.marketing.metrics.activeSegments") }}
            </p>
            <span
              >{{ formatCount(summary.segmentDynamic) }}
              {{
                $t("plugin.idc-core.marketing.metrics.dynamicSegments")
              }}</span
            >
          </article>
          <article class="signal-card signal-card--coral">
            <div class="signal-label">
              {{ $t("plugin.idc-core.marketing.metrics.campaigns") }}
            </div>
            <strong>{{ formatCount(summary.campaignActive) }}</strong>
            <p>
              {{ formatCount(summary.campaignScheduled) }}
              {{ $t("plugin.idc-core.marketing.metrics.scheduled") }}
            </p>
            <span
              >{{ formatCount(summary.campaignReservedUnits) }} /
              {{ formatCount(summary.campaignConsumedUnits) }}
              {{
                $t("plugin.idc-core.marketing.metrics.reservedConsumed")
              }}</span
            >
          </article>
          <article class="signal-card signal-card--amber">
            <div class="signal-label">
              {{ $t("plugin.idc-core.marketing.metrics.assets") }}
            </div>
            <strong>{{ formatCount(summary.assetActive) }}</strong>
            <p>
              {{ formatCount(summary.assetIssued) }}
              {{ $t("plugin.idc-core.marketing.metrics.unclaimed") }}
            </p>
            <span
              >{{ formatCount(summary.assetReservedUses) }} /
              {{ formatCount(summary.assetConsumedUses) }}
              {{
                $t("plugin.idc-core.marketing.metrics.reservedConsumed")
              }}</span
            >
          </article>
          <article class="signal-card signal-card--cyan">
            <div class="signal-label">
              {{ $t("plugin.idc-core.marketing.metrics.points") }}
            </div>
            <strong>{{ formatCount(summary.pointsAvailable) }}</strong>
            <p>
              {{ formatCount(summary.pointsFrozen) }}
              {{ $t("plugin.idc-core.marketing.metrics.frozen") }}
            </p>
            <span :class="{ 'signal-danger': summary.pointsDebt > 0 }"
              >{{ formatCount(summary.pointsDebt) }}
              {{ $t("plugin.idc-core.marketing.metrics.debt") }}</span
            >
          </article>
          <article class="signal-card signal-card--green">
            <div class="signal-label">
              {{ $t("plugin.idc-core.marketing.metrics.consent") }}
            </div>
            <strong>{{ formatCount(summary.consentOptedIn) }}</strong>
            <p>
              {{ formatCount(summary.consentOptedOut) }}
              {{ $t("plugin.idc-core.marketing.metrics.optedOut") }}
            </p>
            <span>{{
              $t("plugin.idc-core.marketing.metrics.failClosed")
            }}</span>
          </article>
        </section>

        <section v-show="activeTab === 'overview'" class="section-block">
          <div class="section-heading">
            <div>
              <span>{{
                $t("plugin.idc-core.marketing.currency.eyebrow")
              }}</span>
              <h2>
                {{ $t("plugin.idc-core.marketing.currency.title") }}
              </h2>
            </div>
            <p>{{ $t("plugin.idc-core.marketing.currency.note") }}</p>
          </div>
          <div v-if="data.currencies.length" class="currency-grid">
            <article
              v-for="currency in data.currencies"
              :key="currency.currencyCode"
              class="currency-card"
            >
              <div class="currency-code">{{ currency.currencyCode }}</div>
              <div class="currency-primary">
                <span>{{
                  $t("plugin.idc-core.marketing.currency.available")
                }}</span>
                <strong>{{ formatCount(currency.availableMinor) }}</strong>
              </div>
              <dl>
                <div>
                  <dt>
                    {{ $t("plugin.idc-core.marketing.currency.grants") }}
                  </dt>
                  <dd>{{ formatCount(currency.grantCount) }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-core.marketing.currency.pending") }}
                  </dt>
                  <dd>{{ formatCount(currency.pendingMinor) }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-core.marketing.currency.frozen") }}
                  </dt>
                  <dd>{{ formatCount(currency.frozenMinor) }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-core.marketing.currency.consumed") }}
                  </dt>
                  <dd>{{ formatCount(currency.consumedMinor) }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-core.marketing.currency.reversed") }}
                  </dt>
                  <dd>{{ formatCount(currency.reversedMinor) }}</dd>
                </div>
              </dl>
              <small>{{
                $t("plugin.idc-core.marketing.currency.minorUnit")
              }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.currency.empty')"
          />
        </section>

        <section
          v-show="activeTab === 'campaigns'"
          class="directory-section"
          data-testid="marketing-directory"
        >
          <div class="section-heading">
            <div>
              <span>{{
                $t("plugin.idc-core.marketing.directory.eyebrow")
              }}</span>
              <h2>
                {{ $t("plugin.idc-core.marketing.directory.title") }}
              </h2>
            </div>
            <p>{{ $t("plugin.idc-core.marketing.directory.note") }}</p>
          </div>
          <Alert
            v-if="directoryError"
            closable
            :description="directoryError"
            :message="$t('plugin.idc-core.marketing.directory.loadFailed')"
            show-icon
            type="error"
            @close="directoryError = ''"
          />
          <div class="directory-grid">
            <Card
              :bordered="false"
              class="directory-card"
              data-testid="segment-directory"
            >
              <template #title>
                <div class="card-title">
                  <span>{{
                    $t("plugin.idc-core.marketing.directory.segments")
                  }}</span>
                  <strong>{{
                    $t("plugin.idc-core.marketing.directory.segments")
                  }}</strong>
                </div>
              </template>
              <div class="directory-filters">
                <Input.Search
                  v-model:value="segmentNamePrefix"
                  allow-clear
                  :placeholder="
                    $t('plugin.idc-core.marketing.directory.searchSegment')
                  "
                  @search="resetSegmentSearch"
                />
                <Select
                  v-model:value="segmentMode"
                  :options="[
                    {
                      label: $t('plugin.idc-core.marketing.directory.allModes'),
                      value: '',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.mode.static'),
                      value: 'static',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.mode.dynamic'),
                      value: 'dynamic',
                    },
                  ]"
                  @change="resetSegmentSearch"
                />
                <Select
                  v-model:value="segmentStatus"
                  :options="[
                    {
                      label: $t(
                        'plugin.idc-core.marketing.directory.allStatuses',
                      ),
                      value: '',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.active'),
                      value: 'active',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.draft'),
                      value: 'draft',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.paused'),
                      value: 'paused',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.archived'),
                      value: 'archived',
                    },
                  ]"
                  @change="resetSegmentSearch"
                />
              </div>
              <VbenDataGrid
                :columns="segmentColumns"
                :data="segmentRows"
                :pagination="segmentPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadSegments(currentPage, pageSize)
                "
                :height="360"
                :loading="segmentLoading"
                row-key="segmentNo"
              >
                <template #cell="{ column, record, value }">
                  <div v-if="column.key === 'segment'" class="entity-cell">
                    <strong>{{
                      localizedName(record, record.segmentNo)
                    }}</strong>
                    <code>{{ record.segmentCode }}</code>
                  </div>
                  <Tag v-else-if="column.key === 'mode'">{{
                    $t(`plugin.idc-core.marketing.mode.${record.mode}`)
                  }}</Tag>
                  <Tag
                    v-else-if="column.key === 'status'"
                    :color="statusColor(record.status)"
                    >{{
                      $t(`plugin.idc-core.marketing.status.${record.status}`)
                    }}</Tag
                  >
                  <strong v-else-if="column.key === 'members'">{{
                    formatCount(record.activeMemberCount)
                  }}</strong>
                  <span v-else-if="column.key === 'rules'">{{
                    formatCount(record.conditionCount)
                  }}</span>
                  <Button
                    v-else-if="column.key === 'action'"
                    size="small"
                    type="link"
                    @click="openSegmentDetail(record.segmentNo)"
                    >{{
                      $t("plugin.idc-core.marketing.directory.details")
                    }}</Button
                  >
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>

            <Card
              :bordered="false"
              class="directory-card"
              data-testid="campaign-directory"
            >
              <template #title>
                <div class="card-title">
                  <span>{{
                    $t("plugin.idc-core.marketing.directory.campaigns")
                  }}</span>
                  <strong>{{
                    $t("plugin.idc-core.marketing.directory.campaigns")
                  }}</strong>
                </div>
              </template>
              <div class="directory-filters">
                <Input.Search
                  v-model:value="campaignNamePrefix"
                  allow-clear
                  :placeholder="
                    $t('plugin.idc-core.marketing.directory.searchCampaign')
                  "
                  @search="resetCampaignSearch"
                />
                <Select
                  v-model:value="campaignType"
                  :options="[
                    {
                      label: $t('plugin.idc-core.marketing.directory.allTypes'),
                      value: '',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.type.discount'),
                      value: 'discount',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.type.coupon'),
                      value: 'coupon',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.type.cashback'),
                      value: 'cashback',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.type.points'),
                      value: 'points',
                    },
                  ]"
                  @change="resetCampaignSearch"
                />
                <Select
                  v-model:value="campaignStatus"
                  :options="[
                    {
                      label: $t(
                        'plugin.idc-core.marketing.directory.allStatuses',
                      ),
                      value: '',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.active'),
                      value: 'active',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.scheduled'),
                      value: 'scheduled',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.paused'),
                      value: 'paused',
                    },
                    {
                      label: $t('plugin.idc-core.marketing.status.ended'),
                      value: 'ended',
                    },
                  ]"
                  @change="resetCampaignSearch"
                />
              </div>
              <VbenDataGrid
                :columns="campaignManagementColumns"
                :data="campaignRows"
                :pagination="campaignPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadCampaigns(currentPage, pageSize)
                "
                :height="360"
                :loading="campaignLoading"
                row-key="campaignNo"
              >
                <template #cell="{ column, record, value }">
                  <div v-if="column.key === 'campaign'" class="entity-cell">
                    <strong>{{
                      localizedName(record, record.campaignNo)
                    }}</strong>
                    <code>{{ record.campaignCode }}</code>
                  </div>
                  <Tag v-else-if="column.key === 'type'">{{
                    $t(`plugin.idc-core.marketing.type.${record.type}`)
                  }}</Tag>
                  <Tag
                    v-else-if="column.key === 'status'"
                    :color="statusColor(record.status)"
                    >{{
                      $t(`plugin.idc-core.marketing.status.${record.status}`)
                    }}</Tag
                  >
                  <div v-else-if="column.key === 'quota'" class="quota-cell">
                    <strong>{{
                      formatCount(record.globalConsumedCount)
                    }}</strong
                    ><span
                      >/
                      {{
                        record.globalQuotaLimit === null
                          ? "∞"
                          : formatCount(record.globalQuotaLimit)
                      }}</span
                    >
                  </div>
                  <span v-else-if="column.key === 'rules'">{{
                    formatCount(record.scopeCount + record.benefitCount)
                  }}</span>
                  <Button
                    v-else-if="column.key === 'action'"
                    size="small"
                    type="link"
                    @click="openCampaignDetail(record.campaignNo)"
                    >{{
                      $t("plugin.idc-core.marketing.directory.details")
                    }}</Button
                  >
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>
          </div>
        </section>

        <section
          v-show="activeTab === 'assets'"
          class="asset-directory-section"
          data-testid="marketing-asset-directory"
        >
          <div class="section-heading">
            <div>
              <span>{{ $t("plugin.idc-core.marketing.assets.eyebrow") }}</span>
              <h2>
                {{ $t("plugin.idc-core.marketing.assets.title") }}
              </h2>
            </div>
            <p>{{ $t("plugin.idc-core.marketing.assets.note") }}</p>
          </div>
          <Alert
            v-if="assetDirectoryError"
            closable
            show-icon
            type="error"
            :description="assetDirectoryError"
            :message="$t('plugin.idc-core.marketing.assets.loadFailed')"
            @close="assetDirectoryError = ''"
          />
          <div class="asset-directory-grid">
            <Card
              :bordered="false"
              class="directory-card"
              data-testid="asset-directory"
            >
              <template #title>{{
                $t("plugin.idc-core.marketing.assets.assets")
              }}</template>
              <template v-if="canIssueAsset || canGenerateCouponBatch" #extra>
                <Space>
                  <Button
                    v-if="canGenerateCouponBatch"
                    data-testid="marketing-coupon-batch-generate"
                    size="small"
                    @click="marketingCouponBatchModalApi.open()"
                    >{{
                      $t("plugin.idc-core.marketing.couponBatch.button")
                    }}</Button
                  >
                  <Button
                    v-if="canIssueAsset"
                    data-testid="marketing-asset-issue"
                    size="small"
                    type="primary"
                    @click="openMarketingAction('asset_issue')"
                    >{{
                      $t("plugin.idc-core.marketing.actions.asset_issue.button")
                    }}</Button
                  >
                </Space>
              </template>
              <Input
                v-model:value="assetCustomerNo"
                allow-clear
                :placeholder="
                  $t('plugin.idc-core.marketing.assets.searchCustomer')
                "
                @clear="resetAssetSearch"
                @press-enter="resetAssetSearch"
              />
              <VbenDataGrid
                :columns="assetColumns"
                :data="assetRows"
                :pagination="assetPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadAssets(currentPage, pageSize)
                "
                :height="360"
                :loading="assetLoading"
                row-key="assetNo"
              >
                <template #cell="{ column, record, value }">
                  <div v-if="column.key === 'asset'" class="entity-cell">
                    <strong>{{
                      $t(`plugin.idc-core.marketing.type.${record.type}`)
                    }}</strong>
                    <code>{{ record.assetNo }}</code>
                  </div>
                  <code v-else-if="column.key === 'customer'">{{
                    record.customerNo || "—"
                  }}</code>
                  <strong v-else-if="column.key === 'value'">{{
                    assetValue(record)
                  }}</strong>
                  <Tag
                    v-else-if="column.key === 'status'"
                    :color="statusColor(record.status)"
                    >{{
                      $t(`plugin.idc-core.marketing.status.${record.status}`)
                    }}</Tag
                  >
                  <Space v-else-if="column.key === 'action'" :size="0">
                    <Button
                      size="small"
                      type="link"
                      @click="openAssetDetail(record.assetNo)"
                      >{{
                        $t("plugin.idc-core.marketing.directory.details")
                      }}</Button
                    >
                    <Button
                      v-if="
                        canRevokeAsset &&
                        ['active', 'issued'].includes(record.status) &&
                        record.reservedUses === 0
                      "
                      data-testid="marketing-asset-revoke"
                      danger
                      size="small"
                      type="link"
                      @click="
                        openMarketingAction('asset_revoke', { asset: record })
                      "
                      >{{
                        $t(
                          "plugin.idc-core.marketing.actions.asset_revoke.button",
                        )
                      }}</Button
                    >
                  </Space>
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>

            <Card
              :bordered="false"
              class="directory-card"
              data-testid="points-directory"
            >
              <template #title>{{
                $t("plugin.idc-core.marketing.assets.points")
              }}</template>
              <template v-if="canCreditPoints" #extra>
                <Button
                  data-testid="marketing-points-credit"
                  size="small"
                  type="primary"
                  @click="openMarketingAction('points_credit')"
                  >{{
                    $t("plugin.idc-core.marketing.actions.points_credit.button")
                  }}</Button
                >
              </template>
              <Input
                v-model:value="pointsCustomerNo"
                allow-clear
                :placeholder="
                  $t('plugin.idc-core.marketing.assets.searchCustomer')
                "
                @clear="resetPointsSearch"
                @press-enter="resetPointsSearch"
              />
              <VbenDataGrid
                :columns="pointsColumns"
                :data="pointsRows"
                :pagination="pointsPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadPointsAccounts(currentPage, pageSize)
                "
                :height="360"
                :loading="pointsLoading"
                row-key="customerNo"
              >
                <template #cell="{ column, record, value }">
                  <code v-if="column.key === 'customer'">{{
                    record.customerNo
                  }}</code>
                  <strong v-else-if="column.key === 'available'">{{
                    formatCount(record.availablePoints)
                  }}</strong>
                  <div
                    v-else-if="column.key === 'frozenDebt'"
                    class="balance-cell"
                  >
                    <span
                      >{{ $t("plugin.idc-core.marketing.currency.frozen") }}
                      {{ formatCount(record.frozenPoints) }}</span
                    >
                    <span
                      >{{ $t("plugin.idc-core.marketing.assets.debt") }}
                      {{ formatCount(record.debtPoints) }}</span
                    >
                  </div>
                  <span v-else-if="column.key === 'history'"
                    >{{ formatCount(record.lotCount) }} /
                    {{ formatCount(record.entryCount) }}</span
                  >
                  <Space v-else-if="column.key === 'action'" :size="0">
                    <Button
                      size="small"
                      type="link"
                      :data-testid="`marketing-points-detail-${record.customerNo}`"
                      @click="openPointsDetail(record.customerNo)"
                      >{{
                        $t("plugin.idc-core.marketing.directory.details")
                      }}</Button
                    >
                    <Button
                      v-if="canCreditPoints"
                      size="small"
                      type="link"
                      @click="
                        openMarketingAction('points_credit', {
                          pointsAccount: record,
                        })
                      "
                      >{{
                        $t(
                          "plugin.idc-core.marketing.actions.points_credit.button",
                        )
                      }}</Button
                    >
                  </Space>
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>

            <Card
              :bordered="false"
              class="directory-card"
              data-testid="value-grant-directory"
            >
              <template #title>{{
                $t("plugin.idc-core.marketing.assets.grants")
              }}</template>
              <Input
                v-model:value="valueGrantCustomerNo"
                allow-clear
                :placeholder="
                  $t('plugin.idc-core.marketing.assets.searchCustomer')
                "
                @clear="resetValueGrantSearch"
                @press-enter="resetValueGrantSearch"
              />
              <VbenDataGrid
                :columns="valueGrantColumns"
                :data="valueGrantRows"
                :pagination="valueGrantPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadValueGrants(currentPage, pageSize)
                "
                :height="360"
                :loading="valueGrantLoading"
                row-key="grantNo"
              >
                <template #cell="{ column, record, value }">
                  <div v-if="column.key === 'grant'" class="entity-cell">
                    <strong>{{
                      $t(`plugin.idc-core.marketing.type.${record.type}`)
                    }}</strong>
                    <code>{{ record.grantNo }}</code>
                  </div>
                  <code v-else-if="column.key === 'customer'">{{
                    record.customerNo
                  }}</code>
                  <strong v-else-if="column.key === 'balance'">{{
                    formatMinor(
                      record.availableAmountMinor,
                      record.currencyCode,
                    )
                  }}</strong>
                  <Tag
                    v-else-if="column.key === 'status'"
                    :color="statusColor(record.status)"
                    >{{
                      $t(`plugin.idc-core.marketing.status.${record.status}`)
                    }}</Tag
                  >
                  <Space v-else-if="column.key === 'action'" :size="0">
                    <Button
                      :data-testid="`value-grant-detail-${record.grantNo}`"
                      size="small"
                      type="link"
                      @click="openValueGrantDetail(record.grantNo)"
                      >{{
                        $t("plugin.idc-core.marketing.directory.details")
                      }}</Button
                    >
                    <Button
                      v-if="canPostGrant && record.status === 'pending'"
                      data-testid="value-grant-post-request"
                      size="small"
                      type="link"
                      @click="
                        openMarketingAction('grant_post', { grant: record })
                      "
                      >{{
                        $t("plugin.idc-core.marketing.actions.post")
                      }}</Button
                    >
                    <Button
                      v-if="
                        canReverseGrant &&
                        record.status !== 'pending' &&
                        record.status !== 'cancelled' &&
                        record.availableAmountMinor > 0
                      "
                      data-testid="value-grant-reverse-request"
                      size="small"
                      type="link"
                      @click="
                        openMarketingAction('grant_reverse', { grant: record })
                      "
                      >{{
                        $t("plugin.idc-core.marketing.actions.reverse")
                      }}</Button
                    >
                  </Space>
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>

            <Card
              v-if="canReviewGrant"
              :bordered="false"
              class="directory-card"
              data-testid="value-grant-adjustment-queue"
            >
              <template #title>{{
                $t("plugin.idc-core.marketing.actions.reviewQueue")
              }}</template>
              <Alert
                class="mb-3"
                show-icon
                type="info"
                :message="
                  $t('plugin.idc-core.marketing.actions.reviewQueueNote')
                "
              />
              <VbenDataGrid
                :columns="adjustmentColumns"
                :data="adjustmentRows"
                :pagination="adjustmentPager"
                @page-change="
                  ({ currentPage, pageSize }) =>
                    loadGrantAdjustments(currentPage, pageSize)
                "
                :height="360"
                :loading="adjustmentLoading"
                row-key="adjustmentNo"
              >
                <template #cell="{ column, record, value }">
                  <div v-if="column.key === 'adjustment'" class="entity-cell">
                    <strong>{{
                      $t(
                        `plugin.idc-core.marketing.actions.${record.adjustmentType}`,
                      )
                    }}</strong>
                    <code>{{ record.adjustmentNo }}</code>
                    <small>{{ record.grantNo }}</small>
                  </div>
                  <strong v-else-if="column.key === 'amount'">{{
                    formatMinor(Number(record.amountMinor), record.currencyCode)
                  }}</strong>
                  <div
                    v-else-if="column.key === 'requester'"
                    class="entity-cell"
                  >
                    <code>#{{ record.requestedBy }}</code>
                    <small>{{ formatTimestamp(record.createdAt) }}</small>
                  </div>
                  <Button
                    v-else-if="column.key === 'action'"
                    data-testid="value-grant-adjustment-review"
                    size="small"
                    type="primary"
                    @click="
                      openMarketingAction('grant_review', {
                        adjustment: record,
                      })
                    "
                    >{{
                      $t("plugin.idc-core.marketing.actions.review")
                    }}</Button
                  >
                  <template v-else>{{ value }}</template>
                </template>
              </VbenDataGrid>
            </Card>
          </div>
        </section>

        <section v-show="activeTab === 'recent'" class="table-stack">
          <Card :bordered="false" class="operations-card">
            <template #title>
              <div class="card-title">
                <span>{{
                  $t("plugin.idc-core.marketing.campaign.eyebrow")
                }}</span>
                <strong>{{
                  $t("plugin.idc-core.marketing.campaign.title")
                }}</strong>
              </div>
            </template>
            <VbenDataGrid
              :columns="campaignColumns"
              :data="data.recentCampaigns"
              :height="340"
              row-key="campaignNo"
            >
              <template #cell="{ column, record, value }">
                <div v-if="column.key === 'campaign'" class="entity-cell">
                  <strong>{{ localizedCampaignName(record) }}</strong>
                  <code>{{ record.campaignNo }}</code>
                </div>
                <Tag v-else-if="column.key === 'type'">{{
                  $t(`plugin.idc-core.marketing.type.${record.type}`)
                }}</Tag>
                <Tag
                  v-else-if="column.key === 'status'"
                  :color="statusColor(record.status)"
                  >{{
                    $t(`plugin.idc-core.marketing.status.${record.status}`)
                  }}</Tag
                >
                <div v-else-if="column.key === 'quota'" class="quota-cell">
                  <strong>{{ formatCount(record.globalConsumedCount) }}</strong>
                  <span
                    >/
                    {{
                      record.globalQuotaLimit === null
                        ? "∞"
                        : formatCount(record.globalQuotaLimit)
                    }}</span
                  >
                  <small
                    >{{ formatCount(record.globalReservedCount) }}
                    {{
                      $t("plugin.idc-core.marketing.metrics.reserved")
                    }}</small
                  >
                </div>
                <div v-else-if="column.key === 'window'" class="window-cell">
                  <span>{{ formatTimestamp(record.startsAt) }}</span>
                  <span>→ {{ formatTimestamp(record.endsAt) }}</span>
                </div>
                <template v-else>{{ value }}</template>
              </template>
            </VbenDataGrid>
          </Card>

          <Card :bordered="false" class="operations-card">
            <template #title>
              <div class="card-title">
                <span>{{ $t("plugin.idc-core.marketing.grant.eyebrow") }}</span>
                <strong>{{
                  $t("plugin.idc-core.marketing.grant.title")
                }}</strong>
              </div>
            </template>
            <VbenDataGrid
              :columns="grantColumns"
              :data="data.recentGrants"
              :height="340"
              row-key="grantNo"
            >
              <template #cell="{ column, record, value }">
                <div v-if="column.key === 'grant'" class="entity-cell">
                  <strong>{{
                    $t(`plugin.idc-core.marketing.type.${record.grantType}`)
                  }}</strong>
                  <code>{{ record.grantNo }}</code>
                </div>
                <code v-else-if="column.key === 'customer'">{{
                  record.customerNo
                }}</code>
                <div v-else-if="column.key === 'balance'" class="balance-cell">
                  <strong>{{
                    formatMinor(record.availableMinor, record.currencyCode)
                  }}</strong>
                  <span
                    >{{ $t("plugin.idc-core.marketing.currency.consumed") }}
                    {{
                      formatMinor(record.consumedMinor, record.currencyCode)
                    }}</span
                  >
                </div>
                <Tag
                  v-else-if="column.key === 'status'"
                  :color="statusColor(record.status)"
                  >{{
                    $t(`plugin.idc-core.marketing.status.${record.status}`)
                  }}</Tag
                >
                <span v-else-if="column.key === 'updated'">{{
                  formatTimestamp(record.updatedAt)
                }}</span>
                <template v-else>{{ value }}</template>
              </template>
            </VbenDataGrid>
          </Card>
        </section>
      </template>

      <VbenDrawerCompat
        :open="Boolean(segmentDetail)"
        :title="$t('plugin.idc-core.marketing.directory.segmentDetail')"
        :width="640"
        @after-open-change="handleDetailOpenChange"
        @close="segmentDetail = undefined"
      >
        <Skeleton v-if="detailLoading" active />
        <template v-else-if="segmentDetail">
          <div class="detail-heading">
            <h3>
              {{
                localizedName(
                  segmentDetail.segment,
                  segmentDetail.segment.segmentNo,
                )
              }}
            </h3>
            <code>{{ segmentDetail.segment.segmentNo }}</code>
          </div>
          <p class="detail-description">
            {{
              preferences.app.locale === "zh-CN"
                ? segmentDetail.description.zhCn
                : segmentDetail.description.enUs
            }}
          </p>
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.status')"
              ><Tag :color="statusColor(segmentDetail.segment.status)">{{
                $t(
                  `plugin.idc-core.marketing.status.${segmentDetail.segment.status}`,
                )
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.mode')"
              >{{
                $t(
                  `plugin.idc-core.marketing.mode.${segmentDetail.segment.mode}`,
                )
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.members')"
              >{{
                formatCount(segmentDetail.segment.activeMemberCount)
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.version')"
              >{{ segmentDetail.segment.version }}</DescriptionsItem
            >
          </Descriptions>
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.directory.conditions") }}
          </h4>
          <div v-if="segmentDetail.conditions.length" class="rule-list">
            <article
              v-for="condition in segmentDetail.conditions"
              :key="condition.conditionNo"
              class="rule-row"
            >
              <Tag>{{ condition.fact }}</Tag
              ><strong>{{ condition.operator }}</strong
              ><span>{{ conditionValue(condition) }}</span
              ><small>OR {{ condition.orGroupNo }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="
              $t('plugin.idc-core.marketing.directory.noConditions')
            "
          />
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.directory.snapshots") }}
          </h4>
          <div v-if="segmentDetail.snapshots.length" class="snapshot-list">
            <article
              v-for="snapshot in segmentDetail.snapshots"
              :key="snapshot.snapshotNo"
            >
              <div>
                <code>{{ snapshot.snapshotNo }}</code
                ><Tag :color="statusColor(snapshot.status)">{{
                  snapshot.status
                }}</Tag>
              </div>
              <strong>{{ formatCount(snapshot.memberCount) }}</strong
              ><small>{{ formatTimestamp(snapshot.startedAt) }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.directory.noSnapshots')"
          />
        </template>
      </VbenDrawerCompat>

      <VbenDrawerCompat
        :open="Boolean(campaignDetail)"
        :title="$t('plugin.idc-core.marketing.directory.campaignDetail')"
        :width="680"
        @after-open-change="handleDetailOpenChange"
        @close="campaignDetail = undefined"
      >
        <Skeleton v-if="detailLoading" active />
        <template v-else-if="campaignDetail">
          <div class="detail-heading">
            <h3>
              {{
                localizedName(
                  campaignDetail.campaign,
                  campaignDetail.campaign.campaignNo,
                )
              }}
            </h3>
            <code>{{ campaignDetail.campaign.campaignNo }}</code>
          </div>
          <p class="detail-description">
            {{
              preferences.app.locale === "zh-CN"
                ? campaignDetail.description.zhCn
                : campaignDetail.description.enUs
            }}
          </p>
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.status')"
              ><Tag :color="statusColor(campaignDetail.campaign.status)">{{
                $t(
                  `plugin.idc-core.marketing.status.${campaignDetail.campaign.status}`,
                )
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.type')"
              >{{
                $t(
                  `plugin.idc-core.marketing.type.${campaignDetail.campaign.type}`,
                )
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.window')"
              >{{ formatTimestamp(campaignDetail.campaign.startsAt) }} –
              {{
                formatTimestamp(campaignDetail.campaign.endsAt)
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.version')"
              >{{ campaignDetail.version }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="
                $t('plugin.idc-core.marketing.directory.newcomerExclusive')
              "
            >
              <Space>
                <Switch
                  v-model:checked="campaignDetail.campaign.isNewcomerExclusive"
                  :disabled="!canSetCampaignNewcomer || newcomerSaving"
                />
                <Button
                  v-if="canSetCampaignNewcomer"
                  :loading="newcomerSaving"
                  size="small"
                  @click="saveCampaignNewcomerSetting"
                  >{{
                    $t("plugin.idc-core.marketing.directory.saveSetting")
                  }}</Button
                >
              </Space>
            </DescriptionsItem>
          </Descriptions>
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.directory.scopes") }}
          </h4>
          <div v-if="campaignDetail.scopes.length" class="rule-list">
            <article
              v-for="scope in campaignDetail.scopes"
              :key="`${scope.kind}-${scope.matchMode}-${scope.referenceCode}`"
              class="rule-row"
            >
              <Tag>{{ scope.kind }}</Tag
              ><strong>{{ scope.matchMode }}</strong
              ><code>{{ scope.referenceCode || "*" }}</code>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.directory.noScopes')"
          />
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.directory.benefits") }}
          </h4>
          <div v-if="campaignDetail.benefits.length" class="benefit-list">
            <article
              v-for="benefit in campaignDetail.benefits"
              :key="benefit.benefitNo"
            >
              <div>
                <Tag>{{ benefit.type }}</Tag
                ><code>{{ benefit.benefitNo }}</code>
                <Button
                  v-if="
                    canIssueAsset &&
                    campaignDetail.campaign.status === 'active' &&
                    ['coupon', 'voucher'].includes(
                      campaignDetail.campaign.type,
                    ) &&
                    benefit.status === 'active'
                  "
                  size="small"
                  type="link"
                  @click="
                    openMarketingAction('asset_issue', {
                      campaignNo: campaignDetail.campaign.campaignNo,
                      benefitNo: benefit.benefitNo,
                    })
                  "
                  >{{
                    $t("plugin.idc-core.marketing.actions.asset_issue.button")
                  }}</Button
                >
              </div>
              <strong>{{ benefitValue(benefit) }}</strong
              ><small
                >{{ benefit.purchasePhase }} · {{ benefit.cycleScope }}</small
              >
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.directory.noBenefits')"
          />
        </template>
      </VbenDrawerCompat>

      <VbenDrawerCompat
        :open="Boolean(assetDetail)"
        :title="$t('plugin.idc-core.marketing.assets.assetDetail')"
        :width="680"
        @after-open-change="handleDetailOpenChange"
        @close="assetDetail = undefined"
      >
        <Skeleton v-if="detailLoading" active />
        <template v-else-if="assetDetail">
          <div class="detail-heading">
            <h3>
              {{
                $t(`plugin.idc-core.marketing.type.${assetDetail.asset.type}`)
              }}
            </h3>
            <code>{{ assetDetail.asset.assetNo }}</code>
          </div>
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.customer')"
              >{{ assetDetail.asset.customerNo || "—" }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.status')"
              ><Tag :color="statusColor(assetDetail.asset.status)">{{
                $t(
                  `plugin.idc-core.marketing.status.${assetDetail.asset.status}`,
                )
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.value')"
              >{{ assetValue(assetDetail.asset) }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.usage')"
              >{{ assetDetail.asset.reservedUses }} /
              {{ assetDetail.asset.consumedUses }} /
              {{ assetDetail.asset.usageLimit }}</DescriptionsItem
            >
          </Descriptions>
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.assets.reservations") }}
          </h4>
          <div v-if="assetDetail.reservations.length" class="snapshot-list">
            <article
              v-for="item in assetDetail.reservations"
              :key="item.reservationNo"
            >
              <div>
                <code>{{ item.reservationNo }}</code
                ><Tag :color="statusColor(item.status)">{{
                  $t(`plugin.idc-core.marketing.status.${item.status}`)
                }}</Tag>
              </div>
              <strong>{{
                formatMinor(
                  item.discountAmountMinor,
                  assetDetail.asset.currencyCode,
                )
              }}</strong
              ><small>{{ formatTimestamp(item.createdAt) }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.assets.noReservations')"
          />
        </template>
      </VbenDrawerCompat>

      <VbenDrawerCompat
        :open="Boolean(pointsDetail)"
        :title="$t('plugin.idc-core.marketing.assets.pointsDetail')"
        :width="760"
        @after-open-change="handleDetailOpenChange"
        @close="pointsDetail = undefined"
      >
        <Skeleton v-if="detailLoading" active />
        <template v-else-if="pointsDetail">
          <div class="detail-heading">
            <h3>{{ pointsDetail.account.customerNo }}</h3>
            <code>{{
              $t("plugin.idc-core.marketing.assets.nonMonetary")
            }}</code>
          </div>
          <Descriptions bordered :column="3" size="small">
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.available')"
              >{{
                formatCount(pointsDetail.account.availablePoints)
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.currency.frozen')"
              >{{
                formatCount(pointsDetail.account.frozenPoints)
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.debt')"
              >{{
                formatCount(pointsDetail.account.debtPoints)
              }}</DescriptionsItem
            >
          </Descriptions>
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.assets.lots") }}
          </h4>
          <div v-if="pointsDetail.lots.length" class="benefit-list">
            <article v-for="item in pointsDetail.lots" :key="item.lotNo">
              <div>
                <code>{{ item.lotNo }}</code
                ><Tag :color="statusColor(item.status)">{{
                  $t(`plugin.idc-core.marketing.status.${item.status}`)
                }}</Tag>
                <Button
                  v-if="
                    canReversePoints &&
                    ['order', 'activity', 'manual', 'migration'].includes(
                      item.sourceType,
                    ) &&
                    item.reversedPoints < item.originalPoints
                  "
                  data-testid="marketing-points-reverse"
                  danger
                  size="small"
                  type="link"
                  @click="
                    openMarketingAction('points_reverse', {
                      pointsAccount: pointsDetail.account,
                      lot: item,
                    })
                  "
                  >{{
                    $t(
                      "plugin.idc-core.marketing.actions.points_reverse.button",
                    )
                  }}</Button
                >
              </div>
              <strong
                >{{ formatCount(item.availablePoints) }} /
                {{ formatCount(item.originalPoints) }}</strong
              ><small>{{ item.sourceType }} · {{ item.sourceReference }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.assets.noLots')"
          />
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.assets.entries") }}
          </h4>
          <div v-if="pointsDetail.entries.length" class="benefit-list">
            <article v-for="item in pointsDetail.entries" :key="item.entryNo">
              <div>
                <code>{{ item.entryNo }}</code
                ><Tag>{{ item.type }}</Tag>
              </div>
              <strong
                >{{ item.availableDelta >= 0 ? "+" : ""
                }}{{ formatCount(item.availableDelta) }}</strong
              ><small>{{ item.sourceType }} · {{ item.sourceReference }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.assets.noEntries')"
          />
        </template>
      </VbenDrawerCompat>

      <MarketingActionModalRef @success="handleMarketingActionSuccess" />
      <MarketingCouponBatchModalRef @success="loadAssets()" />

      <VbenDrawerCompat
        :open="Boolean(valueGrantDetail)"
        :title="$t('plugin.idc-core.marketing.assets.grantDetail')"
        :width="760"
        @after-open-change="handleDetailOpenChange"
        @close="valueGrantDetail = undefined"
      >
        <Skeleton v-if="detailLoading" active />
        <template v-else-if="valueGrantDetail">
          <div class="detail-heading">
            <h3>
              {{
                $t(
                  `plugin.idc-core.marketing.type.${valueGrantDetail.grant.type}`,
                )
              }}
            </h3>
            <code>{{ valueGrantDetail.grant.grantNo }}</code>
          </div>
          <Descriptions bordered :column="2" size="small">
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.customer')"
              >{{ valueGrantDetail.grant.customerNo }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.directory.status')"
              ><Tag :color="statusColor(valueGrantDetail.grant.status)">{{
                $t(
                  `plugin.idc-core.marketing.status.${valueGrantDetail.grant.status}`,
                )
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.available')"
              >{{
                formatMinor(
                  valueGrantDetail.grant.availableAmountMinor,
                  valueGrantDetail.grant.currencyCode,
                )
              }}</DescriptionsItem
            >
            <DescriptionsItem
              :label="$t('plugin.idc-core.marketing.assets.posting')"
              >{{
                valueGrantDetail.grant.financialPostingReference || "—"
              }}</DescriptionsItem
            >
          </Descriptions>
          <h4 class="detail-section-title">
            {{ $t("plugin.idc-core.marketing.assets.entries") }}
          </h4>
          <div v-if="valueGrantDetail.entries.length" class="benefit-list">
            <article
              v-for="item in valueGrantDetail.entries"
              :key="item.entryNo"
            >
              <div>
                <code>{{ item.entryNo }}</code
                ><Tag>{{ item.type }}</Tag>
              </div>
              <strong>{{
                formatMinor(
                  Math.abs(
                    item.availableDeltaMinor ||
                      item.pendingDeltaMinor ||
                      item.frozenDeltaMinor ||
                      item.consumedDeltaMinor ||
                      item.expiredDeltaMinor ||
                      item.reversedDeltaMinor,
                  ),
                  valueGrantDetail.grant.currencyCode,
                )
              }}</strong
              ><small>{{ item.sourceType }} · {{ item.sourceReference }}</small>
            </article>
          </div>
          <Empty
            v-else
            :description="$t('plugin.idc-core.marketing.assets.noEntries')"
          />
        </template>
      </VbenDrawerCompat>
    </div>
  </Page>
</template>

<style scoped>
.marketing-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.marketing-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 4px 0;
}

.marketing-page-header h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
}

.marketing-page-header p {
  margin: 4px 0 0;
  color: hsl(var(--muted-foreground));
  font-size: 13px;
}

.marketing-tabs {
  margin-bottom: -12px;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  padding: 0 16px;
}

.marketing-eyebrow,
.section-heading span,
.card-title span {
  color: #1677ff;
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.section-heading p {
  color: hsl(var(--muted-foreground));
  margin: 0;
  max-width: 720px;
}

.snapshot-time {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  white-space: nowrap;
}

.signal-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.signal-card {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-top: 3px solid #64748b;
  border-radius: 0;
  display: flex;
  flex-direction: column;
  min-height: 118px;
  padding: 13px 14px 12px;
}

.signal-card--violet {
  border-top-color: #1677ff;
}
.signal-card--coral {
  border-top-color: #1677ff;
}
.signal-card--amber {
  border-top-color: #f59e0b;
}
.signal-card--cyan {
  border-top-color: #1677ff;
}
.signal-card--green {
  border-top-color: #16a34a;
}

.signal-label {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  font-weight: 650;
}

.signal-card strong {
  font-size: 22px;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1;
  margin: 14px 0 10px;
}

.signal-card p,
.signal-card > span {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  line-height: 1.5;
  margin: 0;
}

.signal-danger {
  color: #dc2626 !important;
}

.section-block {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 16px;
}

.section-heading {
  align-items: flex-end;
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}

.section-heading h2 {
  font-size: 18px;
  margin: 4px 0 0;
}

.section-heading p {
  font-size: 12px;
}

.currency-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
}

.currency-card {
  background: hsl(var(--muted) / 0.32);
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 14px;
}

.currency-code {
  color: #1677ff;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.12em;
}

.currency-primary {
  align-items: baseline;
  display: flex;
  justify-content: space-between;
  margin: 12px 0;
}

.currency-primary span,
.currency-card dt,
.currency-card small {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}

.currency-primary strong {
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}
.currency-card dl {
  display: grid;
  gap: 7px;
  grid-template-columns: 1fr 1fr;
  margin: 0 0 12px;
}
.currency-card dl div {
  display: flex;
  justify-content: space-between;
}
.currency-card dd {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  margin: 0;
}

.table-stack {
  display: grid;
  gap: 12px;
}
.directory-section {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 16px;
}
.asset-directory-section {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  padding: 16px;
}
.asset-directory-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 640px), 1fr));
}
.asset-directory-grid :deep(.ant-input-affix-wrapper) {
  margin-bottom: 14px;
}
.directory-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.directory-card {
  border: 1px solid hsl(var(--border));
  min-width: 0;
}
.directory-filters {
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(160px, 1fr) 132px 132px;
  margin-bottom: 14px;
}
.directory-pager {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 12px;
}
.detail-heading {
  align-items: flex-start;
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 10px;
}
.detail-heading h3 {
  font-size: 20px;
  margin: 0;
}
.detail-heading code,
.detail-description,
.rule-row small,
.snapshot-list small,
.benefit-list small {
  color: hsl(var(--muted-foreground));
}
.detail-description {
  line-height: 1.65;
  margin: 0 0 18px;
}
.detail-section-title {
  font-size: 14px;
  margin: 24px 0 10px;
}
.rule-list,
.snapshot-list,
.benefit-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.rule-row,
.snapshot-list article,
.benefit-list article {
  align-items: center;
  background: hsl(var(--muted) / 0.35);
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: flex;
  gap: 10px;
  min-height: 46px;
  padding: 9px 12px;
}
.rule-row span,
.rule-row code,
.snapshot-list article div,
.benefit-list article div {
  flex: 1;
  min-width: 0;
}
.snapshot-list article div,
.benefit-list article div {
  align-items: center;
  display: flex;
  gap: 8px;
}
.operations-card {
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  overflow: hidden;
}
.card-title {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.card-title strong {
  font-size: 16px;
}
.entity-cell,
.quota-cell,
.window-cell,
.balance-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.entity-cell code,
.quota-cell small,
.window-cell,
.balance-cell span {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.quota-cell strong,
.balance-cell strong {
  font-variant-numeric: tabular-nums;
}

:deep(.ant-btn),
:deep(.ant-card),
:deep(.ant-input),
:deep(.ant-input-affix-wrapper),
:deep(.ant-select-selector) {
  border-radius: 0 !important;
}

@media (max-width: 1180px) {
  .signal-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .directory-grid {
    grid-template-columns: 1fr;
  }
  .asset-directory-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 16px;
  }
  .signal-grid {
    grid-template-columns: 1fr 1fr;
  }
  .directory-filters {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .signal-grid {
    grid-template-columns: 1fr;
  }
}
</style>
