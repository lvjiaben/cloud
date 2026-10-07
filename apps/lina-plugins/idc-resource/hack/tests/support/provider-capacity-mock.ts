import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const localeCatalogs = {
  "en-US": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/en-US/plugin.json", import.meta.url),
      "utf8",
    ),
  ) as Record<string, string>,
  "zh-CN": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
      "utf8",
    ),
  ) as Record<string, string>,
};

interface ProviderCapacityMockOperation {
  channelName: string;
  channelNo: string;
  completedAt: null | number;
  failureCode: string;
  itemCount: number;
  kind: string;
  lastObservedAt: number;
  pageCount: number;
  requestedAt: number;
  observationNo: string;
  status: string;
  trigger: string;
}

export const firstOperation: ProviderCapacityMockOperation = {
  channelName: "魔方云生产渠道",
  channelNo: "channel_mofang_prod",
  completedAt: 1_786_161_680_000,
  failureCode: "",
  itemCount: 428,
  kind: "plan",
  lastObservedAt: 1_786_161_680_000,
  pageCount: 5,
  requestedAt: 1_786_161_600_000,
  observationNo: "observation_complete_001",
  status: "complete",
  trigger: "system",
};

const secondOperation = {
  ...firstOperation,
  channelName: "新加坡备用渠道",
  channelNo: "channel_sg_backup",
  completedAt: null,
  failureCode: "provider_unavailable",
  itemCount: 0,
  kind: "image",
  pageCount: 0,
  observationNo: "observation_failed_002",
  status: "failed",
  trigger: "manual",
};

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const segments = key.split(".");
    let current = result;
    for (const segment of segments.slice(0, -1)) {
      const child = current[segment];
      if (!child || typeof child !== "object" || Array.isArray(child)) {
        current[segment] = {};
      }
      current = current[segment] as Record<string, unknown>;
    }
    const leaf = segments.at(-1);
    if (leaf) current[leaf] = value;
  }
  return result;
}

export interface ProviderCapacityMockState {
  listRequests: URL[];
  syncRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    url: URL;
  }>;
}

export async function installProviderCapacityMock(
  page: Page,
  options: { canSynchronize?: boolean } = {},
): Promise<ProviderCapacityMockState> {
  const state: ProviderCapacityMockState = {
    listRequests: [],
    syncRequests: [],
  };
  let queuedOperation: ProviderCapacityMockOperation | null = null;
  let currentLocale: keyof typeof localeCatalogs = "zh-CN";

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data) {
      payload.data.permissions = ["idc:admin:resource:provider-capacity:list"];
      if (options.canSynchronize !== false) {
        payload.data.permissions.push(
          "idc:admin:resource:provider-capacity:refresh",
        );
      }
    }
    await route.fulfill({ json: payload, response });
  });

  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    if (route.request().headers()["accept-language"]?.includes("en")) {
      currentLocale = "en-US";
    }
    await route.fulfill({
      headers: { ETag: `\"provider-capacity-e2e-${currentLocale}\"` },
      json: apiEnvelope({
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
        locale: currentLocale,
      }),
    });
  });

  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const requested = new URL(route.request().url()).searchParams.get("lang");
      const locale = requested === "en-US" ? "en-US" : "zh-CN";
      currentLocale = locale;
      await route.fulfill({
        headers: {
          "Cache-Control": "private, must-revalidate",
          ETag: `\"provider-capacity-e2e-${locale}\"`,
        },
        json: apiEnvelope({ messages: nestedMessages(localeCatalogs[locale]) }),
      });
    },
  );

  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({
      json: apiEnvelope({
        list: [
          {
            children: [
              {
                component: "system/plugin/dynamic-page",
                meta: {
                  icon: "lucide:database-zap",
                  title: "plugin.idc-resource.providerCapacity.title",
                },
                name: "IDCResourceProviderCapacity",
                path: "/idc/resource/provider-capacity",
              },
            ],
            component: "BasicLayout",
            meta: {
              icon: "lucide:server-cog",
              order: 30,
              title: "IDC Operations",
            },
            name: "IDCOperations",
            path: "/idc",
          },
        ],
      }),
    });
  });

  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/channels/*/provider-capacity-observations",
    async (route: Route) => {
      const request = route.request();
      const body = request.postDataJSON() as Record<string, unknown>;
      state.syncRequests.push({
        body,
        method: request.method(),
        url: new URL(request.url()),
      });
      const createdOperation: ProviderCapacityMockOperation = {
        ...firstOperation,
        completedAt: null,
        itemCount: 0,
        kind: String(body.kind),
        pageCount: 0,
        observationNo: "observation_collecting_003",
        status: "collecting",
        trigger: "manual",
      };
      queuedOperation = createdOperation;
      await route.fulfill({
        json: apiEnvelope({
          observationNo: createdOperation.observationNo,
          status: "collecting",
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/provider-capacity-observations**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.listRequests.push(url);
      const currentPage = Number(url.searchParams.get("page") || 1);
      const status = url.searchParams.get("status");
      const items =
        currentPage === 2
          ? [secondOperation]
          : [
              ...(queuedOperation ? [queuedOperation] : []),
              ...(status === "failed" ? [secondOperation] : [firstOperation]),
            ];
      await route.fulfill({
        json: apiEnvelope({
          channels: [
            {
              channelNo: "channel_mofang_prod",
              displayName: "魔方云生产渠道",
            },
            {
              channelNo: "channel_sg_backup",
              displayName: "新加坡备用渠道",
            },
          ],
          items,
          nextCursor: "",
          total: status ? items.length : 11,
        }),
      });
    },
  );

  return state;
}
