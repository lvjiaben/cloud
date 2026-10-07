<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import type { PluginHostMountContext } from "../host-context";
import IsolatedEcharts from "../runtime/IsolatedEcharts.vue";
import { createTranslator } from "../runtime/i18n";
import {
  createOperationsAPI,
  type OperationsAlert,
  type OperationsCustomerStats,
  type OperationsExpiringService,
  type OperationsProductSalesRank,
  type OperationsReport,
  type OperationsRevenueMetric,
  type OperationsSnapshot,
  type OperationsTask,
  type OperationsTrendDay,
} from "./api";
import { operationsMessages } from "./messages";

const props = defineProps<{ context: PluginHostMountContext }>();
const api = createOperationsAPI(() => props.context);
const tr = createTranslator(() => props.context, operationsMessages);
const loading = ref(true);
const failed = ref(false);
const snapshot = ref<OperationsSnapshot | null>(null);
const report = ref<OperationsReport | null>(null);
const alerts = ref<OperationsAlert[]>([]);
const tasks = ref<OperationsTask[]>([]);
const revenue = ref<OperationsRevenueMetric[]>([]);
const customers = ref<OperationsCustomerStats | null>(null);
const trend = ref<OperationsTrendDay[]>([]);
const expiring = ref<OperationsExpiringService[]>([]);
const salesRank = ref<OperationsProductSalesRank[]>([]);
const isShortcutMode = computed(() =>
  props.context.routePath.toLowerCase().includes("/operations/shortcuts"),
);

const quickEntries = [
  { key: "quickCustomers", path: "/idc/customers" },
  { key: "quickCatalog", path: "/idc/catalog" },
  { key: "quickOrders", path: "/idc/orders" },
  { key: "quickServices", path: "/idc/resource/overview" },
  { key: "quickTickets", path: "/idc/support" },
  { key: "quickFinance", path: "/idc/finance" },
  { key: "quickNotifications", path: "/idc/notifications/deliveries" },
  { key: "quickSettings", path: "/system/configuration-center" },
] as const;

function adminHref(path: string) {
  return `${globalThis.location.pathname.startsWith("/admin/") ? "/admin" : ""}${path}`;
}

function queueHref(code: string) {
  const paths: Record<string, string> = {
    contract_exceptions: "/idc/compliance/electronic-contract",
    expiring_services: "/idc/resource/overview",
    notification_failures: "/idc/notifications/deliveries",
    open_operations: "/idc/resource/tasks",
    open_tasks: "/idc/product-fulfillment/tasks",
    open_tickets: "/idc/support",
    overdue_tickets: "/idc/support",
    paused_products: "/idc/catalog",
    paused_services: "/idc/resource/overview",
    payment_exceptions: "/idc/payment-exception-funds",
    pending_commission_settlements: "/idc/sales-channel",
    pending_compliance: "/idc/compliance/providers/aliyun-face",
    pending_contracts: "/idc/compliance/electronic-contract",
    pending_enterprise_identity: "/idc/compliance/enterprise-verifications",
    pending_identity: "/idc/compliance/providers/aliyun-face",
    pending_invoices: "/idc/finance",
    pending_ip_changes: "/idc/resource/server-accessories",
    pending_orders: "/idc/orders",
    pending_refunds: "/idc/finance",
    pending_services: "/idc/product-fulfillment/tasks",
    resource_alerts: "/idc/resource/health",
  };
  return adminHref(paths[code.toLowerCase()] ?? "/idc/operations/workbench");
}

