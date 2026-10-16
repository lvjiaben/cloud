import type { Page, Route } from "@host-tests/support/playwright";
import { readFile } from "node:fs/promises";
import { config } from "@host-tests/fixtures/config";

const categories = [
  {
    categoryNo: "ctg_00000000000000000000000000000001",
    deleted: false,
    nameEnUs: "Digital products",
    nameZhCn: "数字产品",
    parentCategoryNo: "",
    sortOrder: 10,
    status: "active",
    version: 1,
  },
  {
    categoryNo: "ctg_00000000000000000000000000000002",
    deleted: false,
    nameEnUs: "Certificates",
    nameZhCn: "证书服务",
    parentCategoryNo: "ctg_00000000000000000000000000000001",
    sortOrder: 20,
    status: "active",
    version: 1,
  },
  {
    categoryNo: "ctg_00000000000000000000000000000003",
    deleted: false,
    nameEnUs: "Physical products",
    nameZhCn: "实物商品",
    parentCategoryNo: "",
    sortOrder: 30,
    status: "active",
    version: 1,
  },
  {
    categoryNo: "ctg_00000000000000000000000000000004",
    deleted: false,
    nameEnUs: "Professional services",
    nameZhCn: "专业服务",
    parentCategoryNo: "",
    sortOrder: 40,
    status: "active",
    version: 1,
  },
];

export const products = [
  {
    billingRuleCount: 1,
    categoryNameEnUs: "Certificates",
    categoryNameZhCn: "证书服务",
    categoryNo: categories[1]!.categoryNo,
    createdAt: 1_786_161_600_000,
    currencyCount: 1,
    nameEnUs: "Wildcard SSL certificate",
    nameZhCn: "通配符 SSL 证书",
    productNo: "product-ssl",
    publicSlug: "wildcard-ssl-certificate",
    publishedAt: 1_786_161_600_000,
    source: "manual",
    status: "active",
    summaryEnUs: "Protect multiple subdomains",
    summaryZhCn: "保护同一主域名下的多个子域名",
    updatedAt: 1_786_161_660_000,
    versionNo: "version-ssl-1",
    versionNumber: 1,
    versionStatus: "published",
  },
  {
    billingRuleCount: 1,
    categoryNameEnUs: "Physical products",
    categoryNameZhCn: "实物商品",
    categoryNo: categories[2]!.categoryNo,
    createdAt: 1_786_161_600_000,
    currencyCount: 1,
    nameEnUs: "Business phone",
    nameZhCn: "商务手机",
    productNo: "product-phone",
    publicSlug: "business-phone",
    publishedAt: 1_786_161_600_000,
    source: "manual",
    status: "active",
    summaryEnUs: "A physical phone product",
    summaryZhCn: "可配送的实体手机商品",
    updatedAt: 1_786_161_660_000,
    versionNo: "version-phone-1",
    versionNumber: 1,
    versionStatus: "published",
  },
  {
    billingRuleCount: 0,
    categoryNameEnUs: "Professional services",
    categoryNameZhCn: "专业服务",
    categoryNo: categories[3]!.categoryNo,
    createdAt: 1_786_161_600_000,
    currencyCount: 0,
    nameEnUs: "Manual migration service",
    nameZhCn: "人工迁移服务",
    productNo: "product-manual-service",
    publicSlug: "manual-migration-service",
    publishedAt: null,
    source: "manual",
    status: "draft",
    summaryEnUs: "A service fulfilled by staff",
    summaryZhCn: "由工作人员人工交付的服务",
    updatedAt: 1_786_161_660_000,
    versionNo: "version-manual-service-1",
    versionNumber: 1,
    versionStatus: "draft",
  },
];

