import { expect, type Page } from "@playwright/test";

type Configuration = {
  plugins: Array<{ pluginId: string; name: string }>;
  selectedPlugin: string;
  common: Record<string, string>;
  config: Record<string, unknown>;
  configurations?: Record<string, Record<string, unknown>>;
};

export class PortalBrandPage {
  constructor(readonly page: Page) {}
  get root() {
    return this.page.locator('[data-plugin-frontend-root="idc-portal"]');
  }
  field(label: string) {
    return this.root.getByLabel(label, { exact: true });
  }
  button(name: string) {
    return this.root.getByRole("button", {
      name: new RegExp(`^${name.split("").join("\\s*")}$`),
    });
  }
  async expectActivePlugin(name: string) {
    const active = this.root.locator(".website-plugin").filter({
      has: this.page.locator(".ant-tag"),
    });
    await expect(active).toHaveCount(1);
    await expect(active).toContainText(name);
    await expect(active).toContainText("使用中");
  }
  async goto() {
    await this.page.goto("/admin/system/website-configuration");
  }
  async confirm() {
    await this.page.getByRole("button", { name: /^确\s*定$/ }).click();
  }
  async selectPlugin(name: string) {
    await this.root.getByRole("button", { name, exact: false }).click();
    await expect(this.button("保存并启用")).toBeEnabled();
  }
  async save() {
    const pending = this.page.waitForResponse((response) =>
      response.url().endsWith("/admin/site/configuration") && response.request().method() === "PUT",
    );
    await this.button("保存并启用").click();
    const payload = await (await pending).json();
    expect(payload.code).toBe(0);
    await expect(this.page.getByText("官网配置已生效", { exact: true }).last()).toBeVisible();
  }
  private async call(path: string, body?: unknown) {
    return this.page.evaluate(async ({ path, body }) => {
      const key = Object.keys(localStorage).find((key) => key.endsWith("-core-access"));
      const access = key ? JSON.parse(localStorage.getItem(key) ?? "{}") : {};
      const response = await fetch(path, {
        method: body === undefined ? "GET" : "PUT",
        headers: { Authorization: `Bearer ${access.accessToken}`, "Content-Type": "application/json" },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      });
      const payload = await response.json();
      if (!response.ok || payload.code !== 0) throw new Error(payload.message ?? "官网接口失败");
      return payload.data;
    }, { path, body });
  }
  async configuration(): Promise<Configuration> {
    return this.call("/x/idc-portal/api/v1/admin/site/configuration");
  }
  async restoreConfiguration(body: Omit<Configuration, "plugins" | "configurations">) {
    await this.call("/x/idc-portal/api/v1/admin/site/configuration", body);
  }
  async publicBrand() {
    return (await this.call("/x/idc-portal/api/v1/public/brand")).brand;
  }

}
