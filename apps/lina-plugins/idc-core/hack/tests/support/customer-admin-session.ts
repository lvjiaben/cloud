// Keep this module's browser session independent of other local test runs' shared auth file.
import { readFileSync } from "node:fs";
import type { Page } from "@host-tests/support/playwright";
import { adminStorageStatePath } from "../../../../../../hack/tests/fixtures/auth-state";
import { test as base } from "../../../../../../hack/tests/fixtures/auth";
import { config } from "../../../../../../hack/tests/fixtures/config";

export async function installCustomerAdminSession(page: Page) {
  const response = await page.request.post(`${config.apiBaseURL}auth/login`, {
    data: { username: config.adminUser, password: config.adminPass, clientType: "desktop" },
  });
  const payload = await response.json();
  if (!response.ok() || payload.code !== 0 || !payload.data?.accessToken) {
    throw new Error(`Customer test login failed (${response.status()})`);
  }
  const state = JSON.parse(readFileSync(adminStorageStatePath, "utf8"));
  const entries = state.origins[0].localStorage as Array<{name:string;value:string}>;
  const access = entries.find(item => item.name.endsWith("core-access"));
  if (!access) throw new Error("Missing admin access storage namespace");
  const value = JSON.parse(access.value);
  value.accessToken = payload.data.accessToken;
  value.refreshToken = payload.data.refreshToken;
  access.value = JSON.stringify(value);
  for (const item of entries) {
    if (item.name.endsWith("preferences-locale")) item.value = JSON.stringify({value:"zh-CN"});
    if (item.name.endsWith("preferences")) {
      const preferences = JSON.parse(item.value);
      if (preferences.value?.app) preferences.value.app.locale = "zh-CN";
      item.value = JSON.stringify(preferences);
    }
  }
  await page.addInitScript((items) => {
    if (sessionStorage.getItem("idc-customer-test-session")) return;
    for (const item of items) localStorage.setItem(item.name,item.value);
    sessionStorage.setItem("idc-customer-test-session","1");
  }, entries);
}

export const customerTest=base.extend({
 adminPage: async({authenticatedPage},use)=>{
  await installCustomerAdminSession(authenticatedPage);
  await use(authenticatedPage);
 },
});
