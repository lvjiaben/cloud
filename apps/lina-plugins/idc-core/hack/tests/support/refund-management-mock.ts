import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import type { Page, Route } from "@host-tests/support/playwright";

import { installFinancialManagementMock } from "./financial-management-mock";

export const managementRefundNo = "RFD_MANAGEMENT_0001";
const now = 1_786_406_400_000;

function pluginMessages(locale: "en-US" | "zh-CN") {
  const file = resolve(
    import.meta.dirname,
    "../../../manifest/i18n",
    locale,
    "plugin.json",
  );
  const flat = JSON.parse(readFileSync(file, "utf8")) as Record<string, string>;
  const nested: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(flat)) {
    const segments = key.split(".");
    let target = nested;
    segments.forEach((segment, index) => {
      if (index === segments.length - 1) {
        target[segment] = value;
        return;
      }
      const next = target[segment];
      if (!next || typeof next !== "object" || Array.isArray(next)) {
        target[segment] = {};
      }
      target = target[segment] as Record<string, unknown>;
    });
  }
  return nested;
}

function mergeMessages(
  target: Record<string, unknown>,
  source: Record<string, unknown>,
) {
  for (const [key, value] of Object.entries(source)) {
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const current = target[key];
      if (!current || typeof current !== "object" || Array.isArray(current)) {
        target[key] = {};
      }
      mergeMessages(
        target[key] as Record<string, unknown>,
        value as Record<string, unknown>,
      );
      continue;
    }
    target[key] = value;
  }
}

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function summary(highAmount: boolean, statusCode = "pending_review") {
  return {
    amountMinor: "500000",
    approvalModeCode: "manual",
    currencyCode: "CNY",
    customerNo: "CUS_REFUND_0001",
    customerCode: "10086",
    customerDisplayName: "退款测试客户",
    customerPrimaryPhone: "138****0001",
    destinationCode: "original_channel",
    hasDangerousOperation: false,
    hasResourceImpact: true,
    highAmount,
    impactDigest: "a".repeat(64),
    invoiceNo: "INV_REFUND_0001",
    orderNo: "ORD_REFUND_0001",
    reasonCode: "wrong_configuration",
    reasonDetail: "客户选择了错误的内存规格",
    refundNo: managementRefundNo,
    requiresServiceTermination: true,
    reused: false,
    statusCode,
    submittedAt: now,
    version: statusCode === "pending_review" ? 1 : 2,
  };
}

export interface RefundManagementMockState {
  detailRequests: string[];
  highAmount: boolean;
  listRequests: URL[];
  proofBodies: Array<Record<string, unknown>>;
  reviewBodies: Array<Record<string, unknown>>;
  customerListRequests: URL[];
  statusCode: string;
}

