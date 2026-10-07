import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { openStandaloneWorkspace } from "../support/standalone-workspace";
import { installServerBatchMock } from "../support/server-batch-mock";
import { ServerBatchPage } from "../pages/ServerBatchPage";

test.describe("TC012 独立服务器批量入口退役", () => {
  test("TC012b: 保留的独立产物预览表使用 Vben 且不改变逐条执行结果", async ({
    authenticatedPage: adminPage,
  }) => {
    const state = await installServerBatchMock(adminPage);
    await openStandaloneWorkspace(adminPage, "/idc/resource/server-batches");
    const batch = new ServerBatchPage(adminPage);
    await adminPage.route(
      "**/x-assets/vben-adapter/v0.1.0/isolated/runtime.js",
      (route) => route.abort("failed"),
      { times: 1 },
    );
    await batch.preview("SVC-SERVER-0001\nSVC-SERVER-0002");
    const grid = batch.pageRoot.locator(".vxe-grid");
    await expect(batch.pageRoot.getByRole("alert")).toContainText(
      "无法生成影响预览，请检查服务器编号和当前权限。",
    );
    await expect(grid).toHaveCount(0);
    await expect(adminPage.getByTestId("server-batch-execute")).toBeDisabled();
    await batch.pageRoot
      .getByRole("alert")
      .getByRole("button", { name: "重试", exact: true })
      .click();
    await expect(grid).toBeVisible();
    await expect(
      grid.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(2);
    expect(state.previews).toHaveLength(1);
    expect(state.executions).toHaveLength(0);
    await batch.execute();
    await expect.poll(() => state.executions.length).toBe(1);
    await expect(grid).toContainText("operation-1");
    await expect(grid).toContainText("upstream_unavailable");
    await batch.screenshot("server-batch-standalone-vben");
  });
  test("TC012a: 旧批量控制台不可访问且不再装载旧页面", async ({
    adminPage,
  }) => {
    await adminPage.goto(workspacePath("/idc/resource/server-batches"), {
      waitUntil: "domcontentloaded",
    });

    await expect(
      adminPage.getByText("未找到页面", { exact: false }),
    ).toBeVisible();
    await expect(adminPage.getByTestId("server-batch-page")).toHaveCount(0);
  });
});
