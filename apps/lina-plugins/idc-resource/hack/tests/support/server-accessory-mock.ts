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

const permissions = [
  "idc:admin:resource:server-accessory:view",
  "idc:admin:resource:server-accessory:snapshot:create",
  "idc:admin:resource:server-accessory:snapshot:delete",
  "idc:admin:resource:server-accessory:snapshot:restore",
  "idc:admin:resource:server-accessory:security-group:attach",
  "idc:admin:resource:server-accessory:traffic:add",
];

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

export interface ServerAccessoryMockState {
  batchRequests: Array<{ body: Record<string, unknown>; url: string }>;
  requests: Array<{
    body: Record<string, unknown>;
    method: string;
    url: string;
  }>;
}

export async function installServerAccessoryMock(
  page: Page,
  options: { activeOperation?: boolean; permissions?: string[]; trafficPackages?: boolean } = {},
): Promise<ServerAccessoryMockState> {
  const state: ServerAccessoryMockState = { batchRequests: [], requests: [] };
  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data)
      payload.data.permissions = options.permissions ?? permissions;
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
                  icon: "lucide:layers-3",
                  title: "plugin.idc-resource.serverAccessories.title",
                },
                name: "IDCResourceServerAccessories",
                path: "/idc/resource/server-accessories",
              },
            ],
            component: "BasicLayout",
            meta: { title: "服务器与资源" },
            name: "IDCOperations",
            path: "/idc",
          },
        ],
      }),
    });
  });

  const api =
    "**/x/idc-core/api/v1/admin/resource/server-accessories/svc-10001";
  await page.route(
    "**/x/idc-core/api/v1/admin/resource/server-services**",
    async (route: Route) => {
      await route.fulfill({
        json: envelope({
          items: [
            {
              customerNo: "customer-10001",
              entitlementStatus: "active",
              productNameEnUS: "Cloud Server",
              productNameZhCN: "云服务器",
              providerStatus: "succeeded",
              runtimeStatus: "running",
              serviceNo: "svc-10001",
            },
            {
              customerNo: "customer-10002",
              entitlementStatus: "active",
              productNameEnUS: "Bare Metal",
              productNameZhCN: "裸金属服务器",
              providerStatus: "succeeded",
              runtimeStatus: "stopped",
              serviceNo: "svc-10002",
            },
          ],
          total: 2,
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/resource/server-batches/**",
    async (route: Route) => {
      const body = (route.request().postDataJSON() ?? {}) as Record<string, unknown>;
      state.batchRequests.push({ body, url: route.request().url() });
      const selected = (body.items ?? [
        { serviceNo: "svc-10001" },
        { serviceNo: "svc-10002" },
      ]) as Array<{ serviceNo: string }>;
      await route.fulfill({
        json: envelope({
          batch: {
            action: body.action ?? "renewal_reminder",
            batchNo: "sbt-test-0001",
            dangerous: false,
            items: selected.map((item) => ({
              actionAmountMinor: 0,
              customerNo: item.serviceNo === "svc-10001" ? "customer-10001" : "customer-10002",
              entitlementStatus: "active",
              executionStatus: route.request().url().endsWith("safe-executions") ? "succeeded" : "pending",
              failureCode: "",
              irreversible: false,
              previewStatus: "eligible",
              providerStatus: "succeeded",
              resourceSummary: "cloud_server / primary",
              runtimeStatus: "running",
              serviceNo: item.serviceNo,
              unpaidAmountMinor: 0,
            })),
            previewDigest: "a".repeat(64),
            status: route.request().url().endsWith("safe-executions") ? "completed" : "previewed",
          },
        }),
      });
    },
  );
  await page.route(api, async (route: Route) => {
    const observedAt = Date.now();
    await route.fulfill({
      json: envelope({
        detail: {
          accessories: [
            {
              kind: "snapshot",
              name: "升级前快照",
              objectNo: "snap-10001",
              projection: { sizeGiB: 80 },
              state: "available",
            },
            {
              kind: "security_group",
              name: "Web 安全组",
              objectNo: "sg-10001",
              projection: { cidr: "10.0.0.0/24" },
              state: "available",
            },
          ],
          accessoriesExpiresAt: observedAt + 86_400_000,
          accessoriesObservedAt: observedAt,
          activeOperationNo: options.activeOperation ? "op-result-unknown" : "",
          availableActions: [
            "snapshot_create",
            "snapshot_delete",
            "snapshot_restore",
            "security_group_attach",
            ...(options.trafficPackages ? ["traffic_add"] : []),
          ],
          customerNo: "customer-10001",
          entitlementStatus: "active",
          providerTaskStatus: options.activeOperation
            ? "result_unknown"
            : "succeeded",
          runtimeStatus: "running",
          serviceNo: "svc-10001",
        },
      }),
    });
  });
  await page.route(`${api}/operations/*`, async (route: Route) => {
    state.requests.push({
      body: (route.request().postDataJSON() ?? {}) as Record<string, unknown>,
      method: route.request().method(),
      url: route.request().url(),
    });
    const charged = route.request().url().endsWith("/snapshot-create");
    await route.fulfill({
      json: envelope({
        action: charged ? "snapshot_create" : "snapshot_restore",
        amountMinor: charged ? "2500" : "0",
        currencyCode: charged ? "CNY" : "",
        invoiceNo: charged ? "ain-pending-10001" : "",
        operationNo: charged ? "aop-10002" : "op-10001",
        orderNo: charged ? "aor-pending-10001" : "",
        paymentRequired: charged,
        status: charged ? "awaiting_financial_settlement" : "ready",
      }),
    });
  });
  return state;
}

/** Overrides only the fixed-package contract, without performing real writes. */
export async function installStaffTrafficPackageMock(page: Page) {
  const state = { failNext: false, requests: [] as Record<string, unknown>[] };
  const api = "**/x/idc-core/api/v1/admin/resource/server-accessories/svc-10001";
  await page.route(`${api}/traffic-packages**`, async (route: Route) => {
    await route.fulfill({ json: envelope({ total: 2, items: [
      { packageNo: "TPK_staff_100", version: 3, nameZhCN: "固定100GB", nameEnUS: "Fixed 100GB", trafficGb: 100, currencyCode: "CNY", amountMinor: 2500, available: true },
      { packageNo: "TPK_staff_500", version: 1, nameZhCN: "固定500GB", nameEnUS: "Fixed 500GB", trafficGb: 500, currencyCode: "CNY", amountMinor: 8000, available: false },
    ] }) });
  });
  await page.route(`${api}/operations/traffic-add`, async (route: Route) => {
    state.requests.push(route.request().postDataJSON() as Record<string, unknown>);
    if (state.failNext) {
      state.failNext = false;
      await route.fulfill({ status: 503, json: { code: 500, message: "测试请求暂未完成，请重试", data: null } });
      return;
    }
    await route.fulfill({ json: envelope({ action: "traffic_add", amountMinor: 2500, currencyCode: "CNY", invoiceNo: "INV_staff_traffic", operationNo: "AOP_staff_traffic", orderNo: "ORD_staff_traffic", paymentRequired: true, status: "awaiting_financial_settlement" }) });
  });
  return state;
}
