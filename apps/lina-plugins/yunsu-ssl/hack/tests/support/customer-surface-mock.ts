import { readFile } from "node:fs/promises";
import { extname, resolve } from "node:path";

import type { Page, Route } from "@playwright/test";
import { installIsolatedTableAssets } from './isolated-table-assets';

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: "OK" });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ body: envelope(data), contentType: "application/json", status: 200 });
}

const certificate = {
  availableActions: ["sync", "update-validation", "revalidate", "cancel"],
  entitlementNo: "ENT_SSL_00000001", customerName: "当前客户", expiresAt: 1_810_000_000_000, issuedAt: 0,
  primaryDomain: "example.test", productNameEnUs: "Domain validated certificate", productNameZhCn: "域名型证书",
  statusCode: "awaiting_validation", updatedAt: 1_778_000_100_000, validationMethod: "dns",
  validation: [{ domain: "example.test", host: "_dnsauth.example.test", method: "dns", recordType: "CNAME", value: "validation.example.test" }],
};

export async function installSSLCustomerSurfaceMock(page: Page, options: { locale?: "en-US" | "zh-CN"; slowAssetMs?: number; pagedRecords?: boolean; failFirstRuntime?: boolean } = {}) {
  let releaseRuntimeFailure = () => {};
  const firstListResponse = new Promise<void>((resolveResponse) => { releaseRuntimeFailure = resolveResponse; });
  // Inject after the first bridge list response, not before initial host context.
  // This exercises the visible Retry path rather than initialization recovery.
  const isolated = await installIsolatedTableAssets(page, {
    failFirstRuntime: options.failFirstRuntime,
    beforeFailure: () => firstListResponse,
  });
  let assetRequests = 0;
  let coldAssetReads = 0;
  const assetCache = new Map<string, Buffer>();
  const actionRequests: Array<{ body: unknown; path: string }> = [];
  const downloadRequests: string[] = [];
  const listRequests: Array<{cursor:string;limit:string}> = [];
  let currentCertificate = { ...certificate, validation: certificate.validation.map((item) => ({ ...item })) };
  let enabled = true;
  let newBusinessState: "accepting" | "stopped" = "accepting";
  const locale = options.locale ?? "zh-CN";
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/config/public/frontend")) {
      await fulfill(route, { app: { name: "IDC 客户中心" }, auth: {}, ui: {}, workspace: { basePath: "/" } });
      return;
    }
    if (path.endsWith("/i18n/runtime/locales")) {
      await fulfill(route, {
        enabled: true,
        items: [
          { isDefault: locale === "zh-CN", locale: "zh-CN", name: "简体中文", nativeName: "简体中文" },
          { isDefault: locale === "en-US", locale: "en-US", name: "English", nativeName: "English" },
        ],
        locale,
      });
      return;
    }
    if (path.endsWith("/i18n/runtime/messages")) {
      await fulfill(route, { messages: {} });
      return;
    }
    if (path.endsWith("/auth/login")) {
      await fulfill(route, { accessToken: "customer-e2e-token", refreshToken: "customer-e2e-refresh", tenants: [] });
      return;
    }
    if (path.endsWith("/auth/refresh")) {
      await fulfill(route, { accessToken: "customer-e2e-token", refreshToken: "customer-e2e-refresh" });
      return;
    }
    if (path.endsWith("/user/info")) {
      await fulfill(route, {
        avatar: "",
        homePath: "/account",
        id: 1001,
        menus: [],
        permissions: [],
        realName: "测试客户",
        roles: [],
        username: "customer",
      });
      return;
    }
    if (path.endsWith("/customer/activation/initial-password")) {
      await fulfill(route, { required: false });
      return;
    }
    if (path.endsWith("/menus/all")) {
      await fulfill(route, { list: [] });
      return;
    }
    await fulfill(route, {});
  });
  await page.route("**/api/v1/plugins/dynamic", async (route) => {
    await fulfill(route, { list: [{
      customerSurfaces: [{ bridgeCapabilities: ["service-entitlements.customer.v1"], entry: "customer/certificates/index.html", entryUrl: "/x-assets/yunsu-ssl/v0.3.4/customer/certificates/index.html", icon: "lucide:badge-check", key: "certificates", pluginId: "yunsu-ssl", pluginType: "dynamic", routePath: "/account/plugins/yunsu-ssl/ssl-certificates", slot: "customer.products", status: "active", title: options.locale === "en-US" ? "SSL Certificates" : "SSL 证书", version: "v1" }],
      enabled: enabled ? 1 : 0, generation: 1, id: "yunsu-ssl", installed: 1, newBusinessState, runtimeState: "normal", statusKey: "sys_plugin.status:yunsu-ssl", type: "dynamic", version: "v0.3.4",
    }] });
  });
  await page.route("**/api/v1/plugins/page-bridge/grant", async (route) => {
    await fulfill(route, { expiresAt: Date.now() + 120_000, grant: "signed-owner-grant", ownerPluginId: "idc-core" });
  });
  await page.route("**/x/idc-core/api/v1/account/service-entitlements?**", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    listRequests.push({cursor:params.get('cursor') ?? '',limit:params.get('limit') ?? ''});
    const second = params.get('cursor') === 'ssl-second-page';
    const rows = options.pagedRecords ? Array.from({length:30},(_,index) => {
      const number = index + (second ? 30 : 0);
      return number === 0 ? currentCertificate : {...currentCertificate,entitlementNo:`ENT_SSL_DEMO_${number}`,primaryDomain:`ssl-${number}.example.test`,statusCode:number % 2 ? 'issued' : 'awaiting_validation',availableActions:number % 2 ? ['download'] : ['revalidate']};
    }) : [currentCertificate];
    await fulfill(route, { items: rows, nextCursor: options.pagedRecords && !second ? 'ssl-second-page' : '' });
    releaseRuntimeFailure();
  });
  await page.route("**/x/idc-core/api/v1/account/service-entitlements/ENT_SSL_00000001", async (route) => {
    await fulfill(route, { item: currentCertificate });
  });
  await page.route("**/x/idc-core/api/v1/account/service-entitlements/ENT_SSL_00000001/actions/*", async (route) => {
    actionRequests.push({ body: route.request().postDataJSON(), path: new URL(route.request().url()).pathname });
    await fulfill(route, { actionKey: route.request().url().split("/").at(-1), status: "queued", taskNo: "TASK_SSL_1" });
  });
  await page.route("**/x/idc-core/api/v1/account/service-entitlements/ENT_SSL_00000001/downloads/certificate/tickets", async (route) => {
    downloadRequests.push(new URL(route.request().url()).pathname);
    await fulfill(route, { expiresAt: Date.now() + 60_000, fileName: "example-test.zip", ticket: "A".repeat(40) });
  });
  await page.route(`**/x/idc-core/api/v1/account/service-downloads/${"A".repeat(40)}`, async (route) => {
    downloadRequests.push(new URL(route.request().url()).pathname);
    await route.fulfill({ body: "certificate archive", contentType: "application/zip", status: 200 });
  });
  const root = resolve(process.cwd(), "../../apps/lina-plugins/yunsu-ssl/frontend/dist");
  await page.route("**/x-assets/yunsu-ssl/v0.3.4/**", async (route) => {
    assetRequests += 1;
    const suffix = new URL(route.request().url()).pathname.split("/v0.3.4/")[1] ?? "";
    if (!/^[\w./-]+$/.test(suffix) || suffix.includes("..")) { await route.abort(); return; }
    if (suffix.endsWith("index.html") && options.slowAssetMs) await new Promise((resolveDelay) => setTimeout(resolveDelay, options.slowAssetMs));
    const path = resolve(root, suffix);
    const contentType = extname(path) === ".html" ? "text/html; charset=utf-8" : extname(path) === '.css' ? 'text/css; charset=utf-8' : "text/javascript; charset=utf-8";
    let body = assetCache.get(path);
    if (!body) { body = await readFile(path); assetCache.set(path, body); coldAssetReads += 1; }
    await route.fulfill({ body, contentType, headers: { "Access-Control-Allow-Origin": "*", "Cache-Control": "public, max-age=31536000, immutable" }, status: 200 });
  });
  return {
    runtimeRequests: isolated.runtimeRequests,
    assetRequests: () => assetRequests,
    coldAssetReads: () => coldAssetReads,
    actionRequests: () => [...actionRequests],
    downloadRequests: () => [...downloadRequests],
    listRequests: () => [...listRequests],
    disable: () => { enabled = false; },
    issueCertificate: () => {
      currentCertificate = { ...currentCertificate, availableActions: ["sync", "download"], issuedAt: 1_779_000_000_000, statusCode: "issued" };
    },
    stopNewBusiness: () => { newBusinessState = "stopped"; },
  };
}
