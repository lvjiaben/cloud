// This fixture isolates the public website from local catalog data while preserving exact HTTP shapes.
import type { Page } from "@playwright/test";

const product = {
  availability: "manual_confirmation",
  categoryNameEnUS: "Smart devices",
  categoryNameZhCN: "智能设备",
  categoryNo: "ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  currencies: ["CNY", "USD"],
  nameEnUS: "Everyday smartphone package",
  nameZhCN: "日常手机套餐",
  pricingPresentation: "base_price",
  publicSlug: "everyday-phone",
  summaryEnUS: "A smartphone package with clear model and delivery choices.",
  summaryZhCN: "提供清楚型号与配送选项的手机套餐。",
};

const sslProduct = {
  availability: "available",
  categoryNameEnUS: "Certificates",
  categoryNameZhCN: "证书服务",
  categoryNo: "ctg_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  currencies: ["CNY"],
  nameEnUS: "Managed SSL certificate",
  nameZhCN: "托管 SSL 证书",
  pricingPresentation: "base_price",
  publicSlug: "managed-ssl",
  summaryEnUS: "A managed certificate with domain validation guidance.",
  summaryZhCN: "包含域名验证指引的托管证书服务。",
};

const sslConfiguration = {
  configurationRevision: "opaque_ssl_configuration_revision",
  fields: [
    {
      descriptionEnUS: "Enter the domain protected by this certificate.",
      descriptionZhCN: "填写需要证书保护的域名。",
      disabled: false,
      fieldToken: "opaque_domain_field",
      format: "domain",
      labelEnUS: "Certificate domain",
      labelZhCN: "证书域名",
      maximumLength: 253,
      required: true,
      type: "single_line",
      visible: true,
    },
    {
      descriptionEnUS: "Supporting documents are uploaded after sign-in.",
      descriptionZhCN: "证明材料将在登录后上传。",
      disabled: true,
      fieldToken: "opaque_supporting_file_field",
      labelEnUS: "Supporting documents",
      labelZhCN: "证明材料",
      required: false,
      type: "file",
      uploadPolicy: {
        allowedExtensions: [".pdf", ".png"],
        allowedMimeTypes: ["application/pdf", "image/png"],
        maximumFileBytes: 5_242_880,
        maximumFiles: 3,
        maximumTotalBytes: 10_485_760,
        safetyScanRequired: true,
      },
      visible: true,
    },
    {
      descriptionEnUS: "Choose the nearest validation service region.",
      descriptionZhCN: "选择就近的验证服务区域。",
      disabled: false,
      fieldToken: "opaque_region_field",
      labelEnUS: "Validation region",
      labelZhCN: "验证区域",
      options: [
        {
          children: [
            {
              labelEnUS: "Singapore",
              labelZhCN: "新加坡",
              optionToken: "opaque_singapore_option",
            },
          ],
          labelEnUS: "Asia Pacific",
          labelZhCN: "亚太",
          optionToken: "opaque_asia_option",
        },
      ],
      required: false,
      type: "tree_select",
      visible: true,
    },
    {
      descriptionEnUS: "Choose the certificate coverage.",
      descriptionZhCN: "选择证书保护范围。",
      disabled: false,
      fieldToken: "opaque_certificate_type_field",
      labelEnUS: "Certificate type",
      labelZhCN: "证书类型",
      options: [
        {
          labelEnUS: "Single domain",
          labelZhCN: "单域名",
          optionToken: "opaque_single_domain_option",
        },
        {
          labelEnUS: "Wildcard domain",
          labelZhCN: "通配符域名",
          optionToken: "opaque_wildcard_option",
        },
      ],
      required: true,
      type: "single_select",
      visible: true,
    },
  ],
  values: [
    {
      fieldToken: "opaque_domain_field",
      textValue: "example.com",
      type: "single_line",
    },
    {
      fieldToken: "opaque_certificate_type_field",
      optionTokens: ["opaque_single_domain_option"],
      type: "single_select",
    },
  ],
};

const publicCampaign = {
  campaignNo: "mcp_public_starter",
  campaignType: "discount",
  descriptionEnUS: "Save on a new everyday phone package.",
  descriptionZhCN: "新购日常手机套餐可享优惠。",
  endsAt: 1_799_971_200_000,
  nameEnUS: "Everyday phone special",
  nameZhCN: "日常手机套餐活动",
  startsAt: 1_767_225_600_000,
};

export interface PublicCatalogMockState {
  configurationEvaluations: Array<Record<string, unknown>>;
  configurationQuotes: Array<Record<string, unknown>>;
  exposureAttempts: Array<{
    campaignNo: string;
    currencyCode: string;
    deviceId: string;
    recorded: boolean;
  }>;
  recordedExposureCount: number;
}

