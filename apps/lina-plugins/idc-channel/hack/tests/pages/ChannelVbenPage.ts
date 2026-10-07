import type { Page } from "@playwright/test";
export class ChannelVbenPage {
  constructor(readonly page: Page) {}
  get root() {
    return this.page.getByTestId("idc-channel-admin-page");
  }
  get notice() {
    return this.root.getByRole("status");
  }
  get secret() {
    return this.root.getByTestId("one-time-token");
  }
  async goto() {
    await this.page.goto("/admin/idc/channel", {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
  }
  async selectChannel(label = "主渠道 · channel_primary") {
    await this.root.getByTestId("channel-selector").click();
    await this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: label })
      .click();
  }
  async tab(label: string) {
    await this.root.getByRole("tab", { name: label, exact: true }).click();
  }
  panel(key: string) {
    return this.root.locator('[data-panel="' + key + '"]:not(button)');
  }
  async createChannel() {
    await this.root.locator('input[name="channelId"]').fill("new_channel");
    await this.root.locator('input[name="customerNo"]').fill("PUBLIC-101");
    await this.root.locator('input[name="displayName"]').fill("新渠道");
    await this.root.getByTestId("save-channel").click();
  }
  async status(label: string) {
    await this.root.getByRole("button", { name: label, exact: true }).click();
  }
  async createToken() {
    await this.root
      .locator('textarea[name="scopes"]')
      .fill("catalog.read\norder.create");
    await this.root
      .getByRole("button", { name: "创建一次性令牌", exact: true })
      .click();
  }
  async revokeToken() {
    await this.panel("tokens")
      .getByRole("button", { name: "撤销", exact: true })
      .first()
      .click();
  }
  async publish() {
    const root = this.panel("publications");
    for (const [name, value] of Object.entries({
      productVersionId: "101",
      publicSlug: "server-basic",
      configurationRevision: "r1",
      billingTermToken: "monthly",
    }))
      await root.locator('input[name="' + name + '"]').fill(value);
    await root.getByTestId("add-publication-value").click();
    await root.locator('input[name="fieldToken"]').fill("cpu");
    await root.locator('input[name="fieldValue"]').fill("4");
    await root.getByTestId("save-publication").click();
  }
  async nextPage(panel: string) {
    await this.panel(panel).locator(".vxe-pager--next-btn").first().click();
  }
  grid(panel: string, index = 0) {
    return this.panel(panel).locator(".vxe-grid").nth(index);
  }
  async numberedPage(panel: string, number: number, index = 0) {
    await this.grid(panel, index)
      .locator(".vxe-pager--num-btn")
      .filter({ hasText: new RegExp(`^${number}$`, "u") })
      .click();
  }
  async pageSize(panel: string, size: number, index = 0) {
    await this.grid(panel, index).locator(".vxe-pager--sizes").click();
    await this.page
      .locator(".vxe-select-option")
      .filter({ hasText: new RegExp(`^${size}\\s*条/页$`, "u") })
      .click();
  }
  async refresh() {
    await this.root.getByTestId("refresh-channel").click();
  }
  async selectStandaloneChannel() {
    await this.root
      .getByTestId("channel-selector")
      .selectOption("channel_primary");
  }
  async loadStandaloneCatalog() {
    const root = this.page.getByTestId("idc-channel-page");
    await root
      .locator('input[name="catalogChannelId"]')
      .fill("channel_primary");
    await root
      .getByRole("button", { name: "读取商品目录", exact: true })
      .click();
  }
  async scrollStandaloneTokens() {
    await this.page.getByTestId("channel-token-list").scrollIntoViewIfNeeded();
  }
  async snapshot(name: string) {
    await this.page.screenshot({
      path: new URL(
        "../../../../../../temp/20260904/" +
          new Date().toISOString().slice(11, 19).replaceAll(":", "") +
          "-" +
          name +
          ".png",
        import.meta.url,
      ).pathname.replace(/^\/(?=[A-Z]:)/, ""),
      fullPage: false,
    });
  }
  async geometry() {
    return this.root.evaluate((root) =>
      [...root.querySelectorAll<HTMLElement>(".vxe-grid")]
        .filter((grid) => grid.getBoundingClientRect().height > 0)
        .map((grid) => {
          const box = grid.getBoundingClientRect();
          const body = grid.querySelector<HTMLElement>(
            ".vxe-table--main-wrapper .vxe-table--body-wrapper",
          );
          const header = grid.querySelector<HTMLElement>(
            ".vxe-table--header-wrapper",
          );
          const dataRow = grid.querySelector<HTMLElement>(".vxe-body--row");
          return {
            width: box.width,
            right: box.right,
            viewport: innerWidth,
            height: box.height,
            rows: grid.querySelectorAll(
              ".vxe-table--main-wrapper .vxe-body--row",
            ).length,
            body: body ? getComputedStyle(body).backgroundColor : "",
            header: header ? getComputedStyle(header).backgroundColor : "",
            row: dataRow ? getComputedStyle(dataRow).backgroundColor : "",
            pager: Boolean(grid.querySelector(".vxe-pager")),
          };
        }),
    );
  }
}
