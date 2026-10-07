import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class MarketingOperationsPage {
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly refreshButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("marketing-overview-page");
    this.refreshButton = page.getByTestId("marketing-refresh");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/marketing"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByText(/加载菜单中|Loading menu/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
    await this.waitForDataSettled();
  }

  async openTab(name: "活动与人群" | "经营总览" | "权益与积分" | "最近记录") {
    await this.pageRoot.getByRole("tab", { name }).click();
  }

  currencyCard(currencyCode: string) {
    return this.pageRoot.locator(".currency-card").filter({
      hasText: currencyCode,
    });
  }

  campaignRow(campaignNo: string) {
    return this.pageRoot.locator(`tr:has-text("${campaignNo}")`);
  }

  grantRow(grantNo: string) {
    return this.pageRoot.locator(`tr:has-text("${grantNo}")`);
  }

  segmentRow(segmentCode: string) {
    return this.page
      .getByTestId("segment-directory")
      .locator(`tr:has-text("${segmentCode}")`);
  }

  managementCampaignRow(campaignCode: string) {
    return this.page
      .getByTestId("campaign-directory")
      .locator(`tr:has-text("${campaignCode}")`);
  }

  assetRow(assetNo: string) {
    return this.page
      .getByTestId("asset-directory")
      .locator(`tr:has-text("${assetNo}")`);
  }

  pointsRow(customerNo: string) {
    return this.page
      .getByTestId("points-directory")
      .locator(`tr:has-text("${customerNo}")`);
  }

  valueGrantDirectoryRow(grantNo: string) {
    return this.page
      .getByTestId("value-grant-directory")
      .locator(`tr:has-text("${grantNo}")`);
  }

  adjustmentRow(adjustmentNo: string) {
    return this.page
      .getByTestId("value-grant-adjustment-queue")
      .locator(`tr:has-text("${adjustmentNo}")`);
  }

  async openAssetDetail(assetNo: string) {
    await this.assetRow(assetNo).getByRole("button").click();
    await this.waitForDrawer();
  }

  async openPointsDetail(customerNo: string) {
    await this.page
      .getByTestId(`marketing-points-detail-${customerNo}`)
      .click();
    await this.waitForDrawer();
  }

  async openValueGrantDirectoryDetail(grantNo: string) {
    await this.page.getByTestId(`value-grant-detail-${grantNo}`).click();
    await this.waitForDrawer();
  }

  async openSegmentDetail(segmentCode: string) {
    await this.segmentRow(segmentCode).getByRole("button").click();
    await this.waitForDrawer();
  }

  async openCampaignDetail(campaignCode: string) {
    await this.managementCampaignRow(campaignCode).getByRole("button").click();
    await this.waitForDrawer();
  }

  async waitForDrawer() {
    await this.page.getByRole("dialog").last().waitFor({ state: "visible" });
  }

  async closeDrawer() {
    const drawer = this.page.getByRole("dialog").last();
    await this.page.keyboard.press("Escape");
    await drawer.waitFor({ state: "hidden" });
  }

  async nextSegmentPage() {
    await this.page
      .getByTestId("segment-directory")
      .locator(".vxe-pager--next-btn")
      .click();
    await this.segmentRow("dormant_customers").waitFor({ state: "visible" });
  }

  async refresh() {
    await this.refreshButton.click();
    await this.waitForDataSettled();
  }

  async screenshot(description: string) {
    await this.waitForDataSettled();
    await this.page.waitForTimeout(400);
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }

  async screenshotAssetDirectory(description: string) {
    await this.waitForDataSettled();
    const directorySection = this.page.getByTestId("marketing-asset-directory");
    await directorySection.scrollIntoViewIfNeeded();
    await directorySection
      .locator(".ant-table-content, .ant-table-body")
      .evaluateAll((containers) => {
        for (const container of containers) {
          container.scrollLeft = 0;
        }
      });
    await this.page.waitForTimeout(400);
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await directorySection.screenshot({ path });
    return path;
  }

  async waitForDataSettled() {
    await this.refreshButton.waitFor({ state: "visible", timeout: 15_000 });
    await this.pageRoot
      .locator(".ant-skeleton")
      .waitFor({ state: "hidden", timeout: 15_000 });
  }
}
