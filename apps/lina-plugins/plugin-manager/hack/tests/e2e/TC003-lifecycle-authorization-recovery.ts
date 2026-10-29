import { test, expect } from "../support/test";
import { PluginManagerPage } from "../pages/PluginManagerPage";

const extension = {
  point: "payment-method",
  version: "v1",
  role: "provider",
  owner: "idc-payment",
  methods: ["capabilities.describe"],
};
const ok = (data: unknown) => ({ json: { code: 0, message: "OK", data } });
function payment(installed = 1) {
  return {
    id: "idc-payment-alipay",
    name: "支付宝支付",
    type: "dynamic",
    distribution: "managed",
    version: "v0.1.0",
    effectiveVersion: installed ? "v0.1.0" : "",
    discoveredVersion: "v0.1.0",
    installed,
    enabled: installed,
    newBusinessState: "stopped",
    runtimeState: "normal",
    installedAt: installed ? 1792857600000 : null,
    upgradeAvailable: false,
    authorizationRequired: 0,
    authorizationStatus: "not_required",
    businessExtensions: [extension],
    authorizedBusinessExtensions: [] as (typeof extension)[],
    requestedHostServices: [],
  };
}

test.describe("TC-3 一次确认自动完成插件生命周期", () => {
  for (const operation of ["uninstall", "disable"] as const) {
    test(`TC-3${operation === "uninstall" ? "a" : "b"}: ${operation}自动补齐授权，权限拒绝及校验失败可重试`, async ({
      adminPage,
    }) => {
      let row = { ...payment(), newBusinessState: "accepting" };
      const operations: string[] = [];
      let attempts = 0;
      await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
        route.fulfill(ok({ list: [{ id: "payment", name: "支付" }] })),
      );
      await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
        route.fulfill(ok({ list: [row], total: 1 })),
      );
      await adminPage.route(
        /\/api\/v1\/plugins\/idc-payment-alipay(?:\?.*)?$/,
        async (route) => {
          if (route.request().method() === "DELETE") {
            operations.push("uninstall");
            expect(row.newBusinessState).toBe("stopped");
            expect(row.authorizedBusinessExtensions).toEqual([extension]);
            expect(
              new URL(route.request().url()).searchParams.get(
                "purgeStorageData",
              ),
            ).toBe("0");
            expect(
              new URL(route.request().url()).searchParams.has("force"),
            ).toBe(false);
            row = { ...row, installed: 0, enabled: 0 };
          }
          await route.fulfill(ok(row));
        },
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/new-business/stop",
        async (route) => {
          operations.push("stop");
          row = { ...row, newBusinessState: "stopped" };
          await route.fulfill(ok({}));
        },
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/enable",
        async (route) => {
          operations.push("authorize");
          attempts++;
          if (attempts < 3)
            return route.fulfill({
              status: attempts === 1 ? 403 : 500,
              json: {
                code: 51,
                message:
                  attempts === 1
                    ? "没有启用插件的权限。"
                    : "业务规范校验失败。",
              },
            });
          row = { ...row, authorizedBusinessExtensions: [extension] };
          await route.fulfill(ok({}));
        },
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/disable",
        async (route) => {
          operations.push("disable");
          expect(row.newBusinessState).toBe("stopped");
          expect(row.authorizedBusinessExtensions).toEqual([extension]);
          row = { ...row, enabled: 0 };
          await route.fulfill(ok({}));
        },
      );
      const manager = new PluginManagerPage(adminPage);
      await manager.goto();
      for (let attempt = 1; attempt <= 3; attempt++) {
        if (operation === "uninstall") {
          await manager.action("uninstall", row.id);
          await expect(manager.popconfirm).toContainText(
            "确认卸载支付宝支付？",
          );
          await manager.submit();
        } else await manager.toggleStatus(row.id);
        await expect.poll(() => attempts).toBe(attempt);
        await expect(manager.modal).toBeHidden();
        await expect(
          adminPage.getByTestId("plugin-authorization-confirm"),
        ).toHaveCount(0);
        if (attempt < 3) {
          await expect(
            manager.root.getByTestId(
              `plugin-${operation === "uninstall" ? "uninstall" : "status"}-${row.id}`,
            ),
          ).toBeEnabled();
          expect(operations).toEqual([
            "stop",
            ...Array(attempt).fill("authorize"),
          ]);
          await expect(
            manager.root.getByTestId(`plugin-business-${row.id}`),
          ).toContainText("已停止");
          await manager.screenshot(
            `${operation}-automatic-recheck-rejected-${attempt}`,
          );
        }
      }
      if (operation === "uninstall")
        await expect(
          manager.root.getByTestId(`plugin-install-${row.id}`),
        ).toBeVisible();
      else
        await expect(
          manager.root.getByTestId(`plugin-status-${row.id}`),
        ).toHaveAttribute("aria-checked", "false");
      expect(operations).toEqual([
        "stop",
        "authorize",
        "authorize",
        "authorize",
        operation,
      ]);
      await manager.screenshot(`${operation}-automatic-recheck-completed`);
    });
  }
  for (const resumeFails of [false, true]) {
    test(`TC-3${resumeFails ? "d" : "c"}: 重装自动启用并开放业务${resumeFails ? "，开放失败显示真实结果可重试" : ""}`, async ({
      adminPage,
    }) => {
      let row = payment(0);
      const operations: string[] = [];
      let resumes = 0;
      await adminPage.route("**/x/plugin-manager/api/v1/categories", (route) =>
        route.fulfill(ok({ list: [{ id: "payment", name: "支付" }] })),
      );
      await adminPage.route("**/x/plugin-manager/api/v1/plugins?*", (route) =>
        route.fulfill(ok({ list: [row], total: 1 })),
      );
      await adminPage.route("**/api/v1/plugins/idc-payment-alipay", (route) =>
        route.fulfill(ok(row)),
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/install",
        async (route) => {
          operations.push("install");
          row = { ...row, installed: 1, installedAt: 1792857600000 };
          await route.fulfill(ok({}));
        },
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/enable",
        async (route) => {
          operations.push("enable");
          row = {
            ...row,
            enabled: 1,
            authorizedBusinessExtensions: [extension],
          };
          await route.fulfill(ok({}));
        },
      );
      await adminPage.route(
        "**/api/v1/plugins/idc-payment-alipay/new-business/resume",
        async (route) => {
          operations.push("resume");
          resumes++;
          if (resumeFails && resumes === 1)
            return route.fulfill({
              status: 403,
              json: { code: 51, message: "没有恢复新业务的权限。" },
            });
          row = { ...row, newBusinessState: "accepting" };
          await route.fulfill(ok({}));
        },
      );
      const manager = new PluginManagerPage(adminPage);
      await manager.goto();
      await manager.action("install", row.id);
      await expect(manager.modal).toBeHidden();
      await expect(manager.popconfirm).toContainText("确认安装支付宝支付？");
      await manager.submit();
      await expect(
        manager.root.getByTestId(`plugin-status-${row.id}`),
      ).toHaveAttribute("aria-checked", "true");
      expect(operations).toEqual(["install", "enable", "resume"]);
      if (resumeFails) {
        await expect(
          manager.root.getByTestId(`plugin-business-${row.id}`),
        ).toContainText("已停止");
        await expect(
          adminPage.getByText("没有恢复新业务的权限。", { exact: true }),
        ).toBeVisible();
        await expect(
          adminPage.getByText("插件已安装并启用", { exact: true }),
        ).toHaveCount(0);
        await manager.screenshot("install-resume-denied");
        await manager.stopBusiness(row.id); // Same business toggle now resumes.
      }
      await expect(
        manager.root.getByTestId(`plugin-business-${row.id}`),
      ).toContainText("接收中");
      await expect(manager.modal).toBeHidden();
      await manager.screenshot("reinstalled-automatically-accepting");
    });
  }
});
