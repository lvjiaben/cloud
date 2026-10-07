import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { MainLayout } from "../../../../../../hack/tests/pages/MainLayout";
import { ChannelVbenPage } from "../pages/ChannelVbenPage";
import { mockChannelWorkspace } from "../support/channel-workspace-mock";
import { openChannelStandalone } from "../support/channel-standalone";

test.describe("TC-8 Channel Vben tables and forms", () => {
  test("TC-8f: numeric pages, size changes, exact totals and failed page retries", async ({
    adminPage,
  }) => {
    const mock = await mockChannelWorkspace(adminPage);
    mock.largeLists = true;
    const page = new ChannelVbenPage(adminPage);
    await page.goto();
    const accounts = page.grid("accounts");
    await expect(accounts.locator(".vxe-pager--total")).toContainText("26");
    await expect(
      accounts.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
    await page.numberedPage("accounts", 3);
    await expect(accounts).toContainText("渠道20");
    await expect(
      accounts.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(6);
    expect(
      mock.pageReads.some(
        (read) =>
          read.path === "accounts" && read.page === 3 && read.pageSize === 10,
      ),
    ).toBe(true);
    await page.pageSize("accounts", 20);
    await expect(accounts.locator(".vxe-pager--num-btn.is--active")).toHaveText(
      "1",
    );
    await expect(
      accounts.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(20);
    mock.failPage = "accounts";
    await page.numberedPage("accounts", 2);
    await expect(page.notice).toContainText("暂时无法读取渠道数据");
    await expect(accounts.locator(".vxe-pager--num-btn.is--active")).toHaveText(
      "1",
    );
    await expect(accounts).toContainText("主渠道");
    await page.snapshot("channel-page-failed-retained");
    mock.failPage = "";
    await page.refresh();
    await expect(accounts.locator(".vxe-pager--num-btn.is--active")).toHaveText(
      "2",
    );
    await expect(accounts).toContainText("渠道20");
    await page.selectChannel();
    await page.tab("额度与结算");
    for (const [index, prefix] of [
      [0, "INV-001"],
      [1, "CST-001"],
      [2, "CRP-001"],
    ] as const) {
      await expect(
        page.grid("finance", index).locator(".vxe-pager--total"),
      ).toContainText("27");
      await page.numberedPage("finance", 3, index);
      await expect(page.grid("finance", index)).toContainText(prefix + "-21");
      await expect(
        page
          .grid("finance", index)
          .locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(7);
    }
    await page.snapshot("channel-finance-independent-pages");
    for (const [tab, panel, total, rows] of [
      ["商品发布", "publications", 27, 7],
      ["令牌", "tokens", 24, 4],
      ["健康与审计", "health", 27, 7],
    ] as const) {
      await page.tab(tab);
      await expect(page.grid(panel).locator(".vxe-pager--total")).toContainText(
        String(total),
      );
      await page.numberedPage(panel, 3);
      await expect(
        page.grid(panel).locator(".vxe-pager--num-btn.is--active"),
      ).toHaveText("3");
      await expect(
        page.grid(panel).locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(rows);
    }
    await page.tab("能力清单");
    await expect(
      page.grid("capabilities").locator(".vxe-pager--total"),
    ).toContainText("3");
  });
  test("TC-8g: isolated runtime uses the same exact backend pages", async ({
    adminPage,
  }) => {
    const mock = await mockChannelWorkspace(adminPage);
    mock.largeLists = true;
    await openChannelStandalone(adminPage, "admin-mount");
    const page = new ChannelVbenPage(adminPage);
    await expect(
      page.grid("accounts").locator(".vxe-pager--total"),
    ).toContainText("26");
    await page.numberedPage("accounts", 3);
    await expect(page.grid("accounts")).toContainText("渠道20");
    await expect(
      page.grid("accounts").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(6);
    await page.pageSize("accounts", 20);
    await expect(
      page.grid("accounts").locator(".vxe-pager--num-btn.is--active"),
    ).toHaveText("1");
    expect(
      mock.pageReads.some(
        (read) => read.path === "accounts" && read.pageSize === 20,
      ),
    ).toBe(true);
    // The opaque bridge deliberately hides HTTP status. Unknown denial must fail closed.
    mock.failPage = "accounts";
    await page.numberedPage("accounts", 2);
    await expect(
      page.grid("accounts").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(0);
    await expect(page.notice).not.toBeEmpty();
    mock.failPage = "";
    await page.refresh();
    await expect(
      page.grid("accounts").locator(".vxe-pager--total"),
    ).toContainText("26");
    await expect(
      page.grid("accounts").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
    await page.selectStandaloneChannel();
    await page.tab("额度与结算");
    await page.numberedPage("finance", 3, 1);
    await expect(page.grid("finance", 1)).toContainText("CST-001-21");
    await expect(page.grid("finance", 0)).toContainText("INV-001");
    await page.snapshot("channel-isolated-full-pagination");
    for (const [tab, panel, total, rows] of [
      ["商品发布", "publications", 27, 7],
      ["令牌", "tokens", 24, 4],
      ["健康与审计", "health", 27, 7],
    ] as const) {
      await page.tab(tab);
      await expect(page.grid(panel).locator(".vxe-pager--total")).toContainText(
        String(total),
      );
      await page.numberedPage(panel, 3);
      await expect(
        page.grid(panel).locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(rows);
    }
  });
  test("TC-8a: all eight tables render bounded Vben grids with paging and preserved data", async ({
    adminPage,
  }) => {
    const mock = await mockChannelWorkspace(adminPage);
    const page = new ChannelVbenPage(adminPage);
    await page.goto();
    await expect(page.panel("accounts")).toContainText("主渠道");
    await expect(page.panel("accounts").locator(".vxe-grid")).toHaveCount(1);
    await expect(page.root).not.toContainText("plugin.idc-channel.workspace.");
    for (const width of [1366, 1920, 768]) {
      await adminPage.setViewportSize({ width, height: 1000 });
      await expect
        .poll(async () =>
          (await page.geometry()).every(
            (grid) => grid.right <= width + 1 && grid.width > 200,
          ),
        )
        .toBe(true);
      const [grid] = await page.geometry();
      expect(grid!.rows).toBeGreaterThan(1);
      expect(grid!.height).toBeLessThan(600);
      expect(grid!.pager).toBe(true);
      await page.snapshot("channel-" + width);
    }
    await page.nextPage("accounts");
    await expect(page.panel("accounts")).toContainText("渠道10");
    await adminPage.setViewportSize({ width: 1366, height: 1000 });
    await page.selectChannel();
    for (const [tab, panel, text, count] of [
      ["商品发布", "publications", "CNY 88.00", 1],
      ["令牌", "tokens", "catalog.read", 1],
      ["额度与结算", "finance", "INV-001", 3],
      ["健康与审计", "health", "req-001", 1],
      ["能力清单", "capabilities", "已启用（业务校验保护）", 1],
    ] as const) {
      await page.tab(tab);
      await expect(page.panel(panel)).toContainText(text);
      await expect(page.panel(panel).locator(".vxe-grid")).toHaveCount(count);
      await expect(
        page.panel(panel).locator(".data-table,.table-shell"),
      ).toHaveCount(0);
      expect((await page.geometry()).every((grid) => grid.right <= 1367)).toBe(
        true,
      );
      await page.snapshot("channel-" + panel);
    }
    expect(mock.reads.filter((item) => item === "finance")).toHaveLength(1);
  });
  test("TC-8b: Vben forms preserve channel, publication and one-time-token operations and errors", async ({
    adminPage,
  }) => {
    const mock = await mockChannelWorkspace(adminPage);
    const page = new ChannelVbenPage(adminPage);
    await page.goto();
    await expect(page.panel("accounts")).toContainText("主渠道");
    await page.createChannel();
    await expect(page.notice).toContainText("渠道已保存");
    expect(mock.created).toEqual({
      channelId: "new_channel",
      customerNo: "PUBLIC-101",
      displayName: "新渠道",
    });
    await expect(page.panel("accounts")).toContainText("新渠道");
    await page.snapshot("channel-created");
    await page.selectChannel();
    await page.status("暂停停新");
    await expect(page.notice).toContainText("渠道状态操作已完成");
    expect(mock.status).toBe("pause");
    await page.tab("商品发布");
    await expect(page.panel("publications")).toContainText("server-basic");
    await page.publish();
    await expect(page.notice).toContainText("商品发布已通过核心价格校验并保存");
    expect(mock.publication).toMatchObject({
      channelId: "channel_primary",
      productVersionId: 101,
      publicSlug: "server-basic",
      configurationRevision: "r1",
      billingTermToken: "monthly",
      currency: "CNY",
      purchaseCycles: 1,
      quantity: 1,
      stock: 0,
      status: "published",
      values: [{ fieldToken: "cpu", type: "single_line", textValue: "4" }],
    });
    await page.snapshot("channel-published");
    await page.tab("令牌");
    await expect(page.panel("tokens")).toContainText("catalog.read");
    await page.createToken();
    await expect(page.secret).toHaveText("test-one-time-secret");
    expect(mock.token).toMatchObject({
      channelId: "channel_primary",
      scopes: ["catalog.read", "order.create"],
      sourceCidrs: [],
    });
    await page.revokeToken();
    await expect(page.notice).toContainText("令牌已撤销");
    expect(mock.revoked).toBe(true);
    await expect(page.panel("tokens")).toContainText("已撤销");
    await page.snapshot("channel-token-revoked");
    mock.failTokens = true;
    await page.refresh();
    await expect(page.notice).toContainText("暂时无法读取渠道数据");
    await expect(page.secret).not.toContainText("test-one-time-secret");
    await expect(
      page.panel("tokens").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(0);
    await page.selectChannel("渠道1 · channel_1");
    await expect(page.notice).toContainText("暂时无法读取渠道数据");
    await expect(page.panel("tokens").locator(".vxe-body--row")).toHaveCount(0);
    await page.snapshot("channel-token-forbidden");
  });
  test("TC-8c: slow token writes stay attached to their original channel after switching", async ({
    adminPage,
  }) => {
    const mock = await mockChannelWorkspace(adminPage);
    const page = new ChannelVbenPage(adminPage);
    await page.goto();
    await page.selectChannel();
    await page.tab("令牌");
    await expect(page.panel("tokens")).toContainText("catalog.read");
    let release = () => {};
    mock.delayTokenWrite = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.createToken();
    await expect.poll(() => mock.token?.channelId).toBe("channel_primary");
    await page.selectChannel("渠道1 · channel_1");
    await expect(page.panel("tokens")).toContainText("channel-one.read");
    release();
    await expect.poll(() => mock.completedTokenWrites).toBe(1);
    await expect(
      page.root.getByRole("button", { name: "创建一次性令牌", exact: true }),
    ).toBeEnabled();
    await expect(page.panel("tokens")).toContainText("channel-one.read");
    await expect(page.secret).not.toContainText("test-one-time-secret");
    await expect(page.notice).toHaveCount(0);
    await page.snapshot("channel-slow-create-isolated");

    await page.selectChannel();
    await expect(page.panel("tokens")).toContainText("catalog.read");
    mock.delayTokenWrite = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.revokeToken();
    await page.selectChannel("渠道1 · channel_1");
    await expect(page.panel("tokens")).toContainText("channel-one.read");
    release();
    await expect.poll(() => mock.completedTokenWrites).toBe(2);
    await expect(
      page.root.getByRole("button", { name: "创建一次性令牌", exact: true }),
    ).toBeEnabled();
    await expect(page.panel("tokens")).toContainText("channel-one.read");
    await expect(page.notice).toHaveCount(0);
    await page.snapshot("channel-slow-revoke-isolated");

    await page.selectChannel();
    await expect(page.panel("tokens")).toContainText("catalog.read");
    mock.delayNextTokenRead = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.createToken();
    await expect.poll(() => mock.startedDelayedTokenReads).toBe(1);
    await page.numberedPage("tokens", 3);
    await expect(
      page.grid("tokens").locator(".vxe-pager--num-btn.is--active"),
    ).toHaveText("3");
    await expect(
      page.grid("tokens").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(4);
    release();
    await expect(
      page.root.getByRole("button", { name: "创建一次性令牌", exact: true }),
    ).toBeEnabled();
    await expect(
      page.grid("tokens").locator(".vxe-pager--num-btn.is--active"),
    ).toHaveText("3");
    await expect(
      page.grid("tokens").locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(4);
    await page.snapshot("channel-write-read-page-isolated");
  });
  test("TC-8d: English columns, forms and state labels are translated", async ({
    adminPage,
  }) => {
    await mockChannelWorkspace(adminPage);
    const page = new ChannelVbenPage(adminPage);
    await page.goto();
    await new MainLayout(adminPage).switchLanguage("English");
    await expect(page.root).toContainText("Downstream channel operations");
    await expect(page.panel("accounts")).toContainText("Approved");
    await expect(page.root).toContainText("Channel ID");
    await expect(page.root).not.toContainText("plugin.idc-channel.workspace.");
    await page.selectChannel();
    await page.tab("Product publications");
    await expect(page.panel("publications")).toContainText("Published");
    await expect(page.panel("publications")).toContainText("Product version");
    await page.snapshot("channel-english");
  });
  test("TC-8e: delivered standalone channel assets render real Vben tables", async ({
    adminPage,
  }) => {
    await mockChannelWorkspace(adminPage);
    await openChannelStandalone(adminPage, "admin-mount");
    const page = new ChannelVbenPage(adminPage);
    await expect(page.panel("accounts").locator(".vxe-grid")).toHaveCount(1);
    await expect(page.panel("accounts")).toContainText("主渠道");
    await page.selectStandaloneChannel();
    await page.tab("令牌");
    await expect(page.panel("tokens").locator(".vxe-grid")).toHaveCount(1);
    await expect(page.panel("tokens")).toContainText("catalog.read");
    await page.snapshot("channel-standalone-token-grid");
    await page.tab("额度与结算");
    await expect(page.panel("finance").locator(".vxe-grid")).toHaveCount(3);
    await expect(page.panel("finance")).toContainText("INV-001");
    await page.snapshot("channel-standalone-finance-grid");

    await openChannelStandalone(adminPage, "mount");
    const legacy = adminPage.getByTestId("idc-channel-page");
    await expect(legacy.locator(".vxe-grid")).toHaveCount(1);
    await expect(legacy).toContainText("catalog.read");
    await page.loadStandaloneCatalog();
    await expect(legacy.locator(".vxe-grid")).toHaveCount(2);
    await expect(
      adminPage.getByTestId("channel-product-catalog"),
    ).toContainText("88.00");
    await page.scrollStandaloneTokens();
    await page.snapshot("channel-standalone-basic-grid");
  });
});