export async function installRefundManagementMock(
  page: Page,
  options: { highAmount?: boolean; failedDelivery?: boolean } = {},
) {
  await installFinancialManagementMock(page);
  const refundPermissions = [
    "idc:admin:billing:refund:list",
    "idc:admin:billing:refund:query",
    "idc:admin:billing:refund:review",
  ];
  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (!payload.data) throw new Error("user-info payload is unavailable");
    payload.data.permissions = [
      ...new Set([...(payload.data.permissions ?? []), ...refundPermissions]),
    ];
    await route.fulfill({ json: payload, response });
  });
  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const response = await route.fetch();
      const payload = (await response.json()) as {
        data?: { messages?: Record<string, unknown> };
      };
      if (!payload.data) throw new Error("runtime i18n payload is unavailable");
      payload.data.messages ??= {};
      const requestedLocale =
        new URL(route.request().url()).searchParams.get("lang") === "en-US"
          ? "en-US"
          : "zh-CN";
      mergeMessages(payload.data.messages, pluginMessages(requestedLocale));
      await route.fulfill({
        headers: {
          ...response.headers(),
          etag: `refund-e2e-${requestedLocale}`,
        },
        json: payload,
        status: 200,
      });
    },
  );
  await page.evaluate((permissions) => {
    const accessStorageKey = "lina-web-antd-5.6.0-dev-core-access";
    const rawAccessState = localStorage.getItem(accessStorageKey);
    const accessState = rawAccessState ? JSON.parse(rawAccessState) : {};
    localStorage.setItem(
      accessStorageKey,
      JSON.stringify({
        ...accessState,
        accessCodes: [
          ...new Set([...(accessState.accessCodes ?? []), ...permissions]),
        ],
      }),
    );
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith("linapro:i18n:runtime:")) {
        localStorage.removeItem(key);
      }
    }
  }, refundPermissions);
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { list?: Array<Record<string, unknown>> };
    };
    const routes = payload.data?.list ?? [];
    const hasFinancialRoute = JSON.stringify(routes).includes("/idc/finance");
    if (!hasFinancialRoute) {
      routes.push({
        children: [
          {
            component: "system/plugin/dynamic-page",
            meta: {
              icon: "lucide:landmark",
              title: "Financial Management",
            },
            name: "IdcFinancialManagementRefundE2E",
            path: "/idc/finance",
          },
        ],
        component: "BasicLayout",
        meta: {
          icon: "lucide:server-cog",
          order: 4,
          title: "IDC Operations",
        },
        name: "IdcOperationsRefundE2E",
        path: "/idc",
      });
    }
    await route.fulfill({
      json: payload,
      status: response.status(),
    });
  });
  const state: RefundManagementMockState = {
    detailRequests: [],
    highAmount: options.highAmount ?? false,
    listRequests: [],
    proofBodies: [],
    reviewBodies: [],
    customerListRequests: [],
    statusCode: "pending_review",
  };
  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route: Route) => {
    const url = new URL(route.request().url());
    state.customerListRequests.push(url);
    await route.fulfill({ json: envelope({ customers: [{ customerNo: "CUS_REFUND_0001", customerCode: "10086", displayName: "退款测试客户", primaryPhone: "138****0001" }], nextCursor: "" }) });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/billing/refunds**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const method = request.method();
      if (path.endsWith(`/${managementRefundNo}`) && method === "GET") {
        state.detailRequests.push(managementRefundNo);
        await route.fulfill({
          json: envelope({
            detail: {
              items: [
                {
                  amountMinor: "500000",
                  descriptionEnUS: "Production cloud server",
                  descriptionZhCN: "生产云服务器",
                  impacts: options.failedDelivery ? [] : [
                    {
                      entitlementNo: "ENT_REFUND_0001",
                      entitlementVersion: 7,
                      requiresTermination: true,
                      statusCode: "active",
                    },
                  ],
                  invoiceItemNo: "IVI_REFUND_0001",
                  orderItemNo: "ORI_REFUND_0001",
                  quantity: 1,
                  refundableAmountMinor: "500000",
                  refundItemNo: "RFI_REFUND_0001",
                },
              ],
              refund: { ...summary(state.highAmount, state.statusCode), ...(options.failedDelivery ? {hasResourceImpact:false, requiresServiceTermination:false, reasonCode:"service_issue", reasonDetail:"供应商拒绝开通，管理员选择退款"} : {}) },
            },
          }),
        });
        return;
      }
      if (path.endsWith("/review-proofs") && method === "POST") {
        state.proofBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: envelope({
            proof: {
              expiresAt: now + 120_000,
              proofToken: "opaque-refund-proof",
            },
          }),
        });
        return;
      }
      if (path.endsWith("/reviews") && method === "POST") {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.reviewBodies.push(body);
        state.statusCode =
          body.decisionCode === "approve" ? "approved" : "rejected";
        await route.fulfill({
          json: envelope({
            refund: { ...summary(state.highAmount, state.statusCode), ...(options.failedDelivery ? {hasResourceImpact:false, requiresServiceTermination:false, reasonCode:"service_issue", reasonDetail:"供应商拒绝开通，管理员选择退款"} : {}) },
          }),
        });
        return;
      }
      if (path.endsWith("/refunds") && method === "GET") {
        state.listRequests.push(url);
        await route.fulfill({
          json: envelope({
            nextCursor: "",
            refunds: [summary(state.highAmount, state.statusCode)],
            total: 1,
          }),
        });
        return;
      }
      await route.fulfill({ status: 404, json: envelope({}) });
    },
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  return state;
}
