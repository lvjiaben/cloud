import type { PluginHostMountContext } from '../host-context';
import { createPluginRequest } from '../runtime/request';

export type OperationsSnapshot = {
  generatedAt: number;
  queues: Array<{ code: string; count: number }>;
  currencies: Array<{ currencyCode: string; invoiceCount: number; totalAmountMinor: number; paidAmountMinor: number; refundedAmountMinor: number }>;
  watermark?: number;
  changed?: boolean;
};

export type OperationsReportMetric = OperationsSnapshot['currencies'][number] & {
  revenueMinor?: number;
  receivableMinor?: number;
  paidMinor?: number;
  cashFlowMinor?: number;
  balanceMinor?: number;
  refundMinor?: number;
  costMinor?: number;
  costKnown?: boolean;
  grossProfitMinor?: number;
  customerCount?: number;
  orderCount?: number;
  serviceCount?: number;
  productCount?: number;
  salesCount?: number;
  affiliateCount?: number;
  supplierSuccess?: number;
  supplierFailure?: number;
};
export type OperationsReport = { from: number; to: number; currencies: OperationsReportMetric[] };
export type OperationsAlert = { code: string; severity: string; count: number; oldestAt: number };
export type OperationsTask = {
  taskNo: string;
  taskType: string;
  subjectType: string;
  subjectNo: string;
  status: string;
  errorCode: string;
  errorSummary: string;
  attemptCount: number;
  createdAt: number;
  updatedAt: number;
};
export type OperationsAnalyticsCapabilities = {
  riskEvents: Array<{ code: string }>;
  riskActions: Array<{ code: string }>;
  manualReview: { reviewableStatuses: string[]; decisions: string[]; reasonRequired: boolean };
  report: { metricCodes: string[]; originalCurrencyGrouped: boolean; exchangeRateOptional: boolean; convertedValueLedgerSafe: boolean };
  auditExport: { format: 'xlsx'; asynchronous: boolean; maxRows: number; shortLivedLink: boolean; rechecksScope: boolean };
};

export type OperationsRevenueMetric = {
  currencyCode: string;
  todayMinor: number;
  thisMonthMinor: number;
  thisYearMinor: number;
  totalMinor: number;
  orderTodayCount: number;
  orderMonthCount: number;
  order7DayCount: number;
  pendingToday: number;
  pendingMonth: number;
  pending7Day: number;
};

export type OperationsCustomerStats = {
  generatedAt: number;
  todayRegistered: number;
  totalCustomers: number;
  pendingActivate: number;
};

export type OperationsTrendDay = {
  date: string;
  currentMinor: number;
  currentCount: number;
  previousMinor: number;
  previousCount: number;
};

export type OperationsExpiringService = {
  productTypeCode: string;
  count: number;
};

export type OperationsProductSalesRank = {
  productNo: string;
  productName: string;
  salesCount: number;
};

export function createOperationsAPI(currentContext: () => PluginHostMountContext) {
  const request = createPluginRequest(currentContext);
  const apiBase = '/x/idc-core/api/v1';
  return {
    snapshot: (since?: number) => request<OperationsSnapshot>(since ? `${apiBase}/admin/operations/workbench?since=${encodeURIComponent(since)}` : `${apiBase}/admin/operations/workbench`),
    report: () => request<OperationsReport>(`${apiBase}/admin/operations/report`),
    alerts: () => request<{ alerts: OperationsAlert[] }>(`${apiBase}/admin/operations/alerts`),
    tasks: (status?: string) => request<{ tasks: OperationsTask[] }>(status ? `${apiBase}/admin/operations/tasks?status=${encodeURIComponent(status)}` : `${apiBase}/admin/operations/tasks`),
    capabilities: () => request<OperationsAnalyticsCapabilities>(`${apiBase}/admin/operations/analytics/capabilities`),
    dashboardSummary: () =>
      request<{ generatedAt: number; currencies: OperationsRevenueMetric[] }>(`${apiBase}/admin/operations/dashboard/summary`),
    customerStats: () => request<OperationsCustomerStats>(`${apiBase}/admin/operations/dashboard/customers`),
    revenueTrend: (currencyCode: string, days = 30) =>
      request<{ currencyCode: string; days: OperationsTrendDay[] }>(
        `${apiBase}/admin/operations/dashboard/revenue-trend?currencyCode=${encodeURIComponent(currencyCode)}&days=${days}`,
      ),
    expiringServices: () => request<{ services: OperationsExpiringService[] }>(`${apiBase}/admin/operations/dashboard/expiring-services`),
    productSalesRank: () => request<{ products: OperationsProductSalesRank[] }>(`${apiBase}/admin/operations/dashboard/product-sales`),
  };
}
