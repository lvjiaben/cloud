import { expect, test } from "@host-tests/fixtures/auth";
import { TrafficPackagePage } from "../pages/TrafficPackagePage";
import { installTrafficPackageMock } from "../support/traffic-package-mock";

test.describe("TC-22 固定流量套餐管理", () => {
  test("TC-22d: 已选商品名称缺失不显示内部编号或清除原适用范围", async ({ adminPage }) => {
    const state = await installTrafficPackageMock(adminPage);
    state.missingProductName = true;
    const view = new TrafficPackagePage(adminPage);
    await view.open();
    await view.edit("回归流量包 1");
    await expect(view.editor).toContainText("商品名称暂不可用");
    await expect(view.editor).not.toContainText("prd_test_server");
    await view.screenshot("missing-product-name");
    await view.confirm();
    await expect(view.editor).toBeHidden();
    expect(state.writes.at(-1)).toMatchObject({ productNos: ["prd_test_server"] });
    state.missingProductName = false;
    await view.edit("回归流量包 1");
    await expect(view.editor).toContainText("回归云服务器");
    await expect(view.editor).not.toContainText("商品名称暂不可用");
    await view.cancel();
  });

  test("TC-22c: 商品选项失败禁止保存，关闭后可以重新加载", async ({ adminPage }) => {
    const state = await installTrafficPackageMock(adminPage);
    const view = new TrafficPackagePage(adminPage);
    await view.open();
    state.lookupFailure = true;
    await view.root.getByRole("button", { name: "新增套餐", exact: true }).click();
    const dialog = adminPage.getByRole("dialog", { name: "新增套餐", exact: true });
    await expect(adminPage.getByText("商品选项暂时不可用", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /^确\s*认$/u })).toBeDisabled();
    expect(state.writes).toHaveLength(0);
    await dialog.getByRole("button", { name: /^取\s*消$/u }).click();
    await expect(dialog).toBeHidden();
    state.lookupFailure = false;
    await view.create();
    await expect(view.editor.getByRole("textbox", { name: "中文名称" })).toHaveValue("");
    await expect(view.editor.getByRole("button", { name: /^确\s*认$/u })).toBeEnabled();
    await view.cancel();
  });

  test("TC-22a: 数字分页、搜索、总量编辑和下架保持已有占用", async ({ adminPage }) => {
    const state = await installTrafficPackageMock(adminPage);
    const view = new TrafficPackagePage(adminPage);
    await view.open();
    await expect(view.rows).toHaveCount(20);
    await expect(view.pager).toContainText("共 25 条记录");
    await view.screenshot("list");
    await view.pager.getByRole("button", { name: "2", exact: true }).click();
    await expect(view.rows).toHaveCount(5);
    await expect.poll(() => state.pages.at(-1)).toBe(2);
    await view.search("回归流量包 1");
    await expect(view.rows).toHaveCount(11);
    await expect.poll(() => state.pages.at(-1)).toBe(1);
    await view.screenshot("search");
    await view.edit("回归流量包 1");
    await expect(view.editor).toContainText("总可售份数");
    await expect(view.editor).toContainText("回归云服务器");
    await view.setTotal(20);
    await view.editor.getByRole("switch", { name: "上架销售" }).click();
    await view.screenshot("edit");
    await view.confirm();
    await expect(view.editor).not.toBeVisible();
    expect(state.writes.at(-1)).toMatchObject({ stockQuantity: 20, expectedVersion: 1, enabled: false });
    expect(state.rows[0]).toMatchObject({ occupiedQuantity: 30, remainingQuantity: 0, version: 2 });
    await expect(view.rows.first()).toContainText("已下架");
    await view.screenshot("saved");
  });

  test("TC-22b: 新建冻结含税总价、错误不关闭、重复打开清空旧表单", async ({ adminPage }) => {
    const state = await installTrafficPackageMock(adminPage);
    const view = new TrafficPackagePage(adminPage);
    await view.open();
    await view.create();
    await view.fillNew("界面回归专用套餐");
    await expect(view.editor).toContainText("最终售价（含税）");
    await expect(view.editor).not.toContainText("plugin.idc-core");
    state.failNext = true;
    await view.confirm();
    await expect(adminPage.getByText("流量套餐已变更，请重新确认", { exact: true })).toBeVisible();
    await expect(view.editor).toBeVisible();
    await view.screenshot("error");
    expect(state.writes[0]).toMatchObject({ amountMinor: 1250, trafficGb: 100, stockQuantity: 100, productNos: ["prd_test_server"], enabled: true });
    await view.confirm();
    await expect(view.editor).not.toBeVisible();
    await expect(view.rows.first()).toContainText("界面回归专用套餐");
    await view.screenshot("created");
    await view.create();
    await expect(view.editor.getByRole("textbox", { name: "中文名称" })).toHaveValue("");
    await expect(view.editor.getByRole("switch", { name: "上架销售" })).not.toBeChecked();
    await view.cancel();
    await view.create();
    await expect(view.editor.getByRole("textbox", { name: "中文名称" })).toHaveValue("");
    await view.cancel();
  });
});
