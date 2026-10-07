import { readFileSync } from "node:fs";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

import type {
  Locator,
  Page,
  Route,
} from "../../../../../../hack/tests/fixtures/auth";

import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForDialogReady,
  waitForRouteReady,
} from "../../../../../../hack/tests/support/ui";

const catalogs = {
  "zh-CN": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
      "utf8",
    ),
  ),
} as Record<string, Record<string, string>>;

type PoolStatus = "disabled" | "draining" | "enabled" | "maintenance";

interface ProviderConfigurationMockOptions {
  missingHistoricalSchema?: boolean;
  candidates?: Array<{
    displayName: string;
    pluginId: string;
    providerId: string;
    releaseVersion: string;
    interfaceConfiguration?: Record<string, unknown>;
  }>;
}

interface PoolUpdateRequest {
  displayName: string;
  resourceKind: string;
  routingPriority: number;
  routingWeight: number;
  status: PoolStatus;
  version: number;
}

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const parts = key.split(".");
    let cursor = root;
    for (const part of parts.slice(0, -1)) {
      if (!cursor[part] || typeof cursor[part] !== "object") {
        cursor[part] = {};
      }
      cursor = cursor[part] as Record<string, unknown>;
    }
    cursor[parts.at(-1)!] = value;
  }
  return root;
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data) });
}

