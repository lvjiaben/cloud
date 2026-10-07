import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import { expect } from "@host-tests/support/playwright";
import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ElectronicContractPage {
  readonly autoSignAmount: Locator;
  readonly autoSignCurrency: Locator;
  readonly autoSignEnable: Locator;
  readonly autoSignExpiry: Locator;
  readonly autoSignLimit: Locator;
  readonly autoSignManager: Locator;
  readonly autoSignOptional: Locator;
  readonly autoSignRequired: Locator;
  readonly autoSignRevoke: Locator;
  readonly autoSignTemplates: Locator;
  readonly contractFieldEditor: Locator;
  readonly contractManager: Locator;
  readonly contractPreview: Locator;
  readonly contractDetail: Locator;
  readonly contractTemplateEdit: Locator;
  readonly contractTemplateEditor: Locator;
  readonly contractTemplateManager: Locator;
  readonly contractTemplateNameFilter: Locator;
  readonly contractTemplateOnlineEditor: Locator;
  readonly contractTemplateProviderMapping: Locator;
  readonly contractTemplateConfirmPreview: Locator;
  readonly contractTemplateGeneratePreview: Locator;
  readonly contractTemplateSave: Locator;
  readonly contractTemplateSaveFields: Locator;
  readonly contractTemplateSaveSource: Locator;
  readonly contractTemplateSaveProviderMapping: Locator;
  readonly contractTemplateSearch: Locator;
  readonly contractTemplateSubmitTencent: Locator;
  readonly contractTemplateSync: Locator;
  readonly openTencentButton: Locator;
  readonly page: Page;
  readonly profile: Locator;
  readonly recheckButton: Locator;
  readonly root: Locator;
  readonly unavailable: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("electronic-contract-page");
    this.autoSignAmount = page.getByTestId("contract-auto-sign-amount");
    this.autoSignCurrency = page.getByTestId("contract-auto-sign-currency");
    this.autoSignEnable = page.getByTestId("contract-auto-sign-enable");
    this.autoSignExpiry = page.getByTestId("contract-auto-sign-expiry");
    this.autoSignLimit = page.getByTestId("contract-auto-sign-limit");
    this.autoSignManager = page.getByTestId("contract-auto-sign-manager");
    this.autoSignOptional = page.getByTestId("contract-auto-sign-optional");
    this.autoSignRequired = page.getByTestId("contract-auto-sign-required");
    this.autoSignRevoke = page.getByTestId("contract-auto-sign-revoke");
    this.autoSignTemplates = page.getByTestId("contract-auto-sign-templates");
    this.contractFieldEditor = page.getByTestId("contract-field-editor");
    this.contractManager = page.getByTestId("electronic-contract-manager");
    this.contractPreview = page.getByTestId("contract-preview");
    this.contractDetail = page.getByTestId("contract-detail");
    this.contractTemplateEdit = page.getByTestId("contract-template-edit");
    this.contractTemplateEditor = page.getByTestId("contract-template-editor");
    this.contractTemplateManager = page.getByTestId(
      "contract-template-manager",
    );
    this.contractTemplateNameFilter = page.getByTestId(
      "contract-template-name-filter",
    );
    this.contractTemplateOnlineEditor = page.getByTestId(
      "contract-online-editor",
    );
    this.contractTemplateProviderMapping = page.getByTestId(
      "contract-template-provider-mapping",
    );
    this.contractTemplateConfirmPreview = page.getByTestId(
      "contract-template-confirm-preview",
    );
    this.contractTemplateGeneratePreview = page.getByTestId(
      "contract-template-generate-preview",
    );
    this.contractTemplateSave = page.getByTestId("contract-template-save");
    this.contractTemplateSaveFields = page.getByTestId(
      "contract-template-save-fields",
    );
    this.contractTemplateSaveSource = page.getByTestId(
      "contract-template-save-source",
    );
    this.contractTemplateSaveProviderMapping = page.getByTestId(
      "contract-template-save-provider-mapping",
    );
    this.contractTemplateSearch = page.getByTestId("contract-template-search");
    this.contractTemplateSubmitTencent = page.getByTestId(
      "contract-template-submit-tencent",
    );
    this.contractTemplateSync = page.getByTestId("contract-template-sync");
    this.profile = page.getByTestId("electronic-contract-profile");
    this.openTencentButton = page.getByTestId(
      "electronic-contract-open-tencent",
    );
    this.recheckButton = page.getByTestId("electronic-contract-recheck-button");
    this.unavailable = page.getByTestId("electronic-contract-unavailable");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/compliance/electronic-contract"), {
      waitUntil: "domcontentloaded",
    });
    await waitForRouteReady(this.page, 15_000);
    await this.root.waitFor({ state: "visible" });
  }

  async copyTemplate(name: string) {
    await this.searchTemplatesByName(name);
    await this.contractTemplateManager
      .getByTestId("contract-template-copy")
      .first()
      .click();
    await this.contractTemplateEditor.waitFor({ state: "visible" });
  }

  async openTemplateEditor() {
    await this.openTemplateManager();
    await this.searchTemplatesByName("Merchant service agreement");
    await this.contractTemplateManager
      .getByTestId("contract-template-edit")
      .first()
      .click();
    await this.contractTemplateEditor.waitFor({ state: "visible" });
    await this.contractTemplateOnlineEditor.waitFor({ state: "visible" });
  }

  async openTemplateEditorByName(name: string) {
    await this.openTemplateManager();
    await this.searchTemplatesByName(name);
    await this.contractTemplateManager
      .getByTestId("contract-template-edit")
      .first()
      .click();
    await this.contractTemplateEditor.waitFor({ state: "visible" });
  }

  async openTemplateManager() {
    await this.openStableTab(
      /合同模板|Contract templates/u,
      this.contractTemplateManager,
    );
  }

  async openAutoSignManager() {
    await this.openStableTab(
      /自动签授权|Auto-sign authorization/u,
      this.autoSignManager,
    );
  }

  async openContractManager() {
    await this.openStableTab(/合同管理|Contracts/u, this.contractManager);
  }

  private async openStableTab(name: RegExp, target: Locator) {
    await expect(async () => {
      await this.page.getByRole("tab", { name }).click();
      await expect(target).toBeVisible({ timeout: 3_000 });
    }).toPass({ intervals: [100, 250, 500], timeout: 15_000 });
  }

  async openContractDetail(contractNo: string) {
    const row = this.contractManager.locator("tbody tr").filter({
      hasText: contractNo,
    });
    await row.getByRole("button", { name: /查看详情|View details/u }).click();
    await this.contractDetail.waitFor({ state: "visible" });
  }

  async openOptionalContractCreate() {
    await this.page.getByTestId("contract-create-open").click();
  }

  async completeOptionalPreview(
    orderNo: string,
    templateText: string,
    signerText: string,
  ) {
    await this.page.getByTestId("contract-create-order").fill(orderNo);
    await this.page.getByTestId("contract-create-load-order").click();
    await this.page.getByTestId("contract-create-template").click();
    await this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: templateText })
      .click();
    await this.page.getByTestId("contract-create-signer").click();
    await this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: signerText })
      .click();
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /确认并生成预览|Confirm and create preview/u })
      .click();
    const confirmation = this.page.getByRole("dialog").last();
    await confirmation
      .getByRole("button", { name: /确认并生成预览|Confirm and create preview/u })
      .click();
    await this.contractPreview.waitFor({ state: "visible" });
  }

  async submitVisiblePreview() {
    await this.contractPreview.getByTestId("contract-submit-signing").click();
    await this.contractDetail.waitFor({ state: "visible" });
  }

  async reissueUnchanged() {
    await this.contractDetail
      .getByTestId("contract-reissue-unchanged")
      .click();
    await this.page
      .getByRole("dialog")
      .last()
      .getByRole("button", { name: /按原内容重新发起|Reissue unchanged/u })
      .click();
  }

  async openChangedReplacementCreate() {
    await this.contractDetail.getByTestId("contract-reissue-changed").click();
  }

  async completeChangedReplacementPreview(
    templateText: string,
    signerText: string,
  ) {
    const dialog = this.page.getByRole("dialog", {
      name: /生成新的合同版本|Create a new contract version/u,
    });
    await dialog.getByTestId("contract-replacement-template").click();
    await this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: templateText })
      .click();
    await dialog.getByTestId("contract-replacement-signer").click();
    await this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: signerText })
      .click();
    await dialog
      .getByRole("button", { name: /生成新版本预览|Create new version preview/u })
      .click();
    await dialog.waitFor({ state: "hidden" });
    await this.contractPreview.waitFor({ state: "visible" });
  }

  async openTerminationCreate() {
    await this.contractDetail.getByTestId("contract-terminate-open").click();
  }

  async submitTermination(reason: string) {
    const dialog = this.page.getByRole("dialog", {
      name: /填写合同解除内容|Enter termination terms/u,
    });
    await dialog.getByTestId("contract-termination-reason").fill(reason);
    await dialog
      .getByRole("button", { name: /检查解除内容|Review termination/u })
      .click();
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /确认提交解除|Confirm termination/u })
      .click();
  }

  async openContractDetailSection(name: RegExp) {
    await this.contractDetail.getByRole("tab", { name }).click();
  }

  async searchTemplatesByName(name: string) {
    await this.contractTemplateNameFilter.fill(name);
    await this.contractTemplateSearch.click();
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, stamp + "-" + description + ".png");
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }
}
