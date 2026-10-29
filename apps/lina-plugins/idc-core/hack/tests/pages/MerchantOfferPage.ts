import { expect, type Page } from "@host-tests/support/playwright";
import { CatalogWorkspacePage } from "./CatalogWorkspacePage";
export class MerchantOfferPage extends CatalogWorkspacePage {
  constructor(page: Page) {
    super(page);
  }
  get offers() {
    return this.page.getByTestId("catalog-offer-options");
  }
  async setPreviewVisible(visible: boolean) {
    const toggle = this.page.getByTestId("catalog-option-preview-toggle");
    if ((await toggle.getAttribute("aria-expanded")) !== String(visible))
      await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", String(visible));
    if (visible)
      await expect(this.page.getByTestId("catalog-offer-preview")).toBeVisible();
    else
      await expect(this.page.getByTestId("catalog-offer-preview")).toBeHidden();
  }
  field(key: string) {
    return this.page.getByTestId("catalog-option-tab-" + key);
  }
  chooseField(key: string) {
    return this.field(key).click();
  }
  get children() {
    return this.offers.locator(
      '[data-testid$="_choices"]:visible, [data-testid$="_numbers"]:visible',
    );
  }
  async setChildren(text: string) {
    if (text.includes("|"))
      await this.offers
        .getByRole("checkbox", { name: "分组、排序与多语言设置", exact: true })
        .check();
    while (
      await this.children.getByTestId("catalog-option-child-remove").count()
    )
      await this.children.getByTestId("catalog-option-child-remove").click();
    for (const line of text.split(/\r?\n/u).filter(Boolean)) {
      const [value = "", name = value, english = name] = line.split("|");
      await this.children.getByTestId("catalog-option-child-add").click();
      await this.children.getByTestId("catalog-option-child-value").fill(value);
      await this.children.getByTestId("catalog-option-child-zhCN").fill(name);
      if (await this.children.getByTestId("catalog-option-child-enUS").count())
        await this.children
          .getByTestId("catalog-option-child-enUS")
          .fill(english);
    }
  }
  async mapChildToUpstream(index: number, label: string) {
    await this.children.getByTestId(`catalog-option-child-${index}`).click();
    const control = this.children.getByTestId("catalog-option-child-upstream");
    await expect(control).toBeEnabled();
    const input = control.getByRole("combobox");
    await input.press("ArrowDown");
    if (await input.isEditable()) await input.fill(label);
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(label, { exact: true })
      .click();
    await expect(control).toContainText(label);
  }
  async expectChildUpstream(index: number, label: string) {
    await this.children.getByTestId(`catalog-option-child-${index}`).click();
    await expect(
      this.children.getByTestId("catalog-option-child-upstream"),
    ).toContainText(label);
  }
  async compactField() {
    await this.offers
      .getByRole("checkbox", { name: "分组、排序与多语言设置", exact: true })
      .uncheck();
  }
  async expectChildren(text: string) {
    const lines = text.split(/\r?\n/u).filter(Boolean);
    await expect(
      this.children.getByRole("list").getByRole("button"),
    ).toHaveCount(lines.length);
    for (const [index, line] of lines.entries()) {
      const [value = "", name, english] = line.split("|");
      await this.children.getByTestId(`catalog-option-child-${index}`).click();
      await expect(
        this.children.getByTestId("catalog-option-child-value"),
      ).toHaveValue(value);
      if (name !== undefined)
        await expect(
          this.children.getByTestId("catalog-option-child-zhCN"),
        ).toHaveValue(name);
      if (
        english !== undefined &&
        (await this.children.getByTestId("catalog-option-child-enUS").count())
      )
        await expect(
          this.children.getByTestId("catalog-option-child-enUS"),
        ).toHaveValue(english);
    }
  }
  input(label: string) {
    const suffix = (
      {
        最小可售数量: "minimum",
        最大可售数量: "maximum",
        数量步长: "step",
        "配置名称（中文）": "name_zhCN",
        "配置名称（英文）": "name_enUS",
        配置子项: "choices",
        可选规格: "numbers",
      } as Record<string, string>
    )[label];
    if (suffix)
      return this.offers.locator(
        `[data-testid$="_${suffix}"] input, input[data-testid$="_${suffix}"], textarea[data-testid$="_${suffix}"]`,
      );
    label =
      ({ 可选规格: "可选数值" } as Record<string, string>)[label] || label;
    return this.offers
      .getByRole("textbox", { name: label, exact: true })
      .or(this.offers.getByRole("spinbutton", { name: label, exact: true }));
  }
  async addField(label: string) {
    const control = this.page
      .getByTestId("catalog-customer-option-parameter")
      .getByRole("combobox");
    await control.fill(label);
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(label, { exact: true })
      .click();
    await this.page.getByTestId("catalog-customer-option-add").click();
  }
  enable() {
    return this.offers
      .getByRole("switch", { name: "启用该配置", exact: true })
      .click();
  }
  async control(label: string) {
    label =
      (
        { 下拉选择规格: "下拉选择", 规格按钮: "单选按钮" } as Record<
          string,
          string
        >
      )[label] || label;
    const control = this.offers
      .locator('[data-testid$="_control"]')
      .getByRole("combobox");
    await control.focus();
    await control.press("ArrowDown");
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(label, { exact: true })
      .click();
  }
  save() {
    return this.page.getByTestId("catalog-customer-options-save").click();
  }
  async scrollPrices(direction: "start" | "end") {
    const scrollbar = this.page
      .getByTestId("catalog-offer-field-prices")
      .locator(".vxe-table--scroll-y-handle");
    // Scroll the browser's physical scrollbar, not the recycled row wrapper.
    await scrollbar.evaluate((element, end) => {
      element.scrollTo({
        top: end ? element.scrollHeight : 0,
        behavior: "instant",
      });
    }, direction === "end");
    await expect
      .poll(async () => {
        return scrollbar.evaluate(
          (element, end) =>
            end
              ? element.scrollTop + element.clientHeight >=
                element.scrollHeight - 1
              : element.scrollTop <= 1,
          direction === "end",
        );
      })
      .toBe(true);
  }
}
