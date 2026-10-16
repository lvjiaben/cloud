import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";

import { ProviderConfigurationPage } from "../pages/ProviderConfigurationPage";
import { installProviderConfigurationMock } from "../support/provider-configuration-mock";

const secretInput = {
  accessKeyId: "LTAI5tExampleK9M2",
  accessKeySecret: "provider-secret-never-returned-2026",
  sceneId: "100000000001",
};

test.describe("TC-1 阿里云实名认证服务商配置", () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, "idc-compliance");
  });

  test("TC-1a: 中文安全投影只显示掩码和商家责任边界", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    await installProviderConfigurationMock(adminPage, {
      initialStatus: "configured",
    });
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();

    await expect(provider.pageRoot).toContainText("阿里云实名认证服务商");
    await expect(provider.pageRoot).toContainText("已配置");
    await expect(provider.pageRoot).toContainText("K9M2");
    await expect(provider.pageRoot).toContainText(
      "服务费用由商家承担",
    );
    await expect(provider.pageRoot).not.toContainText(
      secretInput.accessKeySecret,
    );
    await expect(provider.pageRoot).not.toContainText("secret_ref");
    await expect(provider.pageRoot).not.toContainText("plugin.idc-compliance");
    await expect(provider.pageRoot).not.toContainText("Endpoint");
    await expect(provider.pageRoot).not.toContainText("ExecuteRequest");
    await expect(provider.pageRoot).not.toContainText("fin_face_verify");
    await provider.screenshot("aliyun-provider-first-load-zh");
  });

  test("TC-1g: 真实未配置服务商按草稿状态打开而不是报服务不可用", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const provider = new ProviderConfigurationPage(adminPage);
    const pageErrors: string[] = [];
    adminPage.on("pageerror", error => pageErrors.push(error.message));
    await provider.goto();

    await expect(provider.pageRoot.getByTestId("aliyun-provider-status")).toHaveText(
      /草稿|未配置/u,
    );
    await expect(provider.pageRoot).toContainText("阿里云实名认证服务商");
    await expect(
      adminPage.getByText("合规服务商配置暂时不可用", { exact: true }),
    ).toHaveCount(0);
    await expect(provider.validationStatus).toHaveText("未测试");
    expect(pageErrors).toEqual([]);
    await provider.screenshot("aliyun-provider-real-draft");
  });

  test("TC-1b: 首次配置整组提交且保存后不回显秘密", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installProviderConfigurationMock(adminPage);
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();

    const dialog = await provider.openConfiguration();
    await expect(dialog).toContainText("只在服务端使用凭据");
    await expect(dialog).toContainText("完整登录后五分钟内");
    await provider.fillConfiguration(secretInput);
    await provider.screenshot("aliyun-provider-configure-modal");
    await provider.confirmDialog();

    await expect(dialog).toBeHidden();
    await expect.poll(() => state.configurationRequests.length).toBe(1);
    expect(state.configurationRequests[0]).toMatchObject({
      accessKeyId: secretInput.accessKeyId,
      accessKeySecret: secretInput.accessKeySecret,
      expectedVersion: 0,
      sceneId: secretInput.sceneId,
    });
    expect(state.configurationRequests[0]?.idempotencyKey).toMatch(
      /^aliyun-face-config-[0-9a-f-]{36}$/,
    );
    await expect(provider.pageRoot).toContainText("已配置");
    await expect(provider.pageRoot).toContainText("K9M2");
    await expect(provider.pageRoot).not.toContainText(
      secretInput.accessKeySecret,
    );
    await provider.screenshot("aliyun-provider-configured");
  });

  test("TC-1c: 连通性验证只调用独立只读动作并更新状态", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installProviderConfigurationMock(adminPage, {
      initialStatus: "configured",
    });
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();

    await provider.validateButton.click();
    await expect.poll(() => state.validationRequests).toBe(1);
    await expect(provider.validationStatus).toHaveText("可连接");
    expect(state.configurationRequests).toHaveLength(0);
    expect(state.recoveryRequests).toHaveLength(0);
    await provider.screenshot("aliyun-provider-validation-reachable");
  });

  test("TC-1d: 中断状态失败关闭且只允许服务端恢复", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installProviderConfigurationMock(adminPage, {
      initialStatus: "updating",
    });
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();

    await expect(provider.recoveryAlert).toContainText("凭据更新需要恢复");
    await expect(provider.configureButton).toHaveCount(0);
    await expect(provider.validateButton).toHaveCount(0);
    const dialog = await provider.openRecovery();
    await expect(dialog.getByTestId("aliyun-provider-scene-id")).toHaveCount(0);
    await expect(dialog).toContainText("恢复不会强制标记成功");
    await provider.fillRecovery({
      accessKeyId: "LTAI5tRecoveredN7P4",
      accessKeySecret: "recovered-secret-never-returned-2026",
    });
    await provider.screenshot("aliyun-provider-recovery-modal");
    await provider.confirmDialog();

    await expect(dialog).toBeHidden();
    await expect.poll(() => state.recoveryRequests.length).toBe(1);
    expect(state.recoveryRequests[0]).toEqual({
      accessKeyId: "LTAI5tRecoveredN7P4",
      accessKeySecret: "recovered-secret-never-returned-2026",
    });
    await expect(provider.pageRoot).toContainText("已配置");
    await expect(provider.pageRoot).toContainText("N7P4");
    await expect(provider.pageRoot).not.toContainText(
      "recovered-secret-never-returned-2026",
    );
    await provider.screenshot("aliyun-provider-recovery-complete");
  });

  test("TC-1e: 最近完整登录过期时保持弹窗和原状态", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installProviderConfigurationMock(adminPage, {
      staleLogin: true,
    });
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();
    const dialog = await provider.openConfiguration();
    await provider.fillConfiguration(secretInput);
    await provider.confirmDialog();

    await expect.poll(() => state.configurationRequests.length).toBe(1);
    await expect(dialog).toBeVisible();
    await expect(provider.pageRoot).toContainText("未配置");
    await provider.screenshot("aliyun-provider-recent-login-required");
  });

  test("TC-1f: 英文界面使用完整译文且不暴露内部键", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("English");
    await installProviderConfigurationMock(adminPage, {
      initialStatus: "configured",
    });
    const provider = new ProviderConfigurationPage(adminPage);
    await provider.goto();

    await expect(provider.pageRoot).toContainText("Aliyun Identity Provider");
    await expect(provider.pageRoot).toContainText(
      "The merchant pays service fees",
    );
    await expect(provider.validateButton).toHaveText("Validate connectivity");
    await expect(provider.pageRoot).not.toContainText("plugin.idc-compliance");
    await provider.screenshot("aliyun-provider-first-load-en");
  });
});
