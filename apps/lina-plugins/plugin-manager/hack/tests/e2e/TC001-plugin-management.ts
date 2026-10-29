import { test, expect } from "../support/test";
import { PluginManagerPage } from "../pages/PluginManagerPage";

const categories = [
  ["payment", "支付"],
  ["sms", "短信"],
  ["server", "服务器"],
  ["cloud", "云产品"],
  ["website", "官网"],
  ["customer", "用户中心"],
  ["system", "系统功能"],
].map(([id, name]) => ({ id, name }));
function plugin(id: string, name: string, installed = 1) {
  return {
    id,
    name,
    version: "v1.0.0",
    effectiveVersion: installed ? "v1.0.0" : "",
    discoveredVersion: "v1.1.0",
    upgradeAvailable: installed === 1,
    installed,
    enabled: installed,
    newBusinessState: "accepting",
    runtimeState: installed ? "pending_upgrade" : "normal",
    installedAt: installed ? 1792857600000 : null,
    authorizationRequired: 0,
    authorizationStatus: "not_required",
  };
}
const ok = (data: unknown) => ({ json: { code: 0, message: "OK", data } });

test.describe("TC-1 插件分类管理", () => {
  test("TC-1a: 分类可扩展、两项搜索和精简表格", async ({ adminPage }) => {
    const rows = [
      plugin("idc-zjmfcloud", "魔方云"),
      plugin("idc-baota", "宝塔面板"),
    ];
    const queries: URL[] = [];
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(
        ok({ list: [...categories, { id: "dns", name: "域名解析" }] }),
      ),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) => {
      const url = new URL(route.request().url());
      queries.push(url);
      const items =
        url.searchParams.get("category") === "server"
          ? rows.filter(
              (row) =>
                row.id.includes(url.searchParams.get("id") || "") &&
                row.name.includes(url.searchParams.get("name") || ""),
            )
          : [];
      return route.fulfill(ok({ list: items, total: items.length }));
    });
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    for (const item of [...categories, { id: "dns", name: "域名解析" }])
      await expect(
        manager.root.getByRole("tab", { name: item.name, exact: true }),
      ).toBeVisible();
    const icons = {
      payment: "credit-card",
      sms: "message-square",
      server: "server",
      cloud: "cloud",
      website: "globe",
      customer: "users-round",
      system: "settings",
      dns: "puzzle",
    };
    for (const [id, icon] of Object.entries(icons)) {
      const rendered = manager.root.getByTestId(`plugin-category-icon-${id}`);
      await expect(rendered).toHaveAttribute("data-icon", `lucide:${icon}`);
      await expect(rendered).toBeVisible();
    }
    await manager.tab("服务器");
    await expect(manager.identity("idc-zjmfcloud")).toContainText("魔方云");
    await expect
      .poll(async () => {
        const width = await manager.actionWidths("idc-zjmfcloud");
        return width.column - width.content;
      })
      .toBeLessThanOrEqual(32);
    await expect(manager.identity("idc-zjmfcloud")).toContainText(
      "idc-zjmfcloud",
    );
    for (const text of [
      "名称 / 标识",
      "版本号",
      "状态",
      "业务",
      "运行时状态",
      "安装时间",
      "操作",
    ])
      await expect(
        manager.root
          .locator(".vxe-header--column")
          .filter({ hasText: text })
          .first(),
      ).toBeVisible();
    await expect(
      manager.root.getByRole("button", { name: "详情", exact: true }),
    ).toHaveCount(0);
    await expect(
      manager.root.getByRole("button", { name: "管理", exact: true }),
    ).toHaveCount(0);
    await expect(manager.root.locator("form input")).toHaveCount(2);
    await manager.screenshot("server");
    await manager.search("baota", "宝塔");
    await expect(manager.identity("idc-zjmfcloud")).toHaveCount(0);
    await expect(manager.identity("idc-baota")).toBeVisible();
    expect(queries.at(-1)?.searchParams.get("id")).toBe("baota");
    expect(queries.at(-1)?.searchParams.get("name")).toBe("宝塔");
    await manager.tab("域名解析");
    await expect
      .poll(() => queries.at(-1)?.searchParams.get("category"))
      .toBe("dns");
    await adminPage.setViewportSize({ width: 760, height: 850 });
    await expect(
      manager.root.getByRole("button", { name: /搜\s*索/, exact: true }),
    ).toBeInViewport();
    await expect(manager.root.locator(".vxe-pager")).toBeInViewport();
    await manager.screenshot("narrow");
  });

  test("TC-1b: 安装按钮一次确认、规范失败重试并自动启用", async ({
    adminPage,
  }) => {
    let row = plugin("idc-zjmfcloud", "魔方云", 0);
    row.authorizationRequired = 1;
    let writes = 0;
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(
        ok({
          list:
            new URL(route.request().url()).searchParams.get("category") ===
            "server"
              ? [row]
              : [],
          total:
            new URL(route.request().url()).searchParams.get("category") ===
            "server"
              ? 1
              : 0,
        }),
      ),
    );
    await adminPage.route("**/api/v1/plugins/idc-zjmfcloud", (route) =>
      route.fulfill(
        ok({
          ...row,
          requestedHostServices: [
            {
              owner: "idc-resource",
              service: "resource",
              version: "v1",
              methods: ["configuration.candidates"],
              resources: [{ ref: "resource:catalog" }],
            },
          ],
        }),
      ),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/dependencies",
      (route) => route.fulfill(ok({ blockers: [] })),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/install",
      async (route) => {
        writes++;
        const payload = route.request().postDataJSON();
        expect(payload.authorization.services[0].owner).toBe("idc-resource");
        if (writes === 1)
          return route.fulfill({
            status: 409,
            json: { code: 51, message: "规范校验未通过，请修复插件后重试。" },
          });
        row = { ...row, installed: 1, authorizationRequired: 0 };
        await route.fulfill(ok({ id: row.id, installed: 1, enabled: 0 }));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/enable",
      async (route) => {
        row = { ...row, enabled: 1 };
        await route.fulfill(ok({}));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.tab("服务器");
    await manager.action("install", row.id);
    await expect(manager.modal).toBeHidden();
    await expect(manager.popconfirm).toContainText("确认安装魔方云？");
    await manager.screenshot("install-inline-confirm");
    await manager.submit();
    await expect.poll(() => writes).toBe(1);
    await expect(
      manager.root.getByTestId(`plugin-install-${row.id}`),
    ).toBeEnabled();
    await expect(
      adminPage.getByText("规范校验未通过，请修复插件后重试。", {
        exact: true,
      }),
    ).toBeVisible();
    await manager.screenshot("install-rejected");
    await manager.action("install", row.id);
    await manager.submit();
    await expect.poll(() => writes).toBe(2);
    await expect(
      manager.root.getByTestId(`plugin-status-${row.id}`),
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      manager.root.getByTestId(`plugin-business-${row.id}`),
    ).toContainText("接收中");
  });

  test("TC-1c: 更新采用目标权限，依赖阻断禁止卸载", async ({ adminPage }) => {
    let row = plugin("idc-zjmfcloud", "魔方云");
    let upgrades = 0;
    let removals = 0;
    let stops = 0;
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/new-business/stop",
      (route) => {
        stops++;
        return route.fulfill(ok({}));
      },
    );
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(ok({ list: [row], total: 1 })),
    );
    await adminPage.route(
      /\/api\/v1\/plugins\/idc-zjmfcloud(?:\?.*)?$/,
      async (route) => {
        if (route.request().method() === "DELETE") {
          removals++;
          return route.fulfill({
            status: 409,
            json: { code: 51, message: "请先卸载依赖业务插件。" },
          });
        }
        return route.fulfill(ok({ ...row, requestedHostServices: [] }));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/upgrade/preview",
      (route) =>
        route.fulfill(
          ok({
            effectiveVersion: "v1.0.0",
            discoveredVersion: "v1.1.0",
            dependencyCheck: { blockers: [] },
            hostServicesDiff: { authorizationRequired: true },
            toManifest: {
              requestedHostServices: [
                { service: "storage", methods: ["get"], paths: ["reports/"] },
              ],
            },
          }),
        ),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/upgrade",
      async (route) => {
        const payload = route.request().postDataJSON();
        expect(payload.confirmed).toBe(true);
        expect(payload.authorization.services[0].paths).toEqual(["reports/"]);
        upgrades++;
        row = {
          ...row,
          version: "v1.1.0",
          effectiveVersion: "v1.1.0",
          upgradeAvailable: false,
          runtimeState: "normal",
        };
        await route.fulfill(ok({ pluginId: row.id }));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfcloud/dependencies",
      (route) =>
        route.fulfill(
          ok({
            reverseDependents: [
              { pluginId: "another-plugin", name: "依赖业务插件" },
            ],
          }),
        ),
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.tab("服务器");
    await manager.action("upgrade", row.id);
    await expect(manager.modal).toContainText("v1.0.0 → v1.1.0");
    await manager.review();
    await manager.submit();
    await expect.poll(() => upgrades).toBe(1);
    await expect(
      manager.root.getByTestId(`plugin-upgrade-${row.id}`),
    ).toHaveCount(0);
    await expect
      .poll(async () => {
        const width = await manager.actionWidths(row.id);
        return width.column - width.content;
      })
      .toBeLessThanOrEqual(32);
    await manager.action("uninstall", row.id);
    await expect(manager.modal).toBeHidden();
    await manager.submit();
    await expect.poll(() => removals).toBe(1);
    await expect(
      adminPage.getByText("请先卸载依赖业务插件。", { exact: true }),
    ).toBeVisible();
    expect(stops).toBe(1);
    await manager.screenshot("uninstall-blocked");
  });
  test("TC-1d: 启停失败恢复、停止新业务及保留数据卸载", async ({
    adminPage,
  }) => {
    let row = {
      ...plugin("idc-zjmfmigrate", "迁移检查"),
      upgradeAvailable: false,
    };
    let disables = 0;
    const operations: string[] = [];
    let removed = false;
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(ok({ list: [row], total: 1 })),
    );
    await adminPage.route(
      /\/api\/v1\/plugins\/idc-zjmfmigrate(?:\?.*)?$/,
      async (route) => {
        if (route.request().method() === "DELETE") {
          const params = new URL(route.request().url()).searchParams;
          expect(params.get("purgeStorageData")).toBe("0");
          expect(params.has("force")).toBe(false);
          removed = true;
          operations.push("uninstall");
          row = { ...row, installed: 0 };
        }
        await route.fulfill(ok(row));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfmigrate/dependencies",
      (route) =>
        route.fulfill(ok({ reverseDependents: [], reverseBlockers: [] })),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfmigrate/disable",
      async (route) => {
        disables++;
        operations.push("disable");
        expect(row.newBusinessState).toBe("stopped");
        if (disables === 1)
          return route.fulfill({
            status: 409,
            json: { code: 51, message: "已有任务执行中，请稍后重试。" },
          });
        row = { ...row, enabled: 0 };
        await route.fulfill(ok({}));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfmigrate/enable",
      async (route) => {
        operations.push("enable");
        row = { ...row, enabled: 1 };
        await route.fulfill(ok({}));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-zjmfmigrate/new-business/stop",
      async (route) => {
        operations.push("stop");
        row = { ...row, newBusinessState: "stopped" };
        await route.fulfill(ok({}));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.tab("系统功能");
    await manager.toggleStatus(row.id);
    await expect.poll(() => disables).toBe(1);
    await expect(
      manager.root.getByTestId(`plugin-status-${row.id}`),
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      manager.root.getByTestId(`plugin-business-${row.id}`),
    ).toContainText("已停止");
    expect(operations).toEqual(["stop", "disable"]);
    await manager.screenshot("disable-incomplete-sales-stopped");
    await manager.toggleStatus(row.id);
    await expect(
      manager.root.getByTestId(`plugin-status-${row.id}`),
    ).toHaveAttribute("aria-checked", "false");
    await manager.toggleStatus(row.id);
    await expect(
      manager.root.getByTestId(`plugin-status-${row.id}`),
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      manager.root.getByTestId(`plugin-business-${row.id}`),
    ).toContainText("已停止");
    await manager.action("uninstall", row.id);
    await expect(manager.modal).toBeHidden();
    await manager.submit();
    await expect.poll(() => removed).toBe(true);
    expect(operations).toEqual([
      "stop",
      "disable",
      "disable",
      "enable",
      "uninstall",
    ]);
    await expect(
      manager.root.getByTestId(`plugin-install-${row.id}`),
    ).toBeVisible();
    await manager.screenshot("uninstalled");
  });
  test("TC-1e: 上传校验失败可重试，成功后继续正式安装", async ({
    adminPage,
  }) => {
    let row = plugin("fixture-tools", "测试系统插件", 0);
    let uploads = 0;
    let installs = 0;
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(
        ok({ list: uploads > 1 ? [row] : [], total: uploads > 1 ? 1 : 0 }),
      ),
    );
    await adminPage.route(
      "**/api/v1/plugins/dynamic/package",
      async (route) => {
        uploads++;
        expect(route.request().headers()["content-type"]).toContain(
          "multipart/form-data",
        );
        expect(route.request().postData()).toContain("fixture-tools.wasm");
        if (uploads === 1)
          return route.fulfill({
            status: 400,
            json: { code: 51, message: "插件包不符合规范，请检查后重试。" },
          });
        await route.fulfill(ok(row));
      },
    );
    await adminPage.route("**/api/v1/plugins/fixture-tools", (route) =>
      route.fulfill(ok(row)),
    );
    await adminPage.route(
      "**/api/v1/plugins/fixture-tools/dependencies",
      (route) => route.fulfill(ok({ blockers: [] })),
    );
    await adminPage.route(
      "**/api/v1/plugins/fixture-tools/install",
      async (route) => {
        installs++;
        row = { ...row, installed: 1 };
        await route.fulfill(ok({ id: row.id, installed: 1, enabled: 0 }));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/fixture-tools/enable",
      async (route) => {
        row = { ...row, enabled: 1 };
        await route.fulfill(ok({}));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.tab("系统功能");
    await manager.selectPackage();
    await manager.submit();
    await expect.poll(() => uploads).toBe(1);
    await expect(manager.modal).toContainText("上传动态插件包");
    await expect(manager.confirm).toBeEnabled();
    await manager.submit();
    await expect.poll(() => installs).toBe(1);
    await expect(manager.modal).toBeHidden();
    await expect(manager.identity(row.id)).toContainText("测试系统插件");
    await expect(
      manager.root.getByTestId(`plugin-uninstall-${row.id}`),
    ).toBeVisible();
    await manager.screenshot("package-installed");
  });
  test("TC-1f: 首屏慢请求期间切换多个分类后显示最后选择的权威结果", async ({
    adminPage,
  }) => {
    let releasePayment!: () => void;
    const paymentReady = new Promise<void>((resolve) => {
      releasePayment = resolve;
    });
    let paymentStarted = false;
    const queries: string[] = [];
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route(
      "**/x/plugin-manager/api/v1/plugins?*",
      async (route) => {
        const category =
          new URL(route.request().url()).searchParams.get("category") || "";
        queries.push(category);
        if (category === "payment") {
          paymentStarted = true;
          await paymentReady;
        }
        const rows =
          category === "cloud"
            ? [plugin("yunsu-ssl", "SSL证书插件")]
            : [plugin("payment-fixture", "支付插件")];
        await route.fulfill(ok({ list: rows, total: rows.length }));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    try {
      await manager.goto();
      await expect.poll(() => paymentStarted).toBe(true);
      await manager.tab("服务器");
      await manager.tab("云产品");
    } finally {
      releasePayment();
    }
    await expect(manager.identity("yunsu-ssl")).toContainText("SSL证书插件");
    await expect(manager.identity("payment-fixture")).toHaveCount(0);
    expect(queries).toEqual(["payment", "cloud"]);
    await expect(
      manager.root.getByRole("tab", { name: "云产品", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await manager.screenshot("queued-final-category");
  });
  test("TC-1g: 开发指南完整可阅读并支持窄屏关闭重开", async ({ adminPage }) => {
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(ok({ list: [], total: 0 })),
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.openGuide();
    await expect(manager.guide).toContainText("从零开始");
    await expect(manager.guide.getByRole("button")).toHaveCount(14);
    await expect(manager.guide).toContainText("14. 验证完整生命周期");
    await expect(manager.guide).toContainText("必须遵守的边界");
    await expect(manager.guide).toContainText("未启用时允许单语言");
    await manager.expandGuide("3. 编写 plugin.yaml");
    await expect(manager.guide.locator("pre")).toContainText(
      "business_extensions:",
    );
    await expect(manager.guide.locator("pre")).toContainText("owner: idc-core");
    await expect(manager.guide).not.toContainText("pages.system.plugin.guide.");
    await manager.screenshot("developer-guide");
    await manager.closeGuide();
    await adminPage.setViewportSize({ width: 760, height: 850 });
    await manager.openGuide();
    await expect(
      manager.guide.getByRole("heading", { name: "从零开始", exact: true }),
    ).toBeInViewport();
    await manager.screenshot("developer-guide-narrow");
    await manager.closeGuide();
    await expect(manager.root).toBeVisible();
  });
  test("TC-1h: 停止新销售失败或权限拒绝时不得停用，修复后自动继续", async ({
    adminPage,
  }) => {
    let row = plugin("idc-payment-alipay", "支付宝支付");
    let stops = 0;
    let disables = 0;
    const operations: string[] = [];
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(ok({ list: [row], total: 1 })),
    );
    await adminPage.route("**/api/v1/plugins/idc-payment-alipay", (route) =>
      route.fulfill(ok(row)),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-payment-alipay/new-business/stop",
      async (route) => {
        stops++;
        operations.push("stop");
        if (stops < 3)
          return route.fulfill({
            status: stops === 1 ? 500 : 403,
            json: {
              code: 51,
              message:
                stops === 1
                  ? "停止新销售失败，请稍后重试。"
                  : "没有停止新销售的权限。",
            },
          });
        row = { ...row, newBusinessState: "stopped" };
        await route.fulfill(ok({}));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-payment-alipay/disable",
      async (route) => {
        disables++;
        operations.push("disable");
        expect(row.newBusinessState).toBe("stopped");
        row = { ...row, enabled: 0 };
        await route.fulfill(ok({}));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    for (let attempt = 1; attempt <= 2; attempt++) {
      await manager.toggleStatus(row.id);
      await expect.poll(() => stops).toBe(attempt);
      await expect(
        manager.root.getByTestId(`plugin-status-${row.id}`),
      ).toBeEnabled();
      await expect(
        manager.root.getByTestId(`plugin-status-${row.id}`),
      ).toHaveAttribute("aria-checked", "true");
      await expect(
        manager.root.getByTestId(`plugin-business-${row.id}`),
      ).not.toContainText("已停止");
      expect(disables).toBe(0);
    }
    await expect(
      adminPage.getByText("没有停止新销售的权限。", { exact: true }),
    ).toBeVisible();
    await manager.screenshot("stop-permission-denied");
    await manager.toggleStatus(row.id);
    await expect(
      manager.root.getByTestId(`plugin-status-${row.id}`),
    ).toHaveAttribute("aria-checked", "false");
    await expect(
      manager.root.getByTestId(`plugin-business-${row.id}`),
    ).toContainText("已停止");
    expect(operations).toEqual(["stop", "stop", "stop", "disable"]);
  });
  test("TC-1i: 卸载自动停止新销售，失败保留停止状态并允许重试", async ({
    adminPage,
  }) => {
    let row = plugin("idc-payment-alipay", "支付宝支付");
    let stops = 0;
    let removals = 0;
    const operations: string[] = [];
    await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
      route.fulfill(ok({ list: categories })),
    );
    await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
      route.fulfill(ok({ list: [row], total: 1 })),
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-payment-alipay/dependencies",
      (route) =>
        route.fulfill(ok({ reverseDependents: [], reverseBlockers: [] })),
    );
    await adminPage.route(
      /\/api\/v1\/plugins\/idc-payment-alipay(?:\?.*)?$/,
      async (route) => {
        if (route.request().method() === "DELETE") {
          removals++;
          operations.push("uninstall");
          expect(row.newBusinessState).toBe("stopped");
          const params = new URL(route.request().url()).searchParams;
          expect(params.get("purgeStorageData")).toBe("0");
          expect(params.has("force")).toBe(false);
          if (removals === 1)
            return route.fulfill({
              status: 409,
              json: { code: 51, message: "仍有业务正在处理，请稍后重试。" },
            });
          row = { ...row, installed: 0, enabled: 0 };
        }
        await route.fulfill(ok(row));
      },
    );
    await adminPage.route(
      "**/api/v1/plugins/idc-payment-alipay/new-business/stop",
      async (route) => {
        stops++;
        operations.push("stop");
        if (stops === 1)
          return route.fulfill({
            status: 500,
            json: { code: 51, message: "停止新销售失败，请稍后重试。" },
          });
        row = { ...row, newBusinessState: "stopped" };
        await route.fulfill(ok({}));
      },
    );
    const manager = new PluginManagerPage(adminPage);
    await manager.goto();
    await manager.action("uninstall", row.id);
    await expect(manager.popconfirm).toContainText("确认卸载支付宝支付？");
    await expect(manager.modal).toBeHidden();
    await manager.submit();
    await expect.poll(() => stops).toBe(1);
    expect(removals).toBe(0);
    await expect(
      manager.root.getByTestId(`plugin-uninstall-${row.id}`),
    ).toBeEnabled();
    await manager.screenshot("uninstall-stop-failed");
    await manager.action("uninstall", row.id);
    await manager.submit();
    await expect.poll(() => removals).toBe(1);
    await expect(
      manager.root.getByTestId(`plugin-business-${row.id}`),
    ).toContainText("已停止");
    await expect(
      manager.root.getByTestId(`plugin-uninstall-${row.id}`),
    ).toBeEnabled();
    await manager.screenshot("uninstall-incomplete-sales-stopped");
    await manager.action("uninstall", row.id);
    await manager.submit();
    await expect(manager.modal).toBeHidden();
    await expect(
      manager.root.getByTestId(`plugin-install-${row.id}`),
    ).toBeVisible();
    expect(operations).toEqual(["stop", "stop", "uninstall", "uninstall"]);
    await manager.screenshot("uninstall-auto-stop-completed");
  });
});