function productDetail(
  product = products[0]!,
  fulfillmentBinding: unknown = null,
  fulfillmentPolicy: unknown = null,
) {
  return {
    billingRules: product.billingRuleCount
      ? [
          {
            billingRuleNo: "billing-monthly",
            chargeType: "recurring",
            maximumRenewalCycles: null,
            meterUnit: null,
            minimumPurchaseCycles: 1,
            periodCount: 1,
            periodUnit: "year",
            pricePhase: "both",
            quantityMode: "fixed",
            ruleCode: "standard",
            status: "active",
            tierMode: null,
            trialQuantity: 0,
            usageCollectionCount: null,
            usageCollectionUnit: null,
            usageMaximumQuantity: null,
            usageMinimumQuantity: null,
            usageMissingPolicy: null,
            usageRoundingMode: null,
            usageRoundingQuantity: null,
          },
        ]
      : [],
    prices: product.billingRuleCount
      ? [
          {
            amountMinor: "19900",
            billingRuleNo: "billing-monthly",
            currencyCode: "CNY",
            maximumQuantity: null,
            minimumQuantity: 1,
            priceNo: "price-base",
            renewalAmountMinor: "19900",
            setupFeeMinor: "0",
            status: "active",
          },
        ]
      : [],
    fulfillmentBinding,
    fulfillmentPolicy,
    inventory: { trackingEnabled: true, sellableQuantity: 25, version: 1 },
    product: {
      categoryNameEnUs: product.categoryNameEnUs,
      categoryNameZhCn: product.categoryNameZhCn,
      categoryNo: product.categoryNo,
      createdAt: product.createdAt,
      nameEnUs: product.nameEnUs,
      nameZhCn: product.nameZhCn,
      productNo: product.productNo,
      publicSlug: product.publicSlug,
      sortOrder: 10,
      status: product.status,
      summaryEnUs: product.summaryEnUs,
      summaryZhCn: product.summaryZhCn,
      updatedAt: product.updatedAt,
    },
    taxPolicies: product.billingRuleCount
      ? [
          {
            currencyCode: "CNY",
            priceIncludesTax: true,
            roundingMode: "half_up",
            roundingQuantumMinor: "1",
            roundingScope: "total",
            taxCategoryCode: "digital-service",
            taxPolicyNo: "tax-cny",
            taxRateBasisPoints: 600,
            taxTreatment: "taxable",
          },
        ]
      : [],
    version: {
      changeSummary: "Initial version",
      contractPolicyCode: "inherit",
      contractPolicyVersion: 0,
      contractTemplateNo: "",
      createdAt: product.createdAt,
      publishedAt: product.publishedAt,
      requiresContract: false,
      retiredAt: null,
      source: product.source,
      sourceVersion: "",
      status: product.versionStatus,
      versionNo: product.versionNo,
      versionNumber: product.versionNumber,
    },
  };
}

export function mockProductDetail(productNo: string) {
  return productDetail(products.find((product) => product.productNo === productNo));
}

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export interface CatalogMockState {
  categoryRequests: Array<{ body?: unknown; method: string; url: URL }>;
  detailRequests: URL[];
  listRequests: URL[];
  previewRequests: URL[];
  productCreateRequests: Array<{ body: unknown; method: string; url: URL }>;
  publishRequests: Array<{ method: string; url: URL }>;
  fulfillmentPolicyRequests: Array<{
    body: unknown;
    method: string;
    url: URL;
  }>;
  productVersionCostRequests: Array<{
    body?: unknown;
    method: string;
    url: URL;
  }>;
  inventoryRequests: Array<{ body?: unknown; method: string; url: URL }>;
}

