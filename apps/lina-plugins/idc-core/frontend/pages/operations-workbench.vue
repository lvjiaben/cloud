<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/operations/workbench",
  title: "Operations Data",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import type { EchartsUIType } from "@vben/plugins/echarts";
import type {
  OperationsData,
  OperationsPeriod,
} from "./operations-data-client";

import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import { EchartsUI, useEcharts } from "@vben/plugins/echarts";
import { preferences } from "@vben/preferences";
import { Alert, Button, Empty, Select, Skeleton, Spin } from "ant-design-vue";
import { $t } from "#/locales";
import { loadOperationsData } from "./operations-data-client";

const router = useRouter();
const data = ref<OperationsData>();
const loading = ref(true);
const failed = ref(false);
const currency = ref<string>();
const trendRef = ref<EchartsUIType>();
const productsRef = ref<EchartsUIType>();
const trendChart = useEcharts(trendRef);
const productsChart = useEcharts(productsRef);
const locale = computed(() => preferences.app.locale || "zh-CN");
const currencyOptions = computed(() =>
  (data.value?.currencies ?? []).map((value) => ({ label: value, value })),
);
const month = computed(() =>
  data.value?.periods.find((period) => period.key === "month"),
);
const today = computed(() =>
  data.value?.periods.find((period) => period.key === "today"),
);
const updatedAt = computed(() =>
  data.value
    ? new Intl.DateTimeFormat(locale.value, {
        timeZone: data.value.timezone,
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
      }).format(data.value.asOf)
    : "",
);
let requestVersion = 0;

async function load() {
  const version = ++requestVersion;
  loading.value = true;
  failed.value = false;
  try {
    const result = await loadOperationsData(currency.value);
    if (version !== requestVersion) return;
    data.value = result;
    currency.value = result.currencyCode;
  } catch {
    if (version === requestVersion) failed.value = true;
  } finally {
    if (version === requestVersion) {
      loading.value = false;
      if (!failed.value) await renderCharts();
    }
  }
}

function count(value: number) {
  return new Intl.NumberFormat(locale.value).format(value);
}

function money(minor: string) {
  const exponent = data.value?.currencyExponent ?? 2;
  const value = BigInt(minor);
  const scale = 10n ** BigInt(exponent);
  const absolute = value < 0n ? -value : value;
  const integer = new Intl.NumberFormat(locale.value).format(absolute / scale);
  const fraction = exponent
    ? `.${String(absolute % scale).padStart(exponent, "0")}`
    : "";
  return `${value < 0n ? "−" : ""}${integer}${fraction}`;
}

type Metric =
  | "income"
  | "orders"
  | "customers"
  | "servers"
  | "tickets"
  | "products";
function openDetails(
  metric: Metric,
  options: {
    startAt?: number;
    endAt?: number;
    status?: string;
    productNo?: string;
  } = {},
) {
  if (!data.value || loading.value || failed.value) return;
  const paths: Record<Metric, string> = {
    income: "/idc/finance",
    orders: "/idc/orders",
    products: "/idc/orders",
    customers: "/idc/customers",
    servers: "/idc/resource/server-accessories",
    tickets: "/idc/support",
  };
  void router.push({
    path: paths[metric],
    query: {
      operationsMetric: metric,
      currencyCode: data.value.currencyCode,
      asOf: data.value.asOf,
      ...options,
    },
  });
}

function openPeriod(metric: "income" | "orders", period: OperationsPeriod) {
  openDetails(metric, { startAt: period.startAt, endAt: period.endAt });
}

function openProduct(index: number) {
  const product = data.value?.products[index];
  if (product && month.value)
    openDetails("products", {
      productNo: product.productNo,
      startAt: month.value.startAt,
      endAt: month.value.endAt,
    });
}

function productName(product: OperationsData["products"][number]) {
  return (
    (locale.value.startsWith("zh")
      ? product.nameZhCN || product.nameEnUS
      : product.nameEnUS || product.nameZhCN) ||
    $t("plugin.idc-core.operationsData.unnamedProduct")
  );
}

