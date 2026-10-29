import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProviderTaskPage } from "../pages/ProviderTaskPage";
import {
  installProviderTaskMock,
  providerTaskFixture,
} from "../support/provider-task-mock";

test.describe("TC-3 操作记录与安全人工接管", () => {
  test("TC-3j: 明确未发送保留独立依据，失败记录不冒充仍在上游执行", async ({adminPage}) => {
    const state=await installProviderTaskMock(adminPage,{provisioning:true,notSent:true,callDiagnostic:JSON.stringify({stage:"request_preparation",errorCode:"provider_unavailable",businessRequestStarted:false,responseReceived:false,businessRequestsAttempted:0})});
    const tasks=new ProviderTaskPage(adminPage);await tasks.goto();await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("资源服务确认本次开通没有尝试发送业务请求");
    await expect(tasks.detailDrawer).toContainText("未发送开通请求的拥有方记录");
    await expect(tasks.detailDrawer.getByTestId("provider-provision-unknown")).toHaveCount(0);
    await expect(tasks.detailDrawer).not.toContainText("上游处理中");
    await expect(tasks.detailDrawer).toContainText("本地服务记录");
    await expect(tasks.detailDrawer).not.toContainText("已关联服务器资源");
    await tasks.screenshot("resource-confirmed-not-sent-summary");
    await tasks.detailDrawer.getByTestId("provider-task-evidence-evidence_owner_not_sent").scrollIntoViewIfNeeded();
    await tasks.screenshot("resource-confirmed-not-sent-proof");expect(state.takeoverRequests).toHaveLength(0);
  });
  test("TC-3i: 无响应记录保留发送前错误环节与脱敏调用详情", async ({adminPage}) => {
    const state=await installProviderTaskMock(adminPage,{provisioning:true,callDiagnostic:JSON.stringify({stage:"request_preparation",errorCode:"IDC_RESOURCE_PROVIDER_LOCAL_AUTH_POLICY",detail:"Local request preparation failed",businessRequestStarted:false,responseReceived:false})});
    const tasks=new ProviderTaskPage(adminPage);await tasks.goto();await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("发送前配置或凭据校验未通过");
    await expect(tasks.detailDrawer).toContainText("接口调用失败环节");
    const call=tasks.detailDrawer.getByTestId("resource-call-diagnostic");await expect(call).toContainText('"businessRequestStarted": false');
    await call.scrollIntoViewIfNeeded();await tasks.screenshot("resource-call-failure-before-send");expect(state.takeoverRequests).toHaveLength(0);
  });
  test("TC-3h: 具体接口响应正文不被固定分类文案覆盖", async ({adminPage}) => {
    const state=await installProviderTaskMock(adminPage,{provisioning:true,responseExcerpt:'{"method":"POST","route":"/clouds","httpStatus":200,"response":{"status":400,"msg":"镜像在当前节点未下载","_redactedFields":true}}'});
    const tasks=new ProviderTaskPage(adminPage);await tasks.goto();await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("上游接口响应（敏感字段已遮盖）");
    const response=tasks.detailDrawer.getByTestId("resource-http-response");
    await expect(response).toContainText("镜像在当前节点未下载");
    await expect(response).toContainText(/"httpStatus":\s*200/u);
    await expect(tasks.detailDrawer).toContainText("接口响应诊断");
    expect(state.takeoverRequests).toHaveLength(0);
    await response.scrollIntoViewIfNeeded();
    await tasks.screenshot("resource-http-response-error");
  });
  test("TC-3f: 开通异常展示已有证据和订单处理位置，不把接管当重开", async ({adminPage}) => {
    const state = await installProviderTaskMock(adminPage, {provisioning: true});
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto(); await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("开通结果未确认，暂不能重新开通");
    await expect(tasks.detailDrawer).toContainText("这条记录未保存更具体的错误原因");
    await expect(tasks.detailDrawer).toContainText("商品履约任务");
    await expect(tasks.detailDrawer).toContainText("不会重新开通或退款");
    await expect(tasks.detailDrawer).not.toContainText("operation submission outcome");
    await expect(tasks.detailDrawer).not.toContainText("plugin.idc-resource");
    expect(state.takeoverRequests).toHaveLength(0);
    await tasks.screenshot("provisioning-unknown-existing-evidence");
  });
  test("TC-3g: 安全接口错误在操作记录中显示具体中文原因", async ({adminPage}) => {
    const state = await installProviderTaskMock(adminPage, {provisioning: true, callTimeout: true});
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto(); await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText("调用上游接口超时");
    await expect(tasks.detailDrawer).toContainText("接口调用异常");
    expect(state.takeoverRequests).toHaveLength(0);
    await tasks.screenshot("provisioning-timeout-safe-evidence");
  });
  test("TC-3a: 中文页只展示有界本地脱敏投影", async ({ adminPage }) => {
    await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();

    await expect(
      tasks.page.getByText("操作记录", { exact: true }).first(),
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
      .filter({ hasText: "停止自动核对，转为人工处理？" });
    await expect(confirm).toContainText("不会向供应商提交操作");
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
        .getByText("Operation Records", { exact: true })
        .first(),
    ).toBeVisible();
    await tasks.openDetail(providerTaskFixture.taskNo);
    await expect(tasks.detailDrawer).toContainText(
      "Switch to Manual Handling",
    );
    await expect(tasks.root).not.toContainText("plugin.idc-resource");
    await tasks.screenshot("provider-task-evidence-en-us");
  });
});
