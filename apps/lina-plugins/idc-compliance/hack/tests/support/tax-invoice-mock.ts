import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";
import { numberedMockPage } from "./numbered-page-mock";

const catalogs = {
  "en-US": JSON.parse(
    readFileSync(new URL("../../../manifest/i18n/en-US/tax-invoice-management.json", import.meta.url), "utf8"),
  ),
  "zh-CN": JSON.parse(
    readFileSync(new URL("../../../manifest/i18n/zh-CN/tax-invoice-management.json", import.meta.url), "utf8"),
  ),
} as Record<string, Record<string, string>>;

const invoiceListURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/tax-invoices(?:\?.*)?$/u;
const invoiceDetailURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/tax-invoices\/([^/?]+)$/u;
const invoiceActionURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/tax-invoices\/([^/]+)\/(rejections|cancellations|approvals|submissions)$/u;
const redListURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/red-letter-tasks(?:\?.*)?$/u;
const redDetailURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/red-letter-tasks\/([^/?]+)$/u;
const redActionURL = /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/red-letter-tasks\/([^/]+)\/(allocation-resolutions|manual-completions)$/u;

type Permission =
  | "idc:admin:compliance:red-letter:handle"
  | "idc:admin:compliance:red-letter:query"
  | "idc:admin:compliance:tax-invoice:cancel"
  | "idc:admin:compliance:tax-invoice:query"
  | "idc:admin:compliance:tax-invoice:review"
  | "idc:admin:compliance:tax-invoice:submit";

