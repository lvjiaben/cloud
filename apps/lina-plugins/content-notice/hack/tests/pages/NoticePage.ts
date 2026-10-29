import type { Page } from "@host-tests/support/playwright";
import { expect } from "@host-tests/support/playwright";

import {
  waitForConfirmOverlay,
  waitForDialogReady,
  waitForRouteReady,
  waitForTableReady,
} from "@host-tests/support/ui";

export class NoticePage {
  constructor(private page: Page) {}

  private resolveLocalizedLabel(label: string) {
    const labelMap: Record<string, RegExp> = {
      公告标题: /公告标题|Title|plugin\.content-notice\.fields\.title/i,
      创建者:
        /创建者|Created By|plugin\.content-notice\.fields\.createdBy/i,
    };
    const localizedLabel = labelMap[label];
    if (localizedLabel) {
      return this.page.getByLabel(localizedLabel).first();
    }
    return this.page.getByLabel(label, { exact: true }).first();
  }

  /** The Vben modal container */
  get modal() {
    return this.page.locator('[role="dialog"]');
  }

  async goto() {
    await this.page.goto("/system/notice");
    await this.page.getByTestId('notice-management-page').waitFor({ state: 'visible', timeout: 15000 });
    await waitForTableReady(this.page);
  }

  /** Create a new notice */
  async createNotice(
    title: string,
    status: "草稿" | "已发布",
    content?: string,
  ) {
    await this.page
      .getByRole("button", { name: /新\s*增/ })
      .first()
      .click();

    await waitForDialogReady(this.modal);

    // Fill title - use placeholder to find the input inside the modal
    const titleInput = this.modal.getByPlaceholder("请输入公告标题").first();
    await titleInput.fill(title);

    // Select status (RadioButton) - using label text since they're button-style radios
    await this.modal
      .locator(".ant-radio-button-wrapper", { hasText: status })
      .click();

    // Type content in Tiptap editor if provided
    if (content) {
      const editor = this.modal.locator('.tiptap[contenteditable="true"]');
      await editor.waitFor({ state: "visible", timeout: 5000 });
      await editor.click();
      await this.page.keyboard.type(content, { delay: 20 });
    }

    // Click confirm button (modal footer)
    await this.modal.getByRole("button", { name: /确\s*认/ }).click();

    await waitForRouteReady(this.page);
    await this.modal
      .waitFor({ state: "hidden", timeout: 10000 })
      .catch(() => {});
  }

  async add() {
    await this.page.getByTestId('notice-add').click();
    await waitForDialogReady(this.modal);
  }

  sortInput(title: string) {
    return this.page.locator('.vxe-body--row:visible', { hasText: title }).getByRole('spinbutton');
  }

  async setSort(title: string, value: string) {
    const input = this.sortInput(title);
    await input.fill(value);
    await input.press('Enter');
  }

  /** Edit a notice: search by title, click edit, update title */
  async editNotice(searchTitle: string, newTitle: string) {
    await this.openEdit(searchTitle);

    const titleInput = this.modal.getByPlaceholder("请输入公告标题").first();
    await expect(titleInput).toHaveValue(searchTitle);
    await titleInput.clear();
    await titleInput.fill(newTitle);

    await this.modal.getByRole("button", { name: /确\s*认/ }).click();

    await waitForRouteReady(this.page);
    await this.modal
      .waitFor({ state: "hidden", timeout: 10000 })
      .catch(() => {});
  }

  async openEdit(searchTitle: string) {
    await this.fillSearchField("公告标题", searchTitle);
    await this.clickSearch();

    const row = this.page.locator(".vxe-body--row:visible", {
      hasText: searchTitle,
    });
    await row.first().waitFor({ state: "visible", timeout: 10000 });
    await row
      .locator("button:visible")
      .filter({ hasText: /编\s*辑/ })
      .first()
      .click();

  }

  /** Delete a notice: search by title, click delete, confirm */
  async deleteNotice(title: string) {
    await this.fillSearchField("公告标题", title);
    await this.clickSearch();

    const row = this.page.locator(".vxe-body--row:visible", { hasText: title });
    await row.first().waitFor({ state: "visible", timeout: 10000 });
    await row
      .locator("button:visible")
      .filter({ hasText: /删\s*除/ })
      .first()
      .click();

    const popconfirm = await waitForConfirmOverlay(this.page);
    const confirmBtn = popconfirm.getByRole("button", {
      name: /确\s*定|OK|是/i,
    });
    if (await confirmBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await confirmBtn.click();
    }

    await waitForRouteReady(this.page);
  }

  async deleteNoticeIfExists(title: string) {
    if (await this.hasNotice(title)) {
      await this.deleteNotice(title);
    }
  }

  /** Check if a notice with the given title is visible */
  async hasNotice(title: string): Promise<boolean> {
    await this.fillSearchField("公告标题", title);
    await this.clickSearch();
    return this.page
      .locator(".vxe-body--row:visible", { hasText: title })
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
  }

  /** Preview a notice: search by title, click preview button */
  async previewNotice(title: string) {
    await this.fillSearchField("公告标题", title);
    await this.clickSearch();

    await this.page
      .getByRole("button", { name: /预\s*览/ })
      .first()
      .click();

    await waitForDialogReady(this.modal);
  }

  /** Fill search form field by label */
  async fillSearchField(label: string, value: string) {
    const input = this.resolveLocalizedLabel(label);
    await input.clear();
    await input.fill(value);
  }

  /** Click search button */
  async clickSearch() {
    await this.page
      .getByRole("button", { name: /搜\s*索|Search/i })
      .first()
      .click();
    await waitForRouteReady(this.page);
  }

  /** Click reset button */
  async clickReset() {
    await this.page
      .getByRole("button", { name: /重\s*置|Reset/i })
      .first()
      .click();
    await waitForRouteReady(this.page);
  }

  /** Get total count from pager */
  async getTotalCount(): Promise<number> {
    const pager = this.page.locator(".vxe-pager--total");
    const text = await pager.textContent();
    const match = text?.match(/(\d+)/);
    return match ? parseInt(match[1], 10) : 0;
  }
}
