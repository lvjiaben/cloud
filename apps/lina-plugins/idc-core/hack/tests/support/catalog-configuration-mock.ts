import type { Page, Route } from "@host-tests/support/playwright";

const label = (zhCN: string, enUS: string) => ({ enUS, zhCN });

const interfaceDeclaration = {
  conditions: [
    {
      actions: [
        { action: "show", field: "notes" },
        { action: "require", field: "notes" },
      ],
      when: { field: "plan", operator: "equal", value: "advanced" },
    },
    {
      actions: [{ action: "hide", field: "account" }],
      when: { field: "enabled", operator: "equal", value: "true" },
    },
    {
      actions: [
        {
          action: "filter_candidates",
          field: "plan",
          values: ["advanced"],
        },
      ],
      when: { field: "quantity", operator: "greater_than", value: "10" },
    },
  ],
  fields: [
    {
      key: "account",
      label: label("接口账号", "Interface account"),
      required: true,
      type: "single_line",
    },
    {
      key: "notes",
      label: label("高级说明", "Advanced notes"),
      maximumLength: 200,
      type: "multi_line",
    },
    {
      key: "quantity",
      label: label("整数数量", "Integer quantity"),
      maximum: "9223372036854775807",
      minimum: "0",
      type: "integer",
    },
    {
      key: "threshold",
      label: label("精确小数", "Exact decimal"),
      type: "decimal",
    },
    {
      key: "enabled",
      label: label("启用高级模式", "Enable advanced mode"),
      type: "boolean",
    },
    {
      key: "plan",
      label: label("单选方案", "Single-select plan"),
      staticCandidates: [
        { label: label("基础方案", "Basic plan"), value: "basic" },
        { label: label("高级方案", "Advanced plan"), value: "advanced" },
      ],
      type: "single_select",
    },
    {
      dynamicCandidates: {
        method: "configuration.candidates",
        version: "v1",
      },
      key: "addons",
      label: label("动态多选项", "Dynamic multi-select"),
      type: "multi_select",
    },
    { key: "startDate", label: label("开始日期", "Start date"), type: "date" },
    {
      key: "startAt",
      label: label("开始时间", "Start time"),
      type: "datetime",
    },
    {
      key: "credential",
      label: label("接口密钥", "Interface secret"),
      required: true,
      type: "secret",
    },
    {
      key: "zone",
      label: label("区域树", "Zone tree"),
      staticCandidates: [
        { label: label("中国", "China"), value: "cn" },
        {
          label: label("华东", "East China"),
          parentValue: "cn",
          value: "cn-east",
        },
      ],
      type: "tree_select",
    },
    {
      key: "address",
      label: label("联系地址", "Contact address"),
      staticCandidates: [
        { label: label("浙江省", "Zhejiang"), value: "zhejiang" },
        {
          label: label("杭州市", "Hangzhou"),
          parentValue: "zhejiang",
          value: "hangzhou",
        },
      ],
      type: "address",
    },
    {
      key: "files",
      label: label("资质文件", "Qualification files"),
      type: "file",
      uploadPolicy: "governed-interface-file-v1",
    },
  ],
  uploadPolicies: [
    {
      id: "governed-interface-file-v1",
      maximumFiles: 1,
      maximumFileBytes: 1048576,
      maximumTotalBytes: 2097152,
      allowedMimeTypes: ["application/pdf"],
      allowedExtensions: [".pdf"],
      safetyScanRequired: true,
    },
  ],
  version: "interface-v1",
};

