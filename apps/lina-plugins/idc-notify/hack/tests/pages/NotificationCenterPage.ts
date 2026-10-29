import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

function artifactDirectory() {
  const cwd = process.cwd();
  const normalized = cwd.replaceAll("\\", "/");
  const repositoryRoot = normalized.endsWith("/hack/tests")
    ? resolve(cwd, "../..")
    : cwd;
  const now = new Date();
  const day = [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, "0"),
    String(now.getDate()).padStart(2, "0"),
  ].join("");
  return resolve(repositoryRoot, "temp", day);
}

export class NotificationCenterPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.locator(
      '[data-plugin-frontend-root="idc-notify"] .notify-workspace',
    );
  }

  async gotoChannels() {
    await this.goto("/idc/notifications/channels");
  }

  async gotoTemplates() {
    await this.goto("/idc/notifications/templates");
  }

  async gotoDeliveries() {
    await this.goto("/idc/notifications/deliveries");
  }

  channelCard(name: RegExp | string) {
    return this.pageRoot.locator("article.channel-card").filter({ hasText: name });
  }

  async openChannelConfiguration(name: RegExp | string) {
    await this.channelCard(name)
      .getByRole("button", { name: /开始配置|更换配置|Configure|Replace configuration/iu })
      .click();
    await this.pageRoot.locator(".channel-wizard").waitFor({ state: "visible" });
  }

  async openChannelTest(name: RegExp | string) {
    await this.channelCard(name)
      .getByRole("button", { name: /测试发送|Test send/iu })
      .click();
    await this.pageRoot.locator(".channel-wizard").waitFor({ state: "visible" });
  }

  channelWizard() {
    return this.pageRoot.locator(".channel-wizard");
  }


  async submitRealChannelTest() {
    return this.confirmNextDialog(() =>
      this.channelWizard()
        .getByRole("button", { name: /确认并真实发送一次|Confirm and send once/iu })
        .click(),
    );
  }

  async fillSMTPConfiguration(password: string) {
    const wizard = this.pageRoot.locator(".channel-wizard");
    await wizard.getByLabel(/邮件服务器地址|Mail server host/iu).fill("smtp.notify.example");
    await wizard.getByLabel(/登录账号|Username/iu).fill("mailer@example.com");
    await wizard.getByLabel(/登录密码|Password/iu).fill(password);
    await wizard.getByLabel(/发件邮箱|From address/iu).fill("notice@example.com");
    await wizard.getByLabel(/发件人名称|Sender name/iu).fill("Lina IDC");
  }

  async saveConfiguration() {
    await this.pageRoot
      .locator(".channel-wizard")
      .getByRole("button", { name: /安全保存配置|Save configuration securely/iu })
      .click();
  }

  async confirmNextDialog(action: () => Promise<unknown>) {
    const dialog = this.page.waitForEvent("dialog");
    const result = action();
    const opened = await dialog;
    const message = opened.message();
    await opened.accept();
    await result;
    return message;
  }

  async screenshot(name: string) {
    await waitForRouteReady(this.page, 15_000);
    const directory = artifactDirectory();
    await mkdir(directory, { recursive: true });
    const now = new Date();
    const time = [
      String(now.getHours()).padStart(2, "0"),
      String(now.getMinutes()).padStart(2, "0"),
      String(now.getSeconds()).padStart(2, "0"),
      String(now.getMilliseconds()).padStart(3, "0"),
    ].join("");
    const path = resolve(directory, `${time}-${name}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }

  private async goto(path: string) {
    await this.page.goto(workspacePath(path), { waitUntil: "domcontentloaded" });
    // Channel and template configuration now opens inside the system drawer.
    const configuration = this.page.getByTestId("system-configuration-center");
    await Promise.race([this.pageRoot.waitFor({state:"visible",timeout:15000}), configuration.waitFor({state:"visible",timeout:15000})]);
    if (!(await this.pageRoot.isVisible()) && (path.endsWith("/channels") || path.endsWith("/templates"))) {
      await configuration.getByRole("tab", {name: /短信与通知|Messaging/iu}).click();
      const title = path.endsWith("/channels") ? /短信与通知渠道|Messaging channels/iu : /通知模板|Notification templates/iu;
      await configuration.locator("article.configuration-item").filter({hasText:title}).getByRole("button",{name:/进入配置|Configure/iu}).click();
    }
    await this.pageRoot.waitFor({ state: "visible", timeout: 15_000 });
    await waitForRouteReady(this.page, 15_000);
    await this.pageRoot
      .getByText(/正在读取|Loading/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
  }
}
