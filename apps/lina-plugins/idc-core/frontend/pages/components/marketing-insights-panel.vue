<script setup lang="ts">
import type { Dayjs } from "dayjs";
import type { TableColumnsType } from "ant-design-vue";
import type {
  MarketingAbuseCaseItem,
  MarketingMissingCostItem,
  MarketingPerformanceItem,
} from "../marketing-client";

import { computed, onMounted, onBeforeUnmount, ref } from "vue";
import { createNumberedPage } from "../numbered-page";

import { useAccess } from "../plugin-access";
import { preferences } from "@vben/preferences";

import dayjs from "dayjs";
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  Modal as AntModal,
  Select,
  Space,
  Tag,
  Textarea,
} from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import VbenDataGrid from "./vben-data-grid.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

import {
  marketingAbuseCases,
  marketingAbuseReview,
  marketingCorrectMissingCost,
  marketingMissingCosts,
  marketingPerformance,
} from "../marketing-client";

const { RangePicker } = DatePicker;
const { hasAccessByCodes } = useAccess();
const canViewPerformance = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:performance:list"]),
);
const canViewAbuse = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:abuse:list"]),
);
const canReviewAbuse = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:abuse:review"]),
);
const canViewMissingCosts = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:cost:missing:list"]),
);
const canCorrectMissingCosts = computed(() =>
  hasAccessByCodes(["idc:admin:marketing:cost:correct"]),
);