async function renderCharts() {
  await nextTick();
  if (!data.value || failed.value) return;
  const snapshot = data.value;
  const instance = await trendChart.renderEcharts({
    animation: false,
    color: ["#2875e2", "#14a393", "#8b98ae"],
    tooltip: { trigger: "axis", confine: true },
    legend: { top: 0, right: 8, itemWidth: 13, itemHeight: 8 },
    grid: { left: 14, right: 16, top: 65, bottom: 12, containLabel: true },
    xAxis: {
      type: "category",
      data: snapshot.trend.map((day) => day.date.slice(8)),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: "#d9e0e8" } },
      axisLabel: { color: "#788599" },
    },
    yAxis: [
      {
        type: "value",
        name: snapshot.currencyCode,
        splitLine: { lineStyle: { type: "dashed", color: "#e9edf2" } },
      },
      {
        type: "value",
        name: $t("plugin.idc-core.operationsData.orderUnit"),
        minInterval: 1,
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: $t("plugin.idc-core.operationsData.dailyIncome"),
        type: "bar",
        barMaxWidth: 20,
        data: snapshot.trend.map(
          (day) => Number(day.currentMinor) / 10 ** snapshot.currencyExponent,
        ),
      },
      {
        name: $t("plugin.idc-core.operationsData.currentOrders"),
        type: "line",
        yAxisIndex: 1,
        symbol: "circle",
        symbolSize: 6,
        data: snapshot.trend.map((day) => day.currentCount),
      },
      {
        name: $t("plugin.idc-core.operationsData.previousOrders"),
        type: "line",
        yAxisIndex: 1,
        symbol: "circle",
        symbolSize: 5,
        lineStyle: { type: "dashed" },
        data: snapshot.trend.map((day) => day.previousCount),
      },
    ],
  });
  instance?.off("click");
  instance?.on("click", (event) => {
    const day = snapshot.trend[event.dataIndex];
    if (!day) return;
    const previous = event.seriesIndex === 2;
    if (previous && (!day.previousStartAt || day.previousCount === null))
      return;
    openDetails(event.seriesIndex === 0 ? "income" : "orders", {
      startAt: previous ? day.previousStartAt : day.startAt,
      endAt: previous ? day.previousEndAt : day.endAt,
    });
  });
  if (!snapshot.products.length) return;
  const rankInstance = await productsChart.renderEcharts({
    animation: false,
    color: ["#2875e2"],
    tooltip: {
      trigger: "axis",
      confine: true,
      axisPointer: { type: "shadow" },
    },
    grid: { left: 10, right: 44, top: 10, bottom: 15, containLabel: true },
    xAxis: {
      type: "value",
      minInterval: 1,
      splitLine: { lineStyle: { type: "dashed", color: "#e9edf2" } },
    },
    yAxis: {
      type: "category",
      inverse: true,
      data: snapshot.products.map(productName),
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: { width: 128, overflow: "truncate", color: "#657287" },
    },
    series: [
      {
        type: "bar",
        barMaxWidth: 18,
        data: snapshot.products.map((product) => product.quantity),
        label: { show: true, position: "right", color: "#657287" },
      },
    ],
  });
  rankInstance?.off("click");
  rankInstance?.on("click", (event) => openProduct(event.dataIndex));
}

onMounted(load);
watch(locale, () => {
  if (!loading.value) void renderCharts();
});
</script>

