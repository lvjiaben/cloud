import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ServerAccessoriesPage } from "../pages/ServerAccessoriesPage";
import { installServerAccessoryMock, installStaffTrafficPackageMock } from "../support/server-accessory-mock";

test.describe("TC-5 服务器附属能力后台入口", () => {
  test("TC-5a: 中文页只展示本地安全投影和员工有权执行的能力", async ({
    adminPage,
  }) => {
    await installServerAccessoryMock(adminPage);
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();

    await expect(
      accessories.root.getByText("服务器管理", { exact: true }),
    ).toBeVisible();
    await expect(accessories.root).toContainText("升级前快照");
    await expect(accessories.root).toContainText("customer-10001");
    await expect(accessories.action("snapshot_restore")).toBeVisible();
    await expect(accessories.root).not.toContainText(
      /provider|credential|secret|endpoint|raw payload/iu,
    );
    await accessories.screenshot("server-accessories-zh-cn");
  });

  test("TC-5b: 恢复操作必须选择对象并手工输入同一服务编号", async ({
    adminPage,
  }) => {
    const state = await installServerAccessoryMock(adminPage);
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();
    await accessories.action("snapshot_restore").click();

    const dialog = adminPage.getByRole("dialog");
    await adminPage.getByTestId("staff-accessory-object").click();
    await adminPage.getByText("升级前快照", { exact: false }).last().click();
    await dialog.getByPlaceholder("svc-10001").fill("wrong-service");
    await dialog.getByRole("button", { name: "确认执行" }).click();
    await expect(dialog).toBeVisible();
    expect(state.requests).toHaveLength(0);

    await dialog.getByPlaceholder("svc-10001").fill("svc-10001");
    await dialog.getByRole("button", { name: "确认执行" }).click();
    await expect(dialog).toBeHidden();
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]?.method).toBe("POST");
    expect(state.requests[0]?.body).toMatchObject({
      confirmed: true,
      parameters: {
        confirmedServiceNo: "svc-10001",
        objectNo: "snap-10001",
      },
    });
  });

  test("TC-5c: 已有结果未知操作时所有新操作入口都不可点击", async ({
    adminPage,
  }) => {
    const state = await installServerAccessoryMock(adminPage, {
      activeOperation: true,
    });
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();

    await expect(accessories.root).toContainText("已有未结束操作");
    await expect(accessories.action("snapshot_delete")).toBeDisabled();
    await expect(accessories.action("snapshot_restore")).toBeDisabled();
    await expect(accessories.action("security_group_attach")).toBeDisabled();
    expect(state.requests).toHaveLength(0);
  });

  test("TC-5d: 后台收费创建只生成客户待付款账单且不提交扣余额字段", async ({
    adminPage,
  }) => {
    const state = await installServerAccessoryMock(adminPage);
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();
    await accessories.action("snapshot_create").click();

    const dialog = adminPage.getByRole("dialog");
    await adminPage.getByTestId("staff-accessory-name").fill("升级前快照");
    await dialog.getByRole("button", { name: "确认执行" }).click();

    await expect(
      adminPage.getByText(/客户付款后才会执行.*ain-pending-10001/),
    ).toBeVisible();
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]?.url).toContain("/operations/snapshot-create");
    expect(state.requests[0]?.body).toMatchObject({
      confirmed: true,
      parameters: { name: "升级前快照" },
    });
    expect(JSON.stringify(state.requests[0]?.body)).not.toMatch(
      /balance|debit|paymentSource|paid/iu,
    );
  });

  test("TC-5f: 员工只按展示的固定套餐和版本生成待付款账单", async ({ adminPage }) => {
    await installServerAccessoryMock(adminPage, { trafficPackages: true });
    const state = await installStaffTrafficPackageMock(adminPage);
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();
    await accessories.selectTrafficPackage();
    await expect(accessories.trafficPackages).toContainText("有效期按上游重置规则执行，暂未提供具体到期时间");
    await expect(accessories.trafficPackages.locator(".vxe-pager")).toContainText("2");
    await expect(accessories.trafficPackages.locator("tr", { hasText: "固定500GB" }).getByRole("button", { name: "为客户购买" })).toBeDisabled();
    const confirm = adminPage.getByRole("dialog", { name: "确认流量套餐", exact: true });
    await expect(confirm).toContainText("100 GB");
    await expect(confirm).toContainText("25.00");
    await confirm.getByRole("button", { name: "确认下单", exact: true }).click();
    await expect(confirm).toBeHidden();
    await expect(adminPage.getByText("已生成客户待付款账单，结清后才会增加流量。")).toBeVisible();
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]).toMatchObject({ packageNo: "TPK_staff_100", packageVersion: 3, parameters: {}, confirmed: true });
    expect(JSON.stringify(state.requests[0])).not.toMatch(/customerNo|amountMinor|balance|debit|paymentSource|paid/iu);
    await accessories.screenshot("staff-traffic-package-created");
  });

  test("TC-5g: 员工下单失败保留确认并以相同请求编号重试", async ({ adminPage }) => {
    await installServerAccessoryMock(adminPage, { trafficPackages: true });
    const state = await installStaffTrafficPackageMock(adminPage);
    state.failNext = true;
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();
    await accessories.selectTrafficPackage();
    const confirm = adminPage.getByRole("dialog", { name: "确认流量套餐", exact: true });
    await confirm.getByRole("button", { name: "确认下单", exact: true }).click();
    await expect(adminPage.getByText("测试请求暂未完成，请重试")).toBeVisible();
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "确认下单", exact: true }).click();
    await expect(confirm).toBeHidden();
    expect(state.requests).toHaveLength(2);
    expect(state.requests[1]).toEqual(state.requests[0]);
  });

  test("TC-5e: 英文和手机宽度下完整翻译且页面不横向溢出", async ({
    adminPage,
    mainLayout,
  }) => {
    await installServerAccessoryMock(adminPage);
    const accessories = new ServerAccessoriesPage(adminPage);
    await accessories.goto();
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ height: 844, width: 390 });

    await expect(
      accessories.root.getByText("Server Management", {
        exact: true,
      }),
    ).toBeVisible();
    await expect(accessories.root).not.toContainText("plugin.idc-resource");
    const overflow = await adminPage.evaluate(
      () =>
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
    await accessories.screenshot("server-accessories-en-us-mobile");
  });
});