export interface TaxInvoiceMockState {
  invoiceActions: Array<{ action: string; body: Record<string, unknown>; taxInvoiceNo: string }>;
  redActions: Array<{ action: string; body: Record<string, unknown>; taskNo: string }>;
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

function invoice(taxInvoiceNo: string, statusCode: string, providerNotIssuedAt = 0) {
  return {
    canceledAt: 0,
    capabilityReady: false,
    capabilityReasonCode: "external_confirmation_required",
    currencyCode: "CNY",
    customerNo: "CUS_BUYER_001",
    decisionInternalNote: "",
    decisionReasonCode: "",
    deliveredAt: 0,
    grossAmountMinor: 12_880,
    invoiceTypeCode: statusCode === "pending_review" ? "pending_confirmation" : "vat_electronic_normal",
    issuedAt: 0,
    maskedInvoiceNumber: "",
    netAmountMinor: 12_151,
    providerCode: statusCode === "pending_review" ? "" : "nuonuo_invoice",
    providerNotIssuedAt,
    reviewedAt: statusCode === "pending_review" ? null : 1_786_464_120_000,
    statusCode,
    submittedAt: 1_786_464_060_000,
    supportedInvoiceTypes: [],
    taxAmountMinor: 729,
    taxInvoiceNo,
    version: 3,
  };
}

function invoiceDetail(item: ReturnType<typeof invoice>) {
  return {
    allocations: [{ allocationNo: "TIAL_001", billingInvoiceNo: "BILL_001", grossAmountMinor: 12_880, netAmountMinor: 12_151, orderNo: "ORDER_001", redLetteredGrossMinor: 0, refundedGrossMinor: 0, taxAmountMinor: 729 }],
    buyer: {
      bankAccountMasked: "****8890",
      bankName: "示例银行",
      invoiceTitle: "示例购买方有限公司",
      profileNo: "IPRF_001",
      registeredAddress: "上海市浦东新区",
      registeredPhone: "021-12345678",
      taxRegistrationNo: "91310000123456789X",
      titleTypeCode: "enterprise",
      version: 2,
    },
    files: [],
    historyTruncated: false,
    item,
    items: [{ allocationNo: "TIAL_001", billingInvoiceItemNo: "BIIT_001", grossAmountMinor: 12_880, itemNo: "TIIT_001", lineNo: 1, netAmountMinor: 12_151, taxAmountMinor: 729, taxClassificationCode: "304020101", taxRatePpm: 60_000 }],
  };
}

function redTask(taskNo: string, statusCode: string, allocationResolved: boolean, customerConfirmationRequired = false) {
  return {
    allocationCount: allocationResolved ? 1 : 0,
    allocationResolved,
    candidateCount: allocationResolved ? 0 : 2,
    completedAt: 0,
    completionEvidenceNo: "",
    createdAt: 1_786_464_060_000,
    currencyCode: "CNY",
    customerConfirmationRequired,
    customerNo: "CUS_BUYER_001",
    grossAmountMinor: 5_000,
    manualReasonCode: allocationResolved ? "provider_automation_unavailable" : "allocation_selection_required",
    netAmountMinor: 4_717,
    refundOperationNo: "REFUND_001",
    statusCode,
    taxAmountMinor: 283,
    taxInvoiceNo: allocationResolved ? "TINV_ISSUED_001" : "",
    taskNo,
    version: 2,
  };
}

export async function installTaxInvoiceMock(
  page: Page,
  permissions: Permission[] = [
    "idc:admin:compliance:tax-invoice:query",
    "idc:admin:compliance:tax-invoice:review",
    "idc:admin:compliance:tax-invoice:submit",
    "idc:admin:compliance:tax-invoice:cancel",
    "idc:admin:compliance:red-letter:query",
    "idc:admin:compliance:red-letter:handle",
  ],
  rowCount?: number,
): Promise<TaxInvoiceMockState> {
  const state: TaxInvoiceMockState = { invoiceActions: [], redActions: [] };
  const invoices = [
    invoice("TINV_PENDING_001", "pending_review"),
    invoice("TINV_APPROVED_001", "approved_pending_submission"),
    invoice("TINV_FAILED_NO_EVIDENCE_001", "failed"),
    invoice("TINV_FAILED_CONFIRMED_001", "failed", 1_786_464_180_000),
  ];
  const redTasks = [
    redTask("RTL_CONFIRM_001", "awaiting_confirmation", true, true),
    redTask("RTL_ALLOCATE_001", "manual_required", false),
    redTask("RTL_EVIDENCE_001", "manual_required", true),
  ];
  const evidenceByTask = new Map<string, Record<string, unknown>>();
  if (rowCount) {
    while (invoices.length < rowCount) invoices.push(invoice(`TINV_PAGE_${invoices.length}`, "pending_review"));
    while (redTasks.length < rowCount) redTasks.push(redTask(`RTL_PAGE_${redTasks.length}`, "manual_required", true));
  }

  await page.route("**/api/v1/config/public/frontend", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ workspace: { basePath: "/admin" } }) });
  });
  await page.route("**/api/v1/user/message/count", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ count: 0 }) });
  });
  await page.route("**/api/v1/user/message?**", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: [], total: 0 }) });
  });
  await page.route("**/api/v1/user/info", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ avatar: "", homePath: "/idc/compliance/tax-invoices", menus: [], permissions, realName: "Admin", roles: ["admin"], userId: 1, username: "admin" }) });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    const locale = route.request().headers()["accept-language"]?.includes("en") ? "en-US" : "zh-CN";
    await route.fulfill({ json: apiEnvelope({ enabled: true, items: [{ direction: "ltr", isDefault: true, locale: "zh-CN", name: "Simplified Chinese", nativeName: "简体中文" }, { direction: "ltr", isDefault: false, locale: "en-US", name: "English", nativeName: "English" }], locale }) });
  });
  await page.route("**/api/v1/i18n/runtime/messages**", async (route: Route) => {
    const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
    await route.fulfill({ json: apiEnvelope({ messages: nestedMessages(catalogs[locale]!) }) });
  });
  await page.route("**/api/v1/plugins/dynamic", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: [] }) });
  });
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: [{ children: [{ component: "system/plugin/dynamic-page", meta: { authority: ["idc:admin:compliance:tax-invoice:query"], title: "plugin.idc-compliance.taxInvoiceManagement.title" }, name: "IDCComplianceTaxInvoices", path: "/idc/compliance/tax-invoices" }], component: "BasicLayout", meta: { title: "IDC Compliance" }, name: "IDCCompliance", path: "/idc/compliance" }] }) });
  });

  await page.route(invoiceActionURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(invoiceActionURL);
    const taxInvoiceNo = decodeURIComponent(match?.[1] ?? "");
    const action = match?.[2] ?? "";
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.invoiceActions.push({ action, body, taxInvoiceNo });
    const item = invoices.find((candidate) => candidate.taxInvoiceNo === taxInvoiceNo)!;
    if (action === "rejections") item.statusCode = "rejected";
    if (action === "cancellations") item.statusCode = "canceled";
    item.version += 1;
    await route.fulfill({ json: apiEnvelope({ result: { applied: true, item, reused: false } }) });
  });
  await page.route(invoiceDetailURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(invoiceDetailURL);
    const item = invoices.find((candidate) => candidate.taxInvoiceNo === decodeURIComponent(match?.[1] ?? ""));
    await route.fulfill(item ? { json: apiEnvelope(invoiceDetail(item)) } : { status: 404 });
  });
  await page.route(invoiceListURL, async (route: Route) => {
    const query = new URL(route.request().url()).searchParams;
    const customerNo = query.get("customerNo") ?? "";
    const statusCode = query.get("statusCode") ?? "";
    await route.fulfill({ json: apiEnvelope(numberedMockPage(invoices.filter((item) => (!customerNo || item.customerNo === customerNo) && (!statusCode || item.statusCode === statusCode)), query)) });
  });

  await page.route(redActionURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(redActionURL);
    const taskNo = decodeURIComponent(match?.[1] ?? "");
    const action = match?.[2] ?? "";
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.redActions.push({ action, body, taskNo });
    const item = redTasks.find((candidate) => candidate.taskNo === taskNo)!;
    if (action === "allocation-resolutions") {
      item.allocationResolved = true;
      item.allocationCount = 1;
      item.candidateCount = 0;
      item.manualReasonCode = "provider_automation_unavailable";
      item.taxInvoiceNo = "TINV_ISSUED_001";
    } else {
      item.statusCode = "completed";
      item.completedAt = 1_786_464_240_000;
      item.completionEvidenceNo = "RLEV_001";
      evidenceByTask.set(taskNo, { byteSize: 0, evidenceNo: "RLEV_001", evidenceReference: body.evidenceReference, evidenceSha256: body.evidenceSha256, mediaType: "", occurredAt: body.occurredAt });
    }
    item.version += 1;
    await route.fulfill({ json: apiEnvelope({ item }) });
  });
  await page.route(redDetailURL, async (route: Route) => {
    const match = new URL(route.request().url()).pathname.match(redDetailURL);
    const item = redTasks.find((candidate) => candidate.taskNo === decodeURIComponent(match?.[1] ?? ""));
    if (!item) {
      await route.fulfill({ status: 404 });
      return;
    }
    await route.fulfill({ json: apiEnvelope({
      allocations: item.allocationResolved ? [{ grossAmountMinor: 5_000, netAmountMinor: 4_717, refundAllocationNo: "RFAL_001", taxAmountMinor: 283, taxInvoiceAllocationNo: "TIAL_ISSUED_001", taxInvoiceItemNo: "TIIT_ISSUED_001", taxInvoiceNo: "TINV_ISSUED_001" }] : [],
      candidates: item.allocationResolved ? [] : [
        { availableGrossMinor: 8_000, availableNetMinor: 7_547, availableTaxMinor: 453, refundAllocationNo: "RFAL_001", requestedGrossMinor: 5_000, taxInvoiceAllocationNo: "TIAL_ISSUED_001", taxInvoiceItemNo: "TIIT_ISSUED_001", taxInvoiceNo: "TINV_ISSUED_001" },
        { availableGrossMinor: 6_000, availableNetMinor: 5_660, availableTaxMinor: 340, refundAllocationNo: "RFAL_001", requestedGrossMinor: 5_000, taxInvoiceAllocationNo: "TIAL_ISSUED_002", taxInvoiceItemNo: "TIIT_ISSUED_002", taxInvoiceNo: "TINV_ISSUED_002" },
      ],
      evidence: evidenceByTask.get(item.taskNo), historyTruncated: false, item,
    }) });
  });
  await page.route(redListURL, async (route: Route) => {
    const query = new URL(route.request().url()).searchParams;
    const customerNo = query.get("customerNo") ?? "";
    const statusCode = query.get("statusCode") ?? "";
    await route.fulfill({ json: apiEnvelope(numberedMockPage(redTasks.filter((item) => (!customerNo || item.customerNo === customerNo) && (!statusCode || item.statusCode === statusCode)), query)) });
  });

  return state;
}
