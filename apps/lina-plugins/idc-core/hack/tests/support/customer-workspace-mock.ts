import { existsSync, readFileSync } from "node:fs";
import { isAbsolute, relative, resolve } from "node:path";
import type { Page } from "@host-tests/support/playwright";
import { installCustomerManagementMock } from "./customer-mock";
export const workspaceCustomerNo = "CUS_0123456789abcdef0123456789abcdef";
const envelope = (data: unknown) => ({ code: 0, data, message: "success" });
// Serve current plugin resources when the test backend runs a previously built release.
export async function installProductManagementLocales(page: Page) {
  const staticUI = process.env.E2E_STATIC_UI_DIR;
  if (staticUI) await page.route("**/api/v1/user/info", async route => {
    const response = await route.fetch(); const body = await response.json();
    body.data.homePath = "/idc/customers";
    await route.fulfill({ json: body });
  });
  if (staticUI) {
    const state = JSON.parse(readFileSync(new URL("../../../../../../hack/tests/temp/storage-state/admin.json", import.meta.url), "utf8"));
    await page.addInitScript((entries: Array<{ name: string; value: string }>) => {
      for (const item of entries) localStorage.setItem(item.name, item.value);
    }, state.origins[0]?.localStorage ?? []);
  }
  if (staticUI) await page.route(url => url.pathname.startsWith("/admin/"), route => {
    const pathname = new URL(route.request().url()).pathname.replace(/^\/admin\//u, "");
    const file = resolve(staticUI, pathname);
    const path = relative(staticUI, file);
    return route.fulfill({ path: path && !path.startsWith("..") && !isAbsolute(path) && existsSync(file) ? file : resolve(staticUI, "index.html") });
  });
  await page.addInitScript(() => {
    for (const key of Object.keys(localStorage)) if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
  });
  await page.route("**/api/v1/i18n/runtime/messages?**", async route => {
    const response = await route.fetch();
    const body = await response.json();
    const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
    for (const relative of [`../../../manifest/i18n/${locale}/plugin.json`, `../../../../idc-resource/manifest/i18n/${locale}/plugin.json`]) {
      const messages = JSON.parse(readFileSync(new URL(relative, import.meta.url), "utf8"));
      for (const [key, value] of Object.entries(messages)) {
        const segments = key.split(".");
        let target = body.data.messages;
        for (const segment of segments.slice(0, -1)) target = target[segment] ??= {};
        target[segments.at(-1)!] = value;
      }
    }
    await route.fulfill({ json: body });
  });
}
export async function installCustomerWorkspaceMock(
  page: Page,
  { channel = true, resource = true } = {},
) {
  const basic = await installCustomerManagementMock(page);
  const balances = [
    {
      accountTypeCode: "customer_balance",
      currencyCode: "CNY",
      statusCode: "active",
      balanceMinor: "20000",
      heldBalanceMinor: "3000",
      availableBalanceMinor: "17000",
      updatedAt: Date.now(),
    },
    {
      accountTypeCode: "customer_gift",
      currencyCode: "USD",
      statusCode: "active",
      balanceMinor: "3000",
      heldBalanceMinor: "0",
      availableBalanceMinor: "3000",
      updatedAt: Date.now(),
    },
  ];
  basic.listExtras = { balances };
  basic.overviewExtras = {
    balances,
    statistics: {
      currencies: [
        {
          currencyCode: "CNY",
          paidMinor: "12000",
          refundedMinor: "1500",
          unpaidMinor: "7000",
          unpaidCount: 2,
        },
        {
          currencyCode: "USD",
          paidMinor: "5000",
          refundedMinor: "0",
          unpaidMinor: "0",
          unpaidCount: 0,
        },
      ],
      tickets: { total: 4, processing: 2, replied: 1, closed: 1 },
      productCategories: [
        { productType: "cloud_server", pluginId: "idc-resource", total: 22 },
        { productType: "yunsu-ssl", pluginId: "yunsu-ssl", total: 1 },
      ],
      products: 23,
      expiring: 1,
      expired: 1,
    },
  };
  const state = {
    productPages: [] as URL[],
    callPages: [] as URL[],
    resets: [] as Record<string, unknown>[],
    metadata: [] as Record<string, unknown>[],
    quotes: [] as Record<string, unknown>[],
    checkouts: [] as Record<string, unknown>[],
    resetFailures: 0,
    callFailures: 0,
    productFailures: 0,
  };
  await page.route("**/api/v1/plugins/dynamic", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    body.data.list = body.data.list.filter(
      (p: { id: string }) => !["idc-channel", "idc-resource"].includes(p.id),
    );
    body.data.list.push(
      { id: "idc-channel", installed: 1, enabled: channel ? 1 : 0 },
      { id: "idc-resource", installed: 1, enabled: resource ? 1 : 0 },
    );
    await route.fulfill({ json: body });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/customers/*/products**",
    async (route) => {
      const url = new URL(route.request().url());
      state.productPages.push(url);
      if (state.productFailures-- > 0) {
        await route.fulfill({
          status: 500,
          json: { code: 51, message: "产品加载失败，请重试" },
        });
        return;
      }
      const current = Number(url.searchParams.get("page") || 1),
        size = Number(url.searchParams.get("pageSize") || 10);
      const items = Array.from({ length: 23 }, (_, i) => ({
        deliveryKind: i === 0 ? "server" : i === 1 ? "entitlement" : "pending",
        number: i === 0 ? "SVC_SERVER" : i === 1 ? "ENT_SSL" : `ITEM_${i}`,
        orderNo: "ORD_1",
        productNo: "PRD_1",
        nameZhCn: i === 1 ? "单域名SSL证书" : "云服务器",
        nameEnUs: i === 1 ? "SSL certificate" : "Cloud server",
        information: i === 1 ? "单域名" : "4核 / 8GB内存",
        informationEnUs: i === 1 ? "Single domain" : "4 cores / 8 GB RAM",
        amountMinor: "12000",
        currencyCode: "CNY",
        productType: i === 1 ? "yunsu-ssl" : "cloud_server",
        pluginId: i === 1 ? "yunsu-ssl" : "idc-resource",
        periodUnit: i === 1 ? "year" : "month",
        periodCount: 1,
        createdAt: 1786000000000,
        expiresAt: Date.now() + 864000000,
        status: "active",
      }));
      await route.fulfill({
        json: envelope({
          items: items.slice((current - 1) * size, current * size),
          total: items.length,
        }),
      });
    },
  );
  let tokenId = 42;
  await page.route(
    "**/x/idc-channel/api/v1/admin/customers/*/open-platform**",
    async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/secrets")) {
        const body = route.request().postDataJSON();
        state.resets.push(body);
        if (state.resetFailures-- > 0) {
          await route.fulfill({
            status: 409,
            json: { code: 51, message: "密钥已变化，请重新打开" },
          });
          return;
        }
        tokenId++;
        await route.fulfill({
          json: envelope({
            appId: "APP_CUSTOMER",
            secret: "new-secret-visible-once",
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/calls")) {
        state.callPages.push(url);
        if (state.callFailures-- > 0) {
          await route.fulfill({
            status: 500,
            json: { code: 51, message: "调用记录加载失败，请重试" },
          });
          return;
        }
        const current = Number(url.searchParams.get("page") || 1),
          size = Number(url.searchParams.get("pageSize") || 10);
        const rows = Array.from({ length: 23 }, (_, i) => ({
          id: i + 1,
          method: "GET",
          operation: "catalog.read",
          succeeded: i % 2 === 0,
          durationMillis: 15,
          createdAt: 1786000000000 + i,
        }));
        await route.fulfill({
          json: envelope({
            list: rows.slice((current - 1) * size, current * size),
            total: 23,
          }),
        });
        return;
      }
      await route.fulfill({
        json: envelope({
          authorized: true,
          appId: "APP_CUSTOMER",
          displayName: "API用户",
          status: "approved",
          tokenId,
          canReset: true,
          scopes: ["catalog.read", "capability.read"],
          sourceCidrs: ["127.0.0.0/8"],
          expiresAt: Date.now() + 86400000,
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/servers/SVC_SERVER/**",
    async (route) => {
      const url = new URL(route.request().url());
      const body =
        route.request().method() === "GET"
          ? {}
          : route.request().postDataJSON();
      if (url.pathname.endsWith("/metadata")) {
        if (route.request().method() === "PUT") state.metadata.push(body);
        await route.fulfill({
          json: envelope({
            metadata: {
              alias: "报表服务器",
              remark: "旧备注",
              version: state.metadata.length + 8,
            },
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/resize-options")) {
        await route.fulfill({
          json: envelope({
            revision: "revision-current",
            currencyCode: "CNY",
            groups: [
              {
                groupNo: "cpu",
                nameZhCN: "处理器",
                nameEnUS: "CPU",
                selectionMode: "single",
                minimumSelected: 1,
                maximumSelected: 1,
                options: [
                  {
                    optionNo: "OPT_4",
                    nameZhCN: "4核",
                    nameEnUS: "4 cores",
                    minimumQuantity: 1,
                    maximumQuantity: 1,
                    defaultQuantity: 1,
                  },
                  {
                    optionNo: "OPT_8",
                    nameZhCN: "8核",
                    nameEnUS: "8 cores",
                    minimumQuantity: 1,
                    maximumQuantity: 1,
                    defaultQuantity: 1,
                  },
                ],
              },
            ],
            currentSelections: [{ optionNo: "OPT_4", quantity: 1 }],
            rules: [],
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/checkout")) {
        state.checkouts.push(body);
        await route.fulfill({
          json: envelope({
            checkout: { orderNo: "ORDER_CHANGE", invoiceNo: "INVOICE_CHANGE" },
          }),
        });
        return;
      }
      state.quotes.push(body);
      await route.fulfill({
        json: envelope({
          quote: {
            quoteNo: "QUOTE_CHANGE",
            direction: "upgrade",
            currencyCode: "CNY",
            chargeAmountMinor: 2000,
            targetRecurringAmountMinor: 20000,
            effectiveAt: Date.now(),
            expiresAt: Date.now() + 900000,
          },
        }),
      });
    },
  );
  return state;
}
