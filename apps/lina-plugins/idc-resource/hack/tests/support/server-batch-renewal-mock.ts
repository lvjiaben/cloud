import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const catalog = JSON.parse(
  readFileSync(
    new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
    "utf8",
  ),
) as Record<string, string>;

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}
function nestedMessages(messages: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(messages)) {
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

export async function installServerBatchRenewalMock(page: Page, options: {currencyCode?:string;amountMinor?:string}={}) {
  const previews: Record<string, unknown>[] = [];
  const confirmations: Record<string, unknown>[] = [];
  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data)
      payload.data.permissions = [
        "idc:admin:resource:server-batch:view",
        "idc:admin:resource:server-batch:safe",
      ];
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", (route: Route) =>
    route.fulfill({
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
        ],
        locale: "zh-CN",
      }),
    }),
  );
  await page.route("**/api/v1/i18n/runtime/messages**", (route: Route) =>
    route.fulfill({ json: envelope({ messages: nestedMessages(catalog) }) }),
  );
  await page.route("**/api/v1/menus/all", (route: Route) =>
    route.fulfill({
      json: envelope({
        list: [
          {
            children: [
              {
                component: "system/plugin/dynamic-page",
                meta: {
                  icon: "lucide:calendar-sync",
                  title: "plugin.idc-resource.serverRenewal.title",
                },
                name: "IDCResourceServerRenewals",
                path: "/idc/resource/server-renewals",
              },
            ],
            component: "BasicLayout",
            meta: { title: "服务器与资源" },
            name: "IDCOperations",
            path: "/idc",
          },
        ],
      }),
    }),
  );
  await page.route(
    /\/x\/idc-resource\/api\/v1\/admin\/resource\/server-renewals(?:\?.*)?$/,
    (route: Route) =>
      route.fulfill({
        json: envelope({
          items: [
            {
              productNameEnUS: "Standard Server",
              productNameZhCN: "标准服务器",
              serverNo: "SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
            },
            {
              productNameEnUS: "Performance Server",
              productNameZhCN: "高性能服务器",
              serverNo: "SVR-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
            },
          ],
          nextCursor: "",
          total: 2,
        }),
      }),
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/server-renewals/previews",
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      previews.push(body);
      const serverNos = body.serverNos as string[];
      await route.fulfill({
        json: envelope({
          confirmationDigest: "a".repeat(64),
          requestKey: body.requestKey,
          renewalCycles: body.renewalCycles,
          items: serverNos.map((serverNo, index) => ({
            currencyCode: options.currencyCode ?? "CNY",
            eligible: index === 0,
            failureCode: index === 0 ? "" : "quote_unavailable",
            originalExpiresAt: 1786464000000,
            productNameEnUS:
              index === 0 ? "Standard Server" : "Performance Server",
            productNameZhCN: index === 0 ? "标准服务器" : "高性能服务器",
            projectedExpiresAt: 1818000000000,
            quoteExpiresAt: 1786464300000,
            serverNo,
            totalAmountMinor: index === 0 ? (options.amountMinor ?? "129900") : "0",
          })),
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/server-renewals/confirmations",
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      confirmations.push(body);
      const serverNos = body.serverNos as string[];
      await route.fulfill({
        json: envelope({
          requestKey: body.requestKey,
          renewalCycles: body.renewalCycles,
          items: serverNos.map((serverNo, index) => ({
            currencyCode: options.currencyCode ?? "CNY",
            failureCode: index === 0 ? "" : "confirmation_failed",
            productNameEnUS: "Server",
            productNameZhCN: "服务器",
            serverNo,
            status: index === 0 ? "pending_payment" : "",
            succeeded: index === 0,
            totalAmountMinor: index === 0 ? (options.amountMinor ?? "129900") : "0",
          })),
        }),
      });
    },
  );
  return { confirmations, previews };
}
