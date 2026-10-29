// Resource workspace feedback: current identity, local facts and exact scoped search/notes.
import { test, expect, menus, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const item = {
  entitlementNo: "ent_resource",
  productNameZhCn: "生产云服务器",
  productNameEnUs: "Cloud",
  entitlementOwnerId: "idc-resource",
  statusCode: "active",
  orderNo: "ORD-TEST-001",
  customerRemark: "网站环境",
  customerRemarkRevision: 1,
  currentPriceAmountMinor: "1550",
  currencyCode: "USD",
  periodUnitCode: "month",
  periodCount: 1,
  chargeCycles: 1,
  expiresAt: now + 86400000,
};
const fact = {
  entitlementNo: item.entitlementNo,
  serverNo: "SVR-resource",
  hostname: "web-01",
  addresses: ["192.0.2.40"],
  addressesConfirmed: true,
  osName: "Ubuntu 24.04",
  lastObservedState: "running",
  stale: false,
  observedAt: now,
};
test.describe("TC-18 资源工作台", () => {
  test("TC-18a 首页展示真实账号、最近订单和有界资源事实", async ({
    consolePage: page,
  }) => {
    let batchCalls = 0;
    await page.route("**/product-directories/all/services?*", (r) => {
      expect(new URL(r.request().url()).searchParams.get("pageSize")).toBe("5");
      return r.fulfill({
        json: { code: 0, data: { items: [item], total: 1 } },
      });
    });
    await page.route("**/account/native-server-facts?*", (r) => {
      batchCalls++;
      expect(
        new URL(r.request().url()).searchParams.getAll("entitlementNos[]"),
      ).toEqual([item.entitlementNo]);
      return r.fulfill({ json: { code: 0, data: { items: [fact] } } });
    });
    const view = new CustomerPage(page);
    await view.open("/account");
    await expect(view.main).toContainText("账号 ID");
    await expect(view.main).toContainText("42");
    await expect(view.main).toContainText("最近订单");
    await expect(view.main).toContainText("ORD-TEST-001");
    await expect(view.main).toContainText("资源列表");
    await expect(view.main).toContainText("web-01");
    await expect(view.main).toContainText("192.0.2.40");
    await expect(view.main).toContainText("USD 15.50");
    expect(batchCalls).toBe(1);
    await view.screenshot("overview-resource-facts");
  });
  test("TC-18b 分类标签、服务端搜索真实总数及备注失败恢复", async ({
    consolePage: page,
  }) => {
    let note = "网站环境",
      revision = 1,
      fail = true;
    const queries: string[] = [];
    await page.route("**/account/product-directories?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              ...menus,
              {
                directoryNo: "db",
                nameZhCn: "数据库",
                navigationType: "product",
                sortOrder: 2,
              },
            ],
            total: menus.length + 1,
          },
        },
      }),
    );
    await page.route("**/product-directories/*/services?*", (r) => {
      const u = new URL(r.request().url());
      queries.push(u.searchParams.get("keyword") || "");
      return r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                ...item,
                customerRemark: note,
                customerRemarkRevision: revision,
              },
            ],
            total: 1,
            providers: [{ pluginId: "idc-resource", total: 1 }],
          },
        },
      });
    });
    await page.route("**/account/native-server-facts?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [fact] } } }),
    );
    await page.route(
      "**/service-entitlements/ent_resource/customer-note",
      (r) => {
        expect(r.request().method()).toBe("PUT");
        const body = r.request().postDataJSON();
        expect(body).toEqual({ remark: "新的生产环境", expectedRevision: 1 });
        if (fail)
          return r.fulfill({
            status: 503,
            json: { code: 503, message: "暂时不能保存，请重试" },
          });
        note = body.remark;
        revision++;
        return r.fulfill({
          json: {
            code: 0,
            data: {
              note: {
                entitlementNo: item.entitlementNo,
                remark: note,
                revision,
              },
            },
          },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cloud");
    await expect(
      page.getByRole("tab", { name: "云服务器", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("combobox", { name: "服务分类" })).toHaveCount(
      0,
    );
    await expect(
      view.main.getByRole("button", { name: "续费", exact: true }),
    ).toHaveCount(0);
    await expect(
      view.main.getByRole("link", { name: "提工单", exact: true }),
    ).toHaveCount(0);
    await view.searchServices("生产");
    await expect.poll(() => queries.includes("生产")).toBe(true);
    await expect(view.serviceBody).toContainText(/共\s*1\s*条/);
    await view.openServiceNote("生产云服务器");
    const dialog = view.serviceNoteDialog;
    await expect(dialog).toContainText("编辑服务备注");
    await dialog.locator("textarea").fill("新的生产环境");
    await view.saveServiceNote();
    await expect(dialog).toContainText("暂时不能保存，请重试");
    await expect(dialog.locator("textarea")).toHaveValue("新的生产环境");
    await view.screenshot("resource-note-retry");
    fail = false;
    await view.saveServiceNote();
    await expect(dialog).toHaveCount(0);
    await expect(view.serviceBody).toContainText("新的生产环境");
    await page.getByRole("tab", { name: "数据库", exact: true }).click();
    await expect(page).toHaveURL(/product-directories\/db/);
    await expect(
      view.serviceBody.getByRole("textbox", { name: "搜索服务", exact: true }),
    ).toHaveValue("");
    await view.screenshot("resource-tabs-search");
  });
  test("TC-18c 服务工单预选本人不透明关联，不将服务号冒充工单标识", async ({
    consolePage: page,
  }) => {
    await page.route("**/service-entitlements/ent_owned/customer-note", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            note: { entitlementNo: "ent_owned", remark: "", revision: 1 },
            supportAssociationReference: "sar_owned",
          },
        },
      }),
    );
    await page.route("**/support/custom-fields", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    await page.route("**/support/categories?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [{ categoryNo: "cat_tech", nameZhCN: "技术支持" }],
            nextCursor: "",
          },
        },
      }),
    );
    await page.route("**/support/entitlement-candidates?*", (r) => {
      const exact = new URL(r.request().url()).searchParams.get(
        "associationReference",
      );
      return r.fulfill({
        json: {
          code: 0,
          data: {
            items: exact
              ? [
                  {
                    associationReference: "sar_owned",
                    nameZhCN: "生产云服务器",
                    status: "active",
                  },
                ]
              : [],
            nextCursor: "",
          },
        },
      });
    });
    let submitted = false;
    await page.route("**/support/tickets", (r) => {
      submitted = true;
      expect(r.request().postDataJSON().associationReference).toBe("sar_owned");
      return r.fulfill({
        status: 503,
        json: { code: 503, message: "工单暂时无法提交，请重试" },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/support/new?service=ent_owned");
    await expect(page.getByPlaceholder("选择或搜索相关服务")).toHaveValue(
      "生产云服务器",
    );
    await page.getByPlaceholder("选择或搜索问题类型").click();
    await page.getByText("技术支持", { exact: true }).click();
    await page.getByPlaceholder("用一句话描述问题").fill("无法连接");
    await page
      .getByPlaceholder(/例如：什么时候/)
      .fill("今天开始无法连接，已经重启本地网络。");
    await view.button("提交工单").click();
    await expect.poll(() => submitted).toBe(true);
    await expect(view.main).toContainText("工单暂时无法提交，请重试");
    await expect(page.getByPlaceholder("用一句话描述问题")).toHaveValue(
      "无法连接",
    );
    await view.screenshot("associated-ticket-error");
  });
});