function money(value: number, currency: string) {
  return `${currency} ${(value / 100).toLocaleString(props.context.locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function metricMoney(value: number | undefined, currency: string) {
  return money(value ?? 0, currency);
}

const primaryCurrency = computed(() => revenue.value[0]);

function queueLabel(code: string) {
  const key = code.toLowerCase();
  const labels: Record<string, keyof typeof operationsMessages> = {
    pending_orders: "queuePendingOrders",
    open_tasks: "queueOpenTasks",
    open_risks: "queueOpenRisks",
    pending_invoices: "queuePendingInvoices",
    pending_services: "queuePendingServices",
    expiring_services: "queueExpiringServices",
    paused_products: "queuePausedProducts",
    payment_exceptions: "queuePaymentExceptions",
    pending_ip_changes: "queuePendingIPChanges",
    pending_contracts: "queuePendingContracts",
    pending_compliance: "queuePendingCompliance",
    notification_failures: "queueNotificationFailures",
    resource_alerts: "queueResourceAlerts",
    paused_services: "queuePausedServices",
    open_tickets: "queueOpenTickets",
    overdue_tickets: "queueOverdueTickets",
    pending_refunds: "queuePendingRefunds",
    pending_identity: "queuePendingIdentity",
    pending_enterprise_identity: "queuePendingEnterpriseIdentity",
    contract_exceptions: "queueContractExceptions",
    pending_commission_settlements: "queuePendingCommissionSettlements",
    open_operations: "queueOpenOperations",
  };
  return tr(labels[key] ?? "queueOther");
}

function alertLabel(code: string) {
  const [kind] = code.split(":", 1);
  if (
    kind === "task_failed" ||
    kind === "task_result_unknown" ||
    kind === "task_stuck_running"
  ) {
    return tr("tasks");
  }
  if (kind === "queue_backlog") return tr("queue");
  if (kind === "adapter_health") return tr("supplierQuality");
  if (kind === "inventory_freshness") return tr("queuePendingServices");
  if (kind === "performance") return tr("tasks");
  return tr("alertOther");
}

function severityLabel(severity: string) {
  switch (severity.toLowerCase()) {
    case "critical":
      return tr("alertSeverityCritical");
    case "high":
      return tr("alertSeverityHigh");
    case "medium":
      return tr("alertSeverityMedium");
    case "warning":
      return tr("alertSeverityWarning");
    case "unknown":
      return tr("alertSeverityUnknown");
    default:
      return tr("alertOther");
  }
}

function taskTypeLabel(type: string) {
  const key = type.toLowerCase();
  if (key.includes("fulfill") || key.includes("provision"))
    return tr("taskTypeFulfillment");
  if (key.includes("payment") || key.includes("refund"))
    return tr("taskTypePayment");
  if (key.includes("notify") || key.includes("message"))
    return tr("taskTypeNotification");
  return tr("taskTypeOther");
}

function subjectTypeLabel(type: string) {
  const key = type.toLowerCase();
  if (key.includes("order")) return tr("subjectOrder");
  if (key.includes("service")) return tr("subjectService");
  if (key.includes("invoice") || key.includes("bill"))
    return tr("subjectInvoice");
  return tr("subjectOther");
}

function taskStatusLabel(status: string) {
  switch (status.toLowerCase()) {
    case "pending":
    case "queued":
      return tr("statusPending");
    case "running":
    case "executing":
      return tr("statusRunning");
    case "retry_wait":
      return tr("statusRetry");
    case "succeeded":
    case "completed":
      return tr("statusSucceeded");
    case "failed":
    case "dead_letter":
      return tr("statusFailed");
    case "result_unknown":
      return tr("statusUnknown");
    case "manual_takeover":
    case "manual_review":
      return tr("statusManual");
    default:
      return tr("statusOther");
  }
}

const actionableQueues = computed(() =>
  [...(snapshot.value?.queues ?? [])]
    .filter((queue) => queue.count > 0)
    .sort(
      (left, right) =>
        right.count - left.count || left.code.localeCompare(right.code),
    ),
);

type TodoEntry = { key: string; count: number; href: string };

const todoEntries = computed<TodoEntry[]>(() => {
  const entries: TodoEntry[] = [];
  const queues = new Map(
    (snapshot.value?.queues ?? []).map((queue) => [queue.code, queue.count]),
  );
  const tickets = queues.get("open_tickets") ?? 0;
  if (tickets > 0)
    entries.push({
      key: "todoPendingTickets",
      count: tickets,
      href: queueHref("open_tickets"),
    });
  const pendingOrders = queues.get("pending_orders") ?? 0;
  if (pendingOrders > 0)
    entries.push({
      key: "todoOrderVerification",
      count: pendingOrders,
      href: queueHref("pending_orders"),
    });
  const identity =
    (queues.get("pending_identity") ?? 0) +
    (queues.get("pending_enterprise_identity") ?? 0);
  if (identity > 0)
    entries.push({
      key: "todoIdentityReview",
      count: identity,
      href: queueHref("pending_identity"),
    });
  const settlements = queues.get("pending_commission_settlements") ?? 0;
  if (settlements > 0)
    entries.push({
      key: "todoWithdrawals",
      count: settlements,
      href: queueHref("pending_commission_settlements"),
    });
  return entries;
});

const revenueCards = computed(() => {
  const metric = primaryCurrency.value;
  if (!metric) return [];
  return [
    { key: "revenueToday", value: money(metric.todayMinor, metric.currencyCode) },
    { key: "revenueThisMonth", value: money(metric.thisMonthMinor, metric.currencyCode) },
    { key: "revenueThisYear", value: money(metric.thisYearMinor, metric.currencyCode) },
    { key: "revenueTotal", value: money(metric.totalMinor, metric.currencyCode) },
  ];
});

const orderOverview = computed(() => {
  const metric = primaryCurrency.value;
  if (!metric) return [];
  return [
    { key: "orderTodayPaid", paid: metric.orderTodayCount - metric.pendingToday, total: metric.orderTodayCount },
    { key: "orderMonthPaid", paid: metric.orderMonthCount - metric.pendingMonth, total: metric.orderMonthCount },
    { key: "order7DayPaid", paid: metric.order7DayCount - metric.pending7Day, total: metric.order7DayCount },
    { key: "order7DayPending", paid: metric.pending7Day, total: metric.order7DayCount },
  ];
});

const trendOption = computed(() => {
  const days = trend.value;
  if (!days.length) return null;
  return {
    grid: { left: 12, right: 16, top: 36, bottom: 8, containLabel: true },
    tooltip: { trigger: "axis" as const },
    legend: { top: 0, right: 0, icon: "roundRect", itemWidth: 14, itemHeight: 4 },
    xAxis: {
      type: "category" as const,
      data: days.map((day) => day.date.slice(5)),
      axisLine: { lineStyle: { color: "#d1d5db" } },
      axisLabel: { color: "#6b7280" },
      axisTick: { show: false },
    },
    yAxis: {
      type: "value" as const,
      axisLine: { show: false },
      axisLabel: { color: "#6b7280" },
      splitLine: { lineStyle: { color: "#f0f0f0" } },
    },
    series: [
      {
        name: tr("revenueTrendCurrent"),
        type: "bar" as const,
        barMaxWidth: 18,
        itemStyle: { color: "#1677ff", borderRadius: [2, 2, 0, 0] },
        data: days.map((day) => day.currentMinor / 100),
      },
      {
        name: tr("revenueTrendPrevious"),
        type: "line" as const,
        smooth: true,
        symbol: "circle" as const,
        symbolSize: 4,
        itemStyle: { color: "#f59e0b" },
        lineStyle: { type: "dashed" as const, width: 2, color: "#f59e0b" },
        data: days.map((day) => day.previousMinor / 100),
      },
    ],
  };
});

function productTypeLabel(code: string) {
  switch (code) {
    case "cloud_server":
      return tr("productTypeCloudServer");
    case "bare_metal":
      return tr("productTypeBareMetal");
    default:
      return tr("productTypeOther");
  }
}

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const [
      nextSnapshot,
      nextReport,
      nextAlerts,
      nextTasks,
      nextRevenue,
      nextCustomers,
      nextExpiring,
      nextSalesRank,
    ] = await Promise.all([
      api.snapshot(),
      api.report(),
      api.alerts(),
      api.tasks(),
      api.dashboardSummary(),
      api.customerStats(),
      api.expiringServices(),
      api.productSalesRank(),
    ]);
    snapshot.value = nextSnapshot;
    report.value = nextReport;
    alerts.value = nextAlerts.alerts;
    tasks.value = nextTasks.tasks;
    revenue.value = nextRevenue.currencies;
    customers.value = nextCustomers;
    expiring.value = nextExpiring.services;
    salesRank.value = nextSalesRank.products;
    if (primaryCurrency.value) {
      const nextTrend = await api.revenueTrend(primaryCurrency.value.currencyCode);
      trend.value = nextTrend.days;
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  if (!isShortcutMode.value) void load();
});
</script>

<template>
  <main
    class="operations-page"
    :class="{ 'operations-page--shortcuts': isShortcutMode }"
    :data-testid="
      isShortcutMode ? 'operations-shortcuts' : 'operations-analytics'
    "
  >
    <nav
      v-if="isShortcutMode"
      class="panel quick-panel shortcut-panel"
      :aria-label="tr('quickEntries')"
    >
      <div class="panel-title shortcut-heading">
        <div>
          <h2>{{ tr("shortcutTitle") }}</h2>
          <span>{{ tr("shortcutSubtitle") }}</span>
        </div>
      </div>
      <div class="quick-grid">
        <a
          v-for="entry in quickEntries"
          :key="entry.path"
          class="quick-link"
          :href="adminHref(entry.path)"
        >
          {{ tr(entry.key) }}
          <span aria-hidden="true">→</span>
        </a>
      </div>
    </nav>
    <header v-else class="operations-header">
      <div>
        <p class="eyebrow">{{ tr("eyebrow") }}</p>
        <h1>{{ tr("analyticsTitle") }}</h1>
        <p class="subtitle">{{ tr("analyticsSubtitle") }}</p>
      </div>
      <button class="refresh" type="button" :disabled="loading" @click="load">
        {{ tr("refresh") }}
      </button>
    </header>
    <div
      v-if="!isShortcutMode && failed"
      class="notice error"
      data-testid="operations-error"
    >
      {{ tr("failed") }}
    </div>
    <div v-else-if="!isShortcutMode && loading" class="notice">
      {{ tr("loading") }}
    </div>
    <template v-else-if="!isShortcutMode && snapshot">
      <section class="dashboard-grid" data-testid="dashboard-revenue-cards">
        <article
          v-for="card in revenueCards"
          :key="card.key"
          class="metric-card"
        >
          <span class="metric-label">{{ tr(card.key) }}</span>
          <strong class="metric-value">{{ card.value }}</strong>
          <small class="metric-hint">{{ tr("revenueCardHint") }}</small>
        </article>
      </section>

      <section class="dashboard-columns">
        <div class="dashboard-main">
          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("orderOverview") }}</h2>
            </div>
            <div class="overview-grid">
              <div v-for="item in orderOverview" :key="item.key" class="overview-item">
                <span class="metric-label">{{ tr(item.key) }}</span>
                <strong class="overview-value">{{ item.paid }}</strong>
                <small class="metric-hint">
                  {{ tr("orderTotalSuffix") }}: {{ item.total }}
                </small>
              </div>
            </div>
          </section>

          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("todoCenter") }}</h2>
              <span>{{ actionableQueues.length }}</span>
            </div>
            <div v-if="todoEntries.length" class="todo-list">
              <a
                v-for="entry in todoEntries"
                :key="entry.key"
                class="todo-item"
                :href="entry.href"
              >
                <span>{{ tr(entry.key) }}</span>
                <strong>{{ entry.count }}</strong>
                <small aria-hidden="true">→</small>
              </a>
            </div>
            <div v-else class="empty">{{ tr("empty") }}</div>
          </section>

          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("revenueTrend") }}</h2>
              <span>{{ tr("revenueCardHint") }}</span>
            </div>
            <IsolatedEcharts
              v-if="trendOption"
              :option="trendOption"
              :height="320"
            />
            <div v-else class="empty">{{ tr("revenueTrendEmpty") }}</div>
          </section>
        </div>

        <div class="dashboard-side">
          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("customerStats") }}</h2>
            </div>
            <dl v-if="customers" class="customer-stats">
              <div>
                <dt>{{ tr("customerTodayRegistered") }}</dt>
                <dd>{{ customers.todayRegistered }}</dd>
              </div>
              <div>
                <dt>{{ tr("customerTotal") }}</dt>
                <dd>{{ customers.totalCustomers }}</dd>
              </div>
              <div>
                <dt>{{ tr("customerPendingActivation") }}</dt>
                <dd>{{ customers.pendingActivate }}</dd>
              </div>
            </dl>
          </section>

          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("expiringServices") }}</h2>
              <span>{{ tr("expiringServicesHint") }}</span>
            </div>
            <ul v-if="expiring.length" class="expiring-list">
              <li v-for="service in expiring" :key="service.productTypeCode">
                <span>{{ productTypeLabel(service.productTypeCode) }}</span>
                <strong>{{ service.count }}</strong>
              </li>
            </ul>
            <div v-else class="empty">{{ tr("expiringEmpty") }}</div>
          </section>

          <section class="panel">
            <div class="panel-title">
              <h2>{{ tr("productSalesRank") }}</h2>
              <span>{{ tr("productSalesRankHint") }}</span>
            </div>
            <ol v-if="salesRank.length" class="sales-rank">
              <li v-for="product in salesRank" :key="product.productNo">
                <span>{{ product.productName }}</span>
                <strong>{{ product.salesCount }}</strong>
              </li>
            </ol>
            <div v-else class="empty">{{ tr("productSalesEmpty") }}</div>
          </section>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <h2>{{ tr("finance") }}</h2>
          <span>{{ tr("currencySeparate") }}</span>
        </div>
        <div v-if="snapshot.currencies.length" class="currency-grid">
          <article
            v-for="metric in snapshot.currencies"
            :key="metric.currencyCode"
            class="currency-card"
          >
            <div class="currency-name">
              {{ metric.currencyCode }}
              <small>{{ tr("invoiceCount") }} {{ metric.invoiceCount }}</small>
            </div>
            <dl>
              <div>
                <dt>{{ tr("total") }}</dt>
                <dd>
                  {{ money(metric.totalAmountMinor, metric.currencyCode) }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("paid") }}</dt>
                <dd>
                  {{ money(metric.paidAmountMinor, metric.currencyCode) }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("refunded") }}</dt>
                <dd>
                  {{ money(metric.refundedAmountMinor, metric.currencyCode) }}
                </dd>
              </div>
            </dl>
          </article>
        </div>
        <div v-else class="empty">{{ tr("empty") }}</div>
      </section>

      <section v-if="report" class="panel">
        <div class="panel-title">
          <h2>{{ tr("report") }}</h2>
          <span
            >{{
              new Date(report.from).toLocaleDateString(props.context.locale)
            }}
            –
            {{
              new Date(report.to).toLocaleDateString(props.context.locale)
            }}</span
          >
        </div>
        <div class="currency-grid">
          <article
            v-for="metric in report.currencies"
            :key="`report-${metric.currencyCode}`"
            class="currency-card"
          >
            <div class="currency-name">{{ metric.currencyCode }}</div>
            <dl>
              <div>
                <dt>{{ tr("revenue") }}</dt>
                <dd>
                  {{
                    metricMoney(
                      metric.revenueMinor ?? metric.totalAmountMinor,
                      metric.currencyCode,
                    )
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("receivable") }}</dt>
                <dd>{{ metricMoney(metric.receivableMinor, metric.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ tr("paid") }}</dt>
                <dd>
                  {{
                    metricMoney(
                      metric.paidMinor ?? metric.paidAmountMinor,
                      metric.currencyCode,
                    )
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("refunded") }}</dt>
                <dd>
                  {{
                    metricMoney(
                      metric.refundMinor ?? metric.refundedAmountMinor,
                      metric.currencyCode,
                    )
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("cashFlow") }}</dt>
                <dd>{{ metricMoney(metric.cashFlowMinor, metric.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ tr("balance") }}</dt>
                <dd>{{ metricMoney(metric.balanceMinor, metric.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ tr("cost") }}</dt>
                <dd>
                  {{
                    metric.costKnown === false
                      ? tr("costUnavailable")
                      : metricMoney(metric.costMinor, metric.currencyCode)
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("grossProfit") }}</dt>
                <dd>
                  {{
                    metric.costKnown === false
                      ? "—"
                      : metricMoney(
                          metric.grossProfitMinor,
                          metric.currencyCode,
                        )
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("customers") }}</dt>
                <dd>{{ metric.customerCount ?? 0 }}</dd>
              </div>
              <div>
                <dt>{{ tr("orders") }}</dt>
                <dd>{{ metric.orderCount ?? 0 }}</dd>
              </div>
              <div>
                <dt>{{ tr("products") }}</dt>
                <dd>{{ metric.productCount ?? 0 }}</dd>
              </div>
              <div>
                <dt>{{ tr("sales") }}</dt>
                <dd>{{ metric.salesCount ?? 0 }}</dd>
              </div>
              <div>
                <dt>{{ tr("affiliates") }}</dt>
                <dd>{{ metric.affiliateCount ?? 0 }}</dd>
              </div>
              <div>
                <dt>{{ tr("supplierQuality") }}</dt>
                <dd>
                  {{ metric.supplierSuccess ?? 0 }} /
                  {{ metric.supplierFailure ?? 0 }}
                </dd>
              </div>
            </dl>
          </article>
        </div>
        <div v-if="!report.currencies.length" class="empty">
          {{ tr("empty") }}
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <h2>{{ tr("alerts") }}</h2>
          <span>{{ alerts.length }}</span>
        </div>
        <div v-if="alerts.length" class="alert-list">
          <article
            v-for="alert in alerts"
            :key="alert.code"
            class="alert-card"
            :data-severity="alert.severity"
          >
            <div>
              <strong>{{ alertLabel(alert.code) }}</strong
              ><span>{{ severityLabel(alert.severity) }}</span>
            </div>
            <p>
              {{ tr("alertCount") }} {{ alert.count }} ·
              {{
                new Date(alert.oldestAt).toLocaleString(props.context.locale)
              }}
            </p>
          </article>
        </div>
        <div v-else class="empty">{{ tr("noAlerts") }}</div>
      </section>
      <section class="panel">
        <div class="panel-title">
          <h2>{{ tr("tasks") }}</h2>
          <span>{{ tasks.length }}</span>
        </div>
        <div v-if="tasks.length" class="task-list">
          <article v-for="task in tasks" :key="task.taskNo" class="task-card">
            <div class="task-heading">
              <strong>{{ taskTypeLabel(task.taskType) }}</strong
              ><span
                >{{ tr("taskStatus") }}:
                {{ taskStatusLabel(task.status) }}</span
              >
            </div>
            <p>
              {{ subjectTypeLabel(task.subjectType) }} · {{ task.subjectNo }}
            </p>
            <small
              >{{ tr("taskAttempts") }} {{ task.attemptCount }} ·
              {{ tr("taskUpdated") }}
              {{
                new Date(task.updatedAt).toLocaleString(props.context.locale)
              }}</small
            >
            <p v-if="task.errorSummary" class="task-error">
              {{ task.errorSummary }}
            </p>
          </article>
        </div>
        <div v-else class="empty">{{ tr("noTasks") }}</div>
      </section>
    </template>
  </main>
</template>
