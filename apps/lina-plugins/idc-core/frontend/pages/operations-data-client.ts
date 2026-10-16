import { pluginApiPath, requestClient } from "#/api/request";

export interface OperationsPeriod {
  key: "today" | "yesterday" | "month" | "all";
  startAt: number;
  endAt: number;
  incomeMinor: string;
  paidOrders: number;
}

export interface OperationsData {
  currencies: string[];
  currencyCode: string;
  currencyExponent: number;
  timezone: string;
  asOf: number;
  periods: OperationsPeriod[];
  trend: Array<{
    date: string;
    previousDate: string;
    startAt: number;
    endAt: number;
    previousStartAt: number;
    previousEndAt: number;
    currentMinor: string;
    currentCount: number;
    previousCount: number | null;
  }>;
  products: Array<{
    productNo: string;
    nameZhCN: string;
    nameEnUS: string;
    quantity: number;
  }>;
  customers: { today: number; total: number; online: number };
  expiringServers: number;
  tickets: { pending: number; processed: number };
}

export function loadOperationsData(currencyCode?: string) {
  return requestClient.get<OperationsData>(
    pluginApiPath("idc-core", "admin/operations/data"),
    { params: { currencyCode } },
  );
}
