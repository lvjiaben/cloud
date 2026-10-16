import { expect, test } from "../support/catalog-auth";

import { CatalogAutomationPage } from "../pages/CatalogAutomationPage";
import { CatalogManagementPage } from "../pages/CatalogManagementPage";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { installCatalogConfigurationMock } from "../support/catalog-configuration-mock";
import { installCatalogManagementMock } from "../support/catalog-mock";

const internalText =
  /hidden-interface|contract-v1|release-v1|selectionToken|interfaceInstanceNo|\{"/iu;
const viewPermission = "idc:admin:catalog:automation-interface:view";
const managePermission = "idc:admin:catalog:automation-interface:manage";
const bindingWritePermission = "idc:admin:catalog:fulfillment-binding:write";
const productQueryPermission = "idc:admin:catalog:product:query";

test.describe("TC-14 模拟数据：商品自动化前端契约", () => {
  test("TC-14-ssl-capabilities: 未确认的证书套餐显示人工交付提示并拒绝保存", async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage, { fulfillmentBinding: {
      boundAt: 1_787_705_720_000, evidenceVerified: true,
      interfaceDisplayName: "云速证书接口", interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    } });
    await installCatalogAutomationMock(adminPage);
    let writes = 0;
    let evaluations = 0;
    const hint = "尚未确认供应商支持的域名类型与数量，暂不能自动发布，请改用人工交付。";
    await adminPage.route("**/admin/catalog/product-versions/*/configuration", async route => {
      if (route.request().method() === "PUT") writes += 1;
      await route.fulfill({ json: { code: 0, data: { configuration: {
        version: 0, merchantValues: [{ fieldKey: "plan_id", optionTokens: ["plan-1"] }], productResolvedCandidates: {},
        customerDeclaration: { version: "v1", fields: [] },
        productDeclaration: { version: "v1", fields: [{
          key: "plan_id", type: "single_select", required: true,
          label: { zhCN: "供应商证书产品", enUS: "Supplier certificate product" },
          description: { zhCN: hint, enUS: "Automatic publication is blocked; use manual delivery." },
          staticCandidates: [{ value: "plan-1", label: { zhCN: "测试证书", enUS: "Test certificate" } }],
        }] },
      } }, message: "success" } });
    });
    await adminPage.route("**/admin/catalog/product-versions/*/configuration/evaluation", async route => {
      evaluations += 1;
      await route.fulfill({ json: { code: 400, data: null, message: hint } });
    });
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.openMerchantConfiguration();
    const dialog = catalog.merchantConfigurationDialog();
    await expect(dialog).toContainText("供应商证书产品");
    await expect(dialog).toContainText(hint);
    await expect.poll(() => evaluations).toBeGreaterThan(0);
    await expect(dialog).toContainText("暂时无法核对最新配置，核对成功前不能保存。");
    await dialog.getByRole("button", { name: /确\s*认/u }).click();
    await expect(dialog).toBeVisible();
    expect(writes).toBe(0);
    await expect(dialog).not.toContainText("plugin.idc-core");
    await catalog.screenshot("ssl-capabilities-blocked");
  });
  test("TC-14-generation: 服务器生成规则保存精确配置且越界零写入", async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage, { fulfillmentBinding: {
      boundAt: 1_787_705_720_000, evidenceVerified: true,
      interfaceDisplayName: "服务器接口", interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    } });
    await installCatalogAutomationMock(adminPage);
    const state = await installCatalogConfigurationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.openMerchantConfiguration();
    const dialog = catalog.merchantConfigurationDialog();
    await expect(dialog).toContainText("主机名总长度");
    await expect(dialog).toContainText("自动生成密码长度");
    await catalog.fillMerchantField("商品整数", "1");
    await catalog.fillMerchantField("商品精确小数", "1.00");
    await catalog.fillMerchantField("主机名前缀", "Xxun");
    await catalog.fillMerchantField("主机名总长度", "64");
    await catalog.fillMerchantField("自动生成密码长度", "12");
    await catalog.selectMerchantCharacter("大写字母");
    await catalog.selectMerchantCharacter("数字");
    await catalog.screenshot("server-generation-rules");
    await catalog.saveMerchantConfiguration();
    await expect(dialog).toBeVisible();
    expect(state.productConfigurationRequests.filter(row => row.method === "PUT")).toHaveLength(0);
    await catalog.fillMerchantField("主机名总长度", "12");
    await catalog.saveMerchantConfiguration();
    await expect(dialog).not.toBeVisible();
    const writes = state.productConfigurationRequests.filter(row => row.method === "PUT");
    expect(writes).toHaveLength(1);
    expect(writes[0]?.body).toMatchObject({ expectedVersion: 0, merchantValues: expect.arrayContaining([
      { fieldKey: "hostname_prefix", textValue: "Xxun" },
      { fieldKey: "hostname_length", integerValue: "12" },
      { fieldKey: "password_length", integerValue: "12" },
      { fieldKey: "password_characters", optionTokens: ["uppercase", "digits"] },
    ]) });
    await catalog.screenshot("server-generation-saved");
  });
  test("TC-14-import: 导入配置明确只读且保留本站价格入口", async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage, { fulfillmentBinding: {
      boundAt: 1_787_705_720_000, evidenceVerified: true,
      interfaceDisplayName: "上游商品接口", interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    }});
    await installCatalogAutomationMock(adminPage);
    const state = await installCatalogConfigurationMock(adminPage, true);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.selectEditSection("购买配置");
    const notice = adminPage.getByTestId("catalog-imported-configuration-notice");
    await expect(notice).toContainText("需要更新上游配置时，请重新导入商品");
    await expect(notice).toContainText("本站售价仍可在价格设置中修改");
    await expect(catalog.detailDrawer.getByRole("button", { name: "编辑配置", exact: true })).toHaveCount(0);
    expect(state.productConfigurationRequests.every((request) => request.method === "GET")).toBe(true);
    await catalog.screenshot("catalog-imported-configuration-readonly");
  });
  test("TC-14a: 模拟前端契约使用业务名称、中文状态和服务端筛选", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage);
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await expect(automation.row("证书供应商 A 接口")).toContainText(
      "接收新销售",
    );
    await expect(automation.row("手机供应商 A 接口")).toContainText(
      "已停止新销售",
    );
    const referencedDelete = adminPage.getByTestId(
      "automation-interface-delete-hidden-interface-phone",
    );
    await expect(referencedDelete).toBeDisabled();
    await referencedDelete.hover();
    await expect(adminPage.getByRole("tooltip")).toContainText(
      "已经被商品版本引用",
    );
    expect(
      state.interfaceRequests.filter((item) => item.method === "DELETE"),
    ).toHaveLength(0);
    await expect(automation.pageRoot).not.toContainText(internalText);
    const tableHeight = await automation.pageRoot
      .locator(".vxe-table")
      .first()
      .evaluate((element) => element.getBoundingClientRect().height);
    expect(tableHeight).toBeGreaterThan(260);

    await adminPage
      .getByTestId("automation-interface-search-name")
      .fill("证书");
    await automation.pageRoot
      .getByRole("button", { name: /搜\s*索|Search/iu })
      .click();
    await expect
      .poll(() => {
        const request = state.interfaceRequests
          .filter((item) => item.method === "GET")
          .at(-1);
        return request?.url.searchParams.get("namePrefix");
      })
      .toBe("证书");
    await automation.waitForDataSettled();
    await expect(automation.row("证书供应商 A 接口")).toBeVisible();
    await expect(automation.row("手机供应商 A 接口")).toHaveCount(0);
    await automation.screenshot("catalog-automation-interface-list");
  });

  test("TC-14b: 模拟前端契约覆盖候选分页且短期令牌不落本地", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage);
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await adminPage.getByTestId("automation-interface-create-open").click();
    await automation.createModal.waitFor({ state: "visible" });
    const pagination = adminPage.getByTestId(
      "automation-interface-provider-pagination",
    );
    await expect(pagination).toBeVisible();
    await pagination.getByTitle("6").click();
    await automation.screenshot("catalog-automation-provider-modal");
    await adminPage.getByTestId("automation-interface-provider").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("手机供应商插件", { exact: true })
      .click();
    await automation.createModal
      .getByTestId("automation-interface-display-name")
      .fill("手机供应商 B 接口");
    await adminPage
      .getByRole("dialog")
      .filter({ has: automation.createModal })
      .getByRole("button", { name: "创建接口" })
      .click();
    await automation.createModal.waitFor({ state: "hidden" });

    const createRequest = state.interfaceRequests.find(
      (item) => item.method === "POST",
    );
    expect(createRequest?.body).toEqual({
      configuration: [],
      displayName: "手机供应商 B 接口",
      selectionToken: "short-lived-provider-101",
    });
    expect(state.providerRequests.at(-1)?.searchParams.get("page")).toBe("6");
    expect(state.providerRequests.at(-1)?.searchParams.get("pageSize")).toBe(
      "20",
    );
    const storedValues = await adminPage.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    expect(storedValues.join(" ")).not.toContain("short-lived-provider-101");
    await expect(automation.row("手机供应商 B 接口")).toBeVisible();
    await automation.screenshot("catalog-automation-create-submitted");
  });

  test("TC-14c: 模拟前端契约的改名、停售和恢复保护历史业务", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage);
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await adminPage
      .getByTestId("automation-interface-rename-hidden-interface-certificate")
      .click();
    const renameModal = adminPage.getByTestId(
      "automation-interface-rename-modal",
    );
    await expect(renameModal).toContainText("历史商品版本和订单仍保留");
    await renameModal
      .getByTestId("automation-interface-rename-name")
      .fill("证书供应商新名称");
    await adminPage
      .getByRole("dialog")
      .filter({ has: renameModal })
      .getByRole("button", { name: "保存名称" })
      .click();
    await expect(automation.row("证书供应商新名称")).toBeVisible();

    await adminPage
      .getByTestId("automation-interface-stop-hidden-interface-certificate")
      .click();
    const stopConfirm = adminPage.locator(".ant-popconfirm:visible");
    await expect(stopConfirm).toContainText(
      "现有服务、续费、售后和历史记录继续保留",
    );
    await stopConfirm.getByRole("button", { name: "确认停售" }).click();
    await expect(automation.row("证书供应商新名称")).toContainText(
      "已停止新销售",
    );

    const stopRequest = state.interfaceRequests.find(
      (item) =>
        item.method === "PUT" &&
        item.url.pathname.endsWith("/new-business-state"),
    );
    expect(stopRequest?.body).toMatchObject({
      expectedVersion: 4,
      newBusinessState: "stopped",
    });

    await adminPage
      .getByTestId("automation-interface-resume-hidden-interface-certificate")
      .click();
    const resumeConfirm = adminPage.locator(".ant-popconfirm:visible");
    await expect(resumeConfirm).toContainText("当前插件的履约证据");
    await expect(resumeConfirm).not.toContainText("接口配置");
    await resumeConfirm.getByRole("button", { name: "确认恢复" }).click();
    await expect(automation.row("证书供应商新名称")).toContainText(
      "接收新销售",
    );
    const stateRequests = state.interfaceRequests.filter(
      (item) =>
        item.method === "PUT" &&
        item.url.pathname.endsWith("/new-business-state"),
    );
    expect(stateRequests.at(-1)?.body).toMatchObject({
      expectedVersion: 5,
      newBusinessState: "accepting",
    });

    await adminPage
      .getByTestId("automation-interface-delete-hidden-interface-certificate")
      .click();
    await adminPage
      .locator(".ant-popconfirm:visible")
      .getByRole("button", { name: "确认删除" })
      .click();
    await expect(automation.row("证书供应商新名称")).toHaveCount(0);
    expect(
      state.interfaceRequests.filter((item) => item.method === "DELETE"),
    ).toHaveLength(1);
  });

  test("TC-14d: 模拟前端契约的商品草稿绑定只提交接口引用", async ({
    adminPage,
  }) => {
    await installCatalogManagementMock(adminPage);
    const state = await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");

    await catalog.selectEditSection('交付设置');
    await catalog.detailDrawer
      .getByRole("button", { name: "绑定接口" })
      .click();
    const bindingModal = adminPage.getByTestId(
      "catalog-fulfillment-binding-modal",
    );
    await bindingModal.waitFor({ state: "visible" });
    await expect(bindingModal).toContainText("连接器类型");
    await expect(bindingModal).toContainText("自动化接口");
    await expect(bindingModal).not.toContainText(
      "plugin.idc-core.catalogAutomation.binding.provider",
    );
    await expect(
      adminPage.getByTestId("catalog-fulfillment-interface"),
    ).toHaveClass(/ant-select-disabled/u);
    const providerPagination = adminPage.getByTestId(
      "catalog-fulfillment-provider-pagination",
    );
    await expect(providerPagination).toBeVisible();
    await providerPagination.getByTitle("6").click();
    await adminPage.getByTestId("catalog-fulfillment-provider").click();
    await expect(
      adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText("手机供应商插件", { exact: true }),
    ).toBeVisible();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("手机供应商插件", { exact: true })
      .click();
    await providerPagination.getByTitle("1").click();
    await adminPage.getByTestId("catalog-fulfillment-provider").click();
    const providerDropdown = adminPage.locator(".ant-select-dropdown:visible");
    await providerDropdown.getByText("证书供应商插件", { exact: true }).click();
    await expect(providerDropdown).toBeHidden();
    await expect(bindingModal).not.toContainText("Required");
    await expect(bindingModal).not.toContainText(
      "plugin.idc-core.catalogAutomation.binding",
    );
    await catalog.screenshot("catalog-fulfillment-two-stage-binding");
    const pagination = adminPage.getByTestId(
      "catalog-fulfillment-interface-pagination",
    );
    await expect(pagination).toBeVisible();
    await pagination.getByTitle("7").click();
    await adminPage.getByTestId("catalog-fulfillment-interface").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText(/批量供应商接口 120/iu)
      .click();
    await adminPage
      .getByRole("dialog")
      .filter({ has: bindingModal })
      .getByRole("button", { name: "确认绑定" })
      .click();
    await bindingModal.waitFor({ state: "hidden" });
    expect(state.bindingRequests[0]).toEqual({
      body: { interfaceInstanceNo: "hidden-interface-bulk-120" },
      method: "PUT",
    });
    const candidateRequest = state.interfaceRequests
      .filter(
        (item) =>
          item.method === "GET" &&
          item.url.searchParams.get("newBusinessState") === "accepting",
      )
      .at(-1);
    expect(candidateRequest?.url.searchParams.get("page")).toBe("7");
    expect(candidateRequest?.url.searchParams.get("pageSize")).toBe("20");
    expect(
      candidateRequest?.url.searchParams.get("providerSelectionToken"),
    ).toBe("short-lived-provider-1");
    expect(
      state.providerRequests.some(
        (request) => request.searchParams.get("page") === "6",
      ),
    ).toBe(true);
  });

  test("TC-14e: 模拟前端契约保留已发布商品的停售接口摘要", async ({
    adminPage,
  }) => {
    await installCatalogManagementMock(adminPage, {
      fulfillmentBinding: {
        boundAt: 1_787_705_720_000,
        evidenceVerified: true,
        interfaceDisplayName: "历史证书供应商接口",
        interfaceInstanceNo: "hidden-interface-certificate",
        interfaceNewBusinessState: "stopped",
      },
    });
    await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-ssl");
    await catalog.selectEditSection("交付设置");

    await expect(catalog.detailDrawer).toContainText("历史证书供应商接口");
    await expect(catalog.detailDrawer).toContainText(
      "历史订单和已有服务继续使用原绑定",
    );
    await expect(
      catalog.detailDrawer.getByRole("button", { name: /更换接口|解除绑定/iu }),
    ).toHaveCount(0);
    await expect(catalog.detailDrawer).not.toContainText(internalText);
  });

  test("TC-14f: 模拟前端契约在英文窄屏没有中文或技术键", async ({
    adminPage,
    mainLayout,
  }) => {
    await installCatalogAutomationMock(adminPage);
    await mainLayout.switchLanguage("English");
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();
    await expect(
      adminPage.getByText("Automation Interfaces", { exact: true }).first(),
    ).toBeVisible();
    await adminPage.setViewportSize({ height: 844, width: 390 });

    await expect(automation.pageRoot).toContainText("Accepting new sales");
    await expect(automation.pageRoot).toContainText("New interface");
    await expect(automation.pageRoot).toContainText("Actions");
    await expect(automation.pageRoot).not.toContainText(
      /新增接口|停止新销售|新销售状态|接口名称|最近变更时间/iu,
    );
    await expect(automation.pageRoot).not.toContainText("plugin.idc-core");
    await automation.screenshot("catalog-automation-english-narrow");
  });

  test("TC-14g: 模拟前端契约可以解除草稿版本绑定", async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage, {
      fulfillmentBinding: {
        boundAt: 1_787_705_720_000,
        evidenceVerified: true,
        interfaceDisplayName: "证书供应商 A 接口",
        interfaceInstanceNo: "hidden-interface-certificate",
        interfaceNewBusinessState: "accepting",
      },
    });
    const state = await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.selectEditSection("交付设置");

    await catalog.detailDrawer
      .getByRole("button", { name: "解除绑定" })
      .click();
    await adminPage
      .locator(".ant-popconfirm:visible")
      .getByRole("button", { name: "确认解除" })
      .click();
    await expect
      .poll(() => state.bindingRequests.at(-1)?.method)
      .toBe("DELETE");
  });

  test("TC-14h: 模拟前端契约的只读权限隐藏全部管理操作", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage, {
      permissions: [viewPermission],
    });
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await expect(automation.row("证书供应商 A 接口")).toBeVisible();
    await expect(
      adminPage.getByTestId("automation-interface-create-open"),
    ).toHaveCount(0);
    await expect(
      adminPage.getByTestId(
        "automation-interface-rename-hidden-interface-certificate",
      ),
    ).toHaveCount(0);
    expect(
      state.interfaceRequests.filter((item) => item.method !== "GET"),
    ).toHaveLength(0);
  });

  test("TC-14i: 模拟前端契约禁止只有管理和绑定写权限的人读取或绑定", async ({
    adminPage,
  }) => {
    await installCatalogManagementMock(adminPage);
    const state = await installCatalogAutomationMock(adminPage, {
      permissions: [
        managePermission,
        bindingWritePermission,
        productQueryPermission,
      ],
    });
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await expect(
      adminPage.getByTestId("catalog-automation-interface-forbidden"),
    ).toBeVisible();
    await automation.screenshot("catalog-automation-permission-denied");
    expect(
      state.interfaceRequests.filter((item) => item.method === "GET"),
    ).toHaveLength(0);

    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await expect(
      catalog.detailDrawer.getByRole("button", { name: "绑定接口" }),
    ).toHaveCount(0);
  });

  test("TC-14j: 模拟前端契约只有查看和绑定写权限同时存在才显示绑定", async ({
    adminPage,
  }) => {
    await installCatalogManagementMock(adminPage);
    await installCatalogAutomationMock(adminPage, {
      permissions: [
        viewPermission,
        bindingWritePermission,
        productQueryPermission,
      ],
    });
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.selectEditSection("交付设置");

    await expect(
      catalog.detailDrawer.getByRole("button", { name: "绑定接口" }),
    ).toBeVisible();
  });

  test("TC-14k: 模拟短期候选失效时显示中文错误且不新增接口", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage, {
      failSelectionToken: true,
    });
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();
    await adminPage.getByTestId("automation-interface-create-open").click();
    await automation.createModal.waitFor({ state: "visible" });
    await adminPage
      .getByTestId("automation-interface-provider-pagination")
      .getByTitle("6")
      .click();
    await adminPage.getByTestId("automation-interface-provider").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("手机供应商插件", { exact: true })
      .click();
    await automation.createModal
      .getByTestId("automation-interface-display-name")
      .fill("失效候选不应创建");
    await automation.waitForDataSettled();
    await adminPage
      .getByRole("dialog")
      .filter({ has: automation.createModal })
      .getByRole("button", { name: "创建接口" })
      .click();

    await automation.screenshotWithBusinessError(
      "catalog-automation-selection-expired",
      "本次插件候选已经失效，请重新加载候选后再试",
    );
    await expect(automation.createModal).toBeVisible();
    await expect(automation.row("失效候选不应创建")).toHaveCount(0);
    expect(
      state.interfaceRequests.filter((item) => item.method === "POST"),
    ).toHaveLength(1);
  });

  test("TC-14l: 模拟删除竞态拒绝时显示中文错误且保留接口", async ({
    adminPage,
  }) => {
    const state = await installCatalogAutomationMock(adminPage, {
      failDeleteRace: true,
    });
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();
    await adminPage
      .getByTestId("automation-interface-delete-hidden-interface-certificate")
      .click();
    await adminPage
      .locator(".ant-popconfirm:visible")
      .getByRole("button", { name: "确认删除" })
      .click();

    await expect(
      adminPage.locator(".ant-message-notice-content").filter({
        hasText: "已被商品版本引用，必须永久保留",
      }),
    ).toBeVisible();
    await expect(adminPage.locator(".ant-popconfirm:visible")).toHaveCount(0);
    await expect(automation.row("证书供应商 A 接口")).toBeVisible();
    expect(
      state.interfaceRequests.filter((item) => item.method === "DELETE"),
    ).toHaveLength(1);
    await automation.screenshot("catalog-automation-delete-race-rejected");
  });

  test("TC-14m: 模拟恢复证据失败时显示英文错误且保持停售", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installCatalogAutomationMock(adminPage, {
      failResumeEvidence: true,
    });
    await mainLayout.switchLanguage("English");
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();
    await adminPage
      .getByTestId("automation-interface-resume-hidden-interface-phone")
      .click();
    await adminPage
      .locator(".ant-popconfirm:visible")
      .getByRole("button", { name: "Resume sales" })
      .click();

    await expect(
      adminPage.locator(".ant-message-notice-content").filter({
        hasText: "cannot be verified right now",
      }),
    ).toBeVisible();
    await expect(adminPage.locator(".ant-popconfirm:visible")).toHaveCount(0);
    await expect(automation.row("手机供应商 A 接口")).toContainText(
      "New sales stopped",
    );
    const resumeRequests = state.interfaceRequests.filter(
      (item) =>
        item.method === "PUT" &&
        item.url.pathname.endsWith("/new-business-state"),
    );
    expect(resumeRequests).toHaveLength(1);
    await automation.screenshot("catalog-automation-resume-evidence-rejected");
  });

  test("TC-14n: 13 类接口配置遵守条件、精度和密钥不回显契约", async ({
    adminPage,
  }) => {
    await installCatalogAutomationMock(adminPage);
    const state = await installCatalogConfigurationMock(adminPage);
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await adminPage
      .getByTestId(
        "automation-interface-configuration-hidden-interface-certificate",
      )
      .click();
    const drawer = adminPage
      .locator('[role="dialog"]:visible')
      .filter({ hasText: "接口配置" })
      .last();
    await expect(drawer).toBeVisible();
    for (const fieldLabel of [
      "接口账号",
      "整数数量",
      "精确小数",
      "启用高级模式",
      "单选方案",
      "动态多选项",
      "开始日期",
      "开始时间",
      "接口密钥",
      "区域树",
      "联系地址",
      "资质文件",
    ]) {
      await expect(drawer.getByText(fieldLabel, { exact: true })).toBeVisible();
    }
    await expect(drawer.getByText("高级说明", { exact: true })).toHaveCount(0);
    await expect(drawer).toContainText("基础监控");
    await expect(drawer).not.toContainText("受治理的候选项服务暂不可用");
    await expect(drawer).toContainText("点击或拖拽文件到这里上传");

    const secret = drawer.getByLabel("接口密钥", { exact: true });
    await expect(secret).toHaveValue("");
    await expect(drawer).not.toContainText("1234");
    await expect(drawer).not.toContainText("opaque-existing-addon");
    await expect(drawer).not.toContainText("opaque-existing-file");

    const interfaceFileInput = drawer
      .locator(".ant-form-item")
      .filter({ hasText: "资质文件" })
      .locator('input[type="file"]');
    await drawer
      .getByRole("button", { name: "从当前配置移除", exact: true })
      .click();
    expect(state.fileUploadRequests).toHaveLength(0);
    await interfaceFileInput.setInputFiles({
      buffer: Buffer.from("%PDF governed interface qualification"),
      mimeType: "application/pdf",
      name: "qualification.pdf",
    });
    await expect(
      drawer.getByText("qualification.pdf", { exact: true }),
    ).toBeVisible();
    await drawer.getByRole("button", { name: "撤销上传", exact: true }).click();
    await expect(
      drawer.getByText("qualification.pdf", { exact: true }),
    ).toHaveCount(0);
    await interfaceFileInput.setInputFiles({
      buffer: Buffer.from("%PDF governed replacement qualification"),
      mimeType: "application/pdf",
      name: "qualification.pdf",
    });
    await expect(
      drawer.getByText("qualification.pdf", { exact: true }),
    ).toBeVisible();

    const advancedMode = drawer.getByRole("checkbox", {
      name: "启用高级模式",
    });
    await advancedMode.check();
    await expect(drawer.getByText("接口账号", { exact: true })).toHaveCount(0);
    await advancedMode.uncheck();
    await expect(drawer.getByText("接口账号", { exact: true })).toHaveCount(1);
    await drawer
      .getByLabel("接口账号", { exact: true })
      .fill("merchant-account");

    await drawer
      .getByLabel("单选方案", { exact: true })
      .locator("xpath=../..")
      .click();
    await drawer
      .locator(".ant-select-dropdown:visible")
      .getByText("高级方案", { exact: true })
      .click();
    await expect(drawer.getByText("高级说明", { exact: true })).toHaveCount(1);
    await expect
      .poll(() => state.interfaceEvaluationRequests.length)
      .toBeGreaterThan(1);
    const addons = drawer
      .getByLabel("动态多选项", { exact: true })
      .locator("xpath=../..");
    await addons.click();
    await expect(
      drawer
        .locator(".ant-select-dropdown:visible")
        .getByText("日志审计", { exact: true }),
    ).toBeVisible();
    await drawer
      .locator(".ant-select-dropdown:visible")
      .getByText("日志审计", { exact: true })
      .click();
    await drawer.getByLabel("整数数量", { exact: true }).fill("11");
    await drawer
      .getByLabel("精确小数", { exact: true })
      .fill("9007199254740993.020000000000000001");

    await drawer
      .getByLabel("单选方案", { exact: true })
      .locator("xpath=../..")
      .click();
    const filteredCandidates = drawer.locator(".ant-select-dropdown:visible");
    await expect(
      filteredCandidates.getByText("基础方案", { exact: true }),
    ).toHaveCount(0);
    await expect(
      filteredCandidates.getByText("高级方案", { exact: true }),
    ).toBeVisible();
    await filteredCandidates.getByText("高级方案", { exact: true }).click();
    await expect(drawer).toBeVisible();
    await drawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(drawer).toContainText("此项为必填项");
    await drawer
      .getByLabel("高级说明", { exact: true })
      .fill("高级方案需要人工复核");
    await drawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(drawer).toHaveCount(0);

    const update = state.interfaceConfigurationRequests.find(
      (request) => request.method === "PUT",
    );
    expect(update?.pathname).toBe(
      "/x/idc-core/api/v1/admin/catalog/automation/interfaces/hidden-interface-certificate/configuration",
    );
    expect(update?.body).toMatchObject({ expectedRevision: 2 });
    const rows = (update?.body as { values?: Array<Record<string, unknown>> })
      ?.values;
    expect(rows).toEqual(
      expect.arrayContaining([
        { fieldKey: "quantity", integerValue: "11" },
        {
          decimalValue: "9007199254740993.020000000000000001",
          fieldKey: "threshold",
        },
        { fieldKey: "notes", textValue: "高级方案需要人工复核" },
        { fieldKey: "addons", optionTokens: ["opaque-audit-addon"] },
        {
          fieldKey: "files",
          fileTokens: ["MCF_00000000000000000000000000000002"],
        },
      ]),
    );
    expect(state.fileUploadRequests.map(({ method }) => method)).toEqual(
      expect.arrayContaining(["POST", "PUT", "DELETE"]),
    );
    expect(rows?.some((row) => row.fieldKey === "credential")).toBe(false);
    expect(state.interfaceEvaluationRequests.at(-1)).toMatchObject({
      body: { expectedRevision: 2 },
      method: "POST",
      pathname:
        "/x/idc-core/api/v1/admin/catalog/automation/interfaces/hidden-interface-certificate/configuration/evaluation",
    });

    await adminPage
      .getByTestId(
        "automation-interface-configuration-hidden-interface-certificate",
      )
      .click();
    const reopened = adminPage
      .locator('[role="dialog"]:visible')
      .filter({ hasText: "接口配置" })
      .last();
    await expect(reopened.getByLabel("接口密钥", { exact: true })).toHaveValue(
      "",
    );
    await automation.screenshot("catalog-configuration-secret-preserved");
  });

  test("TC-14o: 商品首次 version=0 配置保留精度且客户配置只读", async ({
    adminPage,
  }) => {
    const fulfillmentBinding = {
      boundAt: 1_787_705_720_000,
      evidenceVerified: true,
      interfaceDisplayName: "证书供应商 A 接口",
      interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    };
    await installCatalogManagementMock(adminPage, { fulfillmentBinding });
    await installCatalogAutomationMock(adminPage);
    const state = await installCatalogConfigurationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.selectEditSection("购买配置");

    const productCard = catalog.detailDrawer
      .locator(".ant-card")
      .filter({ hasText: "后台商品配置" })
      .first();
    const customerCard = catalog.detailDrawer
      .locator(".ant-card")
      .filter({ hasText: "客户购买配置" })
      .first();
    await expect(productCard).toBeVisible();
    await expect(customerCard).toContainText("证书域名");
    await expect(customerCard).toContainText("证书类型");
    await expect(
      customerCard.getByRole("button", { name: "编辑配置" }),
    ).toHaveCount(0);
    expect(state.productConfigurationRequests[0]).toMatchObject({
      method: "GET",
      pathname:
        "/x/idc-core/api/v1/admin/catalog/product-versions/version-manual-service-1/configuration",
    });

    await productCard
      .getByRole("button", { name: "编辑配置", exact: true })
      .click();
    const drawer = adminPage
      .locator('[role="dialog"]:visible')
      .filter({ hasText: "后台商品配置" })
      .last();
    await drawer
      .getByLabel("商品整数", { exact: true })
      .fill("9223372036854775808");
    await drawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(drawer).toContainText("请输入 64 位整数");
    await drawer
      .getByLabel("商品整数", { exact: true })
      .fill("9223372036854775807");
    await drawer
      .getByLabel("商品精确小数", { exact: true })
      .fill("9007199254740993.020000000000000001");
    await drawer
      .getByLabel("商品方案", { exact: true })
      .locator("xpath=../..")
      .click();
    await drawer
      .locator(".ant-select-dropdown:visible")
      .getByText("高级方案", { exact: true })
      .click();
    await expect(drawer.getByText("高级商品说明", { exact: true })).toHaveCount(
      1,
    );
    await drawer
      .locator(".ant-form-item")
      .filter({ hasText: "商品资质" })
      .locator('input[type="file"]')
      .setInputFiles({
        buffer: Buffer.from("%PDF governed product qualification"),
        mimeType: "application/pdf",
        name: "qualification.pdf",
      });
    await expect(
      drawer.getByText("qualification.pdf", { exact: true }),
    ).toBeVisible();
    await drawer
      .getByLabel("可售区域", { exact: true })
      .locator("xpath=../..")
      .click();
    const candidates = drawer.locator(".ant-select-dropdown:visible");
    await expect(candidates.getByText("北区", { exact: true })).toBeVisible();
    await expect(candidates.getByText("南区", { exact: true })).toHaveCount(0);
    await candidates.getByText("北区", { exact: true }).click();

    await drawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(drawer).toContainText("此项为必填项");
    await drawer
      .getByLabel("高级商品说明", { exact: true })
      .fill("面向证书业务的高级版本");
    await drawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(
      adminPage.getByLabel("商品整数", { exact: true }),
    ).not.toBeVisible();

    const update = state.productConfigurationRequests.find(
      (request) => request.method === "PUT",
    );
    expect(update?.pathname).toBe(
      "/x/idc-core/api/v1/admin/catalog/product-versions/version-manual-service-1/configuration",
    );
    expect(update?.body).toEqual({
      expectedVersion: 0,
      merchantValues: [
        { fieldKey: "quantity", integerValue: "9223372036854775807" },
        {
          decimalValue: "9007199254740993.020000000000000001",
          fieldKey: "amount",
        },
        { fieldKey: "plan", optionTokens: ["advanced"] },
        { fieldKey: "region", optionTokens: ["north"] },
        {
          fieldKey: "description",
          textValue: "面向证书业务的高级版本",
        },
        {
          fieldKey: "files",
          fileTokens: ["MCF_00000000000000000000000000000001"],
        },
      ],
    });
    expect(state.productEvaluationRequests.at(-1)).toMatchObject({
      body: { expectedVersion: 0 },
      method: "POST",
      pathname:
        "/x/idc-core/api/v1/admin/catalog/product-versions/version-manual-service-1/configuration/evaluation",
    });

    const deleteCountBeforeBoundDetach = state.fileUploadRequests.filter(
      ({ method }) => method === "DELETE",
    ).length;
    await productCard
      .getByRole("button", { name: "编辑配置", exact: true })
      .click();
    const replacementDrawer = adminPage
      .locator('[role="dialog"]:visible')
      .filter({ hasText: "后台商品配置" })
      .last();
    await replacementDrawer
      .getByRole("button", { name: "从当前配置移除", exact: true })
      .click();
    expect(
      state.fileUploadRequests.filter(({ method }) => method === "DELETE"),
    ).toHaveLength(deleteCountBeforeBoundDetach);
    await replacementDrawer
      .locator(".ant-form-item")
      .filter({ hasText: "商品资质" })
      .locator('input[type="file"]')
      .setInputFiles({
        buffer: Buffer.from("%PDF replacement product qualification"),
        mimeType: "application/pdf",
        name: "replacement.pdf",
      });
    await replacementDrawer.getByRole("button", { name: /保\s*存/u }).click();
    await expect(
      adminPage.getByLabel("商品整数", { exact: true }),
    ).not.toBeVisible();
    const productUpdates = state.productConfigurationRequests.filter(
      ({ method }) => method === "PUT",
    );
    expect(productUpdates).toHaveLength(2);
    expect(productUpdates[0]?.body).toMatchObject({
      expectedVersion: 0,
      merchantValues: expect.arrayContaining([
        {
          fieldKey: "files",
          fileTokens: ["MCF_00000000000000000000000000000001"],
        },
      ]),
    });
    expect(productUpdates[1]?.body).toMatchObject({
      expectedVersion: 1,
      merchantValues: expect.arrayContaining([
        {
          fieldKey: "files",
          fileTokens: ["MCF_00000000000000000000000000000002"],
        },
      ]),
    });
    await catalog.screenshot("catalog-product-configuration-version-zero");
  });

  test("TC-14p: 英文窄屏动态候选和文件上传可用且不泄露技术值", async ({
    adminPage,
    mainLayout,
  }) => {
    await installCatalogAutomationMock(adminPage);
    await installCatalogConfigurationMock(adminPage);
    await mainLayout.switchLanguage("English");
    const automation = new CatalogAutomationPage(adminPage);
    await automation.goto();

    await adminPage
      .getByTestId(
        "automation-interface-configuration-hidden-interface-certificate",
      )
      .click();
    await adminPage.setViewportSize({ height: 844, width: 390 });
    const drawer = adminPage
      .locator('[role="dialog"]:visible')
      .filter({ hasText: "Interface configuration" })
      .last();
    await expect(drawer).toContainText("Basic monitoring");
    await expect(drawer).not.toContainText(
      "Options cannot be loaded because the governed candidate service is unavailable.",
    );
    await expect(drawer).toContainText("Click or drag a file here to upload");
    await expect(drawer).toContainText("Interface secret");
    await expect(drawer).not.toContainText(/受治理|接口密钥|资质文件/iu);
    await expect(drawer).not.toContainText(
      /opaque-existing|configuration\.candidates|plugin\.idc-core/iu,
    );
    const viewportHasNoHorizontalOverflow = await adminPage.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    );
    expect(viewportHasNoHorizontalOverflow).toBe(true);
    await automation.screenshot("catalog-configuration-english-narrow");
  });

  test("TC-14q: 缺少配置写权限时不读取配置接口也不显示配置入口", async ({
    adminPage,
  }) => {
    const fulfillmentBinding = {
      boundAt: 1_787_705_720_000,
      evidenceVerified: true,
      interfaceDisplayName: "证书供应商 A 接口",
      interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    };
    await installCatalogManagementMock(adminPage, { fulfillmentBinding });
    await installCatalogAutomationMock(adminPage, {
      permissions: [productQueryPermission, viewPermission],
    });
    const state = await installCatalogConfigurationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");

    expect(state.productConfigurationRequests).toHaveLength(0);
    await expect(catalog.detailDrawer).not.toContainText("后台商品配置");
    await expect(catalog.detailDrawer).not.toContainText("客户购买配置");
  });

  test("TC-14r: 无接口商品只能选择完全人工并显示订单项总数量", async ({
    adminPage,
  }) => {
    const state = await installCatalogManagementMock(adminPage);
    await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.screenshot("catalog-fulfillment-policy-initial");

    await catalog.openFulfillmentPolicy();
    const modal = catalog.fulfillmentPolicyModal;
    await expect(modal).toBeVisible();
    await catalog.screenshot("catalog-fulfillment-policy-manual-modal");
    await catalog.openFulfillmentPolicyStrategyOptions();
    await expect(
      catalog.fulfillmentPolicyStrategyOption("人工审核通过后调用插件"),
    ).toHaveClass(/ant-select-item-option-disabled/u);
    await expect(
      catalog.fulfillmentPolicyStrategyOption("确认付款后自动调用插件"),
    ).toHaveClass(/ant-select-item-option-disabled/u);
    await catalog.closeFulfillmentPolicyStrategyOptions();
    const confirm = catalog.fulfillmentPolicyConfirm();
    await expect(confirm).toBeEnabled();
    await confirm.click({ force: true });
    await modal.waitFor({ state: "hidden" });

    expect(state.fulfillmentPolicyRequests).toHaveLength(1);
    expect(state.fulfillmentPolicyRequests[0]).toMatchObject({
      body: { strategyCode: "manual" },
      method: "PUT",
    });
    await expect(catalog.detailDrawer).toContainText("完全人工处理");
    await expect(catalog.detailDrawer).toContainText(
      "整条订单商品一起处理（显示总数量）",
    );
    await expect(catalog.detailDrawer).not.toContainText(
      /per_order_item|maximumAttempts|queryMethod|provisionMethod/iu,
    );
    await catalog.screenshot("catalog-fulfillment-policy-manual-saved");
  });

  test("TC-14s: 已绑定商品可选择插件履约并同步显示中英文能力", async ({
    adminPage,
    mainLayout,
  }) => {
    const fulfillmentBinding = {
      boundAt: 1_787_705_720_000,
      evidenceVerified: true,
      interfaceDisplayName: "证书供应商 A 接口",
      interfaceInstanceNo: "hidden-interface-certificate",
      interfaceNewBusinessState: "accepting",
    };
    const state = await installCatalogManagementMock(adminPage, {
      fulfillmentBinding,
    });
    await installCatalogConfigurationMock(adminPage);
    await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.openFulfillmentPolicy();
    const modal = catalog.fulfillmentPolicyModal;
    await catalog.selectFulfillmentPolicyStrategy("确认付款后自动调用插件");
    await catalog.screenshot("catalog-fulfillment-policy-plugin-modal");
    await catalog.fulfillmentPolicyConfirm().click();
    await modal.waitFor({ state: "hidden" });
    expect(state.fulfillmentPolicyRequests[0]?.body).toEqual({
      strategyCode: "payment_confirmed_plugin",
    });
    await expect(catalog.detailDrawer).toContainText("签发并管理证书");
    await expect(catalog.detailDrawer).toContainText("按购买数量逐件处理");
    await catalog.revealFulfillmentPolicySummary();
    await catalog.screenshot("catalog-fulfillment-policy-plugin-saved");
    await catalog.waitForFulfillmentPolicyNoticeHidden("履约方式已保存");

    await catalog.closeProductDetail();
    await mainLayout.switchLanguage("English");
    await catalog.openProductDetail("product-manual-service");
    await catalog.selectEditSection("Delivery settings");
    await expect(catalog.detailDrawer).toContainText(
      "Call plugin after payment confirmation",
    );
    await expect(catalog.detailDrawer).toContainText(
      "Issue and manage certificates",
    );
    await expect(catalog.detailDrawer).toContainText(
      "Handle each purchased unit separately",
    );
    await expect(catalog.detailDrawer).not.toContainText(
      /插件|履约方式|处理数量/iu,
    );
    await expect(catalog.body).not.toContainText("履约方式已保存");
    await catalog.revealFulfillmentPolicySummary();
    await catalog.screenshot("catalog-fulfillment-policy-plugin-saved-en");
  });

  test("TC-14t: 保存期间锁定按钮且失败后保留弹窗", async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage, {
      fulfillmentPolicyDelayMs: 2_000,
      fulfillmentPolicyFailure: true,
    });
    await installCatalogAutomationMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.openFulfillmentPolicy();
    const modal = catalog.fulfillmentPolicyModal;
    const confirm = catalog.fulfillmentPolicyConfirm();
    const saving = confirm.click({ force: true });
    await expect.poll(() => state.fulfillmentPolicyRequests.length).toBe(1);
    await expect(confirm).toBeDisabled();
    await saving;
    await expect(modal).toBeVisible();
    await expect(
      catalog.fulfillmentPolicyError("商品目录管理请求与当前状态冲突"),
    ).toBeVisible();
    await catalog.screenshot("catalog-fulfillment-policy-save-failed");
    expect(state.fulfillmentPolicyRequests).toHaveLength(1);
  });
});
