import { pluginApiPath, requestClient } from '#/api/request';

export type SalesSection = 'performance' | 'levels' | 'agents' | 'prices' | 'ownership' | 'commission-rules' | 'commission/accounts' | 'commission/records' | 'commission/payouts';
export interface SalesPage<T> { items: T[]; total: number }
export interface SalesRow {
  levelNo?: string; name?: string; englishName?: string; levelName?: string;
  customerNo?: string; customerName?: string; customerCode?: string; status?: string; expiresAt?: number | null;
  priceNo?: string; productNo?: string; productName?: string; currencyCode?: string; chargeTypeCode?: string; periodUnitCode?: string; periodCount?: number;
  adjustmentType?: string; fixedPriceMinor?: string; discountBasisPoints?: number | null;
  staffUserId?: number; staffName?: string; referrerCustomerNo?: string; referrerName?: string; referrerCode?: string;
  ruleNo?: string; beneficiaryType?: string; operationType?: string; calculationType?: string; fixedAmountMinor?: string; rateBasisPoints?: number | null; maturityDays?: number;
  accountNo?: string; beneficiaryName?: string; creditedMinor?: string; pendingMinor?: string; availableMinor?: string; paidMinor?: string; reversedMinor?: string; recoveryDueMinor?: string;
  invoiceNo?: string; commissionNo?: string; orderNo?: string; basisMinor?: string; originalMinor?: string; maturedMinor?: string; maturesAt?: number | null;
  payoutNo?: string; amountMinor?: string; evidenceReference?: string; paidAt?: number; createdAt?: number; recordedBy?: number;
  version?: number;
  percentageBasisPoints?: number | null; pricingVersion?: number;
}
export interface SalesTerm { currencyCode: string; chargeTypeCode: string; periodUnitCode: string; periodCount: number }
export interface SalesProduct { productNo: string; name: string; englishName: string; terms: SalesTerm[] }
const path = (suffix: string) => pluginApiPath('idc-core', `admin/sales-management/${suffix}`);
export function salesList(section: SalesSection, params: { page: number; pageSize: number; search?: string; productNo?: string; target?: 'customer' | 'level' }) {
  return requestClient.get<SalesPage<SalesRow>>(path(section), { params });
}
export function salesSave(section: SalesSection, body: Record<string, unknown>, update: boolean) {
  return update ? requestClient.put<{ item: SalesRow }>(path(section), body) : requestClient.post<{ item: SalesRow }>(path(section), body);
}
export function salesProducts(search = '', page = 1) { return requestClient.get<SalesPage<SalesProduct> & { currencies: string[] }>(path('product-candidates'), { params: { search, page, pageSize: 30 } }); }
export function salesStaff(search = '', page = 1) { return requestClient.get<SalesPage<{ userId: number; name: string; selectable: boolean }>>(path('staff-candidates'), { params: { search, page, pageSize: 30 } }); }

export interface SalesPerformanceRow {
  beneficiaryType: 'sales' | 'affiliate'; beneficiaryReference: string; beneficiaryName: string; currencyCode: string;
  paidOrderCount: number; paidRenewalCount: number; receivedMinor: string; refundedMinor: string; netReceivedMinor: string;
}
export function salesPerformance(params: { month: string; page: number; pageSize: number; search?: string; beneficiaryType?: string; currencyCode?: string }) {
  return requestClient.get<SalesPage<SalesPerformanceRow> & { timezone: string; asOf: number }>(path('performance'), { params });
}

export function salesDeletePrice(priceNo: string, expectedVersion: number) {
  return requestClient.delete<{ item: SalesRow }>(path('prices'), { data: { priceNo, expectedVersion } });
}

export function salesPriceLevels(search = '') {
  return requestClient.get<SalesPage<SalesRow>>(path('price-level-candidates'), { params: { search, page: 1, pageSize: 100 } });
}
export function salesPriceProducts(search = '', productNo = '') {
  return requestClient.get<SalesPage<SalesProduct> & { currencies: string[] }>(path('price-product-candidates'), { params: { search, productNo, page: 1, pageSize: 30 } });
}

export interface SalesLevelPricing { levelNo: string; percentageBasisPoints: number | null; version: number }
export function salesLevelPricing(levelNo: string) {
  return requestClient.get<{ item: SalesLevelPricing }>(path(`levels/${encodeURIComponent(levelNo)}/pricing`));
}
export function salesSaveLevelPricing(levelNo: string, percentageBasisPoints: number | null, expectedVersion: number) {
  return requestClient.put<{ item: SalesLevelPricing }>(path(`levels/${encodeURIComponent(levelNo)}/pricing`), { percentageBasisPoints, expectedVersion });
}
