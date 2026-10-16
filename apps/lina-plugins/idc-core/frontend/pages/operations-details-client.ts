import type { LocationQuery } from 'vue-router';

import { pluginApiPath, requestClient } from '#/api/request';

export type OperationsMetric = 'income' | 'orders' | 'customers' | 'servers' | 'tickets' | 'products';

export interface OperationsRecord {
  id: string;
  label: string;
  customerNo: string;
  customerName: string;
  currencyCode: string;
  amountMinor: string;
  quantity: number;
  statusCode: string;
  occurredAt: number;
}

export function operationsRecordParams(query: LocationQuery) {
  // Keep the dashboard's immutable time window when paging or refreshing.
  const params: Record<string, string> = {};
  for (const key of ['startAt', 'endAt', 'asOf', 'currencyCode', 'status', 'productNo']) {
    const value = query[key];
    if (typeof value === 'string' && value) params[key] = value;
  }
  params.metric = typeof query.operationsMetric === 'string' ? query.operationsMetric : '';
  return params;
}

export function operationsRecords(query: LocationQuery, page: number, pageSize: number) {
  return requestClient.get<{ items: OperationsRecord[]; total: number }>(
    pluginApiPath('idc-core', 'admin/operations/data/records'),
    { params: { ...operationsRecordParams(query), page, pageSize } },
  );
}
