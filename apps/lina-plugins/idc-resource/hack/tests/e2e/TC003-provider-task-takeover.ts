import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProviderTaskPage } from "../pages/ProviderTaskPage";
import {
  installProviderTaskMock,
  providerTaskFixture,
} from "../support/provider-task-mock";

test.describe("TC-3 上游任务核对与安全人工接管", () => {
  test("TC-3a: 中文页只展示有界本地脱敏投影", async ({ adminPage }) => {
    await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();

    await expect(
      tasks.page.getByText("上游任务核对", { exact: true }).first(),
    ).toBeVisible();
    await expect(tasks.row(providerTaskFixture.taskNo)).toContainText(
      "结果未知",
    );
    await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("可用处理依据");
    await expect(tasks.detailDrawer).toContainText("上游任务仍在执行");
    await expect(tasks.detailDrawer).not.toContainText(
      /900100|600100|task_|resource_/u,
    );
    await expect(tasks.detailDrawer).not.toContainText(
      /password|credential|secret|endpoint|raw payload|request params/iu,
    );
    await tasks.screenshot("provider-task-evidence-zh-cn");
  });

  test("TC-3b: 接管只提交版本、受控原因和本任务既有证据", async ({
    adminPage,
  }) => {
    const state = await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();
    await tasks.openDetail(providerTaskFixture.taskNo);

    await tasks
      .evidence(providerTaskFixture.evidenceOne)
      .getByRole("checkbox")
      .check();
    await tasks.selectReason("供应商结果仍无法确定");
    await adminPage.getByTestId("provider-task-takeover-submit").click();
    const confirm = adminPage
      .locator(".ant-modal-confirm")
      .filter({ hasText: "确认关闭自动化并承担处理责任？" });
    await expect(confirm).toContainText("不会发送任何供应商请求");
    await confirm.getByRole("button", { name: "确认接管" }).click();

    await expect(tasks.detailDrawer).toContainText("人工接管结果");
    await expect(tasks.detailDrawer).not.toContainText("takeover_");
    expect(state.takeoverRequests).toHaveLength(1);
    expect(state.takeoverRequests[0]?.method).toBe("POST");
    expect(state.takeoverRequests[0]?.body).toEqual({
      evidenceNos: [providerTaskFixture.evidenceOne],
      expectedVersion: 3,
      reasonCode: "provider_result_unresolved",
    });
    expect(state.takeoverRequests[0]?.body).not.toHaveProperty("status");
    expect(state.takeoverRequests[0]?.body).not.toHaveProperty("result");
    expect(state.takeoverRequests[0]?.body).not.toHaveProperty("retry");
    await tasks.screenshot("provider-task-manual-takeover");
  });

  test("TC-3c: 页面不提供强制结果、危险重试或自由文本入口", async ({
    adminPage,
  }) => {
    const state = await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();
    await tasks.openDetail(providerTaskFixture.taskNo);

    await expect(
      tasks.detailDrawer.getByRole("button", {
        name: /强制成功|强制失败|立即重试|再次写入|force success|force failure|retry now/iu,
      }),
    ).toHaveCount(0);
    await expect(tasks.detailDrawer.getByRole("textbox")).toHaveCount(0);
    await expect(
      adminPage.getByTestId("provider-task-takeover-submit"),
    ).toBeDisabled();
    expect(state.takeoverRequests).toHaveLength(0);
  });

  test("TC-3d: 无接管权限时只读证据仍可见且责任入口不渲染", async ({
    adminPage,
  }) => {
    await installProviderTaskMock(adminPage, { canTakeOver: false });
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();
    await tasks.openDetail(providerTaskFixture.taskNo);

    await expect(tasks.detailDrawer).toContainText("可用处理依据");
    await expect(
      adminPage.getByTestId("provider-task-takeover-submit"),
    ).toHaveCount(0);
    await expect(
      tasks.evidence(providerTaskFixture.evidenceOne).getByRole("checkbox"),
    ).toBeDisabled();
    await tasks.screenshot("provider-task-readonly");
  });

  test("TC-3e: 英文环境完整翻译任务核对和安全边界", async ({
    adminPage,
    mainLayout,
  }) => {
    await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await mainLayout.switchLanguage("English");
    await tasks.goto();

    await expect(
      tasks.page
        .getByText("Provider Task Reconciliation", { exact: true })
        .first(),
    ).toBeVisible();
    await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText(
      "Manual takeover is responsibility transfer, not a result verdict",
    );
    await expect(tasks.root).not.toContainText("plugin.idc-resource");
    await tasks.screenshot("provider-task-evidence-en-us");
  });
});
