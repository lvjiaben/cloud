import { test as hostTest, expect } from '@host-tests/fixtures/auth';
import { config } from '@host-tests/fixtures/config';

// Reuse the normal authenticated fixture and its encrypted application storage.
// Catalog tests do not decode private storage or issue another login.
export const test = hostTest.extend({
  adminPage: async ({ adminPage }, use) => {
    // Route shared public APIs to the explicitly selected test backend.
    await adminPage.route(/\/(?:api\/v1|x\/[^/]+\/api\/v1)\//, async route => {
      const url = new URL(route.request().url());
      const target = new URL(url.pathname + url.search, config.backendBaseURL);
      const response = await route.fetch({ url: target.href, timeout: 15000 });
      await route.fulfill({ response });
    });
    try {
      await use(adminPage);
    } finally {
      await adminPage.unrouteAll({ behavior: 'wait' });
    }
  },
});
export { expect };
