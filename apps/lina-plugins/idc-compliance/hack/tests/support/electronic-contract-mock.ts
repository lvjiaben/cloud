import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";
import { numberedMockPage } from "./numbered-page-mock";

const catalogs = {
  "en-US": JSON.parse(
    readFileSync(
      new URL(
        "../../../manifest/i18n/en-US/electronic-contract.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ),
  "zh-CN": JSON.parse(
    readFileSync(
      new URL(
        "../../../manifest/i18n/zh-CN/electronic-contract.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ),
} as Record<string, Record<string, string>>;

const onboardingURL =
  "**/x/idc-compliance/api/v1/admin/compliance/electronic-contract/onboarding";
const templateListURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates(?:\?.*)?$/u;
const templateEditorURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/([^/]+)\/versions\/(\d+)\/editor(?:\?.*)?$/u;
const templateCopyURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/([^/]+)\/versions\/(\d+)\/copies$/u;
const templateEnablementURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/([^/]+)\/versions\/(\d+)\/enablements$/u;
const templateProviderMappingURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/([^/]+)\/versions\/(\d+)\/provider-mappings$/u;
const templatePreviewURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/contract-template-local\/versions\/1\/content-revisions\/content-revision-\d+\/preview(?:\?.*)?$/u;
const templateOnlineSourceURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/contract-template-local\/versions\/1\/online-source$/u;
const templatePreviewCreateURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/contract-template-local\/versions\/1\/pdf-previews$/u;
const templatePreviewConfirmationURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/contract-template-local\/versions\/1\/preview-confirmations$/u;
const templateFieldSetURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/contract-template-local\/versions\/1\/field-sets$/u;
const templateSyncURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/templates\/tencent-syncs$/u;
const autoSignListURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/auto-sign-authorizations(?:\?.*)?$/u;
const autoSignRevocationURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/auto-sign-authorizations\/([^/]+)\/revocation$/u;
const contractListURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts(?:\?.*)?$/u;
const contractDetailURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/?]+)$/u;
const contractTemplateCandidateURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/template-candidates(?:\?.*)?$/u;
const contractSignerCandidateURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/customers\/([^/]+)\/contract-signer-candidates$/u;
const contractSubmissionURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/signing-submissions$/u;
const contractReissueURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/reissues$/u;
const contractTerminationURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/terminations$/u;
const contractReplacementOptionsURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/replacement-options$/u;
const contractReplacementPreviewURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/replacement-previews$/u;
const contractFailurePolicyURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contract\/failure-policy$/u;
const contractFailureCancellationURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/electronic-contracts\/([^/]+)\/order-cancellation$/u;

type State = "active" | "not_configured" | "ready";

interface OnboardingProjection {
  bindingNo: string;
  capabilities: Array<Record<string, unknown>>;
  consoleUrl?: string;
  merchant: {
    countryCode: string;
    legalName: string;
    merchantNo: string;
  };
  operatorReady: boolean;
  organizationReady: boolean;
  reasonCode: string;
  state: State | "enterprise_pending";
  subjectStatus: string;
}

interface ContractTemplateProjection {
  languageCode: "" | "en-US" | "zh-CN";
  lockVersion: number;
  name: string;
  originCode: "local" | "tencent_synced";
  providerState?: "active" | "disabled" | "offline";
  providerVersion?: string;
  scopeCode: "" | "all_customers" | "enterprise" | "personal";
  signingOrderCode: "" | "customer_first" | "merchant_first" | "parallel";
  statusCode: string;
  templateNo: string;
  updatedAt: number;
  usedCount: number;
  versionNo: number;
}

export interface ElectronicContractMockState {
  autoSignCreateRequests: Array<Record<string, unknown>>;
  autoSignListRequests: string[];
  autoSignRevokeRequests: Array<Record<string, unknown>>;
  contractCreateRequests: Array<Record<string, unknown>>;
  contractListRequests: string[];
  contractReplacementPreviewRequests: Array<Record<string, unknown>>;
  contractReissueRequests: Array<Record<string, unknown>>;
  contractSubmissionRequests: Array<Record<string, unknown>>;
  contractTerminationRequests: Array<Record<string, unknown>>;
  contractFailureCancellationRequests: Array<Record<string, unknown>>;
  contractFailurePolicyRequests: Array<Record<string, unknown>>;
  linkRequests: Array<Record<string, unknown>>;
  previewRequests: string[];
  recheckRequests: Array<Record<string, unknown>>;
  templateCopyRequests: Array<Record<string, unknown>>;
  templateEnablementRequests: Array<Record<string, unknown>>;
  templateEditorRequests: string[];
  templateFieldSetRequests: Array<Record<string, unknown>>;
  templateListRequests: string[];
  templateOnlineSourceRequests: Array<Record<string, unknown>>;
  templateProviderMappingRequests: Array<Record<string, unknown>>;
  templatePreviewConfirmationRequests: Array<Record<string, unknown>>;
  templatePreviewCreateRequests: Array<Record<string, unknown>>;
  templateSyncRequests: string[];
}

export async function setElectronicContractPresentation(
  page: Page,
  locale: "en-US" | "zh-CN",
  theme: "dark" | "light",
) {
  await page.addInitScript(
    ({ locale: nextLocale, theme: nextTheme }) => {
      for (const key of Object.keys(localStorage)) {
        if (key.endsWith("preferences-locale")) {
          localStorage.setItem(key, JSON.stringify({ value: nextLocale }));
        } else if (key.endsWith("preferences-theme")) {
          localStorage.setItem(key, JSON.stringify({ value: nextTheme }));
        } else if (key.endsWith("preferences")) {
          const current = JSON.parse(localStorage.getItem(key) || "{}");
          if (current.value?.app) current.value.app.locale = nextLocale;
          if (current.value?.theme) current.value.theme.mode = nextTheme;
          localStorage.setItem(key, JSON.stringify(current));
        }
      }
    },
    { locale, theme },
  );
}

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const parts = key.split(".");
    let cursor = root;
    for (const part of parts.slice(0, -1)) {
      if (!cursor[part] || typeof cursor[part] !== "object") cursor[part] = {};
      cursor = cursor[part] as Record<string, unknown>;
    }
    cursor[parts.at(-1)!] = value;
  }
  return root;
}

function onePagePDF() {
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>",
    "<< /Length 0 >>\nstream\n\nendstream",
  ];
  let body = "%PDF-1.4\n";
  const offsets = [0];
  for (const [index, object] of objects.entries()) {
    offsets.push(Buffer.byteLength(body, "ascii"));
    body += `${index + 1} 0 obj\n${object}\nendobj\n`;
  }
  const xrefOffset = Buffer.byteLength(body, "ascii");
  body += `xref\n0 ${objects.length + 1}\n`;
  body += "0000000000 65535 f \n";
  body += offsets
    .slice(1)
    .map((offset) => `${String(offset).padStart(10, "0")} 00000 n \n`)
    .join("");
  body += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\n`;
  body += `startxref\n${xrefOffset}\n%%EOF\n`;
  return Buffer.from(body, "ascii");
}

function projection(state: State): OnboardingProjection {
  const active = state === "active";
  return {
    bindingNo: active ? "lsb_0123456789abcdef0123456789abcdef" : "",
    capabilities: active
      ? [
          {
            checkedAt: 1_786_161_660_000,
            code: "flow.download",
            reasonCode: "",
            state: "available",
          },
          {
            checkedAt: 1_786_161_660_000,
            code: "auto_sign.enterprise",
            reasonCode: "",
            state: "disabled",
          },
        ]
      : [],
    merchant: {
      countryCode: "CN",
      legalName: "示例 IDC 有限公司",
      merchantNo: "merchant-local",
    },
    operatorReady: active,
    organizationReady: active,
    reasonCode: state === "not_configured" ? "tencent_app_not_configured" : "",
    state,
    subjectStatus: active ? "active" : "unbound",
  };
}

export async function installElectronicContractMock(
  page: Page,
  initialState: State = "ready",
  options: {
    locale?: "en-US" | "zh-CN";
    manageAutoSign?: boolean;
    manageContracts?: boolean;
    manageTemplates?: boolean;
    directoryRows?: number;
  } = {},
): Promise<ElectronicContractMockState> {
  const state: ElectronicContractMockState = {
    autoSignCreateRequests: [],
    autoSignListRequests: [],
    autoSignRevokeRequests: [],
    contractCreateRequests: [],
    contractListRequests: [],
    contractReplacementPreviewRequests: [],
    contractReissueRequests: [],
    contractSubmissionRequests: [],
    contractTerminationRequests: [],
    contractFailureCancellationRequests: [],
    contractFailurePolicyRequests: [],
    linkRequests: [],
    previewRequests: [],
    recheckRequests: [],
    templateCopyRequests: [],
    templateEnablementRequests: [],
    templateEditorRequests: [],
    templateFieldSetRequests: [],
    templateListRequests: [],
    templateOnlineSourceRequests: [],
    templateProviderMappingRequests: [],
    templatePreviewConfirmationRequests: [],
    templatePreviewCreateRequests: [],
    templateSyncRequests: [],
  };
  let current = projection(initialState);

  await page.route("**/api/v1/config/public/frontend", async (route: Route) => {
    await route.fulfill({
      json: apiEnvelope({ workspace: { basePath: "/admin" } }),
    });
  });
  await page.route("**/api/v1/user/message/count", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ count: 0 }) });
  });
  await page.route("**/api/v1/user/message?**", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: [], total: 0 }) });
  });

  await page.route("**/api/v1/user/info", async (route: Route) => {
    await route.fulfill({
      json: apiEnvelope({
        avatar: "",
        homePath: "/idc/compliance/electronic-contract",
        menus: [],
        permissions: [
          ...(options.manageAutoSign === false
            ? []
            : ["idc:admin:compliance:contract-auto-sign:manage"]),
          ...(options.manageTemplates === false
            ? []
            : ["idc:admin:compliance:contract-template:manage"]),
          ...(options.manageContracts === false
            ? []
            : [
                "idc:admin:compliance:contract:query",
                "idc:admin:compliance:contract:create",
                "idc:admin:compliance:contract:reissue",
                "idc:admin:compliance:contract:terminate",
                "idc:admin:compliance:contract:cancel-order",
                "idc:admin:compliance:contract:failure-policy",
              ]),
          "idc:admin:compliance:contract-template:query",
          "idc:admin:compliance:supplier:query",
          "idc:admin:compliance:supplier:subject:bind",
        ],
        realName: "Admin",
        roles: ["admin"],
        userId: 1,
        username: "admin",
      }),
    });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    const locale =
      options.locale ??
      (route.request().headers()["accept-language"]?.includes("en")
        ? "en-US"
        : "zh-CN");
    await route.fulfill({
      json: apiEnvelope({
        enabled: true,
        items: [
          {
            direction: "ltr",
            isDefault: true,
            locale: "zh-CN",
            name: "Simplified Chinese",
            nativeName: "简体中文",
          },
          {
            direction: "ltr",
            isDefault: false,
            locale: "en-US",
            name: "English",
            nativeName: "English",
          },
        ],
        locale,
      }),
    });
  });
  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const locale =
        new URL(route.request().url()).searchParams.get("lang") === "en-US"
          ? "en-US"
          : "zh-CN";
      await route.fulfill({
        json: apiEnvelope({ messages: nestedMessages(catalogs[locale]!) }),
      });
    },
  );
  await page.route("**/api/v1/plugins/dynamic", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: [] }) });
  });
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({
      json: apiEnvelope({
        list: [
          {
            children: [
              {
                component: "system/plugin/dynamic-page",
                meta: {
                  authority: ["idc:admin:compliance:contract-template:query"],
                  title: "plugin.idc-compliance.contract.title",
                },
                name: "IDCComplianceElectronicContract",
                path: "/idc/compliance/electronic-contract",
              },
            ],
            component: "BasicLayout",
            meta: { title: "IDC Compliance" },
            name: "IDCCompliance",
            path: "/idc/compliance",
          },
        ],
      }),
    });
  });

  const template: ContractTemplateProjection = {
    languageCode: "zh-CN",
    lockVersion: 1,
    name: "Merchant service agreement",
    originCode: "local",
    scopeCode: "all_customers",
    signingOrderCode: "customer_first",
    statusCode: "draft",
    templateNo: "contract-template-local",
    updatedAt: 1_786_161_660_000,
    usedCount: 0,
    versionNo: 1,
  };
  const templates: ContractTemplateProjection[] = [
    {
      ...template,
      lockVersion: 2,
      name: "Tencent Disabled Template",
      originCode: "tencent_synced",
      providerState: "disabled",
      scopeCode: "enterprise",
      signingOrderCode: "parallel",
      statusCode: "provider_disabled",
      templateNo: "contract-template-tencent-disabled",
      updatedAt: 1_786_161_680_000,
    },
    {
      ...template,
      lockVersion: 4,
      name: "Tencent Mapping Template",
      originCode: "tencent_synced",
      providerState: "active",
      providerVersion: "20260816001",
      scopeCode: "enterprise",
      signingOrderCode: "parallel",
      statusCode: "needs_configuration",
      templateNo: "contract-template-tencent-mapping",
      updatedAt: 1_786_161_675_000,
    },
    {
      ...template,
      name: "Tencent Settings Required",
      languageCode: "",
      originCode: "tencent_synced",
      providerState: "active",
      scopeCode: "",
      signingOrderCode: "",
      statusCode: "needs_configuration",
      templateNo: "contract-template-tencent-pending",
      updatedAt: 1_786_161_670_000,
    },
    {
      ...template,
      lockVersion: 3,
      languageCode: "en-US",
      name: "Tencent Revised Template",
      originCode: "tencent_synced",
      providerState: "active",
      providerVersion: "20260815002",
      scopeCode: "enterprise",
      signingOrderCode: "parallel",
      statusCode: "disabled",
      templateNo: "contract-template-tencent-revised",
      updatedAt: 1_786_161_665_000,
      versionNo: 2,
    },
    template,
  ];
  const autoSignCandidates = [
    {
      name: "Enterprise service agreement",
      templateNo: "contract-template-enterprise",
      versionNo: 3,
    },
  ];
  const autoSignAuthorizations: Array<Record<string, unknown>> = [];
  if (options.directoryRows) {
    while (templates.length < options.directoryRows) templates.push({ ...template, templateNo: `ctpl_page_${templates.length}`, name: `Paged template ${templates.length}` });
    for (let index = 0; index < options.directoryRows; index++) autoSignAuthorizations.push({ authorizationNo: `casa_page_${index}`, statusCode: "active", createdAt: 1_786_161_600_000, createdByUserId: 1, expiresAt: 1_889_161_600_000, currencyCode: "CNY", maxAmountMinor: "100000", successLimit: 100, successCount: 0, allowManualOptional: true, allowRequiredOrder: true, templates: autoSignCandidates, version: 1 });
  }

  let editorLockVersion = 1;
  let contentRevisionNo = "content-revision-1";
  let contentRevision = 1;
  let conversionStatusCode: "ready" | "source_saved" = "ready";
  let onlineHtml = "<p>Merchant service agreement body</p>";
  let previewConfirmed = false;
  let templateFields: Array<Record<string, unknown>> = [];
  let providerMappingSaved = false;
  const providerComponents = [
    {
      componentRef: "contract-number",
      name: "Contract number",
      required: true,
      typeCode: "TEXT",
      valueCode: "",
    },
    {
      componentRef: "service-details",
      name: "Service details",
      required: true,
      typeCode: "DYNAMIC_TABLE",
      valueCode: "",
    },
    {
      componentRef: "customer-signing-date",
      name: "Customer signing date",
      recipientRef: "customer",
      required: true,
      typeCode: "SIGN_DATE",
      valueCode: "",
    },
    {
      componentRef: "merchant-signing-date",
      name: "Merchant signing date",
      recipientRef: "merchant",
      required: true,
      typeCode: "SIGN_DATE",
      valueCode: "",
    },
  ];
  const providerRecipients = [
    {
      isPromoter: false,
      partyCode: "",
      recipientRef: "customer",
      requireSign: true,
      roleName: "Customer signer",
      routingOrder: 1,
      signComponentCount: 1,
      typeCode: "ENTERPRISE",
    },
    {
      isPromoter: true,
      partyCode: "",
      recipientRef: "merchant",
      requireSign: true,
      roleName: "Merchant signer",
      routingOrder: 1,
      signComponentCount: 1,
      typeCode: "PROMOTER",
    },
  ];

  function editorItem(editorTemplate: ContractTemplateProjection = template) {
    if (editorTemplate.templateNo === "contract-template-tencent-mapping") {
      return {
        enableReady: providerMappingSaved,
        enableReasonCode: providerMappingSaved
          ? ""
          : "provider_mapping_incomplete",
        fields: [],
        onlineHtml: "",
        previewConfirmed: false,
        providerComponents,
        providerMappingSaved,
        providerRecipients,
        submissionAvailable: false,
        submissionReasonCode: "tencent_template_submission_unavailable",
        template: editorTemplate,
      };
    }
    if (editorTemplate.templateNo !== template.templateNo) {
      return {
        enableReady: false,
        enableReasonCode: "preview_missing",
        fields: [],
        onlineHtml: "",
        previewConfirmed: false,
        providerComponents: [],
        providerMappingSaved: false,
        providerRecipients: [],
        submissionAvailable: false,
        submissionReasonCode: "tencent_template_submission_unavailable",
        template: editorTemplate,
      };
    }
    const ready = conversionStatusCode === "ready";
    const fieldsReady = previewConfirmed && templateFields.length >= 2;
    return {
      content: {
        contentRevisionNo,
        conversionStatusCode,
        createdAt: 1_786_161_660_000 + contentRevision,
        ...(ready
          ? {
              pdfByteSize: 1024,
              pdfPageCount: 1,
              pdfSha256:
                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            }
          : {}),
        revisionNo: contentRevision,
        sourceByteSize: onlineHtml.length,
        sourceMediaType: "text/html",
        sourceModeCode: "online_editor",
        sourceSha256:
          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      },
      enableReady: fieldsReady,
      enableReasonCode: fieldsReady
        ? ""
        : ready && previewConfirmed
          ? "signing_positions_incomplete"
          : ready
            ? "preview_not_confirmed"
            : "preview_missing",
      fields: templateFields,
      onlineHtml,
      previewConfirmed,
      providerComponents: [],
      providerMappingSaved: false,
      providerRecipients: [],
      submissionAvailable: false,
      submissionReasonCode: "tencent_template_submission_unavailable",
      template: { ...editorTemplate, lockVersion: editorLockVersion },
    };
  }

  await page.route(templatePreviewURL, async (route: Route) => {
    state.previewRequests.push(route.request().url());
    await route.fulfill({
      body: onePagePDF(),
      contentType: "application/pdf",
      status: 200,
    });
  });

  await page.route(templateEditorURL, async (route: Route) => {
    const requestURL = route.request().url();
    state.templateEditorRequests.push(requestURL);
    const match = new URL(requestURL).pathname.match(templateEditorURL);
    const editorTemplate = templates.find(
      (item) => item.templateNo === decodeURIComponent(match?.[1] ?? ""),
    );
    if (!editorTemplate || editorTemplate.versionNo !== Number(match?.[2])) {
      await route.fulfill({ status: 404 });
      return;
    }
    await route.fulfill({
      json: apiEnvelope({ item: editorItem(editorTemplate) }),
    });
  });

  await page.route(templateProviderMappingURL, async (route: Route) => {
    const requestURL = route.request().url();
    const match = new URL(requestURL).pathname.match(
      templateProviderMappingURL,
    );
    const templateNo = decodeURIComponent(match?.[1] ?? "");
    const versionNo = Number(match?.[2]);
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.templateProviderMappingRequests.push({
      ...body,
      templateNo,
      versionNo,
    });
    const editorTemplate = templates.find(
      (item) => item.templateNo === templateNo && item.versionNo === versionNo,
    );
    if (
      !editorTemplate ||
      templateNo !== "contract-template-tencent-mapping" ||
      body.expectedVersion !== editorTemplate.lockVersion
    ) {
      await route.fulfill({ status: 409 });
      return;
    }
    const fieldMappings = Array.isArray(body.fieldMappings)
      ? (body.fieldMappings as Array<Record<string, unknown>>)
      : [];
    const recipientMappings = Array.isArray(body.recipientMappings)
      ? (body.recipientMappings as Array<Record<string, unknown>>)
      : [];
    for (const component of providerComponents) {
      const mapping = fieldMappings.find(
        (item) => item.componentRef === component.componentRef,
      );
      component.valueCode = String(mapping?.valueCode ?? "");
    }
    for (const recipient of providerRecipients) {
      const mapping = recipientMappings.find(
        (item) => item.recipientRef === recipient.recipientRef,
      );
      recipient.partyCode = String(mapping?.partyCode ?? "");
    }
    providerMappingSaved = true;
    editorTemplate.lockVersion += 1;
    editorTemplate.statusCode = "disabled";
    editorTemplate.updatedAt += 1;
    await route.fulfill({
      json: apiEnvelope({ item: editorItem(editorTemplate) }),
    });
  });

  await page.route(templateCopyURL, async (route: Route) => {
    const requestURL = route.request().url();
    const match = new URL(requestURL).pathname.match(templateCopyURL);
    const sourceTemplateNo = decodeURIComponent(match?.[1] ?? "");
    const sourceVersionNo = Number(match?.[2]);
    const source = templates.find(
      (item) =>
        item.templateNo === sourceTemplateNo &&
        item.versionNo === sourceVersionNo,
    );
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.templateCopyRequests.push({
      ...body,
      sourceTemplateNo,
      sourceVersionNo,
    });
    if (!source) {
      await route.fulfill({ status: 404 });
      return;
    }
    const copied: ContractTemplateProjection = {
      languageCode: source.languageCode,
      lockVersion: 1,
      name: source.name,
      originCode: "local",
      scopeCode: source.scopeCode,
      signingOrderCode: source.signingOrderCode,
      statusCode: "draft",
      templateNo: `contract-template-copy-${state.templateCopyRequests.length}`,
      updatedAt: 1_786_161_700_000 + state.templateCopyRequests.length,
      usedCount: 0,
      versionNo: 1,
    };
    templates.unshift(copied);
    await route.fulfill({ json: apiEnvelope({ item: copied }) });
  });

  await page.route(templateEnablementURL, async (route: Route) => {
    const requestURL = route.request().url();
    const match = new URL(requestURL).pathname.match(templateEnablementURL);
    const templateNo = decodeURIComponent(match?.[1] ?? "");
    const versionNo = Number(match?.[2]);
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.templateEnablementRequests.push({ ...body, templateNo, versionNo });
    const item = templates.find(
      (candidate) =>
        candidate.templateNo === templateNo &&
        candidate.versionNo === versionNo,
    );
    if (!item || body.expectedVersion !== item.lockVersion) {
      await route.fulfill({ status: 409 });
      return;
    }
    item.lockVersion += 1;
    item.statusCode = "enabled";
    item.updatedAt += 1;
    await route.fulfill({ json: apiEnvelope({ item }) });
  });

  await page.route(templateOnlineSourceURL, async (route: Route) => {
    const request = route.request().postDataJSON() as Record<string, unknown>;
    state.templateOnlineSourceRequests.push(request);
    onlineHtml = String(request.html ?? "");
    editorLockVersion += 1;
    contentRevision += 1;
    contentRevisionNo = `content-revision-${contentRevision}`;
    conversionStatusCode = "source_saved";
    previewConfirmed = false;
    templateFields = [];
    await route.fulfill({ json: apiEnvelope({ item: editorItem() }) });
  });

  await page.route(templatePreviewCreateURL, async (route: Route) => {
    state.templatePreviewCreateRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    editorLockVersion += 1;
    contentRevision += 1;
    contentRevisionNo = `content-revision-${contentRevision}`;
    conversionStatusCode = "ready";
    previewConfirmed = false;
    templateFields = [];
    await route.fulfill({ json: apiEnvelope({ item: editorItem() }) });
  });

  await page.route(templatePreviewConfirmationURL, async (route: Route) => {
    state.templatePreviewConfirmationRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    editorLockVersion += 1;
    previewConfirmed = true;
    await route.fulfill({ json: apiEnvelope({ item: editorItem() }) });
  });

  await page.route(templateFieldSetURL, async (route: Route) => {
    const request = route.request().postDataJSON() as Record<string, unknown>;
    state.templateFieldSetRequests.push(request);
    const requestFields = Array.isArray(request.fields) ? request.fields : [];
    templateFields = requestFields.map((field, index) => ({
      ...(field as Record<string, unknown>),
      fieldNo: `field-${index + 1}`,
      sequenceNo: index + 1,
    }));
    editorLockVersion += 1;
    await route.fulfill({ json: apiEnvelope({ item: editorItem() }) });
  });

  await page.route(templateListURL, async (route: Route) => {
    state.templateListRequests.push(route.request().url());
    const query = new URL(route.request().url()).searchParams;
    const name = query.get("name")?.trim().toLocaleLowerCase() ?? "";
    const scope = query.get("scopeCode") ?? "";
    const status = query.get("statusCode") ?? "";
    const filtered = templates.filter(
      (item) =>
        (!name || item.name.toLocaleLowerCase().includes(name)) &&
        (!scope || item.scopeCode === scope) &&
        (!status || item.statusCode === status),
    );
    await route.fulfill({
      json: apiEnvelope({
        ...numberedMockPage(filtered, query),
      }),
    });
  });

  await page.route(templateSyncURL, async (route: Route) => {
    state.templateSyncRequests.push(route.request().url());
    await route.fulfill({
      json: apiEnvelope({
        item: {
          markedOffline: 0,
          newVersions: 1,
          providerTotal: 2,
          syncedAt: 1_786_161_690_000,
        },
      }),
    });
  });

  await page.route(autoSignRevocationURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      autoSignRevocationURL,
    );
    const authorizationNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.autoSignRevokeRequests.push({ ...body, authorizationNo });
    const item = autoSignAuthorizations.find(
      (candidate) => candidate.authorizationNo === authorizationNo,
    );
    if (!item || item.version !== body.expectedVersion) {
      await route.fulfill({ status: 409 });
      return;
    }
    item.statusCode = "revoked";
    item.revokedAt = Date.now();
    item.revokedByUserId = 1;
    item.version = Number(item.version) + 1;
    await route.fulfill({ json: apiEnvelope({ item }) });
  });

  await page.route(autoSignListURL, async (route: Route) => {
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      state.autoSignCreateRequests.push(body);
      const selected = Array.isArray(body.templates)
        ? (body.templates as Array<Record<string, unknown>>)
        : [];
      const item: Record<string, unknown> = {
        ...body,
        authorizationNo: `casa_mock_${state.autoSignCreateRequests.length}`,
        createdAt: Date.now(),
        createdByUserId: 1,
        statusCode: "active",
        successCount: 0,
        templates: selected.map((reference) => ({
          ...reference,
          name:
            autoSignCandidates.find(
              (candidate) =>
                candidate.templateNo === reference.templateNo &&
                candidate.versionNo === reference.versionNo,
            )?.name ?? "Unknown template",
        })),
        version: 1,
      };
      autoSignAuthorizations.unshift(item);
      await route.fulfill({ json: apiEnvelope({ item }) });
      return;
    }
    state.autoSignListRequests.push(route.request().url());
    await route.fulfill({
      json: apiEnvelope({
        candidates: autoSignCandidates,
        ...numberedMockPage(autoSignAuthorizations, new URL(route.request().url()).searchParams),
      }),
    });
  });

  const baseContract = {
    amountMinor: "129900",
    completedAt: 0,
    contractTypeCode: "service_order",
    createdAt: 1_786_161_660_000,
    currencyCode: "CNY",
    customerNo: "CUS_ENTERPRISE_001",
    languageCode: "zh-CN",
    launchedAt: 1_786_161_670_000,
    orderNo: "ORDER_OPTIONAL_001",
    requiredForFulfillment: false,
    signingExpiresAt: 1_786_766_470_000,
    signingOrderCode: "customer_first",
    subjectTypeCode: "enterprise",
    templateName: "Enterprise service agreement",
    templateNo: "contract-template-enterprise",
    templateVersion: 3,
    terminatedAt: 0,
    updatedAt: 1_786_161_680_000,
    version: 3,
  };
  const contracts: Array<Record<string, unknown>> = [
    {
      ...baseContract,
      contractNo: "CTR_REQUIRED_FAILED_001",
      orderNo: "ORDER_REQUIRED_FAILED_001",
      requiredForFulfillment: true,
      statusCode: "failed",
    },
    {
      ...baseContract,
      contractNo: "CTR_REJECTED_001",
      statusCode: "rejected",
    },
    {
      ...baseContract,
      contractNo: "CTR_EXPIRED_001",
      statusCode: "expired",
    },
    {
      ...baseContract,
      contractNo: "CTR_REVOKED_001",
      statusCode: "revoked",
    },
    {
      ...baseContract,
      completedAt: 1_786_161_750_000,
      contractNo: "CTR_COMPLETED_001",
      customerNo: "CUS_PERSONAL_001",
      orderNo: "ORDER_REQUIRED_001",
      requiredForFulfillment: true,
      statusCode: "completed",
      subjectTypeCode: "individual",
      version: 5,
    },
  ];

  if (options.directoryRows) while (contracts.length < options.directoryRows) contracts.push({ ...baseContract, contractNo: `CTR_PAGE_${contracts.length}`, statusCode: "signing" });

  let failurePolicy = {
    createdAt: 1_786_161_600_000,
    handlingDays: 3,
    policyNo: "cfp_mock_1",
    policyVersion: 1,
    reused: false,
  };

  await page.route(contractFailurePolicyURL, async (route: Route) => {
    if (route.request().method() === "PUT") {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      state.contractFailurePolicyRequests.push(body);
      failurePolicy = {
        ...failurePolicy,
        handlingDays: Number(body.handlingDays),
        policyNo: "cfp_mock_2",
        policyVersion: failurePolicy.policyVersion + 1,
      };
    }
    await route.fulfill({ json: apiEnvelope({ item: failurePolicy }) });
  });

  await page.route(contractFailureCancellationURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractFailureCancellationURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.contractFailureCancellationRequests.push({ ...body, contractNo });
    await route.fulfill({
      json: apiEnvelope({
        item: {
          compensationNo: "ccp_mock_1",
          orderNo: "ORDER_REQUIRED_FAILED_001",
          reasonCode: "required_contract_terminal",
          refundRequestNo: "rfr_mock_1",
          reused: false,
          statusCode: "refund_pending",
        },
      }),
    });
  });

  function preparedContract(contractNo: string, orderNo: string) {
    return {
      contractNo,
      createdAt: 1_786_161_800_000,
      currencyCode: "CNY",
      customerName: "示例企业客户",
      customerType: "enterprise",
      languageCode: "zh-CN",
      lines: [
        {
          amountMinor: "129900",
          orderItemNo: "ORDER_ITEM_001",
          periodCount: 12,
          periodUnitCode: "month",
          purchaseCycles: 1,
          quantity: 1,
          serviceName: "独立服务器",
        },
      ],
      merchantName: "示例 IDC 有限公司",
      orderNo,
      requiredForFulfillment: false,
      signerName: "张*",
      signerNo: "csgn_verified_001",
      signerPhoneMasked: "138****8000",
      signingExpiresAt: 1_786_766_600_000,
      signingOrderCode: "customer_first",
      statusCode: "pending_launch",
      templateName: "Enterprise service agreement",
      templateNo: "contract-template-enterprise",
      templateVersion: 3,
      totalAmountMinor: "129900",
      version: 1,
    };
  }

  await page.route(contractListURL, async (route: Route) => {
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      state.contractCreateRequests.push(body);
      const item = preparedContract(
        "CTR_OPTIONAL_PREVIEW_001",
        String(body.orderNo),
      );
      contracts.unshift({
        ...baseContract,
        contractNo: item.contractNo,
        createdAt: item.createdAt,
        launchedAt: 0,
        orderNo: item.orderNo,
        signingExpiresAt: item.signingExpiresAt,
        statusCode: "pending_launch",
        updatedAt: item.createdAt,
        version: item.version,
      });
      await route.fulfill({ json: apiEnvelope({ item }) });
      return;
    }
    state.contractListRequests.push(route.request().url());
    const query = new URL(route.request().url()).searchParams;
    const orderNo = query.get("orderNo") ?? "";
    const customerNo = query.get("customerNo") ?? "";
    const statusCode = query.get("statusCode") ?? "";
    const items = contracts.filter(
      (item) =>
        (!orderNo || item.orderNo === orderNo) &&
        (!customerNo || item.customerNo === customerNo) &&
        (!statusCode || item.statusCode === statusCode),
    );
    await route.fulfill({
      json: apiEnvelope(numberedMockPage(items, query)),
    });
  });

  await page.route(contractTemplateCandidateURL, async (route: Route) => {
    const orderNo =
      new URL(route.request().url()).searchParams.get("orderNo") ?? "";
    await route.fulfill({
      json: apiEnvelope({
        customerNo: "CUS_ENTERPRISE_001",
        customerType: "enterprise",
        items: [
          {
            languageCode: "zh-CN",
            scopeCode: "enterprise",
            signingOrderCode: "customer_first",
            templateName: "Enterprise service agreement",
            templateNo: "contract-template-enterprise",
            templateVersion: 3,
          },
        ],
        orderNo,
      }),
    });
  });

  await page.route(contractSignerCandidateURL, async (route: Route) => {
    await route.fulfill({
      json: apiEnvelope({
        items: [
          {
            authorizationVersion: 2,
            identityVerificationVersion: 3,
            maskedName: "张*",
            maskedPhone: "138****8000",
            phoneVerificationVersion: 4,
            signerNo: "csgn_verified_001",
            version: 5,
          },
        ],
      }),
    });
  });

  await page.route(contractDetailURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractDetailURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    if (contractNo === "template-candidates") {
      await route.fallback();
      return;
    }
    const item = contracts.find(
      (candidate) => candidate.contractNo === contractNo,
    );
    if (!item) {
      await route.fulfill({ status: 404 });
      return;
    }
    await route.fulfill({
      json: apiEnvelope({
        archiveJobs: [
          {
            artifactTypeCode: "sign_report",
            attemptCount: 0,
            completedAt: 1_786_161_700_000,
            createdAt: 1_786_161_690_000,
            flowNo: "CFW_LOCAL_001",
            jobNo: "CAJ_LOCAL_001",
            lastReasonCode: "report_not_enabled",
            nextAttemptAt: 0,
            statusCode: "skipped",
            supplierObservedAt: 0,
            updatedAt: 1_786_161_700_000,
          },
        ],
        events: [
          {
            createdAt: 1_786_161_680_000,
            decisionCode: "accepted",
            flowNo: "CFW_LOCAL_001",
            observedAt: 1_786_161_680_000,
            projectedStatus: item.statusCode,
            providerStatus: "REJECTED",
            reasonCode: "customer_refused",
            sourceKind: "callback",
            supplierObservedAt: 1_786_161_675_000,
          },
        ],
        files: [
          {
            archivedAt: 1_786_161_700_000,
            byteSize: 24576,
            contentSha256:
              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            createdAt: 1_786_161_690_000,
            fileNo: "CFL_LOCAL_001",
            fileTypeCode: "final_pdf",
            flowNo: "CFW_LOCAL_001",
            immutable: true,
            legalHold: false,
            mediaType: "application/pdf",
            retentionUntil: 1_817_697_600_000,
            statusCode: "archived",
            supplierObservedAt: 1_786_161_675_000,
          },
        ],
        flows: [
          {
            completedAt: 1_786_161_680_000,
            createdAt: 1_786_161_660_000,
            expiresAt: 1_786_766_460_000,
            failureCode: "",
            flowNo: "CFW_LOCAL_001",
            providerStatus: "REJECTED",
            purposeCode: "initial",
            sequenceNo: 1,
            statusCode: item.statusCode,
            submittedAt: 1_786_161_665_000,
          },
        ],
        historyTruncated: false,
        item,
        replacements: [],
      }),
    });
  });

  await page.route(contractSubmissionURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractSubmissionURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.contractSubmissionRequests.push({ ...body, contractNo });
    const item = contracts.find(
      (candidate) => candidate.contractNo === contractNo,
    );
    if (item) {
      item.statusCode = "signing";
      item.version = Number(item.version) + 1;
    }
    await route.fulfill({
      json: apiEnvelope({
        item: {
          contractNo,
          flowNo: "CFW_SUBMITTED_001",
          reused: false,
          statusCode: "signing",
          version: item?.version ?? 2,
        },
      }),
    });
  });

  await page.route(contractReplacementOptionsURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractReplacementOptionsURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    await route.fulfill({
      json: apiEnvelope({
        contractNo,
        customerNo: "CUS_ENTERPRISE_001",
        customerType: "enterprise",
        orderNo: "ORDER_OPTIONAL_001",
        signers: [
          {
            maskedName: "张*",
            maskedPhone: "138****8000",
            signerNo: "csgn_verified_001",
          },
        ],
        templates: [
          {
            languageCode: "zh-CN",
            scopeCode: "enterprise",
            signingOrderCode: "customer_first",
            templateName: "Enterprise service agreement",
            templateNo: "contract-template-enterprise",
            templateVersion: 3,
          },
        ],
      }),
    });
  });

  await page.route(contractReplacementPreviewURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractReplacementPreviewURL,
    );
    const oldContractNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.contractReplacementPreviewRequests.push({ ...body, oldContractNo });
    const item = preparedContract(
      "CTR_REPLACEMENT_PREVIEW_001",
      "ORDER_OPTIONAL_001",
    );
    contracts.unshift({
      ...baseContract,
      contractNo: item.contractNo,
      createdAt: item.createdAt,
      launchedAt: 0,
      orderNo: item.orderNo,
      signingExpiresAt: item.signingExpiresAt,
      statusCode: "pending_launch",
      updatedAt: item.createdAt,
      version: item.version,
    });
    await route.fulfill({ json: apiEnvelope({ item }) });
  });

  await page.route(contractReissueURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractReissueURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.contractReissueRequests.push({ ...body, contractNo });
    const item = contracts.find(
      (candidate) => candidate.contractNo === contractNo,
    );
    if (item) {
      item.statusCode = "reissuing";
      item.version = Number(item.version) + 1;
    }
    await route.fulfill({
      json: apiEnvelope({
        item: {
          contractNo,
          flowNo: "CFW_REISSUE_001",
          reused: false,
          statusCode: "signing",
          version: item?.version ?? 2,
        },
      }),
    });
  });

  await page.route(contractTerminationURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(
      contractTerminationURL,
    );
    const contractNo = decodeURIComponent(match?.[1] ?? "");
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.contractTerminationRequests.push({ ...body, contractNo });
    const item = contracts.find(
      (candidate) => candidate.contractNo === contractNo,
    );
    if (item) {
      item.statusCode = "terminated";
      item.version = Number(item.version) + 1;
    }
    await route.fulfill({
      json: apiEnvelope({
        item: {
          contractNo,
          flowNo: "CFW_TERMINATE_001",
          reused: false,
          statusCode: "signing",
          version: item?.version ?? 6,
        },
      }),
    });
  });

  await page.route(onboardingURL + "/links", async (route: Route) => {
    state.linkRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    current = {
      ...projection("ready"),
      consoleUrl: "https://qian.tencent.com/one-use",
      state: "enterprise_pending",
    };
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  await page.route(onboardingURL + "/rechecks", async (route: Route) => {
    state.recheckRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    current = projection("active");
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  await page.route(onboardingURL, async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  return state;
}
