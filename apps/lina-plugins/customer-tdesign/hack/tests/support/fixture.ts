// This independently distributable plugin owns its fixture and does not import host code.
import { test as base, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { extname } from "node:path";
export const resourceSurface = {
  pluginId: "idc-resource",
  pluginType: "source",
  key: "purchased-services",
  version: "v1",
  title: "服务器",
  slot: "customer.products",
  entry: "customer/index.html",
  bridgeCapabilities: [
    "purchased-services.customer.v1",
    "native-server-facts.customer.v1",
  ],
};
export const sslSurface = {
  pluginId: "yunsu-ssl",
  pluginType: "dynamic",
  key: "certificates",
  version: "v1",
  title: "SSL 证书",
  slot: "customer.products",
  entry: "customer/certificates/index.html",
  entryUrl: "/x-assets/yunsu-ssl/v0.3.8/customer/certificates/index.html",
  bridgeCapabilities: [
    "service-entitlements.customer.v1",
    "purchased-services.customer.v1",
  ],
};
export async function serveServicePageAssets(page: Page) {
  for (const [plugin, version, base, mount] of [
    ["idc-resource", "v0.1.77", "idc-resource/frontend/dist/customer/", "customer/"],
    ["yunsu-ssl", "v0.3.8", "yunsu-ssl/frontend/dist/", ""],
    ["vben-adapter", "v0.1.0", "vben-adapter/frontend/dist/", ""],
  ]) {
    await page.route(`**/x-assets/${plugin}/${version}/**`, async (route) => {
      const path = new URL(route.request().url()).pathname.split(
        `/x-assets/${plugin}/${version}/`,
      )[1];
      if (!path?.startsWith(mount) || path.includes("..")) return route.abort();
      const file = fileURLToPath(
        new URL(
          `../../../../${base}${path.slice(mount.length)}`,
          import.meta.url,
        ),
      );
      const content = await readFile(file);
      const ext = extname(file);
      return route.fulfill({
        body: content,
        contentType:
          (
            {
              ".html": "text/html",
              ".css": "text/css",
              ".js": "text/javascript",
              ".woff2": "font/woff2",
              ".woff": "font/woff",
              ".svg": "image/svg+xml",
            } as Record<string, string>
          )[ext] || "application/octet-stream",
        headers: { "access-control-allow-origin": "*" },
      });
    });
  }
}

export { expect };
export type { Page };
export const menus = [
  ["总览", "/account"],
  ["购买服务", "/account/products/catalog"],
  ["订单", "/account/orders"],
  ["账单", "/account/billing"],
  ["续费管理", "/account/billing/renewals"],
  ["工单", "/account/support"],
  ["账户资料", "/account/profile"],
].map(([nameZhCn, targetUrl], index) => ({
  directoryNo: `menu-${index}`,
  nameZhCn,
  nameEnUs: nameZhCn,
  targetUrl,
  navigationType: "system",
  sortOrder: index,
  serviceCount: 0,
}));
menus.push({
  directoryNo: "cloud",
  nameZhCn: "云服务器",
  nameEnUs: "Cloud servers",
  targetUrl: "",
  navigationType: "product",
  sortOrder: 1,
  serviceCount: 3,
});
export const now = 1792640000000;
export const invoice = {
  invoiceNo: "INV-TEST-001",
  orderNo: "ORD-TEST-001",
  totalAmountMinor: "12800",
  outstandingAmountMinor: "12800",
  paidAmountMinor: "0",
  refundedAmountMinor: "0",
  refundableAmountMinor: "0",
  waivedAmountMinor: "0",
  currencyCode: "CNY",
  statusCode: "open",
  canPayFromBalance: true,
  canMerge: false,
  itemCount: 1,
  createdAt: now,
  paymentDueAt: now + 86400000,
  sourceCode: "purchase",
};
export const order = {
  orderNo: "ORD-TEST-001",
  totalAmountMinor: "12800",
  currencyCode: "CNY",
  statusCode: "awaiting_payment",
  canCancel: true,
  canPreparePayment: true,
  createdAt: now,
  itemCount: 1,
  paymentDueAt: now + 86400000,
  requiresContract: false,
  riskDecisionCode: "allow",
};
export const test = base.extend<{ consolePage: Page; apiPage: Page }>({
  consolePage: async ({ apiPage: page }, use) => {
    await page.addInitScript(
      () =>
        window === window.top &&
        sessionStorage.setItem(
          "customer-tdesign.session",
          JSON.stringify({ accessToken: "isolated-ui-fixture" }),
        ),
    );
    await use(page);
  },
  apiPage: async ({ page }, use) => {
    await serveServicePageAssets(page);
    await page.route("**/api/v1/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      let data: unknown;
      if (path.endsWith("/initial-password/status")) data = { required: false };
      else if (path.endsWith("/user/info")) data = { username: "验收账户" };
      else if (
        path.endsWith("/commercial-operations") ||
        path.endsWith("/accessory-billing")
      )
        data = { items: [], nextCursor: "" };
      else if (
        /\/native-servers\/[^/]+\/operations$/.test(path) &&
        route.request().method() === "GET"
      )
        data = { items: [], total: 0 };
      else if (path.endsWith("/user/profile"))
        data = {
          id: 42,
          username: "验收账户",
          nickname: "测试客户",
          email: "test@example.com",
          phone: "13800138000",
          avatar: "",
        };
      else if (path.endsWith("/plugins/dynamic"))
        data = {
          list: [
            {
              id: "idc-resource",
              version: "v0.1.77",
              installed: 1,
              enabled: 1,
              customerSurfaces: [resourceSurface],
            },
            { id: "idc-compliance", installed: 1, enabled: 1 },
            { id: "idc-notify", installed: 1, enabled: 1 },
          ],
        };
      else if (path.endsWith("/public/customer-center/configuration"))
        data = {
          active: true,
          selectedPlugin: "customer-tdesign",
          config: {
            websiteurl: "https://website.example",
            websitecatalogpath: "/services.html",
            websiteconfigurepath: "/configure.html?product={product}",
            websitecartpath: "/cart.html",
          },
          common: { siteName: "云服务", siteTitle: "云服务 · 用户中心" },
        };
      else if (path.endsWith("/customer/external-menus")) data = { items: [] };
      else if (path.endsWith("/account/product-directories"))
        data = { items: menus, total: menus.length };
      else if (path.endsWith("/account/native-server-facts"))
        data = { items: [] };
      else if (path.endsWith("/product-directories/all/services"))
        data = { items: [], total: 0 };
      else if (path.endsWith("/product-directories/cloud/services"))
        data = {
          items: [],
          total: 3,
          providers: [{ pluginId: "idc-resource", total: 3 }],
        };
      else if (path.endsWith("/billing/overview"))
        data = {
          overview: {
            balances: [
              {
                accountTypeCode: "customer_balance",
                currencyCode: "CNY",
                availableBalanceMinor: "52000",
                heldBalanceMinor: "0",
              },
            ],
            openInvoiceCount: 1,
            openReceivables: [
              { currencyCode: "CNY", outstandingMinor: "12800" },
            ],
            rechargeAvailable: true,
            automaticRenewalEnabled: 0,
          },
        };
      else if (path.endsWith("/billing/renewals"))
        data = { entitlements: [], nextCursor: "" };
      else if (path.endsWith("/billing/renewal-entitlements"))
        data = { entitlements: [], nextCursor: "" };
      else if (path.endsWith("/commerce/orders"))
        data = { orders: [order], nextCursor: "" };
      else if (path.endsWith("/commerce/orders/ORD-TEST-001"))
        data = {
          order: {
            summary: order,
            invoice: { invoiceNo: invoice.invoiceNo },
            items: [],
            deliveries: [],
            reviews: [],
            serviceEntitlements: [],
          },
        };
      else if (path.endsWith("/billing/invoices/INV-TEST-001"))
        data = {
          detail: {
            invoice,
            items: [
              {
                invoiceItemNo: "ITEM-1",
                descriptionZhCN: "云服务器 · 基础型",
                subtotalMinor: "12800",
                discountMinor: "0",
                totalAmountMinor: "12800",
              },
            ],
            nextItemCursor: "",
          },
        };
      else if (path.endsWith("/billing/invoices"))
        data = { invoices: [invoice], nextCursor: "" };
      else if (path.endsWith("/account/payment-channels"))
        data = { channels: [] };
      else if (path.endsWith("/support/tickets"))
        data = { items: [], total: 0 };
      else
        return route.fulfill({
          status: 404,
          json: { code: 404, message: "验收场景未配置此接口" },
        });
      await route.fulfill({ json: { code: 0, data } });
    });
    await use(page);
  },
});
