import type { PluginHostMountContext } from '../host-context';
import { createPluginRequest } from '../runtime/request';

export interface SalesChannelCurrencyOverview { currencyCode: string; accruedMinor: number; maturedMinor: number }
export interface SalesChannelOverview {
  qualifications: number; affiliateLinks: number; attributions: number;
  pendingMaturity: number; pendingSettlement: number; currencies: SalesChannelCurrencyOverview[];
}

export function createSalesChannelApi(context: PluginHostMountContext) {
  const request = createPluginRequest(() => context);
  return { overview: () => request<SalesChannelOverview>('/x/idc-core/api/v1/admin/sales-channel/overview') };
}
