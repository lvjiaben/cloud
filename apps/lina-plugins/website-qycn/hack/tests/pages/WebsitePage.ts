// Page object owns public interactions and timestamped visual evidence.
import type { Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { evidenceDirectory } from "../support/output";
export class WebsitePage {
  constructor(readonly page: Page) {}
  async open(path = "/") {
    await this.page.goto(path);
    await this.page.locator("#brand-status").waitFor({ state: "hidden" });
  }
  async slide(index: number) {
    await this.page.locator(`[data-slide="${index}"]`).click();
  }
  async navigationLink(name: string) {
    await this.page.locator("#header-nav").getByRole("link", { name, exact: true }).click();
  }
  async hoverHero() {
    await this.page.locator(".hero").hover({ position: { x: 700, y: 200 } });
  }
  async nextSlide() {
    await this.page.locator(".next").click();
  }
  async focusHeroWithKeyboard() {
    await this.page.keyboard.press("Tab");
    await this.page.locator("#hero-link").focus();
  }
  async selectLanguage(language: string) {
    await this.page.getByLabel("Language", { exact: true }).selectOption(language);
  }
  async retryConfigurationLoading() {
    await this.page.locator("#commerce-page .status").getByRole("button", { name: "重试", exact: true }).click();
  }
  async focusLanguage() {
    await this.page.locator("#language").focus();
  }
  async category(name: string) {
    await this.page.getByRole("tab", { name, exact: true }).click();
  }
  async productSearch(value: string) {
    await this.page.locator("#product-query").fill(value);
    await this.page
      .getByRole("button", { name: "搜索产品", exact: true })
      .click();
  }
  async openArticle(title: string) { await this.page.locator("#content-items").getByRole("link").filter({ hasText: title }).click(); }
  async contentSearch(value: string) {
    await this.page.locator("#content-query").fill(value);
    await this.page.getByRole("button", { name: "搜索", exact: true }).click();
  }
  async retry(id: string) {
    await this.page
      .locator(`#${id}`)
      .getByRole("button", { name: "重试" })
      .click();
  }
  async moreProducts() {
    await this.page.locator("#catalog-more").click();
  }
  async moreContent() {
    await this.page.locator("#content-more").click();
  }
  async menu() {
    await this.page.locator(".menu-toggle").click();
  }
  async group(name: string) {
    await this.page
      .locator("#header-nav")
      .getByRole("button", { name })
      .click();
  }
  async link(name: string) {
    await this.page.getByRole("link", { name, exact: true }).click();
  }
  async key(key: string) {
    await this.page.keyboard.press(key);
  }
  async download() {
    await this.page.locator("#download").click();
  }
  async question(text: string) {
    await this.page.getByText(text, { exact: true }).click();
  }
  async location(name: string) {
    await this.page.locator(".map-pin").filter({ hasText: name }).click();
  }
  async addToCart() { await this.page.getByRole("button", {name:"加入购物车",exact:true}).click(); }
  async cartAction(label: string) {
    await this.page.getByRole("button", { name: label, exact: true }).click();
  }
  async showPaymentChoices() {
    await this.page.locator(".cart-payment-options").evaluate(node => node.scrollIntoView({ block: "center", behavior: "instant" }));
  }
  async retryPaymentChoices(label: string) {
    await this.page.locator(".cart-payment-options").getByRole("button", { name: label, exact: true }).click();
  }
  async choosePayment(label: string) {
    await this.page.getByRole("radio", { name: label, exact: true }).check();
  }
  async backToTop() { await this.page.getByRole("button", {name:"返回顶部",exact:true}).click(); }
  async mobile() {
    await this.page.setViewportSize({ width: 390, height: 844 });
  }
  async configureNumber(label: string, value: string) {
    await this.page.getByLabel(label, {exact: false}).and(this.page.locator('input[type="number"]')).fill(value);
  }
  async chooseSystemVersion(family: string, token: string) {
    await this.page.locator(".purchase-system-card").filter({ hasText: family }).getByRole("combobox").selectOption(token);
  }
  async stepConfiguration(name: string) {
    await this.page.locator(".purchase-stepper").getByRole("button", { name, exact: true }).click();
  }
  async configureServerLogin(hostname: string, password: string) {
    await this.page.locator("#server-hostname").fill(hostname);
    await this.page.locator("#server-password").fill(password);
  }
  async serverLoginAction(name: string) {
    await this.page.locator("#server-login").getByRole("button", { name, exact: true }).click();
  }
  async addConfiguredProduct(name: string) {
    await this.page.locator(".purchase-add").click();
  }
  async chooseConfiguration(label: string, value: string) {
    await this.page.getByLabel(label, { exact: false }).selectOption(value);
  }
  async chooseConfigurationRadio(field: string, option: string) {
    await this.page.getByRole("group", { name: field, exact: true }).getByRole("radio", { name: option, exact: true }).check();
  }
  async slideConfiguration(label: string, value: string) {
    const slider = this.page.getByRole("slider", { name: label, exact: true });
    await slider.focus();
    await slider.press("Home");
    const minimum = Number(await slider.getAttribute("min"));
    const step = Number(await slider.getAttribute("step") || 1);
    for (let next = minimum; next < Number(value); next += step) await slider.press("ArrowRight");
  }
  async retryConfigurationPrice(label = "重试") {
    await this.page.locator(".price-status").getByRole("button", { name: label, exact: true }).click();
  }
  async finishConfigurationEdit() {
    await this.page.getByRole("heading", {name: "产品配置", exact: true}).click();
  }
  async configureCategory(name: string) {
    await this.page.locator(".purchase-categories").getByRole("button",{name,exact:true}).click();
  }
  async moreConfigureProducts() { await this.page.locator(".catalog-more").click(); }
  async retryConfigureCatalog(name: string) { await this.page.locator(".catalog-status").getByRole("button",{name,exact:true}).click(); }
  async screenshot(name: string, fullPage = false) {
    const date = new Date();
    const output = evidenceDirectory;
    await mkdir(output, { recursive: true });
    const path = resolve(
      output,
      `${date.toISOString().slice(11, 23).replaceAll(":", "").replace(".", "")}-qycn-${name}.png`,
    );
    await this.page.screenshot({ path, fullPage });
    return path;
  }
}