export async function mockPublicCatalog(
  page: Page,
): Promise<PublicCatalogMockState> {
  const exposureKeys = new Set<string>();
  const state: PublicCatalogMockState = {
    configurationEvaluations: [],
    configurationQuotes: [],
    exposureAttempts: [],
    recordedExposureCount: 0,
  };
  await page.route(
    "**/x/idc-core/api/v1/public/catalog/products**",
    async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/managed-ssl/purchase-configuration")) {
        await route.fulfill({
          contentType: "application/json",
          status: 200,
          body: JSON.stringify({
            code: 0,
            message: "OK",
            data: sslConfiguration,
          }),
        });
        return;
      }
      if (
        url.pathname.endsWith("/managed-ssl/purchase-configuration/evaluate")
      ) {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        state.configurationEvaluations.push(body);
        const submittedValues = (body.values ?? []) as Array<{
          fieldToken?: string;
          optionTokens?: string[];
        }>;
        const wildcard = submittedValues.some(
          (value) =>
            value.fieldToken === "opaque_certificate_type_field" &&
            value.optionTokens?.includes("opaque_wildcard_option"),
        );
        const evaluatedFields = sslConfiguration.fields.map((field) =>
          field.fieldToken === "opaque_region_field"
            ? {
                ...field,
                options: wildcard
                  ? [
                      {
                        children: [
                          {
                            labelEnUS: "Frankfurt",
                            labelZhCN: "法兰克福",
                            optionToken: "opaque_frankfurt_option",
                          },
                        ],
                        labelEnUS: "Europe",
                        labelZhCN: "欧洲",
                        optionToken: "opaque_europe_option",
                      },
                    ]
                  : field.options,
              }
            : field,
        );
        await route.fulfill({
          contentType: "application/json",
          status: 200,
          body: JSON.stringify({
            code: 0,
            message: "OK",
            data: {
              ...sslConfiguration,
              fields: evaluatedFields,
              values: body.values,
              issues: [],
              valid: true,
            },
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/managed-ssl/purchase-configuration/quote")) {
        const body = route.request().postDataJSON() as {
          values?: Array<{ fieldToken?: string; textValue?: string }>;
        };
        state.configurationQuotes.push(body);
        const domain = body.values?.find(
          (value) => value.fieldToken === "opaque_domain_field",
        )?.textValue;
        if (domain === "example.com") {
          await new Promise((resolve) => setTimeout(resolve, 400));
        }
        await route.fulfill({
          contentType: "application/json",
          status: 200,
          body: JSON.stringify({
            code: 0,
            message: "OK",
            data: {
              configurationRevision: sslConfiguration.configurationRevision,
              currencyCode: "CNY",
              terms: [
                {
                  billingTermToken: "opaque_yearly_term",
                  labelEnUS: "One year",
                  labelZhCN: "一年",
                  minimumPurchaseCycles: 1,
                  setupAmount: "0",
                  status: "final",
                  subtotalAmount: "9800",
                  taxAmount: "0",
                  totalAmount: domain === "example.com" ? "9800" : "9900",
                },
              ],
            },
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/campaigns")) {
        await route.fulfill({
          contentType: "application/json",
          status: 200,
          body: JSON.stringify({
            code: 0,
            message: "OK",
            data: { campaigns: [publicCampaign] },
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/campaign-exposures")) {
        const body = route.request().postDataJSON() as {
          campaignNo: string;
          currencyCode: string;
          deviceId: string;
        };
        const key = `${body.deviceId}:${body.campaignNo}:${body.currencyCode}`;
        const recorded = !exposureKeys.has(key);
        exposureKeys.add(key);
        if (recorded) state.recordedExposureCount += 1;
        state.exposureAttempts.push({ ...body, recorded });
        await route.fulfill({
          contentType: "application/json",
          status: 200,
          body: JSON.stringify({ code: 0, message: "OK", data: { recorded } }),
        });
        return;
      }
      const isPrimaryDetail = url.pathname.endsWith("/everyday-phone");
      const isSSLDetail = url.pathname.endsWith("/managed-ssl");
      await route.fulfill({
        contentType: "application/json",
        status: 200,
        body: JSON.stringify({
          code: 0,
          message: "OK",
          data:
            isPrimaryDetail || isSSLDetail
              ? { product: isSSLDetail ? sslProduct : product }
              : {
                  availableCurrencies: ["CNY", "USD"],
                  categories: [
                    {
                      categoryNo: product.categoryNo,
                      nameEnUS: product.categoryNameEnUS,
                      nameZhCN: product.categoryNameZhCN,
                    },
                  ],
                  nextCursor: "",
                  products: [product, sslProduct],
                },
        }),
      });
    },
  );
  return state;
}
