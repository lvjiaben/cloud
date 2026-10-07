import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProviderHealthPage } from "../pages/ProviderHealthPage";
import { installProviderHealthMock } from "../support/provider-health-mock";

test.describe("TC-2 上游渠道健康与写保护", () => {
  test("TC-2a: 中文页展示四维健康和本地保护边界", async ({ adminPage }) => {
    await installProviderHealthMock(adminPage);
    const health = new ProviderHealthPage(adminPage);
    await health.goto();
    await expect(
      health.page.getByText("上游渠道健康与写保护", { exact: true }).first(),
    ).toBeVisible();
    await expect(health.row("魔方云生产渠道")).toContainText(
      "禁止新的自动写入",
    );
    await expect(health.row("魔方云生产渠道")).toContainText(
      "认证 · 健康 · 45ms",
    );
    await expect(health.row("魔方云生产渠道")).toContainText(
      "容量观测 · 健康 · 125ms",
    );
    await expect(health.row("魔方云生产渠道")).toContainText("已启用");
    await expect(health.root).not.toContainText(
      /endpoint|credential|raw error|secret/iu,
    );
    await health.screenshot("provider-health-zh-cn");
  });

  test("TC-2b: 策略提交包含版本且不提供手工解锁", async ({ adminPage }) => {
    const state = await installProviderHealthMock(adminPage);
    const health = new ProviderHealthPage(adminPage);
    await health.goto();
    await health.openPolicy("channel_mofang_prod");
    const dialog = health.policyDialog;
    await expect(dialog).toContainText("保护边界不会被策略更新绕过");
    await expect(
      dialog.getByRole("button", { name: /解除|解锁|关闭保护/ }),
    ).toHaveCount(0);
    await health.screenshot("provider-health-policy-modal");
    await dialog.getByRole("button", { name: "保存策略" }).click();
    await expect(dialog).toBeHidden();
    expect(state.policyRequests).toHaveLength(1);
    expect(state.policyRequests[0]?.method).toBe("PUT");
    expect(state.policyRequests[0]?.body).toEqual({
      consecutiveFailureLimit: 3,
      expectedVersion: 1,
      failureLimit: 3,
      minimumSamplesToProtect: 5,
      protectionDurationSeconds: 300,
      unknownResultLimit: 2,
      windowSampleLimit: 20,
    });
    await expect(health.row("魔方云生产渠道")).toContainText("保护时长 300 秒");
    await expect(health.row("魔方云生产渠道")).not.toContainText("v2");
    await health.screenshot("provider-health-policy-updated");
  });

  test("TC-2c: 英文环境完整翻译健康诊断页", async ({
    adminPage,
    mainLayout,
  }) => {
    await installProviderHealthMock(adminPage);
    const health = new ProviderHealthPage(adminPage);
    await health.goto();
    await mainLayout.switchLanguage("English");
    await expect(
      health.page
        .getByText("Provider Health and Write Protection", { exact: true })
        .first(),
    ).toBeVisible();
    await expect(health.row("魔方云生产渠道")).toContainText(
      "New automatic writes blocked",
    );
    await expect(health.root).not.toContainText("plugin.idc-resource");
    await health.screenshot("provider-health-en-us");
  });

  test("TC-2d: 只读权限不渲染策略入口", async ({ adminPage }) => {
    await installProviderHealthMock(adminPage, { canUpdatePolicy: false });
    const health = new ProviderHealthPage(adminPage);
    await health.goto();
    await expect(
      adminPage.getByTestId("provider-health-policy-channel_mofang_prod"),
    ).toHaveCount(0);
    await expect(health.row("魔方云生产渠道")).toBeVisible();
    await health.screenshot("provider-health-readonly");
  });

  test("TC-2e: 非法阈值关系在前端拒绝且不发送写请求", async ({ adminPage }) => {
    const state = await installProviderHealthMock(adminPage);
    const health = new ProviderHealthPage(adminPage);
    await health.goto();
    await health.openPolicy("channel_mofang_prod");
    const dialog = health.policyDialog;
    await dialog.getByLabel("结果未知阈值").fill("4");
    await dialog.getByRole("button", { name: "保存策略" }).click();
    await expect(adminPage.getByText("阈值之间的范围关系不合法")).toBeVisible();
    await expect(dialog).toBeVisible();
    expect(state.policyRequests).toHaveLength(0);
    await health.screenshot("provider-health-policy-invalid");
  });
});
