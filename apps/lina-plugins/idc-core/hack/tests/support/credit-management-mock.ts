import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import type { Page, Route } from "@host-tests/support/playwright";

import { installFinancialManagementMock } from "./financial-management-mock";

export const creditApplicationNo = "CRA_CREDIT_E2E_0001";
const now = 1_786_406_400_000;

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

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
      } else {
        const next = target[segment];
        if (!next || typeof next !== "object" || Array.isArray(next)) {
          target[segment] = {};
        }
        target = target[segment] as Record<string, unknown>;
      }
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
    } else {
      target[key] = value;
    }
  }
}

function application(statusCode = "pending_review", reviewCount = 1) {
  return {
    applicationNo: creditApplicationNo,
    createdAt: now,
    creditAccountNo: statusCode === "approved" ? "CRA_CREDIT_0001" : "",
    currencyCode: "CNY",
    customerNo: "CUS_CREDIT_0001",
    requestedLimitMinor: "100000",
    requestedValidFrom: now,
    requestedValidUntil: now + 365 * 86_400_000,
    requiredApprovalCount: 2,
    reviewCount,
    statusCode,
  };
}

export interface CreditManagementMockState {
  listRequests: Array<{ directory: string; page: number; limit: number }>;
  failLists: boolean;
  deniedDirectories: Partial<Record<string, 401 | 403>>;
  deniedDetail: 0 | 401 | 403;
  rowCount: number;
  proofBodies: Array<Record<string, unknown>>;
  reviewBodies: Array<Record<string, unknown>>;
  statusCode: string;
}

export async function installCreditManagementMock(
  page: Page,
  options: { standalone?: boolean } = {},
) {
  await installFinancialManagementMock(page);
  const state: CreditManagementMockState = {
    listRequests: [],
    failLists: false,
    deniedDirectories: {},
    deniedDetail: 0,
    rowCount: 1,
    proofBodies: [],
    reviewBodies: [],
    statusCode: "pending_review",
  };
  const permissions = [
    "idc:admin:billing:credit-application:list",
    "idc:admin:billing:credit-application:query",
    "idc:admin:billing:credit-application:review",
    "idc:admin:billing:credit-account:list",
    "idc:admin:billing:credit-statement:list",
  ];

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (!payload.data) throw new Error("user-info payload is unavailable");
    payload.data.permissions = [
      ...new Set([...(payload.data.permissions ?? []), ...permissions]),
    ];
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/messages**", async (route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { messages?: Record<string, unknown> };
    };
    if (!payload.data) throw new Error("runtime i18n payload is unavailable");
    payload.data.messages ??= {};
    const locale =
      new URL(route.request().url()).searchParams.get("lang") === "en-US"
        ? "en-US"
        : "zh-CN";
    mergeMessages(payload.data.messages, pluginMessages(locale));
    await route.fulfill({
      headers: { ...response.headers(), etag: `credit-e2e-${locale}` },
      json: payload,
      status: 200,
    });
  });
  if (!options.standalone)
    await page.evaluate((values) => {
      const key = "lina-web-antd-5.6.0-dev-core-access";
      const raw = localStorage.getItem(key);
      const current = raw ? JSON.parse(raw) : {};
      localStorage.setItem(
        key,
        JSON.stringify({
          ...current,
          accessCodes: [
            ...new Set([...(current.accessCodes ?? []), ...values]),
          ],
        }),
      );
      for (const storageKey of Object.keys(localStorage)) {
        if (storageKey.startsWith("linapro:i18n:runtime:")) {
          localStorage.removeItem(storageKey);
        }
      }
    }, permissions);

  await page.route(
    "**/x/idc-core/api/v1/admin/billing/credit/**",
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const method = request.method();
      const directory = /^.*\/credit\/(applications|accounts|statements)$/u.exec(path)?.[1];
      const currentPage = Number(url.searchParams.get("page") || 1);
      const pageSize = Number(url.searchParams.get("limit") || 10);
      if (directory && method === "GET") state.listRequests.push({ directory, page: currentPage, limit: pageSize });
      const denied = directory ? state.deniedDirectories[directory] : state.deniedDetail;
      if (denied && method === "GET") {
        await route.fulfill({ status: denied, json: { code: denied, message: "Access denied" } });
        return;
      }
      if (state.failLists && method === "GET") {
        await route.fulfill({
          status: 503,
          json: { code: 500, message: "Credit data unavailable" },
        });
        return;
      }
      let data: unknown;
      if (path.endsWith("/credit/applications") && method === "GET") {
        data = {
          applications: Array.from({ length: state.rowCount }, (_, index) => ({
            ...application(
              state.statusCode,
              state.statusCode === "approved" ? 2 : 1,
            ),
            applicationNo:
              index === 0
                ? creditApplicationNo
                : `${creditApplicationNo}_${index + 1}`,
          })),
          nextCursor: "",
        };
      } else if (
        path.endsWith(`/applications/${creditApplicationNo}`) &&
        method === "GET"
      ) {
        data = {
          detail: {
            application: application(state.statusCode),
            reviews: [],
          },
          evidence: [
            {
              fileName: "bank-statement.pdf",
              mediaType: "application/pdf",
              sizeBytes: 2048,
              uploadNo: "CEU_CREDIT_0001",
            },
          ],
        };
      } else if (path.endsWith("/review-proofs") && method === "POST") {
        state.proofBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        data = { proof: { proofToken: "opaque-credit-review-proof" } };
      } else if (path.endsWith("/reviews") && method === "POST") {
        state.reviewBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        state.statusCode = "approved";
        data = { application: application("approved", 2) };
      } else if (path.endsWith("/credit/accounts") && method === "GET") {
        data = {
          accounts: [
            {
              availableLimitMinor: "0",
              billedPrincipalMinor: "8800",
              creditAccountNo: "CRA_CREDIT_OVERDUE",
              currencyCode: "CNY",
              customerNo: "CUS_CREDIT_0002",
              delinquentPrincipalMinor: "8800",
              freezeReasonCode: "statement_overdue",
              statusCode: "frozen",
              totalLimitMinor: "100000",
              unbilledPrincipalMinor: "0",
            },
          ],
          nextCursor: "",
        };
      } else if (path.endsWith("/credit/statements") && method === "GET") {
        data = {
          nextCursor: "",
          statements: [
            {
              currencyCode: "CNY",
              customerNo: "CUS_CREDIT_0002",
              outstandingAmountMinor: "8800",
              paymentDueAt: now - 86_400_000,
              statementNo: "CRS_CREDIT_OVERDUE",
              statusCode: "overdue",
              totalAmountMinor: "8800",
            },
          ],
        };
      } else {
        await route.fulfill({ json: envelope({}), status: 404 });
        return;
      }
      if (directory && method === "GET") {
        const response = data as Record<string, unknown>;
        let rows = response[directory] as Record<string, unknown>[];
        if (directory !== "applications") {
          const first = rows[0]!;
          const rowKey = directory === "accounts" ? "creditAccountNo" : "statementNo";
          rows = Array.from({ length: state.rowCount }, (_, index) => ({
            ...first,
            [rowKey]: index === 0 ? first[rowKey] : `${first[rowKey]}_${index + 1}`,
          }));
        }
        response.total = rows.length;
        response[directory] = rows.slice((currentPage - 1) * pageSize, currentPage * pageSize);
      }
      await route.fulfill({ json: envelope(data) });
    },
  );
  return state;
}
