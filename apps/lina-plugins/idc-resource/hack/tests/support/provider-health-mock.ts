import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const catalogs = {
  "en-US": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/en-US/plugin.json", import.meta.url),
      "utf8",
    ),
  ),
  "zh-CN": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
      "utf8",
    ),
  ),
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

const observedAt = 1_786_248_000_000;
export const baseItem = {
  authentication: {
    healthCode: "healthy",
    latencyMs: 45,
    observedAt,
    summaryCode: "succeeded",
  },
  channelName: "魔方云生产渠道",
  channelNo: "channel_mofang_prod",
  channelStatus: "enabled",
  circuitOpen: true,
  consecutiveFailures: 3,
  failureCount: 3,
  healthCode: "unavailable",
  providerCapacity: {
    healthCode: "healthy",
    latencyMs: 125,
    observedAt,
    summaryCode: "succeeded",
  },
  lastObservedAt: observedAt,
  lastSummaryCode: "upstream_timeout",
  policy: {
    consecutiveFailureLimit: 3,
    failureLimit: 3,
    minimumSamplesToProtect: 5,
    protectionDurationSeconds: 300,
    unknownResultLimit: 2,
    updatedAt: null as number | null,
    version: 1,
    windowSampleLimit: 20,
  },
  providerId: "zjmfcloud-v1",
  query: {
    healthCode: "degraded",
    latencyMs: 2_400,
    observedAt,
    summaryCode: "timeout",
  },
  sampleCount: 5,
  unknownCount: 2,
  write: {
    healthCode: "unavailable",
    latencyMs: 3_000,
    observedAt,
    summaryCode: "timeout",
  },
  writeProtectedUntil: observedAt + 300_000,
};

export interface ProviderHealthMockState {
  listRequests: URL[];
  policyRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    url: URL;
  }>;
}

export async function installProviderHealthMock(
  page: Page,
  options: { canUpdatePolicy?: boolean } = {},
): Promise<ProviderHealthMockState> {
  const state: ProviderHealthMockState = {
    listRequests: [],
    policyRequests: [],
  };
  let current = structuredClone(baseItem);

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data) {
      payload.data.permissions = ["idc:admin:resource:health:list"];
      if (options.canUpdatePolicy !== false)
        payload.data.permissions.push("idc:admin:resource:health-policy:write");
    }
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    const locale = route.request().headers()["accept-language"]?.includes("en")
      ? "en-US"
      : "zh-CN";
    await route.fulfill({
      json: envelope({
        enabled: true,
        items: [
          {
            direction: "ltr",
            isDefault: true,
            locale: "zh-CN",
            name: "Simplified Chinese",
            nativeName: "简体中文",
          },
          {
            direction: "ltr",
            isDefault: false,
            locale: "en-US",
            name: "English",
            nativeName: "English",
          },
        ],
        locale,
      }),
    });
  });
  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const locale =
        new URL(route.request().url()).searchParams.get("lang") === "en-US"
          ? "en-US"
          : "zh-CN";
      await route.fulfill({
        json: envelope({ messages: nestedMessages(catalogs[locale]!) }),
      });
    },
  );
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({
      json: envelope({
        list: [
          {
            children: [
              {
                component: "system/plugin/dynamic-page",
                meta: {
                  icon: "lucide:activity",
                  title: "plugin.idc-resource.health.title",
                },
                name: "IDCResourceProviderHealth",
                path: "/idc/resource/health",
              },
            ],
            component: "BasicLayout",
            meta: { title: "IDC Operations" },
            name: "IDCOperations",
            path: "/idc",
          },
        ],
      }),
    });
  });
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/channels/*/health-policy",
    async (route: Route) => {
      const request = route.request();
      const body = request.postDataJSON() as Record<string, unknown>;
      state.policyRequests.push({
        body,
        method: request.method(),
        url: new URL(request.url()),
      });
      current = {
        ...current,
        policy: {
          ...current.policy,
          ...body,
          version: current.policy.version + 1,
          updatedAt: Date.now(),
        },
      };
      await route.fulfill({ json: envelope({ item: current }) });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/channel-health**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.listRequests.push(url);
      await route.fulfill({
        json: envelope({ items: [current], nextCursor: "", total: 1 }),
      });
    },
  );
  return state;
}
