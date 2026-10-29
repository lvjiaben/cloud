import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { expect, type Page } from "@host-tests/support/playwright";
import { config, workspacePath } from "@host-tests/fixtures/config";
import { LoginPage } from "@host-tests/pages/LoginPage";

const hostSourceURL = (path: string) => workspacePath(`/@fs/${resolve(import.meta.dirname, "../../../../../lina-vben", path).replaceAll("\\", "/")}`);

export class AccessoryRenewalPage {
  constructor(readonly page: Page) {}
  get trigger() { return this.content.getByRole("heading"); }
  get headerTrigger() { return this.page.getByTestId("accessory-renewals-trigger"); }
  get content() { return this.page.getByTestId("accessory-renewals-content"); }
  get syncError() { return this.page.getByTestId("accessory-renewals-sync-error"); }
  get listError() { return this.page.getByTestId("accessory-renewals-list-error"); }

  async login() {
    const login = new LoginPage(this.page);
    await login.goto();
    await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
    await this.page.waitForLoadState("domcontentloaded");
  }
  async open() {
    const menu = this.page.getByRole("menu");
    const entry = menu.getByText(/^(附属资源续费待办|Accessory Renewal Work Items)$/);
    if (!await entry.isVisible()) await menu.getByText(/^(服务器与资源|Servers and Resources)$/).click();
    await entry.click();
    await expect(this.content).toBeVisible();
    await expect.poll(async () => {
      const box = await this.content.boundingBox();
      return box ? box.x + box.width <= this.page.viewportSize()!.width + 1 : false;
    }).toBe(true);
  }
  async refresh() { await this.page.getByTestId("accessory-renewals-refresh").click(); }
  async narrow() { await this.page.setViewportSize({ width: 390, height: 844 }); }
  async close() { await this.page.getByRole("menu").getByText(/^(运营数据|Operations Data)$/).click(); await expect(this.content).toBeHidden(); }
  async nextPage() { await this.content.locator(".vxe-pager--next-btn").click(); }
  async pageSize50() {
    await this.content.locator(".vxe-pager--sizes").click();
    await this.page.getByText("50条/页", { exact: true }).click();
  }
  async viewInvoice(workItemNo: string) { await this.page.getByTestId(`accessory-renewal-invoice-${workItemNo}`).click(); }

  // Only used by the isolated source-runtime tests. Public store actions let
  // these tests exercise an in-place permission/session change without forging
  // server credentials or rewriting encrypted application storage.
  async changeScope(kind: "permissions" | "logout", codes: string[] = []) {
    await this.page.evaluate(async ({ kind, codes, moduleURL }) => {
      const stores = await import(/* @vite-ignore */ moduleURL);
      const access = stores.useAccessStore();
      if (kind === "logout") access.setAccessToken(null);
      else access.setAccessCodes(codes);
    }, { kind, codes, moduleURL: hostSourceURL("packages/stores/src/index.ts") });
  }
  async locale(locale: "en-US" | "zh-CN") {
    await this.page.evaluate(async ({ locale, preferencesURL, localesURL }) => {
      const preferences = await import(/* @vite-ignore */ preferencesURL);
      const locales = await import(/* @vite-ignore */ localesURL);
      preferences.updatePreferences({ app: { locale } });
      await locales.loadLocaleMessages(locale);
    }, { locale,
      preferencesURL: hostSourceURL("packages/preferences/src/index.ts"),
      localesURL: hostSourceURL("packages/locales/src/index.ts"),
    });
  }
  async renewalMessages(invoiceNo: string) {
    return this.page.evaluate(async ({ invoiceNo, apiURL, storeURL }) => {
      const api = await import(/* @vite-ignore */ apiURL);
      const { useMessageStore } = await import(/* @vite-ignore */ storeURL);
      const store = useMessageStore();
      await store.fetchMessages(1, 20);
      const candidates = store.messages.filter((message: { title: string }) =>
        ["附属资源续费待处理", "Accessory renewals need attention"].includes(message.title));
      const details = await Promise.all(candidates.map((message: { id: number }) => api.messageInfo(message.id)));
      return details.filter((message: { content: string }) => message.content.includes(invoiceNo))
        .map((message: { id: number; title: string; content: string; sourceId: string }) => ({
          id: message.id, title: message.title, content: message.content, sourceId: message.sourceId,
        }));
    }, { invoiceNo,
      apiURL: hostSourceURL("apps/web-antd/src/api/system/message/index.ts"),
      storeURL: hostSourceURL("apps/web-antd/src/store/message.ts"),
    });
  }
  async openNotifications() { await this.page.locator(".bell-button").click(); }
  async screenshot(name: string) {
    await expect(this.page.locator(".ant-notification-notice")).toHaveCount(0);
    await expect(this.content.locator(".vxe-loading.is--visible")).toHaveCount(0);
    const now = new Date();
    const day = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, "0")}${String(now.getDate()).padStart(2, "0")}`;
    const directory = resolve("temp", day);
    await mkdir(directory, { recursive: true });
    const time = now.toISOString().slice(11, 23).replaceAll(":", "").replace(".", "");
    await this.page.screenshot({ path: resolve(directory, `${time}-renewal-${name}.png`), fullPage: false, animations: "disabled" });
  }
}