const productDeclaration = {
  conditions: [
    {
      actions: [
        { action: "show", field: "description" },
        { action: "require", field: "description" },
      ],
      when: { field: "plan", operator: "equal", value: "advanced" },
    },
    {
      actions: [
        {
          action: "filter_candidates",
          field: "region",
          values: ["north"],
        },
      ],
      when: { field: "plan", operator: "equal", value: "advanced" },
    },
  ],
  fields: [
    {
      key: "quantity",
      label: label("商品整数", "Product integer"),
      maximum: "9223372036854775807",
      minimum: "0",
      required: true,
      type: "integer",
    },
    {
      key: "amount",
      label: label("商品精确小数", "Product exact decimal"),
      required: true,
      type: "decimal",
    },
    {
      key: "plan",
      label: label("商品方案", "Product plan"),
      staticCandidates: [
        { label: label("基础方案", "Basic plan"), value: "basic" },
        { label: label("高级方案", "Advanced plan"), value: "advanced" },
      ],
      type: "single_select",
    },
    {
      key: "region",
      label: label("可售区域", "Available region"),
      staticCandidates: [
        { label: label("北区", "North"), value: "north" },
        { label: label("南区", "South"), value: "south" },
      ],
      type: "single_select",
    },
    {
      key: "description",
      label: label("高级商品说明", "Advanced product notes"),
      type: "multi_line",
    },
    {
      key: "files",
      label: label("商品资质", "Product qualification"),
      type: "file",
      uploadPolicy: "governed-product-file-v1",
    },
  ],
  uploadPolicies: [
    {
      id: "governed-product-file-v1",
      maximumFiles: 1,
      maximumFileBytes: 1048576,
      maximumTotalBytes: 1048576,
      allowedMimeTypes: ["application/pdf"],
      allowedExtensions: [".pdf"],
      safetyScanRequired: true,
    },
  ],
  version: "product-v1",
};

const customerDeclaration = {
  fields: [
    {
      key: "domain",
      label: label("证书域名", "Certificate domain"),
      required: true,
      textFormat: "domain",
      type: "single_line",
    },
    {
      key: "certificateType",
      label: label("证书类型", "Certificate type"),
      staticCandidates: [
        { label: label("普通域名", "Single domain"), value: "single" },
        { label: label("通配符", "Wildcard"), value: "wildcard" },
      ],
      type: "single_select",
    },
  ],
  version: "customer-v1",
};

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export interface CatalogConfigurationMockState {
  interfaceEvaluationRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    pathname: string;
  }>;
  interfaceConfigurationRequests: Array<{
    body?: unknown;
    method: string;
    pathname: string;
  }>;
  productConfigurationRequests: Array<{
    body?: unknown;
    method: string;
    pathname: string;
  }>;
  productEvaluationRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    pathname: string;
  }>;
  fileUploadRequests: Array<{ method: string; pathname: string }>;
}