const range = ref<[Dayjs, Dayjs]>([
  dayjs().subtract(29, "day").startOf("day"),
  dayjs().endOf("day"),
]);
const performanceLoading = ref(false);
const performanceError = ref("");
const performanceRows = ref<MarketingPerformanceItem[]>([]);
const abuseLoading = ref(false);
const abuseError = ref("");
const abuseRows = ref<MarketingAbuseCaseItem[]>([]);
const abuseStatus = ref("");
const reviewOpen = ref(false);
const reviewSubmitting = ref(false);
const reviewDecision = ref<"confirmed_abuse" | "false_positive">(
  "confirmed_abuse",
);
const reviewReason = ref("");
const reviewTarget = ref<MarketingAbuseCaseItem>();
const missingCostsOpen = ref(false);
const missingCostsLoading = ref(false);
const missingCostsError = ref("");
const missingCostRows = ref<MarketingMissingCostItem[]>([]);
const costCorrectionOpen = ref(false);
const costCorrectionSubmitting = ref(false);
const costCorrectionTarget = ref<MarketingMissingCostItem>();
const costCorrectionUnit = ref("");
const costCorrectionReason = ref("");
const costCorrectionRequestNo = ref("");
const performancePager = ref({ currentPage: 1, pageSize: 10, total: 0 });
const abusePager = ref({ currentPage: 1, pageSize: 10, total: 0 });
const missingCostPager = ref({ currentPage: 1, pageSize: 10, total: 0 });
let performanceSequence = 0;
let abuseSequence = 0;
let missingCostSequence = 0;
const performancePages = createNumberedPage<MarketingPerformanceItem>(
  (state) => {
    performanceRows.value = state.items;
    performancePager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
);
const abusePages = createNumberedPage<MarketingAbuseCaseItem>(
  (state) => {
    abuseRows.value = state.items;
    abusePager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  () => {
    reviewOpen.value = false;
    reviewTarget.value = undefined;
  },
);
const missingCostPages = createNumberedPage<MarketingMissingCostItem>(
  (state) => {
    missingCostRows.value = state.items;
    missingCostPager.value = {
      currentPage: state.currentPage,
      pageSize: state.pageSize,
      total: state.total,
    };
  },
  () => {
    costCorrectionOpen.value = false;
    costCorrectionTarget.value = undefined;
  },
);
const performanceGridRows = computed(() =>
  performanceRows.value.map((row) => ({
    ...row,
    groupKey: `${row.campaignNo}:${row.currencyCode}`,
  })),
);

const performanceColumns = computed<TableColumnsType<MarketingPerformanceItem>>(
  () => [
    {
      key: "campaign",
      title: $t("plugin.idc-core.marketing.insights.campaign"),
    },
    {
      dataIndex: "currencyCode",
      key: "currency",
      title: $t("plugin.idc-core.marketing.insights.currency"),
      width: 100,
    },
    ...(["exposure", "claim", "use", "order"] as const).map((key) => ({
      dataIndex: key,
      key,
      title: $t(`plugin.idc-core.marketing.insights.${key}`),
      width: 96,
    })),
    ...(["receiptMinor", "refundMinor", "costMinor", "netMinor"] as const).map(
      (key) => ({
        dataIndex: key,
        key,
        title: $t(`plugin.idc-core.marketing.insights.${key}`),
        width: 126,
      }),
    ),
  ],
);

const abuseColumns = computed<TableColumnsType<MarketingAbuseCaseItem>>(() => [
  {
    dataIndex: "caseNo",
    key: "caseNo",
    title: $t("plugin.idc-core.marketing.insights.caseNo"),
    width: 190,
  },
  {
    key: "signal",
    title: $t("plugin.idc-core.marketing.insights.signal"),
    width: 180,
  },
  {
    dataIndex: "campaignNo",
    key: "campaignNo",
    title: $t("plugin.idc-core.marketing.insights.campaign"),
    width: 170,
  },
  {
    dataIndex: "relatedAccountCount",
    key: "related",
    title: $t("plugin.idc-core.marketing.insights.relatedCount"),
    width: 120,
  },
  {
    key: "evidence",
    title: $t("plugin.idc-core.marketing.insights.evidence"),
    width: 170,
  },
  {
    key: "status",
    title: $t("plugin.idc-core.marketing.insights.status"),
    width: 140,
  },
  {
    key: "actions",
    title: $t("plugin.idc-core.marketing.insights.actions"),
    fixed: "right",
    width: 210,
  },
]);

const missingCostColumns = computed<TableColumnsType<MarketingMissingCostItem>>(
  () => [
    {
      dataIndex: "orderNo",
      key: "orderNo",
      title: $t("plugin.idc-core.marketing.insights.orderNo"),
      width: 190,
    },
    {
      dataIndex: "bindingNo",
      key: "bindingNo",
      title: $t("plugin.idc-core.marketing.insights.bindingNo"),
      width: 190,
    },
    {
      key: "product",
      title: $t("plugin.idc-core.marketing.insights.product"),
      width: 180,
    },
    {
      dataIndex: "currencyCode",
      key: "currencyCode",
      title: $t("plugin.idc-core.marketing.insights.currency"),
      width: 90,
    },
    {
      key: "period",
      title: $t("plugin.idc-core.marketing.insights.period"),
      width: 120,
    },
    {
      dataIndex: "quantity",
      key: "quantity",
      title: $t("plugin.idc-core.marketing.insights.quantity"),
      width: 100,
    },
    {
      dataIndex: "purchaseCycles",
      key: "purchaseCycles",
      title: $t("plugin.idc-core.marketing.insights.purchaseCycles"),
      width: 100,
    },
    {
      key: "actions",
      title: $t("plugin.idc-core.marketing.insights.actions"),
      fixed: "right",
      width: 120,
    },
  ],
);

function money(value: string, currency: string) {
  return currency ? `${value} ${currency}` : value;
}

function performanceMoney(record: MarketingPerformanceItem, key: string) {
  if (!record.currencyCode) return "—";
  if ((key === "costMinor" || key === "netMinor") && !record.costComplete) {
    return $t("plugin.idc-core.marketing.insights.costMissing");
  }
  const value =
    record[key as "receiptMinor" | "refundMinor" | "costMinor" | "netMinor"];
  return money(value, record.currencyCode);
}

async function loadPerformance(
  page = performancePager.value.currentPage,
  pageSize = performancePager.value.pageSize,
) {
  if (!canViewPerformance.value) return;
  if (range.value[1].diff(range.value[0], "day", true) > 366) {
    performanceError.value = $t(
      "plugin.idc-core.marketing.insights.rangeTooLong",
    );
    return;
  }
  performanceLoading.value = true;
  performanceError.value = "";
  const sequence = ++performanceSequence;
  const startedAt = range.value[0].valueOf();
  const endedAt = range.value[1].valueOf();
  try {
    await performancePages.query(
      { currentPage: page, pageSize },
      async (currentPage, limit) => {
        try {
          const result = await marketingPerformance({
            startedAt,
            endedAt,
            page: currentPage,
            limit,
          });
          return { items: result.rows, total: result.total };
        } catch (error) {
          if (sequence === performanceSequence)
            performanceError.value =
              error instanceof Error ? error.message : String(error);
          throw error;
        }
      },
    );
  } finally {
    if (sequence === performanceSequence) performanceLoading.value = false;
  }
}

async function loadAbuse(
  page = abusePager.value.currentPage,
  pageSize = abusePager.value.pageSize,
) {
  if (!canViewAbuse.value) return;
  abuseLoading.value = true;
  abuseError.value = "";
  const sequence = ++abuseSequence;
  const status = abuseStatus.value || undefined;
  try {
    await abusePages.query(
      { currentPage: page, pageSize },
      async (currentPage, limit) => {
        try {
          const result = await marketingAbuseCases({
            limit,
            page: currentPage,
            status,
          });
          return { items: result.cases, total: result.total };
        } catch (error) {
          if (sequence === abuseSequence)
            abuseError.value =
              error instanceof Error ? error.message : String(error);
          throw error;
        }
      },
    );
  } finally {
    if (sequence === abuseSequence) abuseLoading.value = false;
  }
}

async function openMissingCosts() {
  if (!canViewMissingCosts.value) return;
  missingCostsOpen.value = true;
  await loadMissingCosts(1);
}

async function loadMissingCosts(
  page = missingCostPager.value.currentPage,
  pageSize = missingCostPager.value.pageSize,
) {
  if (!canViewMissingCosts.value) return;
  missingCostsLoading.value = true;
  missingCostsError.value = "";
  const sequence = ++missingCostSequence;
  try {
    await missingCostPages.query(
      { currentPage: page, pageSize },
      async (currentPage, limit) => {
        try {
          const result = await marketingMissingCosts({
            limit,
            page: currentPage,
          });
          return { items: result.items, total: result.total };
        } catch (error) {
          if (sequence === missingCostSequence)
            missingCostsError.value =
              error instanceof Error ? error.message : String(error);
          throw error;
        }
      },
    );
  } finally {
    if (sequence === missingCostSequence) missingCostsLoading.value = false;
  }
}

function openCostCorrection(item: MarketingMissingCostItem) {
  costCorrectionTarget.value = item;
  costCorrectionUnit.value = "";
  costCorrectionReason.value = "";
  costCorrectionRequestNo.value = `cost_${crypto.randomUUID().replaceAll("-", "")}`;
  costCorrectionOpen.value = true;
}

function validCostCorrection() {
  const reasonLength = [...costCorrectionReason.value.trim()].length;
  return (
    /^\d+$/.test(costCorrectionUnit.value.trim()) &&
    reasonLength >= 2 &&
    reasonLength <= 500
  );
}

function confirmCostCorrection() {
  if (!costCorrectionTarget.value || !validCostCorrection()) return;
  AntModal.confirm({
    title: $t("plugin.idc-core.marketing.insights.costConfirmTitle"),
    content: $t("plugin.idc-core.marketing.insights.costConfirmDescription"),
    async onOk() {
      if (!costCorrectionTarget.value) return;
      costCorrectionSubmitting.value = true;
      try {
        await marketingCorrectMissingCost(
          costCorrectionTarget.value.orderItemNo,
          {
            expectedVersion: costCorrectionTarget.value.version,
            reason: costCorrectionReason.value.trim(),
            requestNo: costCorrectionRequestNo.value,
            unitCostMinor: costCorrectionUnit.value.trim(),
          },
        );
        costCorrectionOpen.value = false;
        await Promise.all([openMissingCosts(), loadPerformance()]);
      } finally {
        costCorrectionSubmitting.value = false;
      }
    },
  });
}

function openReview(
  item: MarketingAbuseCaseItem,
  decision: "confirmed_abuse" | "false_positive",
) {
  reviewTarget.value = item;
  reviewDecision.value = decision;
  reviewReason.value = "";
  reviewOpen.value = true;
}

function confirmReview() {
  const reason = reviewReason.value.trim();
  if (!reviewTarget.value || reason.length < 2 || reason.length > 500) return;
  AntModal.confirm({
    title: $t("plugin.idc-core.marketing.insights.confirmTitle"),
    content: $t(
      `plugin.idc-core.marketing.insights.confirm.${reviewDecision.value}`,
    ),
    async onOk() {
      if (!reviewTarget.value) return;
      reviewSubmitting.value = true;
      try {
        await marketingAbuseReview(reviewTarget.value.caseNo, {
          decision: reviewDecision.value,
          expectedVersion: reviewTarget.value.version,
          reason,
        });
        reviewOpen.value = false;
        await loadAbuse();
      } finally {
        reviewSubmitting.value = false;
      }
    },
  });
}

onMounted(() => Promise.all([loadPerformance(), loadAbuse()]));
onBeforeUnmount(() => {
  performanceSequence += 1;
  abuseSequence += 1;
  missingCostSequence += 1;
  performancePages.invalidate();
  abusePages.invalidate();
  missingCostPages.invalidate();
});
</script>

<template>
  <section class="insights-grid">
    <Card
      v-if="canViewPerformance"
      data-testid="marketing-performance-report"
      :title="$t('plugin.idc-core.marketing.insights.performanceTitle')"
    >
      <Space direction="vertical" style="width: 100%" :size="16">
        <Space wrap>
          <RangePicker
            v-model:value="range"
            :allow-clear="false"
            :disabled-date="(date) => date.isAfter(dayjs().endOf('day'))"
          />
          <Button
            type="primary"
            :loading="performanceLoading"
            @click="loadPerformance(1)"
            >{{ $t("plugin.idc-core.marketing.insights.query") }}</Button
          >
          <Button v-if="canViewMissingCosts" @click="openMissingCosts">{{
            $t("plugin.idc-core.marketing.insights.missingCosts")
          }}</Button>
          <span>{{ $t("plugin.idc-core.marketing.insights.rangeNote") }}</span>
        </Space>
        <Alert
          v-if="performanceError"
          type="error"
          show-icon
          :message="$t('plugin.idc-core.marketing.insights.loadFailed')"
          :description="performanceError"
        />
        <VbenDataGrid
          :columns="performanceColumns"
          :data="performanceGridRows"
          :height="380"
          :loading="performanceLoading"
          row-key="groupKey"
          :pagination="performancePager"
          @page-change="
            ({ currentPage, pageSize }) =>
              loadPerformance(currentPage, pageSize)
          "
        >
          <template #cell="{ column, record, value }">
            <template v-if="column.key === 'campaign'">
              <strong>{{
                preferences.app.locale === "zh-CN"
                  ? record.nameZhCn
                  : record.nameEnUs
              }}</strong
              ><br />
              <span>{{ record.campaignNo }}</span>
            </template>
            <template v-else-if="column.key === 'currency'">{{
              record.currencyCode ||
              $t("plugin.idc-core.marketing.insights.nonMonetary")
            }}</template>
            <template
              v-else-if="
                [
                  'receiptMinor',
                  'refundMinor',
                  'costMinor',
                  'netMinor',
                ].includes(String(column.key))
              "
            >
              {{ performanceMoney(record, String(column.key)) }}
            </template>
            <template v-else>{{ value }}</template>
          </template>
        </VbenDataGrid>
      </Space>
    </Card>

    <Card
      v-if="canViewAbuse"
      data-testid="marketing-abuse-cases"
      :title="$t('plugin.idc-core.marketing.insights.abuseTitle')"
    >
      <Space direction="vertical" style="width: 100%" :size="16">
        <Space wrap>
          <Select
            v-model:value="abuseStatus"
            style="width: 180px"
            :options="
              ['', 'pending_review', 'confirmed_abuse', 'false_positive'].map(
                (value) => ({
                  value,
                  label: $t(
                    `plugin.idc-core.marketing.insights.statusCode.${value || 'all'}`,
                  ),
                }),
              )
            "
          />
          <Button :loading="abuseLoading" @click="loadAbuse(1)">{{
            $t("plugin.idc-core.marketing.insights.query")
          }}</Button>
        </Space>
        <Alert
          v-if="abuseError"
          type="error"
          show-icon
          :message="$t('plugin.idc-core.marketing.insights.loadFailed')"
          :description="abuseError"
        />
        <VbenDataGrid
          :columns="abuseColumns"
          :data="abuseRows"
          :height="380"
          :loading="abuseLoading"
          row-key="caseNo"
          :pagination="abusePager"
          @page-change="
            ({ currentPage, pageSize }) => loadAbuse(currentPage, pageSize)
          "
        >
          <template #cell="{ column, record, value }">
            <template v-if="column.key === 'signal'">{{
              $t(
                `plugin.idc-core.marketing.insights.signalCode.${record.signalCode}`,
              )
            }}</template>
            <template v-else-if="column.key === 'evidence'">
              {{
                $t(
                  `plugin.idc-core.marketing.insights.evidenceCode.${record.evidenceSummaryCode}`,
                )
              }}
              · {{ record.observedCount }}<br />
              <span
                >{{ formatTimestamp(record.windowStartedAt) }}—{{
                  formatTimestamp(record.windowEndedAt)
                }}</span
              >
            </template>
            <template v-else-if="column.key === 'status'"
              ><Tag>{{
                $t(
                  `plugin.idc-core.marketing.insights.statusCode.${record.statusCode}`,
                )
              }}</Tag></template
            >
            <template
              v-else-if="
                column.key === 'actions' &&
                record.statusCode === 'pending_review' &&
                canReviewAbuse
              "
            >
              <Space>
                <Button
                  danger
                  size="small"
                  @click="openReview(record, 'confirmed_abuse')"
                  >{{
                    $t("plugin.idc-core.marketing.insights.confirmAbuse")
                  }}</Button
                >
                <Button
                  size="small"
                  @click="openReview(record, 'false_positive')"
                  >{{
                    $t(
                      "plugin.idc-core.marketing.insights.confirmFalsePositive",
                    )
                  }}</Button
                >
              </Space>
            </template>
            <template v-else>{{ value }}</template>
          </template>
        </VbenDataGrid>
      </Space>
    </Card>
  </section>

  <VbenModalCompat
    v-model:open="reviewOpen"
    :confirm-loading="reviewSubmitting"
    :title="$t('plugin.idc-core.marketing.insights.reviewTitle')"
    :ok-button-props="{
      disabled:
        reviewReason.trim().length < 2 || reviewReason.trim().length > 500,
    }"
    @ok="confirmReview"
  >
    <p>
      {{ $t(`plugin.idc-core.marketing.insights.confirm.${reviewDecision}`) }}
    </p>
    <Textarea
      v-model:value="reviewReason"
      :maxlength="500"
      :rows="5"
      show-count
      :placeholder="$t('plugin.idc-core.marketing.insights.reasonPlaceholder')"
    />
  </VbenModalCompat>

  <VbenModalCompat
    v-model:open="missingCostsOpen"
    :footer="null"
    width="1100px"
    :title="$t('plugin.idc-core.marketing.insights.missingCostsTitle')"
  >
    <Alert
      v-if="missingCostsError"
      type="error"
      show-icon
      :message="$t('plugin.idc-core.marketing.insights.loadFailed')"
      :description="missingCostsError"
    />
    <VbenDataGrid
      :columns="missingCostColumns"
      :data="missingCostRows"
      :height="440"
      :loading="missingCostsLoading"
      row-key="orderItemNo"
      :pagination="missingCostPager"
      @page-change="
        ({ currentPage, pageSize }) => loadMissingCosts(currentPage, pageSize)
      "
    >
      <template #cell="{ column, record, value }">
        <template v-if="column.key === 'product'">{{
          preferences.app.locale === "zh-CN"
            ? record.productNameZhCn
            : record.productNameEnUs
        }}</template>
        <template v-else-if="column.key === 'period'"
          >{{ record.periodCount }} × {{ record.periodUnitCode }}</template
        >
        <template
          v-else-if="column.key === 'actions' && canCorrectMissingCosts"
        >
          <Button
            size="small"
            type="primary"
            @click="openCostCorrection(record)"
            >{{ $t("plugin.idc-core.marketing.insights.correctCost") }}</Button
          >
        </template>
        <template v-else>{{ value }}</template>
      </template>
    </VbenDataGrid>
  </VbenModalCompat>

  <VbenModalCompat
    v-model:open="costCorrectionOpen"
    :confirm-loading="costCorrectionSubmitting"
    :title="$t('plugin.idc-core.marketing.insights.correctCostTitle')"
    :ok-button-props="{ disabled: !validCostCorrection() }"
    @ok="confirmCostCorrection"
  >
    <Space
      v-if="costCorrectionTarget"
      direction="vertical"
      style="width: 100%"
      :size="12"
    >
      <p>
        {{
          $t("plugin.idc-core.marketing.insights.costReadOnlySummary", {
            orderNo: costCorrectionTarget.orderNo,
            currency: costCorrectionTarget.currencyCode,
            quantity: costCorrectionTarget.quantity,
            cycles: costCorrectionTarget.purchaseCycles,
            periodCount: costCorrectionTarget.periodCount,
            periodUnit: costCorrectionTarget.periodUnitCode,
          })
        }}
      </p>
      <label>{{
        $t("plugin.idc-core.marketing.insights.unitCostMinor")
      }}</label>
      <Input
        v-model:value="costCorrectionUnit"
        inputmode="numeric"
        :maxlength="19"
        :placeholder="
          $t('plugin.idc-core.marketing.insights.unitCostPlaceholder')
        "
      />
      <label>{{
        $t("plugin.idc-core.marketing.insights.correctionReason")
      }}</label>
      <Textarea
        v-model:value="costCorrectionReason"
        :maxlength="500"
        :rows="5"
        show-count
        :placeholder="
          $t('plugin.idc-core.marketing.insights.reasonPlaceholder')
        "
      />
    </Space>
  </VbenModalCompat>
</template>

<style scoped>
.insights-grid {
  display: grid;
  gap: 20px;
  margin-top: 20px;
}
</style>
