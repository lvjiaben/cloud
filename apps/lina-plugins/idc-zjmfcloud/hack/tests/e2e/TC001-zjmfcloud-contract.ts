import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProviderCapacityPage } from "../../../../idc-resource/hack/tests/pages/ProviderCapacityPage";
import { ProviderTaskPage } from "../../../../idc-resource/hack/tests/pages/ProviderTaskPage";
import { installProviderCapacityMock } from "../../../../idc-resource/hack/tests/support/provider-capacity-mock";
import {
  installProviderTaskMock,
  providerTaskFixture,
} from "../../../../idc-resource/hack/tests/support/provider-task-mock";
import {
  countRealCapacityRequests,
  realResourceIntegrationEnabled,
  realTaskStatus,
  seedRealResourceFixture,
} from "../support/real-resource-integration";

// The default mode is a fast resource-owner page contract. Setting
// LINAPRO_IDC_ZJMFCLOUD_REAL_E2E=1 keeps only the harmless identity projection
// mocked and sends every resource business request to the real local backend.
// Neither mode replaces the paid isolated production evidence required by 9.7.
async function installAdminIdentityMock(
  page: Parameters<typeof installProviderTaskMock>[0],
) {
  await page.unroute("**/api/v1/user/info").catch(() => {});
  await page.route("**/api/v1/user/info", async (route) => {
    await route.fulfill({
      json: {
        code: 0,
        data: {
          avatar: "",
          desc: "IDC Provider contract test administrator",
          homePath: "/idc/resource/tasks",
          menus: [],
          permissions: [
            "idc:admin:resource:provider-capacity:list",
            "idc:admin:resource:provider-capacity:refresh",
            "idc:admin:resource:task:list",
            "idc:admin:resource:task:query",
            "idc:admin:resource:task:manual-takeover",
          ],
          realName: "IDC Test Administrator",
          roles: ["admin"],
          userId: 1,
          username: "idc-test-admin",
        },
        message: "success",
      },
    });
  });
}

