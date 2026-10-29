import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-7 账户自助", () => {
  test("TC-7a 联系人新增失败保留内容，成功后可移除", async ({
    consolePage: page,
  }) => {
    let failed = true;
    const items: any[] = [];
    const keys: string[] = [];
    await page.route("**/account/customer/contacts", (r) => {
      if (r.request().method() === "GET")
        return r.fulfill({ json: { code: 0, data: { items } } });
      const input = r.request().postDataJSON();
      keys.push(input.idempotencyKey);
      if (failed)
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "联系人保存失败，请重试" },
        });
      items.push({
        ...input,
        contactNo: "CTC-test",
        version: 1,
        status: "active",
        emailVerified: false,
      });
      return r.fulfill({ json: { code: 0, data: { item: items[0] } } });
    });
    await page.route("**/account/customer/contacts/CTC-test/archive", (r) => {
      expect(r.request().postDataJSON().expectedVersion).toBe(1);
      items[0].status = "archived";
      return r.fulfill({ json: { code: 0, data: { item: items[0] } } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/contacts");
    await view.button("添加联系人").click();
    await page.getByLabel("姓名", { exact: true }).fill("业务通知人");
    await page
      .getByLabel("邮箱", { exact: true })
      .fill("notice@example.invalid");
    await view.button("保存").click();
    await expect(page.getByText("联系人保存失败，请重试")).toBeVisible();
    await expect(page.getByLabel("姓名", { exact: true })).toHaveValue(
      "业务通知人",
    );
    await view.screenshot("contacts-save-error");
    failed = false;
    await view.button("保存").click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    expect(keys[0]).toBe(keys[1]);
    await expect(view.main).toContainText("业务通知人");
    await view.button("移除").click();
    await view.button("确认").click();
    await expect(view.main).toContainText("已移除");
    await view.screenshot("contacts-archived");
  });
  test("TC-7b 导出使用短期证明，注销有阻碍时不展示已完成", async ({
    consolePage: page,
  }) => {
    await page.route("**/account/customer/profile", (r) =>
      r.fulfill({ json: { code: 0, data: { item: { version: 4 } } } }),
    );
    await page.route("**/data-exports/proof", (r) => {
      expect(r.request().postDataJSON()).toMatchObject({
        expectedVersion: 4,
        confirmed: true,
      });
      return r.fulfill({
        json: { code: 0, data: { proofToken: "single-use-proof" } },
      });
    });
    await page.route("**/customer/data-exports", (r) => {
      expect(r.request().postDataJSON()).toMatchObject({
        proofToken: "single-use-proof",
        expectedVersion: 4,
        confirmed: true,
      });
      return r.fulfill({ json: { code: 0, data: { requestNo: "DEX-test" } } });
    });
    await page.route("**/customer/data-exports/DEX-test", (r) =>
      r.fulfill({
        json: { code: 0, data: { status: "queued", requestedAt: now } },
      }),
    );
    await page.route("**/customer/closure-requests", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            item: {
              status: "blocked",
              findings: [{ code: "active_services", count: 2 }],
            },
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/privacy");
    await view.button("申请下载资料").click();
    await view.button("确认").click();
    await expect(page.getByText("等待处理", { exact: true })).toBeVisible();
    await expect(view.button("下载文件")).toHaveCount(0);
    await view.button("申请注销").click();
    await view.button("确认").click();
    await expect(
      page.getByText("暂时无法申请，请先处理以下事项"),
    ).toBeVisible();
    await expect(view.main).toContainText("仍有使用中的服务（2项）");
    await view.screenshot("privacy-blocked");
  });
});
