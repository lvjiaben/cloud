import type { PurchaseOption } from '#/api/idc/purchase-configuration';

export interface DisplayOption { children?: DisplayOption[]; label: string; value: string }

export function mapPurchaseOptions(input: PurchaseOption[], chinese: boolean): DisplayOption[] {
  const convert = (item: PurchaseOption): DisplayOption => ({
    children: item.children?.map(convert),
    label: chinese ? item.labelZhCN || item.labelEnUS : item.labelEnUS || item.labelZhCN,
    value: item.optionToken,
  });
  return input.map(convert);
}
