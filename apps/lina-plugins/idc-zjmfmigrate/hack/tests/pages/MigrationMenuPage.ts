import { expect, type Page } from "@host-tests/support/playwright";

export class MigrationMenuPage {
  constructor(private readonly page: Page) {}

  async expectChineseMenu() {
    await expect(
      this.page.locator("aside").getByText("迁移检查", { exact: true }),
    ).toHaveCount(0);
    await this.page.goto("/admin/system/configuration-center", {
      waitUntil: "domcontentloaded",
    });
    await this.page.getByRole("tab", { name: "数据迁移" }).click();
    await expect(
      this.page.getByText("迁移检查", { exact: true }),
    ).toBeVisible();
    await expect(
      this.page.getByText("Migration Checks", { exact: true }),
    ).toHaveCount(0);
  }

  async switchToEnglish() {
    await this.page.getByTestId("language-toggle-trigger").click();
    await this.page.getByRole("menuitem", { name: "English" }).click();
  }

  async expectEnglishMenu() {
    await this.page.getByRole("tab", { name: "Migration" }).click();
    await expect(
      this.page.getByText("Migration checks", { exact: true }),
    ).toBeVisible();
    await expect(this.page.getByText("迁移检查", { exact: true })).toHaveCount(
      0,
    );
  }
}