<template>
  <Page class="plugin-admin-page operations-data" data-testid="operations-data">
    <header class="operations-header">
      <div class="operations-controls">
        <span v-if="data" class="updated-at"
          >{{ updatedAt }} · {{ data.timezone }}</span
        >
        <Select
          v-model:value="currency"
          :options="currencyOptions"
          :disabled="loading || !data"
          :aria-label="$t('plugin.idc-core.operationsData.currency')"
          class="currency-select"
          @change="load"
        />
        <Button :loading="loading" @click="load">
          <template #icon><IconifyIcon icon="lucide:refresh-cw" /></template>
          {{ $t("plugin.idc-core.operationsData.refresh") }}
        </Button>
      </div>
    </header>

    <Alert
      v-if="failed"
      type="error"
      show-icon
      :message="$t('plugin.idc-core.operationsData.loadFailed')"
    >
      <template #action
        ><Button size="small" @click="load">{{
          $t("plugin.idc-core.operationsData.retry")
        }}</Button></template
      >
    </Alert>
    <Skeleton v-else-if="!data" active :paragraph="{ rows: 12 }" />
    <Spin v-else :spinning="loading">
      <div class="revenue-grid">
        <section
          v-for="period in data.periods"
          :key="period.key"
          class="data-panel revenue-panel"
          :data-testid="`period-${period.key}`"
        >
          <h2>
            {{ $t(`plugin.idc-core.operationsData.period.${period.key}`) }}
          </h2>
          <button class="income-link" @click="openPeriod('income', period)">
            <span class="metric-label"
              >{{ $t("plugin.idc-core.operationsData.income") }}
              <small>{{ data.currencyCode }}</small></span
            >
            <strong>{{ money(period.incomeMinor) }}</strong>
            <IconifyIcon icon="lucide:arrow-up-right" />
          </button>
          <button class="orders-link" @click="openPeriod('orders', period)">
            <span>{{ $t("plugin.idc-core.operationsData.paidOrders") }}</span>
            <strong>{{ count(period.paidOrders) }}</strong
            ><IconifyIcon icon="lucide:chevron-right" />
          </button>
        </section>
      </div>

      <section class="data-panel trend-panel">
        <div class="section-heading">
          <h2>{{ $t("plugin.idc-core.operationsData.trend") }}</h2>
          <span>{{ $t("plugin.idc-core.operationsData.chartHint") }}</span>
        </div>
        <EchartsUI
          ref="trendRef"
          height="320px"
          data-testid="operations-trend"
        />
      </section>

      <div class="lower-grid">
        <div class="service-column">
          <section class="data-panel">
            <div class="section-heading">
              <h2>{{ $t("plugin.idc-core.operationsData.customers") }}</h2>
            </div>
            <div class="customer-grid">
              <button
                @click="
                  openDetails('customers', {
                    startAt: today?.startAt,
                    endAt: today?.endAt,
                  })
                "
              >
                <span>{{
                  $t("plugin.idc-core.operationsData.todayCustomers")
                }}</span
                ><strong>{{ count(data.customers.today) }}</strong>
              </button>
              <button @click="openDetails('customers')">
                <span>{{
                  $t("plugin.idc-core.operationsData.totalCustomers")
                }}</span
                ><strong>{{ count(data.customers.total) }}</strong>
              </button>
              <button @click="openDetails('customers', { status: 'online' })">
                <span
                  ><i class="online-dot"></i
                  >{{
                    $t("plugin.idc-core.operationsData.onlineCustomers")
                  }}</span
                ><strong>{{ count(data.customers.online) }}</strong>
              </button>
            </div>
          </section>
          <section class="data-panel workload-panel">
            <div class="section-heading">
              <h2>{{ $t("plugin.idc-core.operationsData.services") }}</h2>
            </div>
            <button class="workload-row" @click="openDetails('servers')">
              <IconifyIcon icon="lucide:server" /><span
                >{{ $t("plugin.idc-core.operationsData.expiringServers")
                }}<small>{{
                  $t("plugin.idc-core.operationsData.nextSevenDays")
                }}</small></span
              >
              <strong>{{ count(data.expiringServers) }}</strong
              ><IconifyIcon icon="lucide:chevron-right" />
            </button>
            <button
              class="workload-row"
              @click="openDetails('tickets', { status: 'pending' })"
            >
              <IconifyIcon icon="lucide:inbox" /><span>{{
                $t("plugin.idc-core.operationsData.pendingTickets")
              }}</span>
              <strong>{{ count(data.tickets.pending) }}</strong
              ><IconifyIcon icon="lucide:chevron-right" />
            </button>
            <button
              class="workload-row"
              @click="openDetails('tickets', { status: 'processed' })"
            >
              <IconifyIcon icon="lucide:circle-check" /><span>{{
                $t("plugin.idc-core.operationsData.processedTickets")
              }}</span>
              <strong>{{ count(data.tickets.processed) }}</strong
              ><IconifyIcon icon="lucide:chevron-right" />
            </button>
          </section>
        </div>
        <section class="data-panel ranking-panel">
          <div class="section-heading">
            <h2>{{ $t("plugin.idc-core.operationsData.productRank") }}</h2>
            <span>{{ $t("plugin.idc-core.operationsData.salesUnit") }}</span>
          </div>
          <EchartsUI
            v-if="data.products.length"
            ref="productsRef"
            :height="`${Math.max(310, data.products.length * 36)}px`"
            data-testid="operations-products"
          />
          <Empty
            v-else
            :description="$t('plugin.idc-core.operationsData.noSales')"
            class="ranking-empty"
          />
        </section>
      </div>
    </Spin>
  </Page>
