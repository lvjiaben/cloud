import { readFileSync } from "node:fs";
import { extname, resolve } from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { serveChannelVbenRuntime } from "../support/channel-standalone";
import { ChannelVbenPage } from "../pages/ChannelVbenPage";

const messages = JSON.parse(
  readFileSync(new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url), "utf8"),
) as Record<string, string>;
const assetRoot = resolve(process.cwd(), "../../apps/lina-plugins/idc-channel/frontend/pages");

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

async function envelope(route: { fulfill: (options: { json: unknown }) => Promise<void> }, data: unknown) {
  await route.fulfill({ json: { code: 0, data, message: "success" } });
}

class ChannelManagementPage {
  constructor(readonly page: Page) {}
  get root() { return this.page.frameLocator('[data-testid="plugin-sandbox-frame"]').getByTestId("idc-channel-admin-page"); }
  async goto() {
    await this.page.goto("/admin/idc/channel", { waitUntil: "domcontentloaded" });
    await this.root.waitFor({ state: "visible" });
  }
  async showCapabilities() {
    await this.root.getByRole("tab", { name: "能力清单" }).click();
    return this.root.locator('section[data-panel="capabilities"]');
  }
  async createToken() {
    await this.root.getByTestId("channel-selector").selectOption("channel_primary");
    await this.root.getByRole("tab", { name: "令牌" }).click();
    await expect(this.root.locator('section[data-panel="tokens"] .vxe-pager')).toBeVisible();
    await this.root.locator('textarea[name="scopes"]').fill("catalog.read\norder.create");
    const create = this.root.getByRole("button", { name: "创建一次性令牌" });
    await create.scrollIntoViewIfNeeded();
    await expect(create).toBeInViewport();
    await create.click();
    return this.root.getByTestId("one-time-token");
  }
  async revokeToken() {
    await this.root.getByRole("button", { name: "撤销", exact: true }).click();
    return this.root.getByRole("status");
  }
}

test.describe("TC-3 下游渠道管理页面", () => {
  test("TC-3a: 能查看能力并创建、撤销一次性令牌", async ({ adminPage }) => {
    await serveChannelVbenRuntime(adminPage);
    await adminPage.route("**/api/v1/user/info", async (route) => {
      const response = await route.fetch();
      const payload = await response.json();
      payload.data ??= {};
      payload.data.permissions = ["idc:admin:channel:token"];
      await route.fulfill({ json: payload, response });
    });
    await adminPage.route("**/api/v1/i18n/runtime/locales**", async (route) => {
      await envelope(route, { enabled: true, locale: "zh-CN", items: [{ locale: "zh-CN", nativeName: "简体中文", name: "Simplified Chinese", direction: "ltr", isDefault: true }] });
    });
    await adminPage.route("**/api/v1/i18n/runtime/messages**", async (route) => {
      await envelope(route, { messages: nestedMessages(messages) });
    });
    await adminPage.route("**/api/v1/plugins/page-bridge/grant", async (route) => {
      await envelope(route, { grant: "channel-page-grant", ownerPluginId: "idc-channel", expiresAt: Date.now() + 120_000 });
    });
    await adminPage.route("**/api/v1/menus/all", async (route) => {
      await envelope(route, { list: [{ component: "BasicLayout", name: "IDC", path: "/idc", children: [{ component: "PluginSandboxView", name: "IDCChannel", path: "/idc/channel", meta: { title: "plugin.idc-channel.title", pluginSandboxSrc: "/x-assets/idc-channel/v0.1.0/index.html", pluginSurfaceCapabilities: ["channel.admin.v1"], pluginSurfaceHost: "admin", pluginSurfaceId: "idc-channel", pluginSurfaceKey: "channel", pluginSurfaceVersion: "v1" } }] }] });
    });
    await adminPage.route("**/x-assets/idc-channel/v0.1.0/**", async (route) => {
      const suffix = new URL(route.request().url()).pathname.split("/v0.1.0/")[1] ?? "";
      if (!/^[\w./-]+$/.test(suffix) || suffix.includes("..")) { await route.abort(); return; }
      const path = resolve(assetRoot, suffix);
      const contentType = extname(path) === ".html" ? "text/html; charset=utf-8" : "text/javascript; charset=utf-8";
      await route.fulfill({ body: readFileSync(path), contentType, status: 200 });
    });
    await adminPage.route("**/x/idc-channel/api/v1/admin/channel/capabilities", async (route) => {
      await envelope(route, { genericApi: true, adapters: { mofang_finance: "disabled_unverified", v10: "disabled_unverified", whmcs: "disabled_unverified" }, operations: { catalog: "enabled", quote: "enabled", order_create: "enabled" } });
    });
    await adminPage.route("**/x/idc-channel/api/v1/admin/channel/accounts", async (route) => {
      await envelope(route, { total: 1, page: 1, pageSize: 10, list: [{ channelId: "channel_primary", customerNo: "CUS_CHANNEL", displayName: "主渠道", statusCode: "approved", newSalesEnabled: true, updatedAt: Date.parse("2026-08-30T00:00:00Z") }] });
    });
    await adminPage.route("**/x/idc-channel/api/v1/admin/channel/tokens**", async (route) => {
      if (route.request().method() === "GET") {
        await envelope(route, { total: 1, page: 1, pageSize: 10, list: [{ id: 42, channelId: "channel_primary", scopes: ["catalog.read"], revokedAt: null, createdAt: Date.parse("2026-08-30T00:00:00Z") }] });
      } else {
        await envelope(route, { id: 42, token: "channel-token-once", expiresAt: null });
      }
    });
    await adminPage.route("**/x/idc-channel/api/v1/admin/channel/tokens/42", async (route) => {
      expect(route.request().method()).toBe("DELETE");
      await envelope(route, { id: 42, revoked: true });
    });

    const page = new ChannelManagementPage(adminPage);
    await page.goto();
    await expect(await page.showCapabilities()).toContainText("通用 API");
    await expect(await page.createToken()).toHaveText("channel-token-once");
    await expect(await page.revokeToken()).toContainText("令牌已撤销");
    await new ChannelVbenPage(adminPage).snapshot("channel-management");
  });
});
