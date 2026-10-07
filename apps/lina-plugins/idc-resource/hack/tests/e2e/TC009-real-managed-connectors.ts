import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { createHash, randomUUID } from "node:crypto";
import { pluginApiPath } from "../../../../../../hack/tests/fixtures/config";
import {
  createAdminApiContext,
  ensureSourcePluginEnabledViaAPI,
} from "../../../../../../hack/tests/fixtures/plugin";
import { CatalogManagementPage } from "../../../../idc-core/hack/tests/pages/CatalogManagementPage";
import { ProviderConfigurationPage } from "../pages/ProviderConfigurationPage";

type APIEnvelope<T> = { code: number; data: T; message: string };

function digest(value: string) {
  return createHash("sha256").update(value).digest("hex");
}

async function createChannelThroughApi(
  api: any,
  input: (typeof connectorCases)[number],
  name: string,
  endpoint: string,
) {
  const response = await api.post(
    pluginApiPath(
      "idc-resource",
      "admin/resource/provider-configurations/channels",
    ),
    {
      data: {
        displayName: name,
        endpoint,
        providerPluginId: input.pluginId,
        status: "disabled",
        nonSecretSettings:
          "protocolValue" in input && input.protocolValue
            ? { endpoint, protocol: input.protocolValue }
            : {},
      },
    },
  );
  const payload = (await response.json()) as APIEnvelope<{
    item?: { channelNo: string };
    channelNo?: string;
    channel?: { channelNo: string };
  }>;
  if (payload.code !== 0)
    throw new Error(`${input.pluginId}: ${payload.message}`);
  const channelNo =
    payload.data.item?.channelNo ||
    payload.data.channelNo ||
    payload.data.channel?.channelNo;
  expect(channelNo).toBeTruthy();
  for (const [bindingName, value] of Object.entries(input.secrets)) {
    const idempotencyKey = `tc009${Date.now()}${randomUUID().replaceAll("-", "")}`;
    const secretDigest = digest(value);
    const accessResponse = await api.post(
      pluginApiPath(
        "idc-resource",
        `admin/resource/channels/${channelNo}/secrets/access-contexts`,
      ),
      {
        data: {
          operation: "create",
          bindingName,
          secretDigest,
          expectedUpdatedAt: 0,
          rotateLimit: 0,
          idempotencyKey,
        },
      },
    );
    const access = (await accessResponse.json()) as APIEnvelope<{
      item: { token: string };
    }>;
    if (access.code !== 0)
      throw new Error(
        `${input.pluginId}/${bindingName}: ${access.message}; channel=${channelNo}; digest=${secretDigest}; idempotency=${idempotencyKey}`,
      );
    const secretResponse = await api.post(
      pluginApiPath(
        "idc-resource",
        `admin/resource/channels/${channelNo}/secrets`,
      ),
      {
        data: {
          bindingName,
          secret: Buffer.from(value).toString("base64"),
          expectedUpdatedAt: 0,
          idempotencyKey,
          accessContext: access.data.item.token,
        },
      },
    );
    const secret = (await secretResponse.json()) as APIEnvelope<unknown>;
    expect(secret.code, secret.message).toBe(0);
  }
  const enableResponse = await api.put(
    pluginApiPath(
      "idc-resource",
      `admin/resource/provider-configurations/channels/${channelNo}`,
    ),
    {
      data: { displayName: name, endpoint, status: "enabled", version: 1 },
    },
  );
  const enabled = (await enableResponse.json()) as APIEnvelope<unknown>;
  expect(enabled.code, enabled.message).toBe(0);
}

async function createDraftProductThroughApi(api: any) {
  const categoriesResponse = await api.get(
    pluginApiPath("idc-core", "admin/catalog/categories"),
  );
  const categoriesPayload = (await categoriesResponse.json()) as APIEnvelope<{
    categories: Array<{ categoryNo: string }>;
  }>;
  expect(categoriesPayload.code, categoriesPayload.message).toBe(0);
  let categoryNo = categoriesPayload.data.categories[0]?.categoryNo;
  if (!categoryNo) {
    const categoryResponse = await api.post(
      pluginApiPath("idc-core", "admin/catalog/categories"),
      {
        data: {
          nameEnUs: `FB52 connector verification ${runSuffix}`,
          nameZhCn: `FB52 连接器验证 ${runSuffix}`,
          parentCategoryNo: "",
          sortOrder: 900_000,
        },
      },
    );
    const categoryPayload = (await categoryResponse.json()) as APIEnvelope<{
      category: { categoryNo: string };
    }>;
    expect(categoryPayload.code, categoryPayload.message).toBe(0);
    categoryNo = categoryPayload.data.category.categoryNo;
  }
  const productResponse = await api.post(
    pluginApiPath("idc-core", "admin/catalog/products"),
    {
      data: {
        categoryNo,
        contractPolicyCode: "none",
        contractTemplateNo: "",
        inventoryTrackingEnabled: false,
        nameEnUs: `FB52 connector binding ${runSuffix}`,
        nameZhCn: `FB52 连接器绑定 ${runSuffix}`,
        publicSlug: `fb52-connector-binding-${runSuffix}`.toLowerCase(),
        sellableQuantity: 0,
        sortOrder: 900_000,
        summaryEnUs:
          "Dedicated draft for managed connector binding verification.",
        summaryZhCn: "用于托管连接器绑定验证的专用草稿。",
      },
    },
  );
  const productPayload = (await productResponse.json()) as APIEnvelope<{
    productNo: string;
    versionNo: string;
    versionStatus: string;
  }>;
  expect(productPayload.code, productPayload.message).toBe(0);
  expect(productPayload.data.versionStatus).toBe("draft");
  return productPayload.data;
}

