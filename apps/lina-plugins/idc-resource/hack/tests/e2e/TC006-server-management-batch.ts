import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ServerAccessoriesPage } from "../pages/ServerAccessoriesPage";
import { installServerAccessoryMock } from "../support/server-accessory-mock";

test.describe("TC-6 服务器管理内批量操作", () => {
  test("TC-6a: 在服务器列表勾选后通过大抽屉预览并执行", async ({
    adminPage,
  }) => {
    const state = await installServerAccessoryMock(adminPage);
    const servers = new ServerAccessoriesPage(adminPage);
    await servers.gotoManagement();

    await expect(
      servers.root.getByText("云服务器", { exact: true }),
    ).toBeVisible();
    await expect(
      servers.root.getByText("裸金属服务器", { exact: true }),
    ).toBeVisible();
    await servers.selectService("svc-10001");
    await servers.selectService("svc-10002");
    await servers.openBatch();

    const drawer = adminPage
      .getByRole("dialog")
      .filter({ hasText: "服务器批量操作" });
    await expect(drawer).toContainText("已选择 2 台服务器");
    await expect(drawer).not.toContainText("plugin.idc-resource");
    await drawer.getByRole("button", { name: "预览影响" }).click();
    await expect(drawer.getByText("svc-10001", { exact: true })).toBeVisible();
    await expect(drawer.getByText("svc-10002", { exact: true })).toBeVisible();
    await expect(drawer.getByText("可以执行", { exact: true })).toHaveCount(2);
    await expect(
      drawer.getByText("云服务器 / primary", { exact: true }),
    ).toHaveCount(2);
    await expect(
      adminPage.getByText("IDC Operations", { exact: true }),
    ).toHaveCount(0);
    await drawer.getByRole("checkbox").check();
    await adminPage.getByTestId("server-batch-execute").click();

    expect(state.batchRequests).toHaveLength(2);
    expect(state.batchRequests[0]?.body).toMatchObject({
      action: "renewal_reminder",
      items: [
        { parameters: {}, serviceNo: "svc-10001" },
        { parameters: {}, serviceNo: "svc-10002" },
      ],
    });
    await servers.screenshot("server-management-batch-drawer");
  });
});
