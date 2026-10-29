import type { BrowserContext } from "@playwright/test";
import {
  test as host,
  expect,
} from "../../../../../../hack/tests/fixtures/auth";
import { LoginPage } from "../../../../../../hack/tests/pages/LoginPage";
import { config } from "../../../../../../hack/tests/fixtures/config";

type State = Awaited<ReturnType<BrowserContext["storageState"]>>;
// Keep this navigation suite independent of other chats' saved login files.
export const test = host.extend<{}, { resourceNavigationState: State }>({
  resourceNavigationState: [
    async ({ browser }, use) => {
      const context = await browser.newContext({ baseURL: config.baseURL });
      const page = await context.newPage();
      const login = new LoginPage(page);
      await login.goto();
      await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
      await page.getByRole("menu").waitFor({ state: "visible" });
      const state = await context.storageState();
      await context.close();
      await use(state);
    },
    { scope: "worker" },
  ],
  authenticatedPage: async ({ authenticatedPage }, use) => {
    try {
      await use(authenticatedPage);
    } finally {
      await authenticatedPage.unrouteAll({ behavior: "wait" });
    }
  },
  adminContext: async ({ browser, resourceNavigationState }, use) => {
    const context = await browser.newContext({
      baseURL: config.baseURL,
      storageState: resourceNavigationState,
    });
    await use(context);
    await context.close();
  },
});
export { expect };