const runSuffix = new Date()
  .toISOString()
  .replaceAll(/[-:.TZ]/gu, "")
  .slice(-10);
const connectorCases = [
  {
    connector: "IDC 智简魔方云适配器 · v0.2.2",
    key: "魔方云",
    pluginId: "idc-zjmfcloud",
    secrets: {
      password: "fb52-zjmfcloud-password",
      username: "fb52-zjmfcloud-user",
    },
  },
  {
    connector: "DCIM 服务器连接器 · v0.1.0",
    key: "DCIM",
    pluginId: "idc-dcim",
    protocol: "旧版 DCIM 接口",
    protocolValue: "legacy",
    secrets: { password: "fb52-dcim-password", username: "fb52-dcim-user" },
  },
  {
    connector: "魔方 V10 供应商接口 · v0.2.0",
    key: "V10",
    pluginId: "idc-v10",
    secrets: { password: "fb52-v10-password", username: "fb52-v10-user" },
  },
  {
    connector: "魔方财务原生连接器 · v0.1.2",
    key: "魔方财务",
    pluginId: "idc-zjmf-finance",
    secrets: {
      password: "fb52-finance-password",
      username: "fb52-finance-user",
    },
  },
  {
    connector: "宝塔面板连接器 · v0.1.1",
    key: "宝塔",
    pluginId: "idc-baota",
    secrets: { api_key: "fb52-baota-api-key" },
  },
] as const;

test.describe("TC-9 真实托管服务器连接器多接口与商品绑定", () => {
  test("TC-9a: 五种连接器各创建两条独立接口并将商品版本绑定到指定接口", async ({
    adminPage,
  }) => {
    test.setTimeout(240_000);
    const adminApi = await createAdminApiContext();
    const createdNames: string[] = [];
    try {
      for (const item of connectorCases) {
        await ensureSourcePluginEnabledViaAPI(adminApi, item.pluginId);
      }
      for (const item of connectorCases) {
        for (const label of ["A", "B"] as const) {
          const name = `FB52-${runSuffix}-${item.key}-${label}接口`;
          createdNames.push(name);
          await createChannelThroughApi(
            adminApi,
            item,
            name,
            `https://${item.pluginId}-${label.toLowerCase()}.example.test/api`,
          );
        }
      }
      const providerPage = new ProviderConfigurationPage(adminPage);
      await providerPage.goto();

      await expect(providerPage.root).toContainText(createdNames[0]!);
      await expect(providerPage.root).toContainText(createdNames.at(-1)!);
      const providerScreenshot = await providerPage.screenshot(
        "fb52-real-managed-connector-multiple-interfaces",
      );
      test.info().annotations.push({
        description: providerScreenshot,
        type: "evidence",
      });

      await expect
        .poll(
          async () => {
            const response = await adminApi.get(
              pluginApiPath("idc-core", "admin/catalog/automation/interfaces"),
              {
                params: {
                  namePrefix: `FB52-${runSuffix}`,
                  page: 1,
                  pageSize: 100,
                },
              },
            );
            const payload = (await response.json()) as APIEnvelope<{
              items: Array<{ displayName: string }>;
              total: number;
            }>;
            return payload.code === 0 ? payload.data.total : -1;
          },
          { timeout: 60_000 },
        )
        .toBe(10);

      const product = await createDraftProductThroughApi(adminApi);

      const catalog = new CatalogManagementPage(adminPage);
      await catalog.goto();
      await catalog.openProductDetail(product.productNo);
      const removeBinding = catalog.detailDrawer.getByTestId(
        "catalog-fulfillment-binding-remove",
      );
      if (await removeBinding.isVisible()) {
        await removeBinding.click();
      }
      await catalog.detailDrawer
        .getByTestId("catalog-fulfillment-binding-open")
        .click();
      const bindingModal = adminPage.getByTestId(
        "catalog-fulfillment-binding-modal",
      );
      await bindingModal.waitFor({ state: "visible" });
      await adminPage.getByTestId("catalog-fulfillment-provider").click();
      await adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText("IDC 智简魔方云适配器", { exact: true })
        .click();
      await adminPage.getByTestId("catalog-fulfillment-interface").click();
      const selectedInterface = createdNames[0]!;
      await adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText(selectedInterface, { exact: false })
        .click();
      await adminPage
        .getByRole("dialog")
        .filter({ has: bindingModal })
        .getByRole("button", { name: "确认绑定" })
        .click();
      await bindingModal.waitFor({ state: "hidden" });
      await expect(
        catalog.detailDrawer.getByTestId("catalog-fulfillment-binding-summary"),
      ).toContainText(selectedInterface);
      const bindingScreenshot = await catalog.screenshot(
        "fb52-real-product-version-specific-interface-binding",
      );
      test.info().annotations.push({
        description: bindingScreenshot,
        type: "evidence",
      });

      const unbindResponse = await adminApi.delete(
        pluginApiPath(
          "idc-core",
          `admin/catalog/product-versions/${product.versionNo}/fulfillment-binding`,
        ),
      );
      const unbindPayload =
        (await unbindResponse.json()) as APIEnvelope<unknown>;
      expect(unbindPayload.code).toBe(0);
    } finally {
      await adminApi.dispose();
    }
  });
});
