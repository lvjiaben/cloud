import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

import { refreshPluginProjection } from "../../../../../../hack/tests/fixtures/plugin";

const enUSPrivacyMessages = JSON.parse(
  readFileSync(
    new URL("../../../manifest/i18n/en-US/privacy.json", import.meta.url),
    "utf8",
  ),
) as Record<string, string>;
const zhCNPrivacyMessages = JSON.parse(
  readFileSync(
    new URL("../../../manifest/i18n/zh-CN/privacy.json", import.meta.url),
    "utf8",
  ),
) as Record<string, string>;

const routePattern =
  "**/x/idc-compliance/api/v1/admin/compliance/privacy**";

function envelope(data: unknown) {
  return { code: 0, data, message: "OK" };
}

function expandFlatMessages(source: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(source)) {
    const segments = key.split(".");
    let cursor = root;
    for (const segment of segments.slice(0, -1)) {
      const next = cursor[segment];
      if (!next || typeof next !== "object" || Array.isArray(next)) {
        cursor[segment] = {};
      }
      cursor = cursor[segment] as Record<string, unknown>;
    }
    cursor[segments.at(-1)!] = value;
  }
  return root;
}

function menuRoutes() {
  return [
    {
      children: [
        {
          component: "dashboard/analytics/index",
          meta: {
            icon: "lucide:area-chart",
            title: "page.dashboard.analytics",
          },
          name: "Analytics",
          path: "/dashboard/analytics",
        },
      ],
      component: "BasicLayout",
      meta: {
        icon: "lucide:layout-dashboard",
        order: -1,
        title: "page.dashboard.title",
      },
      name: "Dashboard",
      path: "/dashboard",
    },
    {
      children: [
        {
          component: "system/plugin/dynamic-page",
          meta: {
            authority: ["idc:admin:compliance:privacy:query"],
            icon: "lucide:shield-check",
            title: "Privacy Governance",
          },
          name: "IDCCompliancePrivacyGovernance",
          path: "/idc/compliance/privacy",
        },
      ],
      component: "BasicLayout",
      meta: {
        icon: "lucide:shield-check",
        order: 33,
        title: "Compliance Center",
      },
      name: "IDCCompliance",
      path: "/idc/compliance",
    },
  ];
}

async function installPluginHostMock(page: Page) {
  await page.route("**/api/v1/user/info", async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as {
      data?: { permissions?: string[] };
      permissions?: string[];
    };
    const userInfo = body.data ?? body;
    userInfo.permissions = [
      ...new Set([
        ...(userInfo.permissions ?? []),
        "idc:admin:compliance:privacy:query",
        "idc:admin:compliance:privacy:configure",
        "idc:admin:compliance:privacy:incident",
      ]),
    ];
    await route.fulfill({ json: body, response });
  });
  await page.route("**/api/v1/menus/all", async (route) => {
    await route.fulfill({ json: envelope({ list: menuRoutes() }) });
  });
  await page.route("**/api/v1/plugins/dynamic", async (route) => {
    await route.fulfill({
      json: envelope({
        list: [
          {
            enabled: 1,
            generation: 1,
            id: "idc-compliance",
            installed: 1,
            runtimeState: "normal",
            statusKey: "sys_plugin.status:enabled",
            version: "v0.1.15",
          },
        ],
      }),
    });
  });
  await page.route("**/api/v1/i18n/runtime/messages**", async (route) => {
    const request = route.request();
    const locale =
      new URL(request.url()).searchParams.get("lang") ??
      request.headers()["accept-language"] ??
      "zh-CN";
    const flatMessages = locale.toLowerCase().startsWith("en")
      ? enUSPrivacyMessages
      : zhCNPrivacyMessages;
    await route.fulfill({
      headers: { ETag: `\"privacy-${locale}\"` },
      json: envelope({
        messages: expandFlatMessages(flatMessages),
      }),
    });
  });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data) });
}

