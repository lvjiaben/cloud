import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForDialogReady,
  waitForRouteReady,
} from "../../../../../../hack/tests/support/ui";

export class ProviderConfigurationPage {
  readonly configureButton: Locator;
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly recoverButton: Locator;
  readonly recoveryAlert: Locator;
  readonly validateButton: Locator;
  readonly validationStatus: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("aliyun-provider-page");
    this.configureButton = page.getByTestId("aliyun-provider-configure");
    this.recoverButton = page.getByTestId("aliyun-provider-recover");
    this.recoveryAlert = page.getByTestId("aliyun-provider-recovery-alert");
    this.validateButton = page.getByTestId("aliyun-provider-validate");
    this.validationStatus = page.getByTestId(
      "aliyun-provider-validation-status",
    );
  }

  async goto() {
    await this.page.goto(
      workspacePath("/idc/compliance/providers/aliyun-face"),
      { waitUntil: "domcontentloaded" },
    );
    await waitForRouteReady(this.page, 15_000);
    await this.pageRoot.waitFor({ state: "visible" });
  }

  async openConfiguration() {
    await this.configureButton.click();
    const dialog = this.page.getByRole("dialog");
    await waitForDialogReady(dialog);
    return dialog;
  }

  async openRecovery() {
    await this.recoverButton.click();
    const dialog = this.page.getByRole("dialog");
    await waitForDialogReady(dialog);
    return dialog;
  }

  async fillConfiguration(input: {
    accessKeyId: string;
    accessKeySecret: string;
    sceneId: string;
  }) {
    const dialog = this.page.getByRole("dialog");
    await dialog.getByTestId("aliyun-provider-scene-id").fill(input.sceneId);
    await dialog
      .getByTestId("aliyun-provider-access-key-id")
      .fill(input.accessKeyId);
    await dialog
      .getByTestId("aliyun-provider-access-key-secret")
      .fill(input.accessKeySecret);
  }

  async fillRecovery(input: { accessKeyId: string; accessKeySecret: string }) {
    const dialog = this.page.getByRole("dialog");
    await dialog
      .getByTestId("aliyun-provider-access-key-id")
      .fill(input.accessKeyId);
    await dialog
      .getByTestId("aliyun-provider-access-key-secret")
      .fill(input.accessKeySecret);
  }

  async confirmDialog() {
    const dialog = this.page.getByRole("dialog");
    await dialog.locator(".ant-btn-primary").last().click();
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }
}
