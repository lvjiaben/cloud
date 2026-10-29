import { test, expect, menus } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-11 服务管理", () => {
  test("TC-11f 已购服务页签只包含产品菜单，支持两级、空分类和无效深链接", async ({ consolePage: page }) => {
    await page.route("https://api.iconify.design/**/*.svg?*", route => route.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path d="M2 2h12v12H2z"/></svg>' }));
    const items = [
      ...menus.filter(item => item.navigationType !== "product"),
      { directoryNo: "products", nameZhCn: "产品与服务", navigationType: "system", targetUrl: "/account/servers", sortOrder: 10 },
      { directoryNo: "cloud", parentDirectoryNo: "products", nameZhCn: "云服务器", navigationType: "product", icon: "lucide:cloud", sortOrder: 1 },
      { directoryNo: "physical", nameZhCn: "物理机", navigationType: "product", icon: "lucide:server", sortOrder: 20 },
    ];
    await page.route("**/account/product-directories?*", route => route.fulfill({ json: { code: 0, data: { items, total: items.length } } }));
    const categories: string[] = [];
    await page.route("**/product-directories/*/services?*", route => {
      const category = new URL(route.request().url()).pathname.split("/").at(-2)!;
      categories.push(category);
      return route.fulfill({ json: { code: 0, data: { items: category === "cloud" ? [{ entitlementNo: "cloud-service", productNameZhCn: "陕西云服务器", statusCode: "active", orderNo: "ORD-test" }] : [], total: category === "cloud" ? 1 : 0, providers: [{pluginId:"idc-resource",total:category === "cloud" ? 1 : 0}] } } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/product-directories");
    await expect(view.serviceTabs.getByRole("tab")).toHaveText(["云服务器", "物理机"]);
    await expect(view.serviceTabs.getByRole("tab", { name: "云服务器", exact: true }).locator("img")).toHaveAttribute("src", /\/lucide\/cloud\.svg/);
    await expect(view.serviceTabs.getByRole("tab", { name: "物理机", exact: true }).locator("img")).toHaveAttribute("src", /\/lucide\/server\.svg/);
    await expect(view.serviceBody).toContainText("陕西云服务器");
    await expect(page).toHaveURL(/\/product-directories\/cloud$/);
    await view.screenshot("product-only-service-tabs");
    await view.selectServiceCategory("物理机");
    await expect(page).toHaveURL(/\/product-directories\/physical$/);
    await expect(view.serviceBody).toContainText("暂无数据");
    await expect(view.serviceBody.getByRole("textbox", { name: "搜索服务", exact: true })).toBeVisible();
    await expect(view.serviceBody.getByRole("columnheader",{name:"系统",exact:true})).toBeVisible();
    await expect(view.main).not.toContainText("陕西云服务器");
    await view.reload();
    await expect(view.serviceTabs.getByRole("tab", { name: "物理机", exact: true })).toHaveAttribute("aria-selected", "true");
    await view.open("/account/product-directories/menu-2");
    await expect(page).toHaveURL(/\/product-directories\/cloud$/);
    await expect(view.serviceBody).toContainText("陕西云服务器");
    expect(categories).not.toContain("menu-2");
    await view.screenshot("product-category-invalid-link-recovery");
    await page.route("**/account/product-directories?*", route => route.fulfill({ json: { code: 0, data: { items: menus.filter(item => item.navigationType !== "product"), total: menus.length - 1 } } }));
    categories.length = 0;
    await view.open("/account/product-directories/all");
    await expect(view.main).toContainText("暂无已开放的产品服务菜单");
    await expect(view.serviceTabs).toHaveCount(0);
    expect(categories).toEqual([]);
  });
  test("TC-11a 翻页后切换侧栏分类从第一页读取，保持真实总数", async ({
    consolePage: page,
  }) => {
    const dirs = ["cat-a", "cat-b"].map((directoryNo, i) => ({
      directoryNo,
      nameZhCn: i ? "数据库" : "云服务器",
      navigationType: "product",
      sortOrder: 20 + i,
    }));
    await page.route("**/account/product-directories?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: { items: [...menus, ...dirs], total: menus.length + 2 },
        },
      }),
    );
    const calls: string[] = [];
    await page.route("**/product-directories/*/services?*", (r) => {
      const url = new URL(r.request().url()),
        n = Number(url.searchParams.get("page"));
      const kind = url.pathname.includes("cat-b") ? "B" : "A";
      calls.push(kind + n);
      return r.fulfill({
        json: {
          code: 0,
          data: {
            items: Array.from({ length: kind === "A" ? 20 : 1 }, (_, i) => ({
              entitlementNo: `${kind}-${n}-${i}`,
              productNameZhCn: `${kind} 分类第 ${n} 页服务 ${i + 1}`,
              statusCode: "active",
              orderNo: "ORD-test",
              expiresAt: null,
            })),
            total: kind === "A" ? 40 : 1,
            providers:[{pluginId:"idc-resource",total:kind === "A" ? 40 : 1}],
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cat-a");
    await expect(view.serviceBody).toContainText(/共\s*40\s*条/);
    await view.screenshot("native-pagination-before-next");
    await view.nextServicesPage();
    await expect(view.serviceBody).toContainText("A 分类第 2 页服务 1");
    await page
      .getByRole("navigation")
      .getByRole("link", { name: "数据库", exact: true })
      .click();
    await expect(view.serviceBody).toContainText("B 分类第 1 页服务 1");
    expect(calls).not.toContain("B2");
    await expect(view.serviceBody).toContainText(/共\s*1\s*条/);
    await view.screenshot("service-category-switch");
  });
  test("TC-11b 服务操作必须确认，失败保留且重试只生成一项操作", async ({
    consolePage: page,
  }) => {
    let failed = true,
      operations: any[] = [],
      calls = 0,
      preparationCalls = 0;
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SRV-test",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              serverNo: "SRV-test",
              productNameZhCn: "应用服务器",
              runtimeState: "running",
              entitlementStatus: "active",
              addressesConfirmed: true,
              addresses: ["192.0.2.10"],
              orderNo: "ORD-test",
              availableActions: ["reboot"],
              operations,
              credentialAvailable: false,
            },
          },
        }),
    );
    await page.route(
      "**/native-servers/SRV-test/operation-confirmations",
      (r) => {
        preparationCalls++;
        return preparationCalls > 1
          ? r.fulfill({
              status: 409,
              json: { code: 409, message: "已有操作正在处理" },
            })
          : r.fulfill({
              json: { code: 0, data: { required: false, proofToken: "" } },
            });
      },
    );
    await page.route("**/native-servers/SRV-test/operations?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: { items: operations, total: operations.length },
        },
      }),
    );
    await page.route("**/native-servers/SRV-test/operations", (r) => {
      calls++;
      expect(r.request().postDataJSON()).toMatchObject({
        action: "reboot",
        confirmed: true,
      });
      if (failed)
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "暂时无法提交操作，请重试" },
        });
      operations = [
        {
          operationNo: "OP-test",
          action: "reboot",
          status: "pending",
          createdAt: Date.now(),
        },
      ];
      return r.fulfill({
        json: { code: 0, data: { operation: operations[0] } },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SRV-test");
    await view.button("重启").click();
    expect(calls).toBe(0);
    const dialog = page.getByRole("dialog", { name: "重启", exact: true });
    await dialog.getByRole("button", { name: "确认重启", exact: true }).click();
    await expect(dialog).toContainText("暂时无法提交操作，请重试");
    failed = false;
    await dialog
      .getByRole("button", { name: "重试本次提交", exact: true })
      .dblclick();
    await expect(dialog).toHaveCount(0);
    expect(calls).toBe(2);
    expect(preparationCalls).toBe(1);
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await expect(
      view.main.getByRole("row").filter({ hasText: "重启" }),
    ).toHaveCount(1);
    await expect(view.main).toContainText("192.0.2.10");
    await page.setViewportSize({ width: 390, height: 844 });
    await view.screenshot("service-operation-mobile");
  });
  test("TC-11c 缺省管理准备字段的待开通服务进入订单，正常历史服务仍可管理", async ({
    consolePage: page,
  }) => {
    const items = [
      {
        entitlementNo: "pending-manual",
        statusCode: "pending_activation",
        requiresManualProcessing: true,
      },
      {
        entitlementNo: "pending-auto",
        statusCode: "pending_activation",
        requiresManualProcessing: false,
      },
      {
        entitlementNo: "active-legacy",
        statusCode: "active",
        requiresManualProcessing: false,
      },
      {
        entitlementNo: "active-ready",
        statusCode: "active",
        managementPreparation: "ready",
      },
      {
        entitlementNo: "active-preparing",
        statusCode: "active",
        managementPreparation: "preparing",
      },
      {
        entitlementNo: "active-attention",
        statusCode: "active",
        managementPreparation: "needs_attention",
      },
      { entitlementNo: "suspended-legacy", statusCode: "suspended" },
    ].map((item) => ({
      ...item,
      entitlementOwnerId: "idc-resource",
      orderNo: "ORD-TEST-001",
      productNameZhCn: item.entitlementNo,
      expiresAt: null,
    }));
    let resolves = 0;
    await page.route("**/product-directories/cloud/services?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items, total: items.length, providers:[{pluginId:"idc-resource",total:items.length}] } } }),
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/resolve/*",
      (r) => {
        resolves++;
        return r.fulfill({
          json: { code: 0, data: { serverNo: "legacy-server" } },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/legacy-server",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              serverNo: "legacy-server",
              productNameZhCn: "历史服务器",
              runtimeState: "running",
              entitlementStatus: "active",
              addressesConfirmed: true,
              addresses: ["192.0.2.42"],
              availableActions: [],
              operations: [],
            },
          },
        }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories");
    for (const id of ["pending-manual", "pending-auto"]) {
      const row = view.serviceRows.filter({ hasText: id });
      await expect(row.getByText("待开通", { exact: true })).toBeVisible();
      await expect(
        view.serviceActionRows(id).getByRole("link", { name: "管理", exact: true }),
      ).toHaveCount(0);
      await expect(
        view.serviceActionRows(id).getByRole("link", { name: "查看开通进度" }),
      ).toHaveAttribute("href", "/account/orders/ORD-TEST-001");
    }
    await expect(
      view.serviceRows.filter({ hasText: "pending-manual" }),
    ).toContainText("待人工处理");
    await expect(
      view.serviceRows.filter({ hasText: "pending-auto" }),
    ).not.toContainText("待人工处理");
    for (const id of ["active-legacy", "active-ready", "suspended-legacy"])
      await expect(
        view.serviceActionRows(id)
          .getByRole("link", { name: "管理", exact: true }),
      ).toBeVisible();
    for (const id of ["active-preparing", "active-attention"])
      await expect(
        view.serviceActionRows(id)
          .getByRole("link", { name: "管理", exact: true }),
      ).toHaveCount(0);
    await expect(view.main).not.toContainText("状态待确认");
    await view.screenshot("services-pending-activation");
    await view.serviceActionRows("pending-manual")
      .getByRole("link", { name: "查看开通进度" })
      .click();
    await expect(
      page.getByRole("heading", { name: "订单详情", exact: true }),
    ).toBeVisible();
    expect(resolves).toBe(0);
    await view.open("/account/product-directories");
    await view.serviceActionRows("active-legacy")
      .getByRole("link", { name: "管理", exact: true })
      .click();
    await expect(view.main).toContainText("192.0.2.42");
    expect(resolves).toBe(1);
  });
  test("TC-11d 人工待开通完成后刷新才展示服务器管理", async ({
    consolePage: page,
  }) => {
    let ready = false;
    await page.route("**/product-directories/cloud/services?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                entitlementNo: "service-transition",
                productNameZhCn: "新购服务器",
                entitlementOwnerId: "idc-resource",
                orderNo: "ORD-TEST-001",
                statusCode: ready ? "active" : "pending_activation",
                requiresManualProcessing: !ready,
                ...(ready ? { managementPreparation: "ready" } : {}),
                expiresAt: null,
              },
            ],
            total: 1,
            providers:[{pluginId:"idc-resource",total:1}],
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories");
    await expect(view.serviceBody).toContainText("待人工处理");
    await expect(
      view.serviceBody.getByRole("link", { name: "管理", exact: true }),
    ).toHaveCount(0);
    ready = true;
    await view.reloadServices();
    await expect(
      view.serviceBody.getByRole("link", { name: "管理", exact: true }),
    ).toHaveAttribute(
      "href",
      "/account/native-servers/by-entitlement/service-transition",
    );
    await expect(view.serviceBody).toContainText("已激活");
    await expect(view.serviceBody).not.toContainText("待人工处理");
    await expect(view.serviceBody).not.toContainText("待开通");
    await page.setViewportSize({ width: 390, height: 844 });
    await view.screenshot("services-activation-complete");
  });
  test("TC-11e 历史服务器读取恢复后显示真实状态与地址并恢复电源资格", async ({
    consolePage: page,
  }) => {
    let confirmed = false;
    const reads: string[] = [];
    const writes: string[] = [];
    const serverNo = "legacy-observation-server";
    await page.route(
      "**/account/native-servers/resolve/ent-legacy-observation",
      (route) => route.fulfill({ json: { code: 0, data: { serverNo } } }),
    );
    await page.route(`**/account/native-servers/${serverNo}`, (route) => {
      reads.push(route.request().method());
      return route.fulfill({
        json: {
          code: 0,
          data: {
            serverNo,
            entitlementNo: "ent-legacy-observation",
            productNameZhCn: "陕西历史服务器",
            entitlementStatus: "active",
            orderNo: "ORD-LEGACY-OBSERVATION",
            runtimeState: confirmed ? "running" : "unknown",
            addressesConfirmed: confirmed,
            addresses: confirmed ? ["192.0.2.67"] : [],
            observedAt: confirmed ? 1792896000000 : null,
            information: confirmed
              ? {
                  hostname: "legacy-host",
                  osName: "Debian 13",
                  loginUsername: "root",
                  port: 22,
                  cpuCores: 2,
                  memoryMiB: 2048,
                }
              : {},
            availableActions: confirmed ? ["stop", "reboot"] : [],
            capabilities: ["start", "stop", "reboot"].map((action) => ({
              action,
              supported: true,
              available: confirmed && action !== "start",
              reasonCode: confirmed
                ? action === "start"
                  ? "runtime_state_conflict"
                  : ""
                : "runtime_unconfirmed",
            })),
            operations: [],
            credentialAvailable: false,
          },
        },
      });
    });
    await page.route(
      `**/account/native-servers/${serverNo}/management-options`,
      (route) =>
        route.fulfill({
          json: { code: 0, data: { images: [], accessories: [] } },
        }),
    );
    await page.route(
      `**/account/native-servers/${serverNo}/operations`,
      (route) => {
        if (route.request().method() !== "GET")
          writes.push(route.request().method());
        return route.fulfill({
          json: { code: 0, data: { items: [], total: 0 } },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open(
      "/account/native-servers/by-entitlement/ent-legacy-observation",
    );
    await expect(
      view.main.getByRole("heading", { name: "陕西历史服务器", exact: true }),
    ).toBeVisible();
    await expect(
      view.main.getByText("状态待确认", { exact: true }),
    ).toBeVisible();
    for (const name of ["开机", "关机", "重启"])
      await expect(view.button(name)).toBeDisabled();
    await expect(view.main).not.toContainText("192.0.2.67");
    await view.screenshot("legacy-server-observation-unconfirmed", "20261025");
    confirmed = true;
    await view.button("刷新状态").click();
    await expect(view.main.getByText("运行中", { exact: true })).toBeVisible();
    await expect(view.main).not.toContainText("状态待确认");
    await expect(view.main).toContainText("192.0.2.67");
    await expect(view.main).toContainText("legacy-host");
    await expect(view.main).toContainText("Debian 13");
    await expect(
      view.main.locator("p").filter({ hasText: "信息更新时间" }),
    ).not.toContainText("—");
    await expect(view.button("开机")).toBeDisabled();
    await expect(view.button("关机")).toBeEnabled();
    await expect(view.button("重启")).toBeEnabled();
    expect(reads.length).toBeGreaterThanOrEqual(2);
    expect(reads.every((method) => method === "GET")).toBe(true);
    expect(writes).toEqual([]);
    await view.screenshot("legacy-server-observation-restored", "20261025");
  });
});
