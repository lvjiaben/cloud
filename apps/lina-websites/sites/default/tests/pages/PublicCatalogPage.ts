// This page object owns public catalog interactions and safe-projection assertions.
import type { Locator, Page } from "@playwright/test";
import { testRuntime } from "../support/runtime-config";

export class PublicCatalogPage {
  readonly catalog: Locator;
  readonly detail: Locator;
  readonly filters: Locator;
  readonly page: Page;
  readonly products: Locator;
  readonly campaigns: Locator;
  readonly configurator: Locator;

  constructor(page: Page) {
    this.page = page;
    this.catalog = page.getByTestId("public-catalog");
    this.filters = page.getByTestId("public-catalog-filters");
    this.products = page.getByTestId("public-catalog-products");
    this.campaigns = page.getByTestId("public-product-campaigns");
    this.configurator = page.getByTestId("public-product-configurator");
    this.detail = page.getByTestId("public-catalog-detail");
  }

  async goto(path = "/products") {
    const baseURL = `${testRuntime.baseURL.replace(/\/+$/u, "")}/`;
    const relativePath = path.replace(/^\/+/, "");
    const suffix = /\/products(\/[^/]+)?$/u.exec(new URL(relativePath, baseURL).pathname)?.[1] ?? "";
    const loaded = this.page.waitForResponse(response =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname.endsWith(`/public/catalog/products${suffix}`),
    );
    await this.page.goto(new URL(relativePath, baseURL).toString(), {
      // Vite keeps its HMR connection open in local validation, so the page's
      // own visible catalog boundary is the deterministic readiness signal.
      waitUntil: "domcontentloaded",
    });
    await loaded;
    await this.catalog.waitFor({ state: "visible" });
    await (suffix ? this.detail : this.products).waitFor({ state: "visible" });
  }

  async openCurrencyChoices() {
    await this.filters.locator(".ant-select").nth(1).click();
  }

  get currencyChoices() { return this.page.locator(".ant-select-dropdown:visible"); }

  async chooseCurrency(currency: string) {
    await this.openCurrencyChoices();
    await this.page
      .locator(".ant-select-item-option", { hasText: currency })
      .click();
  }

  async chooseCampaignCurrency(currency: string) {
    await this.campaigns.locator(".ant-select").click();
    await this.page
      .locator(".ant-select-item-option", { hasText: currency })
      .click();
  }

  async requestConfigurationQuote() {
    await this.configurator
      .getByRole("button", { name: /准确价格|exact price/u })
      .click();
  }

  async setConfigurationQuantity(label: string, quantity: string) {
    await this.configurator
      .locator(".ant-form-item")
      .filter({ hasText: label })
      .getByRole("textbox")
      .fill(quantity);
  }

  async addQuotedProductToCart() {
    await this.configurator.getByTestId("public-add-to-cart").click();
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
  }
}