export async function installCatalogConfigurationMock(page: Page) {
  const state: CatalogConfigurationMockState = {
    interfaceEvaluationRequests: [],
    interfaceConfigurationRequests: [],
    productConfigurationRequests: [],
    productEvaluationRequests: [],
    fileUploadRequests: [],
  };
  let interfaceRevision = 2;
  let fileSequence = 0;
  let reservedFileToken = "";
  let interfaceValues: Array<Record<string, unknown>> = [
    { fieldKey: "account", textValue: "merchant-account" },
    { fieldKey: "quantity", integerValue: "7" },
    { decimalValue: "9007199254740993.02", fieldKey: "threshold" },
    { booleanValue: false, fieldKey: "enabled" },
    { fieldKey: "plan", optionTokens: ["basic"] },
    { fieldKey: "addons", optionTokens: ["opaque-existing-addon"] },
    { dateValue: "2026-08-26", fieldKey: "startDate" },
    { dateTimeValue: "2026-08-26T08:00:00Z", fieldKey: "startAt" },
    { fieldKey: "zone", treeOptionTokens: ["cn-east"] },
    {
      addressOptionTokens: ["zhejiang", "hangzhou"],
      fieldKey: "address",
    },
    { fieldKey: "files", fileTokens: ["opaque-existing-file"] },
  ];
  let productVersion = 0;
  let merchantValues: Array<Record<string, unknown>> = [];

  const valueRow = (rows: Array<Record<string, unknown>>, fieldKey: string) =>
    rows.find((row) => row.fieldKey === fieldKey);
  const optionValue = (
    rows: Array<Record<string, unknown>>,
    fieldKey: string,
  ) => (valueRow(rows, fieldKey)?.optionTokens as string[] | undefined)?.[0];
  const textValue = (rows: Array<Record<string, unknown>>, fieldKey: string) =>
    valueRow(rows, fieldKey)?.textValue as string | undefined;
  const integerValue = (
    rows: Array<Record<string, unknown>>,
    fieldKey: string,
  ) => valueRow(rows, fieldKey)?.integerValue as string | undefined;
  const booleanValue = (
    rows: Array<Record<string, unknown>>,
    fieldKey: string,
  ) => valueRow(rows, fieldKey)?.booleanValue === true;

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/automation/interfaces/*/configuration/evaluation",
    async (route: Route) => {
      const body = route.request().postDataJSON() as {
        expectedRevision: number;
        values: Array<Record<string, unknown>>;
      };
      state.interfaceEvaluationRequests.push({
        body,
        method: route.request().method(),
        pathname: new URL(route.request().url()).pathname,
      });
      const advanced = optionValue(body.values, "plan") === "advanced";
      const enabled = booleanValue(body.values, "enabled");
      const quantity = BigInt(integerValue(body.values, "quantity") || "0");
      const fieldStates = Object.fromEntries(
        interfaceDeclaration.fields.map((field) => [
          field.key,
          {
            disabled: false,
            required:
              field.required === true || (field.key === "notes" && advanced),
            visible:
              !(field.key === "notes" && !advanced) &&
              !(field.key === "account" && enabled),
          },
        ]),
      );
      const issues = [] as Array<{ fieldKey: string; issueCode: string }>;
      if (!enabled && !textValue(body.values, "account"))
        issues.push({ fieldKey: "account", issueCode: "required" });
      if (advanced && !textValue(body.values, "notes"))
        issues.push({ fieldKey: "notes", issueCode: "required" });
      const normalizedValues = advanced
        ? body.values.flatMap((row) => {
            if (row.fieldKey !== "addons") return [row];
            const allowed = (
              (row.optionTokens as string[] | undefined) ?? []
            ).filter((token) => token === "opaque-audit-addon");
            return allowed.length ? [{ ...row, optionTokens: allowed }] : [];
          })
        : body.values;
      await route.fulfill({
        json: apiEnvelope({
          evaluation: {
            candidates: {
              addons: advanced
                ? [
                    {
                      label: label("日志审计", "Audit logs"),
                      value: "opaque-audit-addon",
                    },
                  ]
                : [
                    {
                      label: label("基础监控", "Basic monitoring"),
                      value: "opaque-existing-addon",
                    },
                  ],
              plan:
                quantity > 10n
                  ? [
                      {
                        label: label("高级方案", "Advanced plan"),
                        value: "advanced",
                      },
                    ]
                  : interfaceDeclaration.fields.find(
                      (field) => field.key === "plan",
                    )?.staticCandidates,
            },
            fieldStates,
            issues,
            valid: issues.length === 0,
            values: normalizedValues,
          },
          revision: interfaceRevision,
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/**/configuration/file-uploads**",
    async (route: Route) => {
      const request = route.request();
      const pathname = new URL(request.url()).pathname;
      state.fileUploadRequests.push({ method: request.method(), pathname });
      if (request.method() === "POST") {
        fileSequence += 1;
        reservedFileToken = `MCF_${String(fileSequence).padStart(32, "0")}`;
      }
      const fileToken =
        pathname.match(/MCF_[0-9A-F]{32}/u)?.[0] || reservedFileToken;
      const item = {
        expiresAt: Date.now() + 604_800_000,
        fileName: "qualification.pdf",
        fileToken,
        mediaType: "application/pdf",
        sizeBytes: 24,
        status: "ready",
      };
      if (request.method() === "POST") {
        return route.fulfill({
          json: apiEnvelope({
            ...item,
            expiresAt: Date.now() + 3_600_000,
            mediaType: "",
            status: "reserved",
          }),
        });
      }
      if (request.method() === "DELETE") {
        return route.fulfill({
          json: apiEnvelope({
            ...item,
            expiresAt: Date.now(),
            status: "revoked",
          }),
        });
      }
      return route.fulfill({ json: apiEnvelope(item) });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/configuration/evaluation",
    async (route: Route) => {
      const body = route.request().postDataJSON() as {
        expectedVersion: number;
        merchantValues: Array<Record<string, unknown>>;
      };
      state.productEvaluationRequests.push({
        body,
        method: route.request().method(),
        pathname: new URL(route.request().url()).pathname,
      });
      const advanced = optionValue(body.merchantValues, "plan") === "advanced";
      const issues = [] as Array<{ fieldKey: string; issueCode: string }>;
      for (const fieldKey of ["quantity", "amount"]) {
        if (!valueRow(body.merchantValues, fieldKey))
          issues.push({ fieldKey, issueCode: "required" });
      }
      if (advanced && !textValue(body.merchantValues, "description"))
        issues.push({ fieldKey: "description", issueCode: "required" });
      await route.fulfill({
        json: apiEnvelope({
          evaluation: {
            candidates: {
              plan: productDeclaration.fields.find(
                (field) => field.key === "plan",
              )?.staticCandidates,
              region: advanced
                ? [
                    {
                      label: label("北区", "North"),
                      value: "north",
                    },
                  ]
                : productDeclaration.fields.find(
                    (field) => field.key === "region",
                  )?.staticCandidates,
            },
            fieldStates: Object.fromEntries(
              productDeclaration.fields.map((field) => [
                field.key,
                {
                  disabled: false,
                  required:
                    field.required === true ||
                    (field.key === "description" && advanced),
                  visible: !(field.key === "description" && !advanced),
                },
              ]),
            ),
            issues,
            valid: issues.length === 0,
            values: body.merchantValues,
          },
          version: productVersion,
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/automation/providers**",
    async (route: Route) => {
      await route.fulfill({
        json: apiEnvelope({
          items: [
            {
              displayName: "完整配置供应商插件",
              interfaceConfiguration: interfaceDeclaration,
              selectionToken: "configuration-provider-token",
            },
          ],
          total: 1,
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/automation/interfaces/*/configuration",
    async (route: Route) => {
      const request = route.request();
      const body =
        request.method() === "PUT" ? request.postDataJSON() : undefined;
      const pathname = new URL(request.url()).pathname;
      state.interfaceConfigurationRequests.push({
        body,
        method: request.method(),
        pathname,
      });
      if (request.method() === "PUT") {
        const input = body as {
          expectedRevision: number;
          values: Array<Record<string, unknown>>;
        };
        if (input.expectedRevision !== interfaceRevision) {
          await route.fulfill({ status: 409 });
          return;
        }
        interfaceRevision += 1;
        interfaceValues = input.values;
      }
      await route.fulfill({
        json: apiEnvelope({
          configuration: {
            declaration: interfaceDeclaration,
            revision: interfaceRevision,
            resolvedCandidates: {
              addons: [
                {
                  label: label("基础监控", "Basic monitoring"),
                  value: "opaque-existing-addon",
                },
              ],
            },
            secrets: [
              {
                configured: true,
                fieldKey: "credential",
                maskedTail: "1234",
                updatedAt: 1_787_705_720_000,
              },
            ],
            values: interfaceValues,
          },
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/configuration",
    async (route: Route) => {
      const request = route.request();
      const body =
        request.method() === "PUT" ? request.postDataJSON() : undefined;
      const pathname = new URL(request.url()).pathname;
      state.productConfigurationRequests.push({
        body,
        method: request.method(),
        pathname,
      });
      if (request.method() === "PUT") {
        const input = body as {
          expectedVersion: number;
          merchantValues: Array<Record<string, unknown>>;
        };
        if (input.expectedVersion !== productVersion) {
          await route.fulfill({ status: 409 });
          return;
        }
        merchantValues = input.merchantValues;
        productVersion += 1;
      }
      await route.fulfill({
        json: apiEnvelope({
          configuration: {
            customerDeclaration,
            customerResolvedCandidates: {},
            merchantValues,
            productDeclaration,
            productResolvedCandidates: {},
            version: productVersion,
          },
        }),
      });
    },
  );

  return state;
}
