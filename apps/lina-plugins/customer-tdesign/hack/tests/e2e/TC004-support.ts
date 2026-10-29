import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-4 工单", () => {
  test("TC-4a 创建失败保留内容，成功后可回复与关闭", async ({
    consolePage: page,
  }) => {
    let fail = true,
      version = 1,
      status = "waiting_staff";
    const messages = [
      {
        messageNo: "M1",
        author: "customer",
        body: "连接失败，请协助检查",
        createdAt: now,
        attachments: [],
        automated: false,
      },
    ];
    const payload = () => ({
      item: {
        ticket: {
          ticketNo: "TKT-1",
          subject: "连接问题",
          status,
          version,
          currentRound: 1,
          createdAt: now,
          category: { nameZhCN: "技术支持" },
        },
        messages,
        allowedActions: status === "closed" ? ["reopen"] : ["reply", "close"],
      },
    });
    await page.route("**/api/v1/account/support/categories*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [{ categoryNo: "CAT-1", nameZhCN: "技术支持" }],
            nextCursor: "",
          },
        },
      }),
    );
    await page.route("**/api/v1/account/support/custom-fields", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    await page.route("**/api/v1/account/support/entitlement-candidates*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [], nextCursor: "" } } }),
    );
    await page.route("**/api/v1/account/support/tickets", (r) =>
      r.fulfill(
        fail
          ? {
              status: 503,
              json: { code: 503, message: "服务繁忙，请稍后重试" },
            }
          : { json: { code: 0, data: payload() } },
      ),
    );
    await page.route("**/api/v1/account/support/tickets/TKT-1", (r) =>
      r.fulfill({ json: { code: 0, data: payload() } }),
    );
    await page.route("**/api/v1/account/support/tickets/TKT-1/replies", (r) => {
      const body = r.request().postDataJSON();
      expect(body.ticketVersion).toBe(version);
      messages.push({
        messageNo: "M2",
        author: "customer",
        body: body.body,
        createdAt: now,
        attachments: [],
        automated: false,
      });
      version++;
      return r.fulfill({ json: { code: 0, data: payload() } });
    });
    await page.route(
      "**/api/v1/account/support/tickets/TKT-1/closures",
      (r) => {
        expect(r.request().postDataJSON()).toMatchObject({
          confirmed: true,
          ticketVersion: version,
        });
        status = "closed";
        return r.fulfill({ json: { code: 0, data: payload() } });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/support/new");
    await page.locator(".form-card .t-select").first().click();
    await page.getByText("技术支持", { exact: true }).click();
    await page.getByPlaceholder("用一句话描述问题").fill("连接问题");
    await page
      .getByPlaceholder(
        "例如：什么时候出现问题、已经尝试过哪些操作。请勿填写密码。",
      )
      .fill("连接失败，请协助检查");
    await view.button("提交工单").click();
    await expect(page.getByText("服务繁忙，请稍后重试")).toBeVisible();
    await expect(page.getByPlaceholder("用一句话描述问题")).toHaveValue(
      "连接问题",
    );
    await view.screenshot("ticket-submit-error");
    fail = false;
    await view.button("提交工单").click();
    await expect(page.getByRole("heading", { name: "连接问题" })).toBeVisible();
    await page
      .getByRole("textbox", { name: "回复内容" })
      .fill("补充：已检查本地网络");
    await view.button("发送回复").click();
    await expect(
      page.getByText("补充：已检查本地网络", { exact: true }),
    ).toBeVisible();
    await view.button("问题已解决，关闭工单").click();
    await page.getByRole("button", { name: "确认", exact: true }).click();
    await expect(page.getByText("已关闭", { exact: true })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "回复内容" })).toHaveCount(
      0,
    );
    await view.screenshot("ticket-closed");
  });
  test("TC-4b 相关服务在下拉框搜索和滚动加载，不显示分页", async ({
    consolePage: page,
  }) => {
    await page.route("**/api/v1/account/support/categories*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [], nextCursor: "" } } }),
    );
    await page.route("**/api/v1/account/support/custom-fields", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    const queries: string[] = [];
    await page.route(
      "**/api/v1/account/support/entitlement-candidates*",
      (r) => {
        const params = new URL(r.request().url()).searchParams;
        queries.push(params.get("keyword") || "");
        const second = params.get("cursor") === "page2";
        return r.fulfill({
          json: {
            code: 0,
            data: {
              items: second ? [{ associationReference: "service-51", nameZhCN: "第二批云服务器" }] : Array.from({length:50}, (_, i) => ({ associationReference: `service-${i+1}`, nameZhCN: `云服务器 ${i+1}` })),
              nextCursor: second ? "" : "page2",
            },
          },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/support/new");
    await expect(page.getByRole("button", { name: "下一页", exact: true })).toHaveCount(0);
    await view.openServiceChoices();
    await view.scrollChoicesToBottom();
    await page.getByText("第二批云服务器", { exact: true }).click();
    await expect(page.locator(".form-card .t-select input").nth(1)).toHaveValue("第二批云服务器");
    await view.searchServiceChoices("目标服务");
    await expect.poll(() => queries.includes("目标服务")).toBeTruthy();
    await view.screenshot("ticket-searchable-select");
  });
  test("TC-4c 附件组件可移除，上传失败保留文件，重试只提交当前选择", async ({consolePage: page}) => {
    await page.route("**/support/categories*", r => r.fulfill({json:{code:0,data:{items:[{categoryNo:"CAT-1",nameZhCN:"技术支持"}],nextCursor:""}}}));
    await page.route("**/support/custom-fields", r => r.fulfill({json:{code:0,data:{items:[]}}}));
    await page.route("**/support/entitlement-candidates*", r => r.fulfill({json:{code:0,data:{items:[],nextCursor:""}}}));
    let fail = true, created = 0;
    const names: string[] = [];
    await page.route("**/support/initial-attachment-uploads", r => {
      names.push(r.request().postDataJSON().fileName);
      return r.fulfill({json:{code:0,data:{uploadNo:"UP-1"}}});
    });
    await page.route("**/support/initial-attachment-uploads/UP-1/content", r => r.fulfill(fail ? {status:503,json:{code:503,message:"附件上传失败，请重试"}} : {json:{code:0,data:{uploadNo:"UP-1",ready:true}}}));
    await page.route("**/support/tickets", r => {
      created++;
      expect(r.request().postDataJSON().uploadNos).toEqual(["UP-1"]);
      return r.fulfill({status:503,json:{code:503,message:"工单提交失败，内容已保留"}});
    });
    const view = new CustomerPage(page);
    await view.open("/account/support/new");
    await page.locator(".form-card .t-select").first().click();
    await page.getByText("技术支持", {exact:true}).click();
    await page.getByPlaceholder("用一句话描述问题").fill("附件检查");
    await page.locator("textarea").fill("附上问题日志");
    await view.attachTicketFiles(["keep.txt", "remove.txt"]);
    await expect(page.getByText("keep.txt", {exact:true})).toBeVisible();
    await view.removeTicketFile("remove.txt");
    await expect(page.getByText("remove.txt", {exact:true})).toHaveCount(0);
    await view.button("提交工单").click();
    await expect(page.getByText("附件上传失败，请重试", {exact:true})).toBeVisible();
    expect(created).toBe(0);
    await expect(page.getByText("keep.txt", {exact:true})).toBeVisible();
    fail = false;
    await view.button("提交工单").click();
    await expect(page.getByText("工单提交失败，内容已保留", {exact:true})).toBeVisible();
    expect(created).toBe(1);
    expect(names).toEqual(["keep.txt", "keep.txt"]);
    await page.setViewportSize({width:390,height:844});
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await view.screenshot("ticket-attachments-mobile");
  });

});
