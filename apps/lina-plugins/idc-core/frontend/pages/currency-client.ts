import { pluginApiPath, requestClient } from '#/api/request';
const currencyPath = (path: string) => pluginApiPath('idc-core', path);
export interface MerchantCurrency { currencyCode: string; enabled: boolean; isDefault: boolean; version: number }
export function currencyList(page: number, pageSize: number) {
  return requestClient.get<{ items: MerchantCurrency[]; total: number }>(currencyPath('admin/currencies'), { params: { page, pageSize } });
}
export function currencyOptions() {
  return requestClient.get<{ currencies: string[] }>(currencyPath('currencies'));
}
export function currencyCreate(currencyCode: string) {
  return requestClient.post<MerchantCurrency>(currencyPath('admin/currencies'), { currencyCode });
}
export function currencyUpdate(item: MerchantCurrency, enabled: boolean, makeDefault = false) {
  return requestClient.put<MerchantCurrency>(currencyPath(`admin/currencies/${encodeURIComponent(item.currencyCode)}`), { expectedVersion: item.version, enabled, makeDefault });
}
export function currencyLabel(code: string, locale: string) {
  try { return `${new Intl.DisplayNames([locale], { type: 'currency' }).of(code) || code} (${code})`; }
  catch { return code; }
}

export function currencyDelete(item: MerchantCurrency) {
 return requestClient.delete(currencyPath(`admin/currencies/${encodeURIComponent(item.currencyCode)}`), {params:{expectedVersion:item.version}});
}
