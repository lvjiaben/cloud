import type { Page } from "@playwright/test";

export class SSLCustomerPage {
  constructor(readonly page: Page) {}

  async login() {
    const baseURL =
      process.env.E2E_CUSTOMER_BASE_URL ??
      "http://127.0.0.1:6032";
    await this.page.goto(`${baseURL.replace(/\/$/u, "")}/auth/login`);
    const inputs = this.page.locator("input");
    await inputs.nth(0).fill("customer");
    await inputs.nth(1).fill("customer-password");
    await this.page.getByRole("button", { name: "login" }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open(expectRuntimeFailure = false) {
    const group = this.page
      .locator("li.vben-sub-menu")
      .filter({ hasText: /产品与服务|Products & Services/u })
      .first();
    await group.waitFor({ state: "visible", timeout: 15_000 });
    // Customer desktop groups are fixed headings, not expandable buttons.
    await group
      .getByText(/SSL 证书|SSL Certificates/u, { exact: true })
      .click();
    await this.page.waitForURL(
      /\/account\/plugins\/yunsu-ssl\/ssl-certificates/u,
    );
    if (expectRuntimeFailure) {
      await this.frame().getByTestId('ssl-error').waitFor({state:'visible',timeout:10_000});
    } else {
      await this.frame().getByText("example.test", { exact: true }).waitFor({timeout:10_000});
    }
    const routeOverlay = this.page.locator(".bg-overlay-content").first();
    await routeOverlay
      .waitFor({ state: "visible", timeout: 3_000 })
      .catch(() => undefined);
    await routeOverlay.waitFor({ state: "hidden", timeout: 10_000 });
    await this.page
      .getByTestId("plugin-sandbox-skeleton")
      .waitFor({ state: "detached", timeout: 10_000 });
  }

  frame() {
    return this.page.frameLocator(
      'iframe[src="/x-assets/yunsu-ssl/v0.3.4/customer/certificates/index.html"]',
    );
  }

  async retry() {
    await this.frame().getByTestId('ssl-retry').click();
  }

  detailDialog() {
    return this.frame()
      .getByRole("dialog")
      .filter({ hasText: /证书详情|Certificate details/u });
  }

  async viewDetails() {
    await this.frame()
      .getByRole("button", { name: /查看详情|View details/u })
      .first()
      .click();
    await this.detailDialog().waitFor();
  }

  async changeValidationToFile() {
    await this.detailDialog()
      .getByRole("button", { name: /切换验证方式|Change validation method/u })
      .click();
    const dialog = this.frame()
      .getByRole("dialog")
      .filter({ hasText: /切换验证方式|Change validation method/u });
    await dialog.getByRole("combobox").selectOption("file");
    await dialog.getByRole("button", { name: /保存|Save/u }).click();
  }

  async revalidate() {
    await this.detailDialog()
      .getByRole("button", { name: /重新验证|Revalidate/u })
      .click();
  }

  async cancelApplication() {
    this.page.once("dialog", async (dialog) => dialog.accept());
    await this.detailDialog()
      .getByRole("button", { name: /取消申请|Cancel application/u })
      .click();
  }

  async downloadCertificate() {
    const download = this.page.waitForEvent("download");
    await this.frame()
      .getByRole("button", { name: /下载证书|Download certificate/u })
      .first()
      .click();
    return download;
  }
}