export async function installCatalogManagementMock(
  page: Page,
  options: {
    fulfillmentBinding?: unknown;
    fulfillmentPolicy?: unknown;
    fulfillmentPolicyDelayMs?: number;
    fulfillmentPolicyFailure?: boolean;
    publicationReady?: boolean;
  } = {},
) {
  const categoryRows = categories.map(row => ({ ...row }));
  // Read the current plugin resources: the running backend may still cache an older bundle.
  await page.addInitScript(() => {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
    }
  });
  await page.route("**/api/v1/i18n/runtime/messages**", async route => {
    const headers = { ...route.request().headers() };
    delete headers["if-none-match"];
    const requested = new URL(route.request().url());
    const response = await route.fetch({ headers, url: new URL(requested.pathname + requested.search, config.backendBaseURL).href });
    const payload = await response.json();
    const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
    const messages = JSON.parse(await readFile(new URL(`../../../manifest/i18n/${locale}/catalog-workspace.json`, import.meta.url), "utf8"));
    payload.data.messages.plugin ??= {};
    payload.data.messages.plugin["idc-core"] ??= {};
    payload.data.messages.plugin["idc-core"].catalog ??= {};
    for (const section of ["workspace", "commerce", "configurationPrice", "copy"]) {
      payload.data.messages.plugin["idc-core"].catalog[section] = { ...payload.data.messages.plugin["idc-core"].catalog[section], ...Object.fromEntries(
        Object.entries(messages).filter(([key]) => key.startsWith(`plugin.idc-core.catalog.${section}.`)).map(([key, value]) => [key.replace(`plugin.idc-core.catalog.${section}.`, ""), value]),
      ) };
    }
    await route.fulfill({ response, json: payload });
  });
  const state: CatalogMockState = {
    categoryRequests: [],
    detailRequests: [],
    listRequests: [],
    previewRequests: [],
    productCreateRequests: [],
    publishRequests: [],
    fulfillmentPolicyRequests: [],
    productVersionCostRequests: [],
    inventoryRequests: [],
  };
  let createdProduct: (typeof products)[number] | undefined;
  let fulfillmentPolicy = options.fulfillmentPolicy ?? null;
  let productVersionCosts = [
    {
      costNo: "cst_00000000000000000000000000000001",
      currencyCode: "CNY",
      effectiveFrom: 1_786_161_600_000,
      effectiveUntil: null as number | null,
      periodCount: 1,
      periodUnit: "year",
      unitCostMinor: "8800",
      version: 1,
    },
  ];

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/costs**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const body = request.method() === "GET" ? undefined : request.postDataJSON();
      state.productVersionCostRequests.push({ body, method: request.method(), url });
      if (request.method() === "POST") {
        const input = body as Record<string, unknown>;
        productVersionCosts = [
          ...productVersionCosts,
          {
            costNo: "cst_00000000000000000000000000000002",
            currencyCode: String(input.currencyCode),
            effectiveFrom: Number(input.effectiveFrom),
            effectiveUntil: input.effectiveUntil ? Number(input.effectiveUntil) : null,
            periodCount: Number(input.periodCount),
            periodUnit: String(input.periodUnit),
            unitCostMinor: String(input.unitCostMinor),
            version: 1,
          },
        ];
        await route.fulfill({ json: apiEnvelope({ cost: productVersionCosts.at(-1) }) });
        return;
      }
	  if (request.method() === "PUT") {
		const input = body as Record<string, unknown>;
		const current = productVersionCosts[0]!;
		productVersionCosts[0] = {
			...current,
			currencyCode: String(input.currencyCode),
			effectiveFrom: Number(input.effectiveFrom),
			effectiveUntil: input.effectiveUntil ? Number(input.effectiveUntil) : null,
			periodCount: Number(input.periodCount),
			periodUnit: String(input.periodUnit),
			unitCostMinor: String(input.unitCostMinor),
			version: current.version + 1,
		};
		await route.fulfill({ json: apiEnvelope({ cost: productVersionCosts[0] }) });
		return;
	  }
      await route.fulfill({ json: apiEnvelope({ costs: productVersionCosts }) });
    },
  );

  await page.route(
	"**/x/idc-core/api/v1/admin/catalog/product-versions/*/inventory",
	async (route: Route) => {
		const request = route.request();
		const body = request.postDataJSON() as Record<string, unknown>;
		state.inventoryRequests.push({ body, method: request.method(), url: new URL(request.url()) });
		await route.fulfill({ json: apiEnvelope({ inventory: {
			trackingEnabled: Boolean(body.trackingEnabled),
			sellableQuantity: Number(body.sellableQuantity),
			version: Number(body.expectedVersion) + 1,
		} }) });
	},
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/fulfillment-policy",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const body = request.postDataJSON() as { strategyCode?: string };
      state.fulfillmentPolicyRequests.push({
        body,
        method: request.method(),
        url,
      });
      if (options.fulfillmentPolicyDelayMs) {
        await new Promise((resolve) =>
          setTimeout(resolve, options.fulfillmentPolicyDelayMs),
        );
      }
      if (options.fulfillmentPolicyFailure) {
        await route.fulfill({
          json: {
            code: 409,
            data: {},
            message: "catalog fulfillment policy conflict",
            messageKey: "error.idc.catalog.management.conflict",
          },
        });
        return;
      }
      const pluginMode = body.strategyCode !== "manual";
      fulfillmentPolicy = {
        capabilityEnUs: pluginMode ? "Issue and manage certificates" : "",
        capabilityZhCn: pluginMode ? "签发并管理证书" : "",
        quantityHandling: pluginMode ? "separately" : "together",
        strategyCode: body.strategyCode,
      };
      await route.fulfill({
        json: apiEnvelope({ policy: fulfillmentPolicy }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/contract-policy/templates**",
    (route: Route) => route.fulfill({ json: apiEnvelope({ templates: [] }) }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/categories**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const body = ["POST", "PUT"].includes(request.method()) ? request.postDataJSON() : undefined;
      state.categoryRequests.push({ body, method: request.method(), url });
      if (request.method() === "POST") {
        const category = { ...body, categoryNo: `ctg_${String(categoryRows.length + 1).padStart(32, "0")}`, deleted: false, status: "active", version: 1 };
        categoryRows.push(category);
        await route.fulfill({
          json: apiEnvelope({
            category,
          }),
        });
        return;
      }
      const parts = url.pathname.split("/");
      const restoring = parts.at(-1) === "restore";
      const row = categoryRows.find(item => item.categoryNo === parts.at(restoring ? -2 : -1));
      if (row && request.method() !== "GET") {
        if (request.method() === "DELETE") row.deleted = true;
        else if (restoring) row.deleted = false;
        else Object.assign(row, body);
        row.version++;
        await route.fulfill({ json: apiEnvelope({ category: row }) });
        return;
      }
      const deletionState = url.searchParams.get("deletionState") ?? "active";
      await route.fulfill({ json: apiEnvelope({ categories: categoryRows.filter(item => deletionState === "all" || item.deleted === (deletionState === "deleted")) }) });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/publication-preview",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.previewRequests.push(url);
      const ready = options.publicationReady !== false;
      await route.fulfill({
        json: apiEnvelope({
          checks: [
            "lifecycle",
            "category",
            "sales_copy",
            "pricing",
            "tax",
          ].map((code) => ({ code, passed: ready || code !== "pricing" })),
          productNo: products[2]!.productNo,
          ready,
          versionNo: products[2]!.versionNo,
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/publish",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.publishRequests.push({ method: route.request().method(), url });
      await route.fulfill({
        json: apiEnvelope({
          productNo: products[2]!.productNo,
          publishedAt: 1_786_166_400_000,
          status: "published",
          supersededVersionNo: "",
          versionNo: products[2]!.versionNo,
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/products**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const marker = "/admin/catalog/products/";
      if (url.pathname.includes(marker)) {
        state.detailRequests.push(url);
        const productNo = decodeURIComponent(
          url.pathname.split(marker)[1] || "",
        );
        const product =
          products.find((item) => item.productNo === productNo) ?? products[0]!;
        await route.fulfill({
          json: apiEnvelope(
            productDetail(
              product,
              options.fulfillmentBinding ?? null,
              fulfillmentPolicy,
            ),
          ),
        });
        return;
      }
      if (request.method() === "POST") {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.productCreateRequests.push({ body, method: "POST", url });
        createdProduct = {
          ...products[2]!,
          categoryNameEnUs: "Professional services",
          categoryNameZhCn: "专业服务",
          categoryNo: String(body.categoryNo),
          nameEnUs: String(body.nameEnUs),
          nameZhCn: String(body.nameZhCn),
          productNo: "product-created",
          publicSlug: String(body.publicSlug),
          summaryEnUs: String(body.summaryEnUs),
          summaryZhCn: String(body.summaryZhCn),
          versionNo: "version-created-1",
        };
        await route.fulfill({
          json: apiEnvelope({
            categoryNo: body.categoryNo,
            productNo: createdProduct.productNo,
            source: "manual",
            status: "draft",
            versionNo: createdProduct.versionNo,
            versionNumber: 1,
            versionStatus: "draft",
          }),
        });
        return;
      }
      state.listRequests.push(url);
      const namePrefix = url.searchParams
        .get("namePrefix")
        ?.toLocaleLowerCase();
      const items = [...products, ...(createdProduct ? [createdProduct] : [])];
      const filtered = namePrefix ? items.filter((item) => `${item.nameZhCn} ${item.nameEnUs}`.toLocaleLowerCase().includes(namePrefix)) : items;
      const size = Number(url.searchParams.get("limit") || 10);
      const offset = (Number(url.searchParams.get("page") || 1) - 1) * size;
      await route.fulfill({
        json: apiEnvelope({
          nextCursor: "",
          products: filtered.slice(offset, offset + size),
          total: filtered.length,
        }),
      });
    },
  );
  return state;
}
