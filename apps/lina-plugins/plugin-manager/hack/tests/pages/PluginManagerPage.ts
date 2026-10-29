import { mkdir } from "node:fs/promises";
import path from "node:path";
import { type Page } from "@playwright/test";

export class PluginManagerPage {
  constructor(readonly page: Page) {}
  get root() {
    return this.page.getByTestId("plugin-manager-page");
  }
  get modal() {
    return this.page.getByTestId("plugin-action-modal");
  }
  get guide() {
    return this.page.getByTestId("plugin-developer-guide");
  }
  async openGuide() {
    await this.root.getByTestId("plugin-developer-guide-trigger").click();
    await this.guide.waitFor();
  }
  async expandGuide(title: string) {
    await this.guide.getByText(title, { exact: true }).click();
  }
  async closeGuide() {
    await this.page.keyboard.press("Escape");
    await this.guide.waitFor({ state: "hidden" });
  }
  get navigationEntry() {
    return this.page.getByRole("menu").getByText("插件管理", { exact: true });
  }
  async openFromSettings() {
    await this.page
      .getByRole("menu")
      .getByText("系统设置", { exact: true })
      .click();
    await this.navigationEntry.click();
    await this.root.waitFor();
  }
  get confirm() {
    return this.page
      .getByRole("button", {
        name: /^(安\s*装|更\s*新|卸\s*载|启\s*用|停\s*用|上传并安装)$/,
        exact: true,
      })
      .last();
  }
  async goto() {
    await this.page.goto("/extension/plugin-manager");
    await this.root.waitFor();
  }
  async tab(name: string) {
    await this.root.getByRole("tab", { name, exact: true }).click();
  }
  identity(id: string) {
    return this.root.getByTestId(`plugin-identity-${id}`);
  }
  async actionWidths(id: string) {
    return this.root
      .getByTestId(`plugin-actions-${id}`)
      .evaluate((element) => ({
        content: element.getBoundingClientRect().width,
        column: element.closest("td")!.getBoundingClientRect().width,
      }));
  }
  async action(mode: string, id: string) {
    await this.root.getByTestId(`plugin-${mode}-${id}`).click();
    if (mode === "install" || mode === "uninstall")
      await this.popconfirm.waitFor();
    else await this.modal.waitFor();
  }
  get popconfirm() {
    return this.page.locator(".ant-popconfirm:visible");
  }
  async submit() {
    if (await this.popconfirm.isVisible())
      await this.popconfirm
        .getByRole("button", { name: /确\s*定|确\s*认/ })
        .click();
    else await this.confirm.click();
  }
  async review() {
    await this.page.getByTestId("plugin-authorization-confirm").check();
  }
  async search(id: string, name: string) {
    await this.root.locator("form input").nth(0).fill(id);
    await this.root.locator("form input").nth(1).fill(name);
    await this.root
      .getByRole("button", { name: /搜\s*索/, exact: true })
      .click();
  }
  async selectPackage() {
    await this.root.getByTestId("plugin-package-install").click();
    await this.page.locator('input[type="file"]').setInputFiles({
      name: "fixture-tools.wasm",
      mimeType: "application/wasm",
      buffer: Buffer.from("wasm-test-fixture"),
    });
  }
  async toggleStatus(id: string) {
    await this.root.getByTestId(`plugin-status-${id}`).click();
  }
  async stopBusiness(id: string) {
    await this.root.getByTestId(`plugin-business-${id}`).click();
    await this.page
      .getByRole("button", { name: /确\s*定|确\s*认/ })
      .last()
      .click();
  }
  async screenshot(label: string) {
    await this.page
      .locator(".ant-notification-notice")
      .first()
      .waitFor({ state: "hidden" });
    const dir =
      process.env.E2E_OUTPUT_DIR ||
      path.resolve(
        "temp",
        new Date().toISOString().slice(0, 10).replaceAll("-", ""),
      );
    await mkdir(dir, { recursive: true });
    await this.page.screenshot({
      path: path.join(dir, `${Date.now()}-plugin-manager-${label}.png`),
      fullPage: false,
      animations: "disabled",
    });
  }
}
