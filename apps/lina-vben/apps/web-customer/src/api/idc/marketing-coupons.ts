import { pluginApiPath, requestClient } from '#/api/request';

import { getOrCreateRegistrationDeviceID } from './customer-onboarding';

const pluginID = 'idc-core';

export interface MarketingCoupon {
  assetNo: string;
  assetType: 'coupon' | 'voucher';
  campaignNo: string;
  claimedAt: null | number;
  consumedUses: number;
  currencyCode: string;
  faceAmountMinor: null | number;
  nameEnUs: string;
  nameZhCn: string;
  rateBasisPoints: null | number;
  status: string;
  usageLimit: number;
  validFrom: number;
  validUntil: number;
}

export function listMyMarketingCoupons() {
  return requestClient.get<{ items: MarketingCoupon[] }>(
    pluginApiPath(pluginID, 'account/marketing/coupons'),
  );
}

export function claimMarketingCoupon(code: string) {
  return requestClient.post<{ item: MarketingCoupon }>(
    pluginApiPath(pluginID, 'account/marketing/coupons/claims'),
    { code, deviceId: getOrCreateRegistrationDeviceID() },
    { silentErrorMessage: true },
  );
}