async function installProviderConfigurationMock(
  page: Page,
  options: ProviderConfigurationMockOptions = {},
) {
  const candidates = options.candidates ?? [
    {
      displayName: "魔方云 KVM 连接器",
      pluginId: "mofang-kvm",
      providerId: "mofang-kvm-v2",
      releaseVersion: "2.4.0",
      interfaceConfiguration: {
        version: "v1",
        fields: [
          {
            key: "region",
            type: "single_select",
            label: { enUS: "Region", zhCN: "接口区域" },
            required: true,
            staticCandidates: [
              {
                value: "east",
                label: { enUS: "East China", zhCN: "华东" },
              },
            ],
          },
          {
            key: "access_user",
            type: "secret",
            label: { enUS: "Access user", zhCN: "访问账号" },
            required: true,
          },
          {
            key: "access_token",
            type: "secret",
            label: { enUS: "Access token", zhCN: "访问令牌" },
            required: true,
          },
        ],
      },
    },
    {
      displayName: "备用服务器连接器",
      pluginId: "server-backup",
      providerId: "server-backup-v1",
      releaseVersion: "1.3.0",
    },
  ];
  const channelSchema = candidates[0]?.interfaceConfiguration;
  const channels = [
    {
      channelNo: "channel_primary",
      displayName: "华东生产渠道",
      endpoint: "https://provider.example.test/api",
      interfaceConfiguration: channelSchema,
      nonSecretSettings: { region: "east" },
      providerPluginId: "mofang-kvm",
      secretFields: [
        { key: "access_user", ready: false, required: true },
        { key: "access_token", ready: false, required: true },
      ],
      status: "disabled",
      version: 3,
    },
    {
      channelNo: "channel_stopped",
      displayName: "已停售 DCIM 接口",
      endpoint: "https://dcim.example.test/api",
      interfaceConfiguration: {
        version: "v1",
        fields: [
          {
            key: "protocol",
            type: "single_select",
            label: { enUS: "Protocol", zhCN: "接口版本" },
            required: true,
            staticCandidates: [
              { value: "legacy", label: { enUS: "Legacy", zhCN: "旧版接口" } },
              { value: "bms", label: { enUS: "BMS", zhCN: "BMS 接口" } },
            ],
          },
          {
            key: "access_token",
            type: "secret",
            label: { enUS: "Access token", zhCN: "访问令牌" },
            required: true,
          },
        ],
      },
      nonSecretSettings: { protocol: "legacy" },
      providerPluginId: "dcim-stopped",
      providerReleaseVersion: "1.0.0",
      secretFields: [
        {
          key: "access_token",
          ready: true,
          required: true,
          updatedAt: "2026-08-28T10:00:00Z",
        },
      ],
      status: "disabled",
      version: 2,
    },
  ];
  const channel = channels[0]!;
  if (options.missingHistoricalSchema)
    channel.interfaceConfiguration = undefined;
  const pool = {
    channelNo: channel.channelNo,
    displayName: "华东生产资源池",
    poolNo: "pool_primary",
    resourceKind: "virtual_server",
    routingPriority: 10,
    routingWeight: 80,
    status: "enabled" as PoolStatus,
    version: 5,
  };
  const poolUpdates: PoolUpdateRequest[] = [];
  const channelCreates: Array<Record<string, unknown>> = [];
  const channelUpdates: Array<Record<string, unknown>> = [];
  const secretCreates: Array<Record<string, unknown>> = [];

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data) {
      payload.data.permissions = [
        "idc:admin:resource:configuration:list",
        "idc:admin:resource:configuration:write",
        "idc:admin:resource:channel-secret:manage",
      ];
    }
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    await fulfill(route, {
      enabled: true,
      items: [
        {
          direction: "ltr",
          isDefault: true,
          locale: "zh-CN",
          name: "Simplified Chinese",
          nativeName: "简体中文",
        },
      ],
      locale: "zh-CN",
    });
  });
  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      await fulfill(route, { messages: nestedMessages(catalogs["zh-CN"]!) });
    },
  );
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await fulfill(route, {
      list: [
        {
          children: [
            {
              component: "system/plugin/dynamic-page",
              meta: {
                icon: "lucide:server-cog",
                title: "plugin.idc-resource.providerConfiguration.title",
              },
              name: "IDCProviderConfiguration",
              path: "/idc/resource/provider-configuration",
            },
          ],
          component: "BasicLayout",
          meta: { title: "IDC 运营" },
          name: "IDCOperations",
          path: "/idc",
        },
      ],
    });
  });
  await page.route(
    /\/x\/idc-resource\/api\/v1\/admin\/resource\/provider-configurations(?:\?.*)?$/,
    async (route: Route) => {
      await fulfill(route, {
        channels,
        channelTotal: channels.length,
        poolTotal: 1,
        pools: [pool],
        providerCandidates: candidates,
      });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/provider-configurations/channels",
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      channelCreates.push(body);
      const created = {
        channelNo: `channel_created_${channelCreates.length}`,
        displayName: body.displayName,
        endpoint: body.endpoint,
        interfaceConfiguration: channelSchema,
        nonSecretSettings: body.nonSecretSettings,
        providerPluginId: body.providerPluginId,
        secretFields: [
          { key: "access_user", ready: false, required: true },
          { key: "access_token", ready: false, required: true },
        ],
        status: "disabled",
        version: 1,
      };
      channels.push(created as (typeof channels)[number]);
      await fulfill(route, { item: created });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/provider-configurations/channels/*",
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      channelUpdates.push(body);
      const target = channels.find((item) =>
        route.request().url().endsWith(`/${item.channelNo}`),
      )!;
      Object.assign(target, body, { version: target.version + 1 });
      await fulfill(route, { item: target });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/channels/*/secrets/access-contexts",
    async (route: Route) => {
      await fulfill(route, { item: { token: "test-secret-access" } });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/channels/*/secrets",
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      secretCreates.push(body);
      const target = channels.find((item) =>
        route.request().url().includes(`/${item.channelNo}/`),
      )!;
      const status = target.secretFields.find(
        (item) => item.key === body.bindingName,
      );
      if (status)
        Object.assign(status, {
          ready: true,
          updatedAt: "2026-08-28T11:00:00Z",
        });
      await fulfill(route, { item: status });
    },
  );
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/provider-configurations/pools/*",
    async (route: Route) => {
      const body = route.request().postDataJSON() as PoolUpdateRequest;
      poolUpdates.push(body);
      Object.assign(pool, body, { version: pool.version + 1 });
      await fulfill(route, { item: pool });
    },
  );

  return { channelCreates, channelUpdates, poolUpdates, secretCreates };
}

class ProviderConfigurationPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("provider-configuration-page");
  }

  async goto() {
    const routePath = workspacePath("/idc/resource/provider-configuration");
    await this.page.goto(routePath, { waitUntil: "domcontentloaded" });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await expect(
      this.page.getByText("加载菜单中...", { exact: true }),
    ).toHaveCount(0, { timeout: 15_000 });
    const reloaded = await this.page
      .waitForEvent("framenavigated", {
        predicate: (frame) =>
          frame === this.page.mainFrame() && frame.url().includes(routePath),
        timeout: 5_000,
      })
      .then(() => true)
      .catch(() => false);
    if (reloaded) {
      await this.root.waitFor({ state: "visible" });
      await waitForRouteReady(this.page, 15_000);
    }
  }

  async openChannelCreate() {
    await this.root.getByRole("button", { name: "新增接口" }).click();
    return waitForDialogReady(this.page.getByRole("dialog"));
  }

  async openChannelEdit(name: string) {
    const row = await this.channelActionRow(name);
    await row.getByRole("button", { name: "编辑" }).click();
    return waitForDialogReady(this.page.getByRole("dialog"));
  }

  async channelActionRow(name: string) {
    const rows = this.root.locator(".vxe-table--main-wrapper .vxe-body--row");
    const rowIndex = await rows.evaluateAll(
      (elements, expectedName) =>
        elements.findIndex((element) =>
          element.textContent?.includes(expectedName),
        ),
      name,
    );
    expect(rowIndex).toBeGreaterThanOrEqual(0);
    return this.root
      .locator(".vxe-table--fixed-right-wrapper .vxe-body--row")
      .nth(rowIndex);
  }

  async openPoolEdit() {
    await this.root.getByRole("tab", { name: "资源池绑定" }).click();
    const row = this.root
      .locator(".vxe-table--fixed-right-wrapper .vxe-body--row")
      .filter({ hasText: "编辑" });
    await row.getByRole("button", { name: "编辑" }).click();
    return waitForDialogReady(this.page.getByRole("dialog"));
  }

  async selectStatus(dialog: Locator, label: string) {
    await dialog.locator(".ant-select-selector").last().click();
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(label, { exact: true })
      .click();
  }

  async screenshot(name: string) {
    const now = new Date();
    const directory = resolve(
      fileURLToPath(new URL("../../../../../../", import.meta.url)),
      "temp",
      now.toISOString().slice(0, 10).replaceAll("-", ""),
    );
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      fullPage: false,
      path: resolve(
        directory,
        `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`,
      ),
    });
  }
}

test.describe("TC-7 供应商接口与资源池配置", () => {
  test("TC-7f: 历史配置缺失只禁用该行操作而不影响正常接口", async ({
    adminPage,
  }) => {
    const state = await installProviderConfigurationMock(adminPage, {
      missingHistoricalSchema: true,
    });
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();
    await expect(
      configuration.root.getByText("配置版本缺失", { exact: true }),
    ).toBeVisible();
    await expect(
      configuration.root.getByText(
        "找不到此接口保存时的插件配置版本，暂不能修改或测试；请联系管理员恢复对应版本。",
        { exact: true },
      ),
    ).toBeVisible();
    const missing = await configuration.channelActionRow("华东生产渠道");
    for (const name of ["编辑", "测试连接", "启用", "删除"]) {
      await expect(
        missing.getByRole("button", { name, exact: true }),
      ).toBeDisabled();
    }
    await configuration.screenshot("provider-missing-history-isolated");
    const dialog = await configuration.openChannelEdit("已停售 DCIM 接口");
    await expect(dialog).toContainText("接口版本");
    expect(state.channelUpdates).toHaveLength(0);
    expect(state.secretCreates).toHaveLength(0);
  });
  test("TC-7a: 同一连接器可以创建多条独立命名接口且不显示插件 ID", async ({
    adminPage,
  }) => {
    const state = await installProviderConfigurationMock(adminPage);
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();

    for (const [index, name] of [
      "华东魔方云接口",
      "华北魔方云接口",
    ].entries()) {
      const dialog = await configuration.openChannelCreate();
      await expect(dialog).toBeVisible();
      if (index === 0) {
        await dialog.locator(".ant-select-selector").first().click();
        const dropdown = adminPage.locator(".ant-select-dropdown:visible");
        await expect(
          dropdown.getByText("魔方云 KVM 连接器 · 2.4.0", { exact: true }),
        ).toBeVisible();
        await expect(
          dropdown.getByText("备用服务器连接器 · 1.3.0", { exact: true }),
        ).toBeVisible();
      }
      await dialog.locator('input[name="displayName"]').fill(name);
      await dialog
        .locator('input[name="endpoint"]')
        .fill(`https://provider-${index + 1}.example.test/api`);
      await dialog.locator(".ant-select-selector").last().click();
      await expect(
        adminPage
          .locator(".ant-select-dropdown:visible")
          .getByText("华东", { exact: true }),
      ).toBeVisible();
      await adminPage.keyboard.press("Enter");
      await dialog
        .locator('input[name="access_user"]')
        .fill(`account-${index + 1}`);
      await dialog
        .locator('input[name="access_token"]')
        .fill(`token-${index + 1}`);
      await dialog.getByRole("button", { name: /确\s*认/ }).click();
      await expect(dialog).toBeHidden();
    }
    expect(state.channelCreates).toHaveLength(2);
    expect(state.channelCreates.map((item) => item.displayName)).toEqual([
      "华东魔方云接口",
      "华北魔方云接口",
    ]);
    expect(
      state.channelCreates.every(
        (item) => item.providerPluginId === "mofang-kvm",
      ),
    ).toBe(true);
    expect(state.secretCreates).toHaveLength(4);
    await expect(
      adminPage.getByText("mofang-kvm", { exact: true }),
    ).not.toBeVisible();
    await expect(
      adminPage.getByText("server-backup", { exact: true }),
    ).not.toBeVisible();
    await configuration.screenshot("provider-channel-connector-selection");
  });

  test("TC-7b: 必填凭据未配置时不能启用，编辑后只显示已配置状态而不回显秘密", async ({
    adminPage,
  }) => {
    const state = await installProviderConfigurationMock(adminPage);
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();

    const row = await configuration.channelActionRow("华东生产渠道");
    await expect(row.getByRole("button", { name: "启用" })).toBeDisabled();
    const dialog = await configuration.openChannelEdit("华东生产渠道");
    await dialog.locator('input[name="access_user"]').fill("new-access-user");
    await dialog.locator('input[name="access_token"]').fill("new-access-token");
    await dialog.getByRole("button", { name: /确\s*认/ }).click();
    await expect(dialog).toBeHidden();
    expect(state.secretCreates.map((item) => item.bindingName).sort()).toEqual([
      "access_token",
      "access_user",
    ]);
    await expect(
      configuration.root.getByText("已配置", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      adminPage.getByText("new-access-token", { exact: true }),
    ).not.toBeVisible();
    await configuration.screenshot("provider-required-secret-readiness");
  });

  test("TC-7c: 连接器停止新业务后已有接口仍按冻结字段编辑且没有原始 JSON 输入", async ({
    adminPage,
  }) => {
    await installProviderConfigurationMock(adminPage);
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();

    const dialog = await configuration.openChannelEdit("已停售 DCIM 接口");
    await expect(dialog).toContainText("接口版本");
    await expect(
      dialog.getByPlaceholder("已经安全保存；留空表示不更换"),
    ).toBeVisible();
    await expect(
      dialog.locator('input[name="providerDisplayName"]'),
    ).toBeDisabled();
    await expect(dialog.locator("textarea")).toHaveCount(0);
    await expect(dialog.getByText(/JSON/i)).toHaveCount(0);
    await configuration.screenshot("provider-stopped-connector-frozen-schema");
  });

  test("TC-7d: 编辑资源池可提交四种状态及当前优先级和权重", async ({
    adminPage,
  }) => {
    const state = await installProviderConfigurationMock(adminPage);
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();

    const cases: Array<{
      label: string;
      priority: number;
      status: PoolStatus;
      weight: number;
    }> = [
      { label: "启用", priority: 11, status: "enabled", weight: 81 },
      {
        label: "停止接收新单",
        priority: 12,
        status: "draining",
        weight: 82,
      },
      { label: "维护中", priority: 13, status: "maintenance", weight: 83 },
      { label: "停用", priority: 14, status: "disabled", weight: 84 },
    ];

    for (const item of cases) {
      const dialog = await configuration.openPoolEdit();
      await expect(dialog).toBeVisible();
      await configuration.selectStatus(dialog, item.label);
      const routingInputs = dialog.getByRole("spinbutton");
      await expect(routingInputs).toHaveCount(2);
      await routingInputs.nth(0).fill(String(item.priority));
      await routingInputs.nth(1).fill(String(item.weight));
      await dialog.getByRole("button", { name: /确\s*认/ }).click();
      await expect(dialog).toBeHidden();
    }

    expect(state.poolUpdates).toHaveLength(cases.length);
    for (const [index, item] of cases.entries()) {
      expect(state.poolUpdates[index]).toMatchObject({
        displayName: "华东生产资源池",
        resourceKind: "virtual_server",
        routingPriority: item.priority,
        routingWeight: item.weight,
        status: item.status,
      });
    }
    await configuration.screenshot("provider-pool-status-updates");
  });

  test("TC-7e: 没有可用连接器时禁用新增接口并显示处理提示", async ({
    adminPage,
  }) => {
    await installProviderConfigurationMock(adminPage, { candidates: [] });
    const configuration = new ProviderConfigurationPage(adminPage);
    await configuration.goto();

    await expect(
      configuration.root.getByRole("button", { name: "新增接口" }),
    ).toBeDisabled();
    const alert = configuration.root.getByRole("alert").filter({
      hasText: "当前没有可用于新业务",
    });
    await expect(alert).toContainText(
      "当前没有可用于新业务的服务器连接器，请先安装并启用对应插件。",
    );
    await expect(alert).not.toContainText("plugin.idc-resource");
    await configuration.screenshot("provider-channel-no-connector");
  });
});
