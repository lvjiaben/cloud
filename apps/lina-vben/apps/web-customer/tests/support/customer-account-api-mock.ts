import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const localCustomerNo = 'CUS_00000000000000000000000000000001';

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: 'OK' });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installCustomerAccountMock(page: Page) {
  await mockCustomerAPIs(page);
  const profileRequestURLs: string[] = [];
  const addressWrites: unknown[] = [];

  await page.route(
    '**/x/idc-core/api/v1/account/customer/**',
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.endsWith('/account/customer/profile')) {
        profileRequestURLs.push(url.toString());
        if (url.searchParams.has('customerNo')) {
          await route.fulfill({
            body: JSON.stringify({
              code: 404,
              data: null,
              message: 'not found',
            }),
            contentType: 'application/json',
            status: 404,
          });
          return;
        }
        await fulfill(route, {
          item: {
            countryCode: 'CN',
            customerCode: '10000',
            customerNo: localCustomerNo,
            displayName: '当前登录客户',
            identifierMasked: '310***********1234',
            legalNameMasked: '当**',
            preferredLocale: 'zh-CN',
            status: 'active',
            type: 'individual',
            verification: 'verified',
            version: 3,
          },
        });
        return;
      }
      if (url.pathname.endsWith('/account/customer/contacts')) {
        await fulfill(route, { items: [] });
        return;
      }
      if (url.pathname.endsWith('/account/customer/addresses')) {
        if (request.method() === 'POST') {
          const input = request.postDataJSON();
          addressWrites.push(input);
          await fulfill(route, {
            item: {
              ...input,
              addressNo: 'ADDR-1',
              status: 'active',
              version: 1,
            },
          });
          return;
        }
        await fulfill(route, { items: [] });
        return;
      }
      await route.fallback();
    },
  );

  return { addressWrites, localCustomerNo, profileRequestURLs };
}