</template>

<style scoped>
.operations-data {
  color: hsl(var(--foreground));
}
.operations-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.operations-header h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.operations-header p {
  margin: 6px 0 0;
  color: #7a8799;
  font-size: 12px;
}
.operations-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}
.updated-at {
  color: #7a8799;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.currency-select {
  width: 90px;
}
.data-panel {
  min-width: 0;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
}
.revenue-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}
.revenue-panel {
  padding: 18px 20px 0;
}
h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
button {
  text-align: left;
}
button:focus-visible {
  outline: 2px solid #2875e2;
  outline-offset: 3px;
}
.income-link {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  gap: 6px;
  margin: 18px 0;
}
.metric-label {
  color: #7a8799;
  font-size: 12px;
}
.metric-label small {
  margin-left: 4px;
  font-size: 10px;
}
.income-link strong {
  font-size: clamp(22px, 2vw, 30px);
  font-weight: 600;
  letter-spacing: -0.6px;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
  padding-right: 18px;
}
.income-link > :last-child {
  position: absolute;
  right: 0;
  bottom: 9px;
  color: #9aa7b7;
}
.income-link:hover strong,
.customer-grid button:hover strong,
.workload-row:hover strong {
  color: #2875e2;
}
.orders-link {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 13px 0;
  border-top: 1px solid hsl(var(--border));
  font-size: 12px;
  color: #7a8799;
}
.orders-link strong {
  margin-left: auto;
  font-size: 16px;
  color: hsl(var(--foreground));
  font-weight: 500;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 18px 20px;
}
.section-heading > span {
  color: #7a8799;
  font-size: 12px;
}
.trend-panel {
  margin-bottom: 16px;
  padding-bottom: 12px;
}
.lower-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.service-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.customer-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 2px 8px 22px;
}
.customer-grid button {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  border-right: 1px solid hsl(var(--border));
}
.customer-grid button:last-child {
  border-right: 0;
}
.customer-grid span {
  color: #7a8799;
  font-size: 12px;
}
.customer-grid strong {
  font-size: 25px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.online-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  background: #14a393;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: 2px;
}
.workload-panel {
  flex: 1;
}
.workload-row {
  display: flex;
  align-items: center;
  width: calc(100% - 40px);
  gap: 12px;
  padding: 12px 0;
  margin: 0 20px;
  border-top: 1px solid hsl(var(--border));
}
.workload-row > :first-child {
  width: 18px;
  height: 18px;
  color: #7a8799;
}
.workload-row > span {
  font-size: 13px;
}
.workload-row small {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  color: #7a8799;
}
.workload-row strong {
  margin-left: auto;
  font-size: 21px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.workload-row > :last-child {
  color: #9aa7b7;
}
.ranking-panel {
  padding-bottom: 10px;
}
.ranking-empty {
  margin: 85px 0;
}
@media (max-width: 1100px) {
  .updated-at {
    display: none;
  }
  .revenue-grid {
    gap: 12px;
  }
  .revenue-panel {
    padding-left: 14px;
    padding-right: 14px;
  }
}
@media (max-width: 760px) {
  .operations-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .revenue-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .lower-grid {
    grid-template-columns: 1fr;
  }
  .section-heading > span {
    font-size: 11px;
  }
}
</style>