export async function installPrivacyGovernanceMock(
  page: Page,
  options: { locale?: "en-US" | "zh-CN" } = {},
) {
  await installPluginHostMock(page);
  const english = options.locale === "en-US";
  const state = {
    deletionConfirmations: [] as Array<Record<string, unknown>>,
    incidentAdvances: [] as Array<Record<string, unknown>>,
    publications: [] as Array<Record<string, unknown>>,
    supportExports: [] as Array<Record<string, unknown>>,
  };
  let deletionStatus = "pending";
  let deletionVersion = 1;
  let incidentStatus = "discovered";
  let incidentVersion = 1;

  const assessment = (type: string, status: "draft" | "published") => ({
    aiAllowedFields:
      type === "ai_safe_projection"
        ? ["personal_verification_status", "aggregate_case_count"]
        : [],
    assessmentNo: `PIA_${type}_${status}`,
    assessmentTypeCode: type,
    auditEvidenceDescription: english
      ? "Immutable local audit evidence"
      : "所有确认与替换都追加本地不可变审计",
    crossBorderTransfer: false,
    impact: english
      ? "New processing stops when the required basis is unavailable."
      : "缺少有效处理依据时停止新的敏感资料处理。",
    merchantRole: english ? "Personal information processor" : "个人信息处理者",
    necessity: english
      ? "Required for the selected local compliance purpose."
      : "仅用于当前已说明的本地合规目的。",
    processingInstruction: english
      ? "Process only this bounded local purpose."
      : "只按当前明确目的处理，不得扩大使用范围。",
    purpose: english
      ? `Published purpose for ${type}`
      : "完成当前客户明确选择的本地合规事项",
    recipients: english
      ? "The merchant and its configured local supplier"
      : "当前商家及其自行配置的本地供应商",
    retentionDescription: english
      ? "Uses the current merchant-configured policy"
      : "使用当前商家按资料类型配置的留存规则",
    rightsEntry: english
      ? "Privacy and authorization in the customer center"
      : "客户中心的隐私与授权页面",
    rowVersion: 1,
    statusCode: status,
    storageLocation: english
      ? "This installation and the configured supplier"
      : "当前安装和商家自行配置的供应商",
    subcontractingBoundary: english
      ? "No unapproved subcontracting"
      : "未明确约定前禁止再委托",
    thirdPartyRole: english ? "Entrusted processor" : "受托处理者",
    updatedAt: 1_787_020_000_000,
    version: 1,
  });

  await page.route(routePattern, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const body = (request.postDataJSON() ?? {}) as Record<string, unknown>;

    if (request.method() === "GET" && path.endsWith("/assessments")) {
      await fulfill(route, {
        items: [
          assessment("personal_verification", "published"),
          assessment("enterprise_verification", "published"),
          assessment("electronic_contract", "published"),
          assessment("tax_invoice", "published"),
          assessment("local_sensitive_export", "published"),
          assessment("ai_safe_projection", "published"),
          assessment("ai_safe_projection", "draft"),
        ],
      });
      return;
    }
    if (request.method() === "POST" && path.endsWith("/assessments/publish")) {
      state.publications.push(body);
      await fulfill(route, {
        item: assessment("ai_safe_projection", "published"),
      });
      return;
    }
    if (request.method() === "GET" && path.endsWith("/retention-policies")) {
      await fulfill(route, {
        items: [
          {
            createdAt: 1_787_020_000_000,
            dataTypeCode: "personal_verification_material",
            policyNo: "PRP_PERSONAL",
            retentionDays: 365,
            version: 1,
          },
          {
            createdAt: 1_787_020_000_000,
            dataTypeCode: "local_sensitive_export",
            policyNo: "PRP_EXPORT",
            retentionDays: 30,
            version: 1,
          },
        ],
      });
      return;
    }
    if (request.method() === "GET" && path.endsWith("/deletion-tasks")) {
      await fulfill(route, {
        items: [
          {
            dataTypeCode: "local_sensitive_export",
            dueAt: 1_787_020_000_000,
            holdReasonCode: "",
            statusCode: deletionStatus,
            targetNo: "PSE_LOCAL_REDACTED_001",
            targetTypeCode: "local_sensitive_export",
            taskNo: "PDT_LOCAL_REDACTED_001",
            version: deletionVersion,
          },
        ],
      });
      return;
    }
    if (
      request.method() === "POST" &&
      path.endsWith("/deletion-tasks/PDT_LOCAL_REDACTED_001/confirmations")
    ) {
      state.deletionConfirmations.push(body);
      deletionStatus = "completed";
      deletionVersion += 1;
      await fulfill(route, { item: { statusCode: deletionStatus } });
      return;
    }
    if (request.method() === "GET" && path.endsWith("/incidents")) {
      await fulfill(route, {
        items: [
          {
            createdAt: 1_787_020_000_000,
            exportBlocked: incidentStatus !== "resolved",
            externalNotificationConfirmed: false,
            incidentNo: "PIN_LOCAL_001",
            safeSummary:
              "A bounded local rule detected an unusual download pattern.",
            severityCode: "high",
            sourceCode: "bulk_download",
            statusCode: incidentStatus,
            timeline: [],
            version: incidentVersion,
          },
        ],
      });
      return;
    }
    if (
      request.method() === "POST" &&
      path.endsWith("/incidents/PIN_LOCAL_001/advances")
    ) {
      state.incidentAdvances.push(body);
      incidentStatus = String(body.nextStageCode ?? "contained");
      incidentVersion += 1;
      await fulfill(route, { item: { statusCode: incidentStatus } });
      return;
    }
    if (request.method() === "POST" && path.endsWith("/support-exports")) {
      state.supportExports.push(body);
      await fulfill(route, {
        content: '{"schemaVersion":"idc-redacted-support-v1"}',
        contentSha256: "a".repeat(64),
        exportNo: "PSE_LOCAL_001",
        fileName: "PSE_LOCAL_001.json",
      });
      return;
    }
    await route.fallback();
  });
  await refreshPluginProjection(page);
  return state;
}
