import { readFile } from "node:fs/promises";
import { extname, resolve } from "node:path";

import type { Page, Route } from "@playwright/test";
import { installIsolatedTableAssets } from './isolated-table-assets';

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: "OK" });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: "application/json",
    status: 200,
  });
}

const certificate = {
  availableActions: ["sync", "update-validation", "revalidate", "cancel"],
  entitlementNo: "ENT_SSL_ADMIN_0001",
  customerName: "示例客户",
  expiresAt: 1_810_000_000_000,
  issuedAt: 0,
  primaryDomain: "admin-example.test",
  productNameEnUs: "Domain validated certificate",
  productNameZhCn: "域名型证书",
  statusCode: "awaiting_validation",
  updatedAt: 1_778_000_100_000,
  validationMethod: "dns",
  validation: [
    {
      domain: "admin-example.test",
      host: "_dnsauth.admin-example.test",
      method: "dns",
      recordType: "CNAME",
      value: "validation.admin-example.test",
    },
  ],
};

export async function installSSLAdminSurfaceMock(page: Page) {
  await installIsolatedTableAssets(page);
  await page.route("**/api/v1/plugins/dynamic", async (route) => {
    await fulfill(route, {
      list: [
        {
          adminSurfaces: [
            {
              bridgeCapabilities: ["service-entitlements.admin.v1"],
              entry: "admin/certificates/index.html",
              entryUrl:
                "/x-assets/yunsu-ssl/v0.3.4/admin/certificates/index.html",
              icon: "lucide:badge-check",
              key: "certificates",
              pluginId: "yunsu-ssl",
              pluginType: "dynamic",
              routePath: "/plugins/yunsu-ssl/ssl-certificates",
              slot: "admin.resource-operations",
              status: "active",
              title: "SSL 证书管理",
              version: "v1",
            },
          ],
          enabled: 1,
          generation: 1,
          id: "yunsu-ssl",
          installed: 1,
          newBusinessState: "accepting",
          runtimeState: "normal",
          statusKey: "sys_plugin.status:yunsu-ssl",
          type: "dynamic",
          version: "v0.3.4",
        },
      ],
    });
  });
  await page.route("**/api/v1/menus/all", async (route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as { data?: { list?: unknown[] } };
    const list = Array.isArray(payload.data?.list) ? payload.data.list : [];
    await route.fulfill({
      response,
      json: {
        ...payload,
        data: {
          ...payload.data,
          list: [
            ...list,
            {
              component: "PluginSandboxView",
              id: -52_001,
              meta: {
                authority: "idc:admin:service-entitlement:list",
                icon: "lucide:badge-check",
                order: 99_999,
                pluginSandboxSrc:
                  "/x-assets/yunsu-ssl/v0.3.4/admin/certificates/index.html",
                pluginSurfaceCapabilities: ["service-entitlements.admin.v1"],
                pluginSurfaceHost: "admin",
                pluginSurfaceId: "yunsu-ssl",
                pluginSurfaceKey: "certificates",
                pluginSurfaceVer: "v1",
                title: "SSL 证书管理",
              },
              name: "YunsuSSLAdminE2E",
              parentId: 0,
              path: "/plugins/yunsu-ssl/ssl-certificates",
            },
          ],
        },
      },
    });
  });
  await page.route("**/api/v1/plugins/page-bridge/grant", async (route) => {
    await fulfill(route, {
      expiresAt: Date.now() + 120_000,
      grant: "signed-admin-owner-grant",
      ownerPluginId: "idc-core",
    });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/service-entitlements?**",
    async (route) => {
      await fulfill(route, { items: [certificate], nextCursor: "" });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/service-entitlements/ENT_SSL_ADMIN_0001",
    async (route) => {
      await fulfill(route, { item: certificate });
    },
  );
  const root = resolve(
    process.cwd(),
    "../../apps/lina-plugins/yunsu-ssl/frontend/dist",
  );
  await page.route("**/x-assets/yunsu-ssl/v0.3.4/**", async (route) => {
    const suffix =
      new URL(route.request().url()).pathname.split("/v0.3.4/")[1] ?? "";
    if (!/^[\w./-]+$/.test(suffix) || suffix.includes("..")) {
      await route.abort();
      return;
    }
    const path = resolve(root, suffix);
    await route.fulfill({
      body: await readFile(path),
      contentType:
        extname(path) === ".html"
          ? "text/html; charset=utf-8"
          : extname(path) === '.css' ? 'text/css; charset=utf-8' : "text/javascript; charset=utf-8",
      headers: { "Access-Control-Allow-Origin": "*", "Cache-Control": "public, max-age=31536000, immutable" },
      status: 200,
    });
  });
}
