import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const now = 1_787_356_800_000;

export interface CustomerMarketingCouponMockState {
  claimBodies: Array<Record<string, unknown>>;
  claimed: boolean;
}

function claimedCoupon() {
  return {
    assetNo: 'PROMO_COUPON_0001',
    assetType: 'coupon',
    campaignNo: 'CAMPAIGN_SUMMER_2026',
    claimedAt: now,
    consumedUses: 0,
    currencyCode: 'CNY',
    faceAmountMinor: 500,
    nameEnUs: 'Summer discount',
    nameZhCn: '夏日优惠',
    rateBasisPoints: null,
    status: 'active',
    usageLimit: 1,
    validFrom: now - 86_400_000,
    validUntil: now + 30 * 86_400_000,
  };
}

export async function installCustomerMarketingCouponMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const state: CustomerMarketingCouponMockState = {
    claimBodies: [],
    claimed: false,
  };

  await page.route(
    '**/x/idc-core/api/v1/account/marketing/coupons**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (request.method() === 'GET' && path.endsWith('/coupons')) {
        await route.fulfill({
          json: {
            code: 0,
            data: { items: state.claimed ? [claimedCoupon()] : [] },
            message: 'OK',
          },
        });
        return;
      }
      if (request.method() === 'POST' && path.endsWith('/coupons/claims')) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.claimBodies.push(body);
        if (String(body.code) !== 'SAVE2026AB') {
          await route.fulfill({
            json: {
              code: 4_001,
              errorCode: 'marketing.coupon.invalid',
              message: 'The promotion code is invalid',
            },
            status: 400,
          });
          return;
        }
        state.claimed = true;
        await route.fulfill({
          json: {
            code: 0,
            data: { item: claimedCoupon() },
            message: 'OK',
          },
        });
        return;
      }
      await route.fulfill({
        json: { code: 4_004, message: 'unexpected marketing request' },
        status: 404,
      });
    },
  );
  return state;
}
