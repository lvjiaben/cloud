import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const catalogs = {
  "en-US": JSON.parse(readFileSync(new URL("../../../manifest/i18n/en-US/plugin.json", import.meta.url), "utf8")),
  "zh-CN": JSON.parse(readFileSync(new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url), "utf8")),
} as Record<string, Record<string, string>>;

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const parts = key.split(".");
    let cursor = root;
    for (const part of parts.slice(0, -1)) {
      if (!cursor[part] || typeof cursor[part] !== "object") cursor[part] = {};
      cursor = cursor[part] as Record<string, unknown>;
    }
    cursor[parts.at(-1)!] = value;
  }
  return root;
}

export interface ResourceOverviewMockState {
  requests: number;
}

export async function installResourceOverviewMock(
  page: Page,
  options: { failFirst?: boolean } = {},
): Promise<ResourceOverviewMockState> {
  const state = { requests: 0 };
  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as { data?: { permissions?: string[] } };
    if (payload.data) payload.data.permissions = ["idc:admin:resource:overview:list"];
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    const locale = route.request().headers()["accept-language"]?.includes("en") ? "en-US" : "zh-CN";
    await route.fulfill({ json: envelope({ enabled: true, items: [
      { direction: "ltr", isDefault: true, locale: "zh-CN", name: "Simplified Chinese", nativeName: "简体中文" },
      { direction: "ltr", isDefault: false, locale: "en-US", name: "English", nativeName: "English" },
    ], locale }) });
  });
  await page.route("**/api/v1/i18n/runtime/messages**", async (route: Route) => {
    const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
    await route.fulfill({ json: envelope({ messages: nestedMessages(catalogs[locale]!) }) });
  });
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({ json: envelope({ list: [{ children: [{
      component: "system/plugin/dynamic-page",
      meta: { icon: "lucide:server-cog", title: "plugin.idc-resource.overview.title" },
      name: "IDCResourceOverview", path: "/idc/resource/overview",
    }], component: "BasicLayout", meta: { title: "IDC Operations" }, name: "IDCOperations", path: "/idc" }] }) });
  });
  await page.route("**/x/idc-resource/api/v1/admin/resource/overview**", async (route: Route) => {
    state.requests += 1;
    if (options.failFirst && state.requests === 1) {
      await route.fulfill({ status: 503, json: { code: 50, message: "unavailable" } });
      return;
    }
    const observedAt = 1_786_248_000_000;
    await route.fulfill({ json: envelope({ summary: {
      activeMappings: 27, enabledChannels: 2, enabledPools: 3,
      poolTotal: 1, financeTotal: 2,
      latestProviderCapacityAt: observedAt, pendingTasks: 4, unknownTasks: 1,
      pools: [{ channelName: "魔方云生产渠道", channelNo: "channel_mofang_prod", displayName: "华东云服务器池",
        poolNo: "pool_east_cloud", resourceKind: "cloud_server", routingPriority: 10, routingWeight: 80,
        status: "enabled", updatedAt: observedAt }],
      finance: [
        { amount: "12850.00", channelName: "魔方云生产渠道", channelNo: "channel_mofang_prod", currencyCode: "CNY",
          expiresAt: null, externalResourceRef: "provider-secret-ref", kind: "balance", observedAt, snapshotNo: "balance_001" },
        { amount: "", channelName: "魔方云生产渠道", channelNo: "channel_mofang_prod", currencyCode: "",
          expiresAt: observedAt + 86_400_000, externalResourceRef: "provider-expiry-ref", kind: "expiry", observedAt, snapshotNo: "expiry_001" },
      ],
    } }) });
  });
  return state;
}
