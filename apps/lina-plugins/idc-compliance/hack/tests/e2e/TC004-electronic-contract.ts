import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";
import { MainLayout } from "../../../../../../hack/tests/pages/MainLayout";

import { ElectronicContractPage } from "../pages/ElectronicContractPage";
import {
  installElectronicContractMock,
  setElectronicContractPresentation,
} from "../support/electronic-contract-mock";

test.describe("TC-4 电子合同", () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, "idc-compliance");
  });

  test("TC-4a-0: 真实本地供应商未配置时页面正常打开", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();

    await expect(contract.root).toContainText("电子合同");
    await expect(
      adminPage.getByText("本地供应商配置暂时不可用", { exact: true }),
    ).toHaveCount(0);
  });

  test("TC-4a-1: 四步向导只读展示既有企业资料并在新窗口打开腾讯", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    await mainLayout.prepareThemeModeWithoutAnimation("light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();

    await expect(contract.root).toContainText("企业资料确认");
    await expect(contract.root).toContainText("核对已有的商家名称、地区和编号。");
    await expect(contract.root).toContainText("示例 IDC 有限公司");
    await expect(contract.root).toContainText("merchant-local");
    await expect(contract.root).toContainText("可以开始");
    await expect(contract.root.locator(".ant-steps")).toBeVisible();
    await expect(contract.root).toContainText("统一社会信用代码");
    await expect(contract.profile.locator("input")).toHaveCount(0);
    await expect(contract.profile.locator("textarea")).toHaveCount(0);

    const popupPromise = adminPage.waitForEvent("popup");
    await contract.openTencentButton.click();
    const popup = await popupPromise;
    await expect.poll(() => popup.url()).toContain("qian.tencent.com/one-use");
    await expect(contract.root).toBeVisible();
    expect(state.linkRequests).toHaveLength(1);
    expect(Object.keys(state.linkRequests[0] ?? {})).toEqual([
      "idempotencyKey",
    ]);
    await popup.close();
    await contract.screenshot("electronic-contract-onboarding-zh-cn");
  });

  test("TC-4a-2: 返回后主动检查权威状态并逐项显示授权", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();

    await contract.recheckButton.click();
    await expect(contract.root).toContainText("企业已开通");
    await expect(contract.root).toContainText("合同下载");
    await expect(contract.root).toContainText("已授权");
    await expect(contract.root).toContainText("企业自动签");
    await expect(contract.root).toContainText("未启用");
    expect(state.recheckRequests).toHaveLength(1);
    expect(Object.keys(state.recheckRequests[0] ?? {})).toEqual([
      "idempotencyKey",
    ]);
  });

  test("TC-4a-3: 未配置购买方腾讯应用时用中英文说明原因", async ({
    adminPage,
    mainLayout,
  }) => {
    await setElectronicContractPresentation(adminPage, "en-US", "dark");
    await installElectronicContractMock(adminPage, "not_configured", {
      locale: "en-US",
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await mainLayout.prepareThemeModeWithoutAnimation("dark");

    expect(await mainLayout.getThemeMode()).toBe("dark");
    await expect(contract.unavailable).toContainText(
      "Onboarding is temporarily unavailable",
    );
    await expect(contract.unavailable).toContainText(
      "not connected to the merchant's Tencent eSign application",
    );
    await expect(contract.openTencentButton).toBeDisabled();
    await expect(contract.root).toContainText("Check the existing merchant name, region and ID.");
    await expect(contract.root).not.toContainText(
      "plugin.idc-compliance",
    );
    await contract.screenshot("electronic-contract-unavailable-en-us-dark");
  });

  test("TC-4b-ordinary-personal: 普通个人订单无需合同时已付款后直接进入开通", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await adminPage.route(
      "**/x/idc-core/api/v1/account/orders/ORDER_PERSONAL_001",
      async (route) => {
        await route.fulfill({
          contentType: "application/json",
          json: {
            code: 0,
            data: {
              item: {
                fulfillmentStatusCode: "ready",
                orderNo: "ORDER_PERSONAL_001",
                paymentStatusCode: "paid",
                requiresContract: false,
              },
            },
            message: "OK",
          },
        });
      },
    );
    await adminPage.goto("/", { waitUntil: "domcontentloaded" });
    const item = await adminPage.evaluate(async () =>
      fetch("/x/idc-core/api/v1/account/orders/ORDER_PERSONAL_001")
        .then((response) => response.json())
        .then((response) => response.data.item),
    );

    expect(item).toMatchObject({
      fulfillmentStatusCode: "ready",
      paymentStatusCode: "paid",
      requiresContract: false,
    });
  });

  test("TC-4c-signer-order: 签署人或订单归属不匹配时拒绝创建合同", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    const requests: Array<Record<string, unknown>> = [];
    await adminPage.route(
      "**/x/idc-compliance/api/v1/admin/compliance/electronic-contracts",
      async (route) => {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        requests.push(body);
        const matches =
          body.orderNo === "ORDER_ENTERPRISE_001" &&
          body.signerNo === "SIGNER_ENTERPRISE_001";
        await route.fulfill({
          contentType: "application/json",
          json: matches
            ? {
                code: 0,
                data: { item: { statusCode: "pending_launch" } },
                message: "OK",
              }
            : {
                code: "CONTRACT_SIGNER_MISMATCH",
                message: "signer does not belong to the order customer",
              },
          status: matches ? 200 : 409,
        });
      },
    );
    await adminPage.goto("/", { waitUntil: "domcontentloaded" });
    const statuses = await adminPage.evaluate(async () => {
      const create = (orderNo: string, signerNo: string) =>
        fetch(
          "/x/idc-compliance/api/v1/admin/compliance/electronic-contracts",
          {
            body: JSON.stringify({ orderNo, signerNo }),
            headers: { "Content-Type": "application/json" },
            method: "POST",
          },
        ).then((response) => response.status);
      return Promise.all([
        create("ORDER_ENTERPRISE_001", "SIGNER_OTHER_CUSTOMER"),
        create("ORDER_OTHER_CUSTOMER", "SIGNER_ENTERPRISE_001"),
      ]);
    });

    expect(statuses).toEqual([409, 409]);
    expect(requests).toHaveLength(2);
  });

  test("TC-4b-0: 模板列表只读本地数据，只有管理员手工点击才同步腾讯", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateManager();

    await expect(contract.contractTemplateManager).toContainText(
      "Tencent Disabled Template",
    );
    await expect(contract.contractTemplateManager).toContainText("腾讯已停用");
    await expect(contract.contractTemplateManager).toContainText("待补充设置");
    expect(state.templateSyncRequests).toHaveLength(0);

    await contract.searchTemplatesByName("Tencent Disabled");
    await expect(contract.contractTemplateManager).toContainText(
      "Tencent Disabled Template",
    );
    await expect(contract.contractTemplateManager).not.toContainText(
      "Tencent Settings Required",
    );
    const filteredURL = new URL(state.templateListRequests.at(-1)!);
    expect(filteredURL.searchParams.get("name")).toBe("Tencent Disabled");
    expect(filteredURL.searchParams.get("page")).toBe("1");
    expect(state.templateSyncRequests).toHaveLength(0);

    await contract.contractTemplateSync.click();
    await expect(adminPage.getByText("同步完成")).toBeVisible();
    expect(state.templateSyncRequests).toHaveLength(1);
    expect(state.templateListRequests.length).toBeGreaterThanOrEqual(3);
    await contract.screenshot("contract-template-directory-manual-sync-zh-cn");
  });

  test("TC-4b-0r: 只读人员可查看模板但看不到管理操作", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "ready", {
      manageTemplates: false,
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateManager();

    await expect(
      adminPage.getByTestId("contract-template-view").first(),
    ).toBeVisible();
    await expect(adminPage.getByTestId("contract-template-sync")).toHaveCount(
      0,
    );
    await expect(adminPage.getByTestId("contract-template-create")).toHaveCount(
      0,
    );
    await expect(adminPage.getByTestId("contract-template-edit")).toHaveCount(
      0,
    );
    await expect(adminPage.getByTestId("contract-template-copy")).toHaveCount(
      0,
    );
    await expect(adminPage.getByTestId("contract-template-enable")).toHaveCount(
      0,
    );
    await expect(
      adminPage.getByTestId("contract-template-disable"),
    ).toHaveCount(0);
    expect(state.templateSyncRequests).toHaveLength(0);
  });

  test("TC-4b-1: 中文后台从插件独立产物加载合同编辑器", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    const mainLayout = new MainLayout(adminPage);
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    const pathnameBeforeEdit = new URL(adminPage.url()).pathname;
    await contract.openTemplateEditor();

    expect(await mainLayout.getThemeMode()).toBe("light");
    await expect(adminPage.getByRole("dialog", { name: "编辑" })).toBeVisible();
    expect(new URL(adminPage.url()).pathname).toBe(pathnameBeforeEdit);
    await expect(contract.contractTemplateOnlineEditor).toContainText(
      "Merchant service agreement body",
    );
    await expect(contract.contractFieldEditor).toBeVisible();
    await expect(contract.contractTemplateEditor).not.toContainText(
      "plugin.idc-compliance",
    );
    expect(state.templateListRequests.length).toBeGreaterThanOrEqual(1);
    expect(state.templateEditorRequests).toHaveLength(1);
    expect(state.previewRequests).toHaveLength(1);
    await expect(contract.contractFieldEditor.locator("canvas")).toBeVisible();
    await contract.screenshot("contract-template-editor-zh-cn");
  });

  test("TC-4b-2: 英文暗色后台仍由插件独立产物加载合同编辑器", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    const mainLayout = new MainLayout(adminPage);
    await setElectronicContractPresentation(adminPage, "en-US", "dark");
    const state = await installElectronicContractMock(adminPage, "ready", {
      locale: "en-US",
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await mainLayout.prepareThemeModeWithoutAnimation("dark");
    await contract.openTemplateEditor();

    expect(await mainLayout.getThemeMode()).toBe("dark");
    await expect(adminPage.getByRole("dialog", { name: "Edit" })).toBeVisible();
    await expect(contract.contractTemplateOnlineEditor).toContainText(
      "Merchant service agreement body",
    );
    await expect(contract.contractFieldEditor).toBeVisible();
    await expect(contract.contractTemplateEditor).not.toContainText(
      "plugin.idc-compliance",
    );
    expect(state.templateListRequests.length).toBeGreaterThanOrEqual(1);
    expect(state.templateEditorRequests).toHaveLength(1);
    expect(state.previewRequests).toHaveLength(1);
    await expect(contract.contractFieldEditor.locator("canvas")).toBeVisible();
    await contract.screenshot("contract-template-editor-en-us-dark");
  });

  test("TC-4b-3: 本地草稿按保存正文、生成预览、确认预览和放置签章顺序操作", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateManager();
    await contract.searchTemplatesByName("Merchant service");
    await expect(contract.contractTemplateNameFilter).toHaveValue(
      "Merchant service",
    );

    await contract.contractTemplateEdit.first().click();
    await contract.contractTemplateEditor.waitFor({ state: "visible" });
    const onlineBody = contract.contractTemplateOnlineEditor.locator(
      '[contenteditable="true"]',
    );
    await onlineBody.fill("Updated merchant service agreement body");
    await expect(contract.contractTemplateSaveSource).toBeEnabled();
    await contract.contractTemplateSaveSource.click();
    await expect(
      adminPage.getByText("正文已保存为新的不可变修订"),
    ).toBeVisible();

    await expect(contract.contractTemplateGeneratePreview).toBeEnabled();
    await contract.contractTemplateGeneratePreview.click();
    await expect(adminPage.getByText("PDF 预览已生成")).toBeVisible();
    await expect(contract.contractTemplateConfirmPreview).toBeEnabled();
    await contract.contractTemplateConfirmPreview.click();
    await expect(adminPage.getByText("当前 PDF 预览已确认")).toBeVisible();

    await adminPage
      .getByTestId("contract-field-tool-signature-customer")
      .click();
    await adminPage
      .getByTestId("contract-field-tool-enterprise_seal-merchant")
      .click();
    await expect(
      contract.contractFieldEditor.locator(".placed-field"),
    ).toHaveCount(2);
    await expect(contract.contractTemplateSaveFields).toBeEnabled();
    await contract.contractTemplateSaveFields.click();
    await expect(adminPage.getByText("字段与签章位置已保存")).toBeVisible();

    await expect(contract.contractTemplateSubmitTencent).toBeDisabled();
    await expect(contract.contractTemplateEditor).toContainText(
      "不会伪造提交成功",
    );
    expect(state.templateOnlineSourceRequests).toHaveLength(1);
    expect(state.templatePreviewCreateRequests).toHaveLength(1);
    expect(state.templatePreviewConfirmationRequests).toHaveLength(1);
    expect(state.templateFieldSetRequests).toHaveLength(1);
    expect(
      (state.templateFieldSetRequests[0]?.fields as unknown[])?.length,
    ).toBe(2);

    await contract.contractTemplateSave.click();
    await contract.contractTemplateManager.waitFor({ state: "visible" });
    await expect(contract.contractTemplateNameFilter).toHaveValue(
      "Merchant service",
    );
  });

  test("TC-4b-4: 复制模板创建独立本地草稿并直接打开空白正文编辑器", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateManager();

    await contract.copyTemplate("Merchant service agreement");
    await expect(
      contract.contractTemplateEditor.getByTestId("contract-template-name"),
    ).toHaveValue("Merchant service agreement");
    await expect(
      contract.contractTemplateEditor.getByTestId("contract-template-scope"),
    ).toContainText("全部客户");
    await expect(
      contract.contractTemplateEditor.getByTestId("contract-template-order"),
    ).toContainText("客户先签");
    await expect(
      contract.contractTemplateEditor.getByTestId("contract-template-language"),
    ).toContainText("中文");
    await expect(contract.contractTemplateOnlineEditor).toHaveCount(0);
    await expect(contract.contractFieldEditor).toHaveCount(0);
    await expect(contract.contractTemplateEditor).not.toContainText(
      "Merchant service agreement body",
    );

    expect(state.templateCopyRequests).toHaveLength(1);
    expect(state.templateCopyRequests[0]?.sourceTemplateNo).toBe(
      "contract-template-local",
    );
    expect(state.templateCopyRequests[0]?.sourceVersionNo).toBe(1);
    expect(state.templateCopyRequests[0]?.idempotencyKey).toMatch(
      /^contract-template-copy-/u,
    );
    expect(state.templateEditorRequests.at(-1)).toContain(
      "/templates/contract-template-copy-1/versions/1/editor",
    );
    await contract.screenshot("contract-template-copy-local-draft-zh-cn");
  });

  test("TC-4b-5z: 中文启用新版本只确认一次并由服务端切换版本", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateManager();
    const revisedRow = contract.contractTemplateManager
      .locator("tbody tr")
      .filter({ hasText: "Tencent Revised Template" });
    const revisedEnable = contract.contractTemplateManager.locator(
      '[data-testid="contract-template-enable"][data-template-no="contract-template-tencent-revised"]',
    );

    await revisedEnable.click();
    let dialog = adminPage.getByRole("dialog");
    await expect(dialog).toContainText("确认启用这个版本？");
    await expect(dialog).toContainText(
      "同一模板原来启用的版本会自动停用，历史合同不会改变。",
    );
    await dialog.getByRole("button", { name: /取\s*消/u }).click();
    await expect(dialog).toBeHidden();
    expect(state.templateEnablementRequests).toHaveLength(0);

    await revisedEnable.click();
    dialog = adminPage.getByRole("dialog");
    await dialog.getByRole("button", { name: /启\s*用/u }).click();
    await expect(adminPage.getByText("模板版本已启用")).toBeVisible();
    expect(state.templateEnablementRequests).toHaveLength(1);
    expect(state.templateEnablementRequests[0]).toMatchObject({
      expectedVersion: 3,
      templateNo: "contract-template-tencent-revised",
      versionNo: 2,
    });
    await expect(revisedRow).toContainText("已启用");
    await expect(
      revisedEnable,
    ).toHaveCount(0);
    await expect(
      contract.contractTemplateManager.locator(
        '[data-testid="contract-template-disable"][data-template-no="contract-template-tencent-revised"]',
      ),
    ).toBeVisible();
  });

  test("TC-4b-5e: 英文启用确认说明自动停用旧版本且只提交一次", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    const mainLayout = new MainLayout(adminPage);
    await setElectronicContractPresentation(adminPage, "en-US", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await mainLayout.switchLanguage("English");
    await contract.openTemplateManager();
    const revisedRow = contract.contractTemplateManager
      .locator("tbody tr")
      .filter({ hasText: "Tencent Revised Template" });
    const revisedEnable = contract.contractTemplateManager.locator(
      '[data-testid="contract-template-enable"][data-template-no="contract-template-tencent-revised"]',
    );

    await revisedEnable.click();
    const dialog = adminPage.getByRole("dialog");
    await expect(dialog).toContainText("Enable this version?");
    await expect(dialog).toContainText(
      "The previously enabled version of the same template will be disabled automatically. Historical contracts will not change.",
    );
    await dialog.getByRole("button", { name: "Enable", exact: true }).click();
    await expect(adminPage.getByText("Template version enabled")).toBeVisible();
    expect(state.templateEnablementRequests).toHaveLength(1);
    await expect(revisedRow).toContainText("Enabled");
    await expect(contract.contractTemplateManager).not.toContainText(
      "plugin.idc-compliance",
    );
  });

  test("TC-4b-6: 腾讯模板位置和双方签署角色全部明确对应后才可确认", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage);
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openTemplateEditorByName("Tencent Mapping Template");

    await expect(contract.contractTemplateProviderMapping).toContainText(
      "对应腾讯填写位置和签署人",
    );
    await expect(contract.contractTemplateSaveProviderMapping).toBeDisabled();

    const chooseMapping = async (
      testId: string,
      option: string,
      optionOffset = 0,
    ) => {
      const row = adminPage.getByTestId(testId);
      await row.locator(".ant-select-selector").click();
      const input = row.locator('input[role="combobox"]');
      for (let index = 0; index < optionOffset; index += 1) {
        await input.press("ArrowDown");
      }
      await input.press("Enter");
      await expect(row.locator(".ant-select-selection-item")).toHaveText(
        option,
      );
    };

    await chooseMapping(
      "contract-template-provider-component-contract-number",
      "合同编号",
    );
    await chooseMapping(
      "contract-template-provider-component-service-details",
      "服务明细表",
    );
    await chooseMapping(
      "contract-template-provider-component-customer-signing-date",
      "签署日期",
    );
    await chooseMapping(
      "contract-template-provider-component-merchant-signing-date",
      "签署日期",
    );
    await chooseMapping(
      "contract-template-provider-recipient-customer",
      "客户",
    );
    await expect(contract.contractTemplateSaveProviderMapping).toBeDisabled();
    await chooseMapping(
      "contract-template-provider-recipient-merchant",
      "商家",
      1,
    );

    await expect(contract.contractTemplateSaveProviderMapping).toBeEnabled();
    await contract.contractTemplateSaveProviderMapping.click();
    await expect(
      adminPage.getByText("腾讯填写位置和签署人对应关系已确认"),
    ).toBeVisible();

    expect(state.templateProviderMappingRequests).toHaveLength(1);
    expect(state.templateProviderMappingRequests[0]).toMatchObject({
      expectedVersion: 4,
      fieldMappings: [
        { componentRef: "contract-number", valueCode: "contract_no" },
        { componentRef: "service-details", valueCode: "service_lines" },
        {
          componentRef: "customer-signing-date",
          valueCode: "signing_date",
        },
        {
          componentRef: "merchant-signing-date",
          valueCode: "signing_date",
        },
      ],
      recipientMappings: [
        { partyCode: "customer", recipientRef: "customer" },
        { partyCode: "merchant", recipientRef: "merchant" },
      ],
      templateNo: "contract-template-tencent-mapping",
      versionNo: 1,
    });
    expect(state.templateProviderMappingRequests[0]?.idempotencyKey).toMatch(
      /^contract-template-provider-mapping-/u,
    );
    await expect(contract.contractTemplateSaveProviderMapping).toHaveCount(0);
    await expect(contract.contractTemplateProviderMapping).toContainText(
      "可以返回列表启用此版本",
    );
    await contract.screenshot("contract-template-provider-mapping-zh-cn");
  });

  test("TC-4c-1: 自动签授权没有独立权限时整个页签不可见", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    await installElectronicContractMock(adminPage, "active", {
      manageAutoSign: false,
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();

    await expect(
      adminPage.getByRole("tab", { name: "自动签授权" }),
    ).toHaveCount(0);
    await expect(contract.autoSignManager).toHaveCount(0);
  });

  test("TC-4c-2: 中文管理员核对完整摘要后启用并可立即撤销", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openAutoSignManager();

    await expect(contract.autoSignManager).toContainText(
      "自动签是高风险企业授权",
    );
    await expect(contract.autoSignRequired).toBeChecked();
    await expect(contract.autoSignOptional).not.toBeChecked();
    await contract.autoSignTemplates.click();
    await adminPage
      .locator(".ant-select-item-option")
      .filter({ hasText: "Enterprise service agreement · v3" })
      .click();
    await contract.autoSignAmount.fill("1000.50");
    await contract.autoSignLimit.fill("25");
    const expiry = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    await contract.autoSignExpiry.locator("input").click();
    const expiryCell = adminPage.locator(`.ant-picker-cell[title="${expiry}"]`);
    if (!(await expiryCell.isVisible())) {
      await adminPage.locator(".ant-picker-header-next-btn").click();
    }
    await expiryCell.click();
    await adminPage.locator(".ant-picker-ok button").click();
    await expect(contract.autoSignEnable).toBeEnabled();
    await contract.autoSignEnable.click();

    let dialog = adminPage.getByRole("dialog");
    await expect(dialog).toContainText("确认启用企业自动签？");
    await expect(dialog).toContainText("Enterprise service agreement · v3");
    await expect(dialog).toContainText("必须签署的订单合同");
    await expect(dialog).not.toContainText("后台人工发起的可选合同");
    await expect(dialog).toContainText("授权到期时间");
    await expect(dialog).toContainText(expiry.slice(0, 4));
    await expect(dialog).toContainText("CNY");
    await expect(dialog).toContainText("CNY 1,000.50");
    await expect(dialog).toContainText("成功发起次数上限");
    await expect(dialog).toContainText("25");
    await expect(dialog).toContainText("启用后不能编辑");
    await contract.screenshot("contract-auto-sign-confirmation-zh-cn");
    await dialog
      .getByRole("button", { name: /核\s*对\s*并\s*启\s*用/u })
      .click();

    await expect(adminPage.getByText("企业自动签授权已启用")).toBeVisible();
    await expect(contract.autoSignManager).toContainText("生效中");
    await expect(contract.autoSignManager).toContainText(
      "Enterprise service agreement · v3",
    );
    expect(state.autoSignCreateRequests).toHaveLength(1);
    expect(state.autoSignCreateRequests[0]).toMatchObject({
      allowManualOptional: false,
      allowRequiredOrder: true,
      confirmed: true,
      currencyCode: "CNY",
      maxAmountMinor: "100050",
      successLimit: 25,
      templates: [{ templateNo: "contract-template-enterprise", versionNo: 3 }],
    });

    await contract.autoSignRevoke.click();
    dialog = adminPage.getByRole("dialog");
    await expect(dialog).toContainText("确认立即撤销？");
    await expect(dialog).toContainText("人工签署、已经发起的合同");
    await dialog.getByRole("button", { name: /立\s*即\s*撤\s*销/u }).click();
    await expect(adminPage.getByText("企业自动签授权已撤销")).toBeVisible();
    await expect(contract.autoSignManager).toContainText("已撤销");
    await expect(contract.autoSignRevoke).toHaveCount(0);
    await expect(dialog).toBeHidden();
    expect(state.autoSignRevokeRequests).toHaveLength(1);
    expect(state.autoSignRevokeRequests[0]).toMatchObject({
      authorizationNo: "casa_mock_1",
      confirmed: true,
      expectedVersion: 1,
    });
    await contract.screenshot("contract-auto-sign-authorization-zh-cn");
  });

  test("TC-4c-3: 英文暗色页面完整说明限制且不显示翻译键", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    const mainLayout = new MainLayout(adminPage);
    await setElectronicContractPresentation(adminPage, "en-US", "dark");
    await installElectronicContractMock(adminPage, "active", {
      locale: "en-US",
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await mainLayout.prepareThemeModeWithoutAnimation("dark");
    await contract.openAutoSignManager();

    expect(await mainLayout.getThemeMode()).toBe("dark");
    await expect(contract.autoSignManager).toContainText(
      "Auto-sign is a high-risk enterprise authorization",
    );
    await expect(contract.autoSignManager).toContainText(
      "Personal auto-sign is always disabled",
    );
    await expect(contract.autoSignRequired).toBeChecked();
    await expect(contract.autoSignOptional).not.toBeChecked();
    await expect(contract.autoSignManager).not.toContainText(
      "plugin.idc-compliance",
    );
    await expect(adminPage.getByText("Loading Menu...")).toBeHidden();
    await contract.screenshot("contract-auto-sign-authorization-en-us-dark");
  });

  test("TC-4d-0: 没有合同查看权限时不显示合同管理页签", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    await installElectronicContractMock(adminPage, "active", {
      manageContracts: false,
    });
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();

    await expect(adminPage.getByRole("tab", { name: "合同管理" })).toHaveCount(
      0,
    );
    await expect(contract.contractManager).toHaveCount(0);
  });

  test("TC-4e: 合同详情显示权威流程、不可变文件和未购买报告的明确原因", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();
    await contract.openContractDetail("CTR_REJECTED_001");

    await expect(contract.contractDetail).toContainText("已拒签");
    await contract.openContractDetailSection(/文件与证据|Files and evidence/u);
    await expect(contract.contractDetail).toContainText("最终合同 PDF");
    await expect(contract.contractDetail).toContainText("安全归档");
    await expect(contract.contractDetail).toContainText(
      "当前商家没有启用签署报告服务",
    );
    expect(state.contractListRequests.length).toBeGreaterThanOrEqual(1);
    await contract.screenshot("contract-management-evidence-zh-cn");
  });

  test("TC-4d-lifecycle: 拒签、过期和撤销保持不同状态且不能互相冒充", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await installElectronicContractMock(adminPage, "active");
    await adminPage.goto("/", { waitUntil: "domcontentloaded" });
    const items = await adminPage.evaluate(async () =>
      fetch(
        "/x/idc-compliance/api/v1/admin/compliance/electronic-contracts",
      )
        .then((response) => response.json())
        .then((response) => response.data.items),
    );
    const statusByNumber = Object.fromEntries(
      items.map((item: { contractNo: string; statusCode: string }) => [
        item.contractNo,
        item.statusCode,
      ]),
    );

    expect(statusByNumber).toMatchObject({
      CTR_EXPIRED_001: "expired",
      CTR_REJECTED_001: "rejected",
      CTR_REVOKED_001: "revoked",
    });
  });

  test("TC-4d-failure-compensation: 后台可设置处理期限并明确发起退款不等于钱已退回", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();

    const policy = adminPage.getByTestId("contract-failure-policy");
    await expect(policy).toContainText("必签合同失败后的处理期限");
    await expect(policy).toContainText(
      "已经失败的合同继续按当时记下的截止时间",
    );
    const days = adminPage.getByTestId("contract-failure-policy-days");
    await days.fill("5");
    await adminPage.getByTestId("contract-failure-policy-save").click();
    let dialog = adminPage.getByRole("dialog").last();
    await expect(dialog).toContainText("只影响以后发生的必签合同失败");
    await dialog.getByRole("button", { name: /确认保存/u }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      adminPage.getByText("处理期限已保存，已失败合同的截止时间没有改变"),
    ).toBeVisible();
    expect(state.contractFailurePolicyRequests).toHaveLength(1);
    expect(state.contractFailurePolicyRequests[0]).toMatchObject({
      expectedVersion: 1,
      handlingDays: 5,
    });
    await contract.screenshot("contract-failure-policy-saved-zh-cn");

    await contract.openContractDetail("CTR_REQUIRED_FAILED_001");
    await contract.contractDetail
      .getByTestId("contract-failure-cancel-order")
      .click();
    dialog = adminPage
      .getByRole("dialog")
      .filter({ hasText: "已发起退款" });
    await expect(dialog).toContainText("已发起退款");
    await expect(dialog).toContainText("钱已退回");
    await dialog.getByRole("button", { name: /确认取消并发起退款/u }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      adminPage.getByText(
        "订单已停止，退款申请已交给支付通道，钱款是否退回仍需等待支付结果",
      ),
    ).toBeVisible();
    await contract.screenshot("contract-failure-refund-pending-zh-cn");
    expect(state.contractFailureCancellationRequests).toEqual([
      { confirmed: true, contractNo: "CTR_REQUIRED_FAILED_001" },
    ]);
    await expect(
      contract.contractDetail.getByTestId("contract-failure-cancel-order"),
    ).toBeDisabled();
  });

  test("TC-4d-2: 可选合同先明确永久不影响开通，再生成预览并单独提交", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();
    await contract.openOptionalContractCreate();

    const createDialog = adminPage.getByRole("dialog");
    await expect(createDialog).toContainText("永久保持“无需合同”");
    await expect(createDialog).toContainText("不会同时提交腾讯");
    await contract.completeOptionalPreview(
      "ORDER_OPTIONAL_001",
      "Enterprise service agreement",
      "张*",
    );
    await expect(contract.contractPreview).toContainText("预览已经固定");
    expect(state.contractCreateRequests).toHaveLength(1);
    expect(state.contractSubmissionRequests).toHaveLength(0);

    await contract.submitVisiblePreview();
    expect(state.contractSubmissionRequests).toHaveLength(1);
    expect(state.contractSubmissionRequests[0]).toMatchObject({
      contractNo: "CTR_OPTIONAL_PREVIEW_001",
      expectedVersion: 1,
    });
  });

  test("TC-4d-3: 内容不变时在同一合同追加新的七天签署流程", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();
    await contract.openContractDetail("CTR_REJECTED_001");
    await contract.reissueUnchanged();

    expect(state.contractReissueRequests).toHaveLength(1);
    expect(state.contractReissueRequests[0]).toMatchObject({
      contractNo: "CTR_REJECTED_001",
      expectedVersion: 3,
    });
    await expect(adminPage.getByText("合同已经按原内容重新发起")).toBeVisible();
  });

  test("TC-4d-4: 内容变化时保留原合同，先生成新版本预览再单独提交", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();
    await contract.openContractDetail("CTR_REJECTED_001");
    await contract.openChangedReplacementCreate();

    const changedDialog = adminPage.getByRole("dialog", {
      name: "生成新的合同版本",
    });
    await expect(changedDialog).toContainText("原合同不会被覆盖");
    await expect(changedDialog).toContainText("这里只生成预览");
    await contract.completeChangedReplacementPreview(
      "Enterprise service agreement",
      "张*",
    );
    await expect(contract.contractPreview).toContainText(
      "CTR_REPLACEMENT_PREVIEW_001",
    );
    expect(state.contractReplacementPreviewRequests).toHaveLength(1);
    expect(state.contractReissueRequests).toHaveLength(0);
    await contract.screenshot("contract-changed-reissue-preview-zh-cn");

    await contract.submitVisiblePreview();
    expect(state.contractReissueRequests).toHaveLength(1);
    expect(state.contractReissueRequests[0]).toMatchObject({
      contractNo: "CTR_REPLACEMENT_PREVIEW_001",
      expectedVersion: 1,
    });
  });

  test("TC-4d-5: 解除合同必须填写原因并再次确认，原合同证据继续保留", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await setElectronicContractPresentation(adminPage, "zh-CN", "light");
    const state = await installElectronicContractMock(adminPage, "active");
    const contract = new ElectronicContractPage(adminPage);
    await contract.goto();
    await contract.openContractManager();
    await contract.openContractDetail("CTR_COMPLETED_001");
    await contract.openTerminationCreate();

    const terminationDialog = adminPage.getByRole("dialog", {
      name: "填写合同解除内容",
    });
    await expect(terminationDialog).toContainText("固定沿用原合同签署人");
    await expect(
      terminationDialog.getByRole("button", { name: "检查解除内容" }),
    ).toBeDisabled();
    await contract.submitTermination("双方协商一致解除");
    expect(state.contractTerminationRequests).toHaveLength(1);
    expect(state.contractTerminationRequests[0]).toMatchObject({
      confirmed: true,
      contractNo: "CTR_COMPLETED_001",
      expectedVersion: 5,
      reason: "双方协商一致解除",
    });
    await expect(adminPage.getByText("合同解除流程已经提交")).toBeVisible();
  });
});
