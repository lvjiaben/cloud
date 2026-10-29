import { test, expect, now, type Page } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const server = {
  serverNo: "SVR-managed",
  entitlementNo: "ent_owned",
  productNameZhCn: "应用云服务器",
  productNameEnUs: "Cloud",
  runtimeState: "running",
  entitlementStatus: "active",
  addressesConfirmed: true,
  addresses: ["192.0.2.51"],
  orderNo: "ORD-TEST-001",
  startedAt: now,
  expiresAt: now + 86400000,
  observedAt: now,
  information: {
    hostname: "app-01",
    osName: "Ubuntu 24.04",
    loginUsername: "root",
    port: 22,
    cpuCores: 4,
    memoryMiB: 8192,
    networkType: "独立网络",
    bootOrder: "系统盘",
  },
  availableActions: [
    "stop",
    "reboot",
    "reset_password",
    "console",
    "reinstall",
    "nat_create",
    "security_group_attach",
  ],
  operations: [],
  credentialAvailable: false,
};
const choices = {
  images: [
    {
      reference: "101",
      name: "Debian 12",
      group: "Debian",
      osType: "linux",
      arch: "x86_64",
      bootMode: "uefi",
      selectable: true,
    },
    {
      reference: "102",
      name: "停用系统",
      group: "其他",
      osType: "linux",
      arch: "x86_64",
      bootMode: "uefi",
      selectable: false,
    },
  ],
  imagesObservedAt: now,
  accessories: [
    {
      objectNo: "group_own",
      kind: "security_group",
      name: "网站安全组",
      state: "available",
      projection: {},
    },
  ],
  accessoriesObservedAt: now,
  accessoriesExpiresAt: Date.now() + 600000,
};
async function mockNativeServer(
  page: Page,
  data: typeof server & { operations: any[] },
) {
  await page.route(
    "**/x/idc-resource/api/v1/account/native-servers/SVR-managed",
    (r) => r.fulfill({ json: { code: 0, data } }),
  );
  await page.route("**/native-servers/SVR-managed/operations?*", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          items: data.operations.map(({ availableOutputs, ...op }) => ({
            ...op,
            availableOutputs: [],
          })),
          total: data.operations.length,
        },
      },
    }),
  );
  for (const operation of data.operations)
    await page.route(
      `**/native-servers/SVR-managed/operations/${operation.operationNo}`,
      (r) => r.fulfill({ json: { code: 0, data: { operation } } }),
    );
}
async function setup(page: Page) {
  await mockNativeServer(page, server);
  await page.route(
    "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/management-options",
    (r) => r.fulfill({ json: { code: 0, data: choices } }),
  );
  await page.route("**/service-entitlements/ent_owned/customer-note", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          note: { entitlementNo: "ent_owned", remark: "线上应用", revision: 1 },
          supportAssociationReference: "sar_owned",
        },
      },
    }),
  );
}
test.describe("TC-19 服务器管理", () => {
  test("TC-19a 完整实例信息、系统候选及确认绑定同一请求，失败后重试不新建操作", async ({
    consolePage: page,
  }) => {
    await setup(page);
    let fail = true;
    const requests: any[] = [];
    let prepared: any;
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operation-confirmations",
      (r) => {
        prepared = r.request().postDataJSON();
        expect(prepared).toMatchObject({
          action: "reinstall",
          parameters: { os: "101" },
        });
        expect(prepared).not.toHaveProperty("password");
        return r.fulfill({
          json: {
            code: 0,
            data: {
              required: true,
              proofToken: "proof-owned-reinstall",
              expiresAt: Date.now() + 60000,
            },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operations",
      (r) => {
        const body = r.request().postDataJSON();
        requests.push(body);
        expect(body).toEqual({
          ...prepared,
          proofToken: "proof-owned-reinstall",
          confirmed: true,
        });
        if (fail)
          return r.fulfill({
            status: 503,
            json: { code: 503, message: "提交暂时失败，请重试" },
          });
        return r.fulfill({
          json: {
            code: 0,
            data: {
              operation: {
                operationNo: "nso_test",
                action: "reinstall",
                status: "pending",
                createdAt: now,
                updatedAt: now,
              },
            },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operations/nso_test",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              operation: {
                operationNo: "nso_test",
                action: "reinstall",
                status: "succeeded",
                createdAt: now,
                updatedAt: now + 1000,
              },
            },
          },
        }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await expect(view.main).toContainText("app-01");
    await expect(view.main).toContainText("192.0.2.51");
    await expect(view.main).toContainText("4 核");
    await expect(view.main).toContainText("8192 MiB");
    await expect(view.main).toContainText("线上应用");
    await view.screenshot("server-management-overview");
    await page.getByRole("tab", { name: "系统与连接", exact: true }).click();
    await view.button("重装系统").click();
    const dialog = page.getByRole("dialog", { name: "重装系统", exact: true });
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText("请选择需要安装的系统");
    await dialog.getByPlaceholder("选择需要安装的系统").click();
    await page
      .getByText("Debian · Debian 12 · linux · x86_64 · uefi", { exact: true })
      .click();
    await view.screenshot("server-reinstall-form");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    const confirm = page.getByRole("dialog", {
      name: "确认重装系统",
      exact: true,
    });
    await expect(confirm).toContainText("Debian 12");
    expect(requests).toHaveLength(0);
    await confirm
      .getByRole("button", { name: "确认操作", exact: true })
      .click();
    await expect(confirm).toContainText("提交暂时失败，请重试");
    await view.screenshot("server-reinstall-retry");
    fail = false;
    await confirm
      .getByRole("button", { name: "重试本次提交", exact: true })
      .dblclick();
    await expect(confirm).toHaveCount(0);
    expect(requests).toHaveLength(2);
    expect(requests[1].idempotencyKey).toBe(requests[0].idempotencyKey);
    await expect(view.main).toContainText("操作已完成，资源信息正在更新。");
    await page.setViewportSize({ width: 390, height: 844 });
    await view.screenshot("server-management-mobile");
  });
  test("TC-19b 密码只在执行请求中发送，确认过期不执行", async ({
    consolePage: page,
  }) => {
    await setup(page);
    let prepares = 0,
      executes = 0;
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operation-confirmations",
      (r) => {
        prepares++;
        expect(r.request().postDataJSON()).toMatchObject({
          action: "reset_password",
          parameters: {},
        });
        expect(r.request().postDataJSON()).not.toHaveProperty("password");
        return r.fulfill({
          json: {
            code: 0,
            data: { required: true, proofToken: "expired-proof", expiresAt: 1 },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operations",
      (r) => {
        executes++;
        return r.fulfill({ status: 500 });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "系统与连接", exact: true }).click();
    await view.button("重置密码").click();
    const dialog = page.getByRole("dialog", { name: "重置密码", exact: true });
    await dialog
      .locator("input[type=password]")
      .nth(0)
      .fill("Synthetic-only-2026");
    await dialog
      .locator("input[type=password]")
      .nth(1)
      .fill("Synthetic-only-2026");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    const confirm = page.getByRole("dialog", {
      name: "确认重置密码",
      exact: true,
    });
    await expect(confirm).not.toContainText("Synthetic-only-2026");
    await confirm
      .getByRole("button", { name: "确认操作", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText(
      "确认已过期，请重新确认操作",
    );
    expect(prepares).toBe(1);
    expect(executes).toBe(0);
    await view.screenshot("server-confirmation-expired");
  });
  test("TC-19c 未知操作结果不自动重发，管理目录失败不显示伪造候选", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/management-options",
      (r) =>
        r.fulfill({
          status: 503,
          json: { code: 503, message: "暂时无法读取管理选项" },
        }),
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operation-confirmations",
      (r) =>
        r.fulfill({
          json: { code: 0, data: { required: false, proofToken: "" } },
        }),
    );
    let calls = 0,
      polls = 0;
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operations",
      (r) => {
        calls++;
        return r.fulfill({
          json: {
            code: 0,
            data: {
              operation: {
                operationNo: "nso_unknown",
                action: "reboot",
                status: "result_unknown",
                createdAt: now,
                updatedAt: now,
              },
            },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed/operations/nso_unknown",
      (r) => {
        polls++;
        return r.fulfill({ status: 500 });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await view.button("重启").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认重启", exact: true })
      .click();
    await expect(view.main).toContainText(
      "结果正在确认，请等待处理或通过工单联系我们。",
    );
    await page.waitForTimeout(2800);
    expect(calls).toBe(1);
    expect(polls).toBe(0);
    await page.getByRole("tab", { name: "快照与备份", exact: true }).click();
    await expect(view.main).toContainText("暂时无法读取管理选项");
    await view.screenshot("server-options-failed");
  });
  test("TC-19d 一次性新密码仅显式获取，关闭清除且失败不自动重放", async ({
    consolePage: page,
  }) => {
    await setup(page);
    const operation = {
      operationNo: "nso_secret",
      action: "reinstall",
      status: "succeeded",
      createdAt: now,
      updatedAt: now,
      availableOutputs: [
        {
          name: "generatedCredential",
          policy: "once",
          expiresAt: Date.now() + 60000,
        },
      ],
    };
    await mockNativeServer(page, { ...server, operations: [operation] });
    let calls = 0;
    await page.route(
      "**/operations/nso_secret/outputs/generatedCredential/reveal",
      (r) => {
        calls++;
        expect(r.request().postDataJSON()).toEqual({ confirmed: true });
        return r.fulfill({
          json: {
            code: 0,
            data: {
              username: "root",
              password: "Synthetic-ONE-TIME",
              port: 22,
            },
          },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    expect(calls).toBe(0);
    await view.button("查看结果").click();
    await view.button("查看新密码").click();
    expect(calls).toBe(0);
    const dialog = page.getByRole("dialog", {
      name: "新系统登录信息",
      exact: true,
    });
    await dialog
      .getByRole("button", { name: "确认获取", exact: true })
      .dblclick();
    await expect(dialog.getByTestId("generated-password")).toHaveText(
      "Synthetic-ONE-TIME",
    );
    expect(calls).toBe(1);
    expect(
      await page.evaluate(() =>
        JSON.stringify({ ...sessionStorage, ...localStorage }),
      ),
    ).not.toContain("Synthetic-ONE-TIME");
    await dialog
      .getByRole("button", { name: "关闭并隐藏", exact: true })
      .click();
    await expect(
      page.getByText("Synthetic-ONE-TIME", { exact: true }),
    ).toHaveCount(0);
    await expect(view.button("查看新密码")).toHaveCount(0);
    await view.screenshot("server-output-consumed");
    // A failed delivery may already be consumed. Never replay it automatically.
    await page.route(
      "**/operations/nso_secret/outputs/generatedCredential/reveal",
      (r) => {
        calls++;
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "信息领取未完成" },
        });
      },
    );
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await view.button("查看结果").click();
    await view.button("查看新密码").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认获取", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText(
      "当前窗口不会再次获取",
    );
    expect(calls).toBe(2);
    await expect(
      page
        .getByRole("dialog")
        .getByRole("button", { name: "确认获取", exact: true }),
    ).toHaveCount(0);
    await view.screenshot("server-output-failed");
  });
  test("TC-19e 远程连接只显示后端返回的安全HTTPS地址", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      operations: [
        {
          operationNo: "nso_console",
          action: "console",
          status: "succeeded",
          createdAt: now,
          updatedAt: now,
          availableOutputs: [{ name: "consoleTicket", policy: "once" }],
        },
      ],
    });
    await page.route(
      "**/operations/nso_console/outputs/consoleTicket/reveal",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              vnc_url_https: "javascript:alert(1)",
              vnc_url: "http://insecure.example/session",
              vnc_password: "Synthetic-console",
            },
          },
        }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await view.button("查看结果").click();
    await view.button("获取连接").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认获取", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText(
      "暂未取得可打开的安全连接地址",
    );
    await expect(
      page.getByRole("link", { name: "打开远程控制台", exact: true }),
    ).toHaveCount(0);
    await view.screenshot("server-console-unsafe-rejected");
  });
  test("TC-19f 领取时登录过期只请求一次，保留管理页地址重新登录", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      operations: [
        {
          operationNo: "nso_expired",
          action: "reinstall",
          status: "succeeded",
          createdAt: now,
          updatedAt: now,
          availableOutputs: [{ name: "generatedCredential", policy: "once" }],
        },
      ],
    });
    let calls = 0,
      refreshes = 0;
    await page.route(
      "**/operations/nso_expired/outputs/generatedCredential/reveal",
      (r) => {
        calls++;
        return r.fulfill({ status: 401, json: { code: 401 } });
      },
    );
    await page.route("**/api/v1/auth/refresh", (r) => {
      refreshes++;
      return r.fulfill({ status: 500 });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await view.button("查看结果").click();
    await view.button("查看新密码").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认获取", exact: true })
      .click();
    await expect(page).toHaveURL(/auth\/login\?redirect=/);
    expect(new URL(page.url()).searchParams.get("redirect")).toBe(
      "/account/native-servers/SVR-managed",
    );
    expect(calls).toBe(1);
    expect(refreshes).toBe(0);
    await expect(
      page.getByRole("heading", { name: "登录用户中心", exact: true }),
    ).toBeVisible();
    await view.screenshot("server-output-session-expired");
  });
  test("TC-19g 救援系统与临时密码分离，提交后清除且不伪造生成密码", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      availableActions: ["enter_rescue", "exit_rescue"],
    });
    let prepared: any, executed: any;
    await page.route(
      "**/native-servers/SVR-managed/operation-confirmations",
      (r) => {
        prepared = r.request().postDataJSON();
        expect(prepared).toMatchObject({
          action: "enter_rescue",
          parameters: { system: "linux" },
        });
        expect(prepared).not.toHaveProperty("password");
        return r.fulfill({
          json: {
            code: 0,
            data: {
              required: true,
              proofToken: "rescue-proof",
              expiresAt: Date.now() + 60000,
            },
          },
        });
      },
    );
    await page.route("**/native-servers/SVR-managed/operations", (r) => {
      executed = r.request().postDataJSON();
      expect(executed).toEqual({
        ...prepared,
        proofToken: "rescue-proof",
        password: "Synthetic-Rescue-Only",
        confirmed: true,
      });
      return r.fulfill({
        json: {
          code: 0,
          data: {
            operation: {
              operationNo: "nso_rescue",
              action: "enter_rescue",
              status: "succeeded",
              createdAt: now,
              updatedAt: now,
            },
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "系统与连接", exact: true }).click();
    await view.button("进入救援").click();
    const dialog = page.getByRole("dialog", { name: "进入救援", exact: true });
    await dialog.getByPlaceholder("选择救援系统").click();
    await page.getByText("Linux", { exact: true }).click();
    await dialog
      .locator("input[type=password]")
      .nth(0)
      .fill("Synthetic-Rescue-Only");
    await dialog
      .locator("input[type=password]")
      .nth(1)
      .fill("Synthetic-Rescue-Only");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    const confirm = page.getByRole("dialog", {
      name: "确认进入救援",
      exact: true,
    });
    await expect(confirm).not.toContainText("Synthetic-Rescue-Only");
    await expect(confirm).toContainText("Linux");
    await confirm
      .getByRole("button", { name: "确认操作", exact: true })
      .click();
    await expect(confirm).toHaveCount(0);
    await expect.poll(() => Boolean(executed)).toBe(true);
    expect(
      await page.evaluate(() =>
        JSON.stringify({ ...sessionStorage, ...localStorage }),
      ),
    ).not.toContain("Synthetic-Rescue-Only");
    await expect(view.button("查看新密码")).toHaveCount(0);
    await view.screenshot("server-rescue-submitted");
  });
  test("TC-19h 网络操作只使用本人候选对象，端口转发校验后精确提交", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await page.route("**/native-servers/SVR-managed/management-options", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            ...choices,
            accessories: [
              ...choices.accessories,
              {
                objectNo: "traffic_own",
                kind: "traffic",
                name: "Traffic",
                state: "active",
                projection: { quota_gb: 1000, temporary_gb: 0, total_gb: 1000 },
              },
            ],
          },
        },
      }),
    );
    const prepared: any[] = [],
      executed: any[] = [];
    await page.route(
      "**/native-servers/SVR-managed/operation-confirmations",
      (r) => {
        const body = r.request().postDataJSON();
        prepared.push(body);
        return r.fulfill({
          json: { code: 0, data: { required: false, proofToken: "" } },
        });
      },
    );
    await page.route("**/native-servers/SVR-managed/operations", (r) => {
      const body = r.request().postDataJSON();
      executed.push(body);
      expect(body).toEqual({
        ...prepared.at(-1),
        confirmed: true,
        proofToken: "",
      });
      return r.fulfill({
        json: {
          code: 0,
          data: {
            operation: {
              operationNo: "nso_network" + executed.length,
              action: body.action,
              status: "succeeded",
              createdAt: now,
              updatedAt: now,
            },
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "网络与安全", exact: true }).click();
    await expect(view.main.locator(".traffic-allowance")).toContainText(
      "1,000 GB",
    );
    await expect(view.main.locator(".traffic-allowance")).toContainText("0 GB");
    await expect(view.main.locator(".traffic-allowance")).toContainText(
      "已分配额度",
    );
    await view.button("应用安全组").click();
    let dialog = page.getByRole("dialog");
    await dialog.getByPlaceholder("选择操作对象").click();
    await page
      .locator(".t-select-option")
      .getByText("网站安全组", { exact: true })
      .click();
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText("网站安全组");
    expect(prepared[0]).toMatchObject({
      action: "security_group_attach",
      parameters: { objectNo: "group_own" },
    });
    await dialog.getByRole("button", { name: "确认操作", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await view.button("添加端口转发").click();
    dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText(
      "请填写名称、协议及正确的外部端口和服务器端口",
    );
    expect(prepared).toHaveLength(1);
    await dialog.getByPlaceholder("例如：网站访问").fill("SSH备用连接");
    await dialog.getByPlaceholder("选择协议").click();
    await page
      .locator(".t-select-option")
      .getByText("TCP", { exact: true })
      .click();
    await dialog.getByPlaceholder("外部端口").fill("10022");
    await dialog.getByPlaceholder("服务器端口").fill("22");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText("10022");
    expect(prepared[1]).toMatchObject({
      action: "nat_create",
      parameters: {
        name: "SSH备用连接",
        protocol: "tcp",
        externalPort: 10022,
        internalPort: 22,
      },
    });
    await view.screenshot("server-network-confirmation");
    await dialog.getByRole("button", { name: "确认操作", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect(executed).toHaveLength(2);
  });
  test("TC-19i 目录不可用保留支持事实，执行前变化给出明确说明", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      availableActions: [],
      capabilities: [
        {
          action: "reinstall",
          supported: true,
          available: false,
          reasonCode: "choices_unavailable",
        },
      ],
      operations: [
        {
          operationNo: "nso_denied",
          action: "reinstall",
          status: "failed",
          failureCode: "owner_admission_denied",
          createdAt: now,
          updatedAt: now,
        },
      ],
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "系统与连接", exact: true }).click();
    await expect(view.button("重装系统")).toBeDisabled();
    await view.button("重装系统").locator("..").hover();
    await expect(
      page.getByText("管理选项暂不可用或已过期，请刷新管理选项后重试。", {
        exact: true,
      }),
    ).toBeVisible();
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await view.button("查看进度").click();
    await expect(view.main).toContainText(
      "服务器归属、服务状态或配置已变化，请刷新核对后重新操作。",
    );
    await expect(view.main).not.toContainText("owner_admission_denied");
    await view.screenshot("server-admission-changed");
  });
  test("TC-19j 受理和等待状态持续读取真实进度，人工接管停止自动查询", async ({
    consolePage: page,
  }) => {
    await setup(page);
    let polls = 0,
      writes = 0;
    const statuses = ["retry_wait", "running", "manual_takeover"];
    await mockNativeServer(page, {
      ...server,
      operations: [
        {
          operationNo: "nso_progress",
          action: "reboot",
          status: "accepted",
          createdAt: now,
          updatedAt: now,
        },
      ],
    });
    await page.route(
      "**/native-servers/SVR-managed/operations/nso_progress",
      (r) => {
        const index = Math.min(polls++, 2);
        return r.fulfill({
          json: {
            code: 0,
            data: {
              operation: {
                operationNo: "nso_progress",
                action: "reboot",
                status: statuses[index],
                createdAt: now,
                updatedAt: now + index + 1,
              },
            },
          },
        });
      },
    );
    await page.route("**/native-servers/SVR-managed/operations", (r) => {
      writes++;
      return r.fulfill({ status: 500 });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await expect(view.main).toContainText("已受理");
    await view.button("查看进度").click();
    await expect(view.main.locator(".server-progress")).toContainText(
      "等待处理",
    );
    await expect(view.main.locator(".server-progress")).toContainText("处理中");
    await expect(view.main.locator(".server-progress")).toContainText(
      "人工处理中",
    );
    await page.waitForTimeout(2800);
    expect(polls).toBe(3);
    expect(writes).toBe(0);
    await view.screenshot("server-task-manual-takeover");
  });
  test("TC-19k 320px真实长度服务编号和高级表单不撑宽页面", async ({
    consolePage: page,
  }) => {
    const no = "SVR-" + "A".repeat(32),
      ent = "ent_" + "a".repeat(32);
    await page.setViewportSize({ width: 320, height: 844 });
    await page.route(
      `**/x/idc-resource/api/v1/account/native-servers/${no}`,
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              ...server,
              serverNo: no,
              entitlementNo: ent,
              information: {
                ...server.information,
                hostname: "production-" + "a".repeat(52),
              },
              addresses: ["2001:db8:1234:5678:90ab:cdef:1234:5678"],
            },
          },
        }),
    );
    await page.route(`**/native-servers/${no}/management-options`, (r) =>
      r.fulfill({ json: { code: 0, data: choices } }),
    );
    await page.route(`**/service-entitlements/${ent}/customer-note`, (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            note: { entitlementNo: ent, remark: "网站环境", revision: 1 },
            supportAssociationReference: "sar_owned",
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open(`/account/native-servers/${no}`);
    await expect(view.main).toContainText(no);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(320);
    await view.screenshot("server-long-identifiers-320");
    await page.getByRole("tab", { name: "系统与连接", exact: true }).click();
    await view.button("重装系统").click();
    const dialog = page.getByRole("dialog", { name: "重装系统", exact: true });
    await dialog.getByText("高级设置（选填）", { exact: true }).click();
    await expect(dialog.getByText("登录端口", { exact: true })).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(320);
    await view.screenshot("server-reinstall-advanced-320");
  });
  test("TC-19l 常用开机和远程连接直接提交，失败重试保留本次请求号", async ({
    consolePage: page,
  }) => {
    await setup(page);
    let running = false,
      fail = true;
    const requests: any[] = [];
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-managed",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              ...server,
              runtimeState: running ? "running" : "stopped",
              availableActions: running ? ["console"] : ["start"],
            },
          },
        }),
    );
    await page.route("**/native-servers/SVR-managed/operations", (r) => {
      const body = r.request().postDataJSON();
      requests.push(body);
      expect(body).toMatchObject({
        parameters: {},
        confirmed: true,
        proofToken: "",
      });
      if (fail)
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "暂时无法提交操作，请重试" },
        });
      running = true;
      return r.fulfill({
        json: {
          code: 0,
          data: {
            operation: {
              operationNo: "nso_direct",
              action: body.action,
              status: "succeeded",
              createdAt: now,
              updatedAt: now,
            },
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await view.button("开机").click();
    await expect(view.main).toContainText("暂时无法提交操作，请重试");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    fail = false;
    await view.button("开机").click();
    await expect(
      view.main
        .locator(".server-heading")
        .getByRole("button", { name: "远程连接", exact: true }),
    ).toBeVisible();
    expect(requests).toHaveLength(2);
    expect(requests[1].idempotencyKey).toBe(requests[0].idempotencyKey);
    await view.main
      .locator(".server-heading")
      .getByRole("button", { name: "远程连接", exact: true })
      .click();
    await expect.poll(() => requests.length).toBe(3);
    expect(requests[2].action).toBe("console");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await view.screenshot("server-direct-power-console");
  });
  test("TC-19m 刷新后历史成功操作通过结果接口发现未领取密码", async ({
    consolePage: page,
  }) => {
    await setup(page);
    const operation = {
      operationNo: "nso_history",
      action: "reinstall",
      status: "succeeded",
      createdAt: now,
      updatedAt: now,
    };
    let reads = 0,
      reveals = 0;
    await page.route("**/native-servers/SVR-managed/operations?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [operation], total: 1 } } }),
    );
    await mockNativeServer(page, { ...server, operations: [operation] });
    await page.route(
      "**/native-servers/SVR-managed/operations/nso_history",
      (r) => {
        reads++;
        return r.fulfill({
          json: {
            code: 0,
            data: {
              operation: {
                ...operation,
                availableOutputs: [
                  { name: "generatedCredential", policy: "once" },
                ],
              },
            },
          },
        });
      },
    );
    await page.route(
      "**/operations/nso_history/outputs/generatedCredential/reveal",
      (r) => {
        reveals++;
        return r.fulfill({
          json: {
            code: 0,
            data: { username: "root", password: "Synthetic-History-Only" },
          },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    expect(reads).toBe(0);
    await expect(view.button("查看新密码")).toHaveCount(0);
    await view.button("查看结果").click();
    await expect(view.button("查看新密码")).toBeVisible();
    expect(reads).toBe(1);
    expect(reveals).toBe(0);
    await view.button("查看新密码").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认获取", exact: true })
      .click();
    await expect(page.getByTestId("generated-password")).toHaveText(
      "Synthetic-History-Only",
    );
    expect(reveals).toBe(1);
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "关闭并隐藏", exact: true })
      .click();
    await expect(
      page.getByText("Synthetic-History-Only", { exact: true }),
    ).toHaveCount(0);
    await view.screenshot("server-history-output-discovery");
  });
  test("TC-19n 操作记录真实分页与筛选，列表读取不会领取敏感结果", async ({
    consolePage: page,
  }) => {
    await setup(page);
    const queries: string[] = [];
    await page.route("**/native-servers/SVR-managed/operations?*", (r) => {
      const url = new URL(r.request().url());
      queries.push(url.search);
      const number = url.searchParams.get("page") === "2" ? 21 : 1;
      return r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                operationNo: `nso_page_${number}`,
                action: "reinstall",
                status: "succeeded",
                createdAt: now,
                updatedAt: now,
              },
            ],
            total: 25,
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    await expect.poll(() => queries.length).toBe(1);
    await expect(view.main).toContainText("共 25 条");
    await view.main.getByRole("listitem").filter({ hasText: /^2$/ }).click();
    await expect
      .poll(() => queries.some((q) => q.includes("page=2")))
      .toBe(true);
    await view.main.locator(".table-toolbar .t-select input").click();
    await page
      .locator(".t-select-option")
      .getByText("重装系统", { exact: true })
      .click();
    await expect
      .poll(() =>
        queries.some(
          (q) => q.includes("page=1") && q.includes("action=reinstall"),
        ),
      )
      .toBe(true);
    await expect(view.button("查看新密码")).toHaveCount(0);
    await view.screenshot("server-operations-filter-page");
  });
  test("TC-19o 快照恢复只使用所属对象，并要求输入完整服务器编号", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      runtimeState: "stopped",
      availableActions: ["snapshot_restore", "snapshot_delete"],
    });
    await page.route("**/native-servers/SVR-managed/management-options", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            ...choices,
            accessories: [
              {
                objectNo: "ACO_own",
                kind: "snapshot",
                name: "更新前快照",
                state: "available",
                projection: {},
              },
            ],
          },
        },
      }),
    );
    let prepared: any,
      writes: any[] = [];
    await page.route(
      "**/native-servers/SVR-managed/operation-confirmations",
      (r) => {
        prepared = r.request().postDataJSON();
        return r.fulfill({
          json: {
            code: 0,
            data: {
              required: true,
              proofToken: "restore-proof",
              expiresAt: Date.now() + 60000,
            },
          },
        });
      },
    );
    await page.route("**/native-servers/SVR-managed/operations", (r) => {
      writes.push(r.request().postDataJSON());
      return r.fulfill({
        json: {
          code: 0,
          data: {
            operation: {
              operationNo: "nso_restore",
              action: "snapshot_restore",
              status: "pending",
              createdAt: now,
              updatedAt: now,
            },
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "快照与备份", exact: true }).click();
    await view.button("恢复").click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText("请输入页面显示的完整服务器编号");
    expect(prepared).toBeUndefined();
    await dialog.getByPlaceholder("完整输入上方服务器编号").fill("SVR-managed");
    await dialog.getByRole("button", { name: "下一步", exact: true }).click();
    await expect(dialog).toContainText("恢复不收费");
    expect(prepared.parameters).toEqual({
      objectNo: "ACO_own",
      confirmedServiceNo: "SVR-managed",
    });
    await view.screenshot("snapshot-restore-confirm");
    await dialog.getByRole("button", { name: "确认操作", exact: true }).click();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0]).toMatchObject({
      ...prepared,
      proofToken: "restore-proof",
      confirmed: true,
    });
    await expect(view.main.locator(".server-progress")).toContainText(
      "操作已提交",
    );
  });
  test("TC-19p 快照就绪未知保留对象和删除入口，不伪造可恢复", async ({
    consolePage: page,
  }) => {
    await setup(page);
    await mockNativeServer(page, {
      ...server,
      availableActions: ["snapshot_delete"],
    });
    await page.route("**/native-servers/SVR-managed/management-options", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            ...choices,
            accessories: [
              {
                objectNo: "ACO_unknown",
                kind: "snapshot",
                name: "上游快照",
                state: "unknown",
                projection: {},
              },
            ],
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-managed");
    await page.getByRole("tab", { name: "快照与备份", exact: true }).click();
    await expect(view.main).toContainText("上游快照");
    await expect(view.main).toContainText("就绪状态未确认");
    await expect(view.button("恢复")).toHaveCount(0);
    await expect(view.button("删除")).toBeEnabled();
    await view.button("删除").click();
    await expect(page.getByRole("dialog")).toContainText("删除后无法恢复");
    await view.screenshot("snapshot-readiness-unknown");
  });
});