test.describe("TC-1 魔方云 Provider 合同", () => {
  test("TC-1a: 容量刷新只提交渠道和容量类别", async ({
    authenticatedPage: page,
  }) => {
    if (realResourceIntegrationEnabled()) {
      const fixture = seedRealResourceFixture();
      await installAdminIdentityMock(page);
      const capacity = new ProviderCapacityPage(page);
      await capacity.goto();
      await capacity.openSync();
      await capacity.selectSyncChannel(fixture.channelName);
      await capacity.selectSyncKind("镜像");
      await capacity.confirmSync();
      expect(countRealCapacityRequests(fixture.channelNo)).toBe(1);
      await capacity.screenshot("zjmfcloud-real-capacity-refresh");
      return;
    }
    const state = await installProviderCapacityMock(page);
    await installAdminIdentityMock(page);
    const capacity = new ProviderCapacityPage(page);
    await capacity.goto();
    await capacity.openSync("channel_mofang_prod");
    await capacity.selectSyncKind("镜像");
    await capacity.confirmSync();

    expect(state.syncRequests).toHaveLength(1);
    expect(state.syncRequests[0]).toMatchObject({
      body: { kind: "image" },
      method: "POST",
    });
    await expect(
      capacity.operationStatus("observation_collecting_003"),
    ).toContainText("采集中");
    await capacity.screenshot("zjmfcloud-capacity-refresh");
  });

  test("TC-1b: 任务轮询只读取本地任务和脱敏上游投影", async ({
    authenticatedPage: page,
  }) => {
    if (realResourceIntegrationEnabled()) {
      const fixture = seedRealResourceFixture();
      await installAdminIdentityMock(page);
      const tasks = new ProviderTaskPage(page);
      await tasks.goto();
      await tasks.openDetail(fixture.taskNo);
      await expect(tasks.detailDrawer).toContainText(fixture.externalTaskRef);
      await expect(tasks.detailDrawer).not.toContainText(
        /password|credential|secret|endpoint|raw payload|request params/iu,
      );
      await tasks.screenshot("zjmfcloud-real-task-polling");
      return;
    }
    const state = await installProviderTaskMock(page);
    await installAdminIdentityMock(page);
    const tasks = new ProviderTaskPage(page);
    await tasks.goto();
    await tasks.openDetail(providerTaskFixture.taskNo);

    expect(state.detailRequests).toHaveLength(1);
    await expect(tasks.detailDrawer).toContainText("上游处理中");
    await expect(tasks.detailDrawer).toContainText("40%");
    await expect(tasks.detailDrawer).not.toContainText(
      /password|credential|secret|endpoint|raw payload|request params/iu,
    );
    await tasks.screenshot("zjmfcloud-task-polling");
  });

  test("TC-1c: 超时后先查任务且页面不提供危险重发", async ({
    authenticatedPage: page,
  }) => {
    if (realResourceIntegrationEnabled()) {
      const fixture = seedRealResourceFixture();
      await installAdminIdentityMock(page);
      const tasks = new ProviderTaskPage(page);
      await tasks.goto();
      await tasks.openDetail(fixture.taskNo);
      await expect(tasks.row(fixture.taskNo)).toContainText("结果未知");
      expect(realTaskStatus(fixture.taskNo)).toBe("result_unknown");
      await expect(
        tasks.detailDrawer.getByRole("button", {
          name: /立即重试|再次写入|retry now|write again/iu,
        }),
      ).toHaveCount(0);
      await tasks.screenshot("zjmfcloud-real-timeout-query-first");
      return;
    }
    const state = await installProviderTaskMock(page);
    await installAdminIdentityMock(page);
    const tasks = new ProviderTaskPage(page);
    await tasks.goto();
    await tasks.openDetail(providerTaskFixture.taskNo);

    await expect(tasks.row(providerTaskFixture.taskNo)).toContainText(
      "结果未知",
    );
    expect(state.detailRequests).toHaveLength(1);
    expect(state.takeoverRequests).toHaveLength(0);
    await expect(
      tasks.detailDrawer.getByRole("button", {
        name: /立即重试|再次写入|retry now|write again/iu,
      }),
    ).toHaveCount(0);
    await tasks.screenshot("zjmfcloud-timeout-query-first");
  });

  test("TC-1d: 未知状态只能带既有证据转人工接管", async ({
    authenticatedPage: page,
  }) => {
    const realFixture = realResourceIntegrationEnabled()
      ? seedRealResourceFixture()
      : undefined;
    const mockState = realFixture
      ? undefined
      : await installProviderTaskMock(page);
    await installAdminIdentityMock(page);
    const tasks = new ProviderTaskPage(page);
    await tasks.goto();
    const taskNo = realFixture?.taskNo ?? providerTaskFixture.taskNo;
    const evidenceNo =
      realFixture?.evidenceNo ?? providerTaskFixture.evidenceOne;
    await tasks.openDetail(taskNo);
    await tasks.evidence(evidenceNo).getByRole("checkbox").check();
    await tasks.selectReason("供应商结果仍无法确定");
    await page.getByTestId("provider-task-takeover-submit").click();

    const confirm = page
      .locator(".ant-modal-confirm")
      .filter({ hasText: "确认关闭自动化并承担处理责任？" });
    await expect(confirm).toContainText("不会发送任何供应商请求");
    await confirm.getByRole("button", { name: "确认接管" }).click();

    if (realFixture) {
      expect(realTaskStatus(realFixture.taskNo)).toBe("manual_takeover");
    } else {
      expect(mockState?.takeoverRequests).toHaveLength(1);
      expect(mockState?.takeoverRequests[0]?.body).toEqual({
        evidenceNos: [providerTaskFixture.evidenceOne],
        expectedVersion: 3,
        reasonCode: "provider_result_unresolved",
      });
      expect(mockState?.takeoverRequests[0]?.body).not.toHaveProperty("retry");
      expect(mockState?.takeoverRequests[0]?.body).not.toHaveProperty("result");
    }
    await expect(tasks.detailDrawer).toContainText("人工接管结果");
    await tasks.screenshot(
      realFixture
        ? "zjmfcloud-real-unknown-manual-takeover"
        : "zjmfcloud-unknown-manual-takeover",
    );
  });
});
