import { test as hostTest, expect } from '@host-tests/fixtures/auth';
import { installCustomerAdminSession } from './customer-admin-session';
import { config } from '@host-tests/fixtures/config';

// Renew an expired test session only in this browser context; never write host fixtures.
export const test = hostTest.extend({
  adminPage: async ({ authenticatedPage }, use) => {
    await installCustomerAdminSession(authenticatedPage);
    // Route shared public APIs to the explicitly selected test backend.
    await authenticatedPage.route(/\/(?:api\/v1|x\/[^/]+\/api\/v1)\//, async route => {
      const url = new URL(route.request().url());
      const target = new URL(url.pathname + url.search, config.backendBaseURL);
      const response = await route.fetch({ url: target.href, timeout: 15000 });
      await route.fulfill({ response });
    });
    // Existing catalog mocks clear runtime translations through localStorage.
    // Give them the application origin before they access browser storage.
    await authenticatedPage.goto(config.baseURL);
    try {
      await use(authenticatedPage);
    } finally {
      await authenticatedPage.unrouteAll({ behavior: 'wait' });
    }
  },
});
export { expect };
