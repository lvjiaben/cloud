import type { BrowserContext } from "@playwright/test";
import {
  test as host,
  expect,
} from "../../../../../../hack/tests/fixtures/auth";
import { LoginPage } from "../../../../../../hack/tests/pages/LoginPage";
import { config } from "../../../../../../hack/tests/fixtures/config";

type State = Awaited<ReturnType<BrowserContext["storageState"]>>;
// One worker login, with an independent context per test and no shared state files.
export const test = host.extend<{}, { managerState: State }>({
  managerState: [
    async ({ browser }, use) => {
      const context = await browser.newContext({ baseURL: config.baseURL });
      const page = await context.newPage();
      const login = new LoginPage(page);
      await login.goto();
      await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
      const state = await context.storageState();
      await context.close();
      await use(state);
    },
    { scope: "worker" },
  ],
  adminPage: async ({ adminPage }, use) => {
    // A missing mock must never mutate a real plugin in the shared local instance.
    await adminPage.route(/\/api\/v1\/plugins(?:\/|$)/, (route) =>
      route.request().method() === "GET"
        ? route.fallback()
        : route.abort("blockedbyclient"),
    );
    await use(adminPage);
  },
  adminContext: async ({ browser, managerState }, use) => {
    const context = await browser.newContext({
      baseURL: config.baseURL,
      storageState: managerState,
    });
    await use(context);
    await context.close();
  },
});
export { expect };
