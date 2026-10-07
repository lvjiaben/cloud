import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { PrivacyGovernancePage } from "../pages/PrivacyGovernancePage";
import { installPrivacyGovernanceMock } from "../support/privacy-governance-mock";

test.describe("TC-6 隐私治理", () => {
  test("TC-6a: 影响评估明确确认后发布且跨境始终关闭", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installPrivacyGovernanceMock(adminPage);
    const privacy = new PrivacyGovernancePage(adminPage);
    await privacy.goto();

    await expect(privacy.root).toContainText("跨境处理已硬关闭");
    const aiCard = privacy.root.locator(".ant-card").filter({
      hasText: "AI 安全投影",
    });
    await aiCard.getByRole("button", { name: "确认并发布" }).click();
    const dialog = adminPage.getByRole("dialog").last();
    await expect(dialog).toContainText("确认发布这份完整评估？");
    await privacy.confirmLastDialog();
    expect(state.publications).toHaveLength(1);
    expect(state.publications[0]).toMatchObject({ confirmed: true });
    await privacy.screenshot("privacy-governance-assessments-zh-cn");
  });

  test("TC-6b: 删除原件需要再次确认且只提交不透明编号", async ({
    adminPage,
  }) => {
    const state = await installPrivacyGovernanceMock(adminPage);
    const privacy = new PrivacyGovernancePage(adminPage);
    await privacy.goto();
    await privacy.openTab("删除任务");
    await adminPage.getByRole("button", { name: "确认删除原件" }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText(
      "合同、税务、争议、审计和法律保全",
    );
    await privacy.confirmLastDialog();
    expect(state.deletionConfirmations).toHaveLength(1);
    expect(state.deletionConfirmations[0]).toMatchObject({ confirmed: true });
    expect(JSON.stringify(state.deletionConfirmations[0])).not.toContain(
      "PSE_LOCAL_REDACTED_001",
    );
  });

  test("TC-6c: 泄露处置不自动通知且支持包必须单独确认", async ({
    adminPage,
  }) => {
    const state = await installPrivacyGovernanceMock(adminPage);
    const privacy = new PrivacyGovernancePage(adminPage);
    await privacy.goto();
    await privacy.openTab("泄露事件");

    await adminPage.getByRole("button", { name: /进入下一阶段/u }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText(
      "不会自动向外部机构发送通知",
    );
    await privacy.confirmLastDialog();
    expect(state.incidentAdvances[0]).toMatchObject({
      confirmExternalNotification: false,
      nextStageCode: "contained",
    });

    await adminPage.getByRole("button", { name: "生成本地脱敏支持包" }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText(
      "不会开启远程登录",
    );
    await privacy.confirmLastDialog();
    expect(state.supportExports[0]).toMatchObject({
      supportAgreementConfirmed: true,
    });
  });

  test("TC-6d: 英文手机页面完整翻译且没有横向溢出", async ({
    adminPage,
    mainLayout,
  }) => {
    await adminPage.setViewportSize({ height: 844, width: 390 });
    await mainLayout.switchLanguage("English");
    await installPrivacyGovernanceMock(adminPage, { locale: "en-US" });
    const privacy = new PrivacyGovernancePage(adminPage);
    await privacy.goto();

    await expect(privacy.root).toContainText("Privacy governance desk");
    await expect(privacy.root).toContainText(
      "Cross-border processing is hard-disabled",
    );
    await expect(privacy.root).not.toContainText(
      "plugin.idc-compliance",
    );
    const overflowDetails = await privacy.horizontalOverflowDetails();
    expect(overflowDetails, JSON.stringify(overflowDetails)).toEqual([]);
    await privacy.screenshot("privacy-governance-mobile-en-us");
  });
});
