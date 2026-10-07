import type { Page, Route } from "@host-tests/support/playwright";

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

const invoiceNo = "INV_FINANCE_0001";
const adjustmentNo = "BAR_FINANCE_0001";
const transactionNo = "BLT_FINANCE_MERCHANT_0001";

function invoice() {
  return {
    createdAt: 1_786_406_400_000,
    currencyCode: "CNY",
    customerNo: "CUS_FINANCE_0001",
    customerCode: "10006",
    customerDisplayName: "财务测试客户",
    customerPrimaryPhone: "13800138006",
    invoiceNo,
    itemCount: 1,
    outstandingAmountMinor: "900719925474099301",
    paidAmountMinor: "0",
    paymentDueAt: 1_786_492_800_000,
    statusCode: "pending_payment",
    totalAmountMinor: "900719925474099301",
    waivedAmountMinor: "0",
  };
}

function adjustment() {
  return {
    adjustmentNo,
    adjustmentTypeCode: "manual_receipt",
    amountMinor: "8800",
    approvalRequired: true,
    createdAt: 1_786_406_400_000,
    currencyCode: "CNY",
    customerNo: "CUS_FINANCE_0001",
    customerCode: "10006",
    customerDisplayName: "财务测试客户",
    customerPrimaryPhone: "13800138006",
    reasonCode: "offline_balance_credit",
    requestedBy: 1001,
    statusCode: "pending_review",
    updatedAt: 1_786_406_400_000,
  };
}

function transaction() {
  return {
    amountMinor: "8800",
    currencyCode: "CNY",
    customerNo: "",
    customerCode: "",
    customerDisplayName: "",
    customerPrimaryPhone: "",
    entryCount: 2,
    postedAt: 1_786_406_400_000,
    transactionNo,
    transactionTypeCode: "manual_receipt",
  };
}

export interface FinancialManagementMockState {
  executeBodies: Array<Record<string, unknown>>;
  exportBodies: Array<Record<string, unknown>>;
  filterBodies: Array<Record<string, unknown>>;
  invoiceRequests: URL[];
  customerRequests: URL[];
  noteBodies: Array<Record<string, unknown>>;
  proofBodies: Array<Record<string, unknown>>;
  resendBodies: Array<Record<string, unknown>>;
  transactionRequests: URL[];
}

export async function installFinancialManagementMock(
  page: Page,
): Promise<FinancialManagementMockState> {
  const state: FinancialManagementMockState = {
    executeBodies: [],
    exportBodies: [],
    filterBodies: [],
    invoiceRequests: [],
    customerRequests: [],
    noteBodies: [],
    proofBodies: [],
    resendBodies: [],
    transactionRequests: [],
  };
  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route) => {
    const url = new URL(route.request().url());
    state.customerRequests.push(url);
    await route.fulfill({
      json: envelope({
        customers: [
          {
            customerCode: "10006",
            customerNo: "CUS_FINANCE_0001",
            displayName: "财务测试客户",
            primaryPhone: "13800138006",
          },
        ],
        nextCursor: "",
      }),
    });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/billing/**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      if (
        request.method() === "GET" &&
        path.endsWith(`/invoices/${invoiceNo}/delivery-preview`)
      ) {
        await route.fulfill({
          json: envelope({
            preview: {
              invoiceNo,
              invoiceVersion: 3,
              maskedEmail: "a***@e***.com",
            },
          }),
        });
        return;
      }
      if (
        request.method() === "POST" &&
        path.endsWith(`/invoices/${invoiceNo}/resends`)
      ) {
        state.resendBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: envelope({ intentNo: "FDI_FINANCE_0001", reused: false }),
        });
        return;
      }
      if (path.endsWith("/internal-notes")) {
        if (request.method() === "POST") {
          state.noteBodies.push(
            request.postDataJSON() as Record<string, unknown>,
          );
          await route.fulfill({
            json: envelope({
              note: {
                content: "private finance note",
                correctsNoteNo: "",
                createdAt: 1_786_406_400_000,
                createdBy: 1001,
                noteNo: "FNT_FINANCE_0001",
                targetNo: invoiceNo,
                targetType: "invoice",
              },
            }),
          });
          return;
        }
        await route.fulfill({
          json: envelope({
            notes: state.noteBodies.length
              ? [
                  {
                    content: "private finance note",
                    correctsNoteNo: "",
                    createdAt: 1_786_406_400_000,
                    createdBy: 1001,
                    noteNo: "FNT_FINANCE_0001",
                    targetNo: invoiceNo,
                    targetType: "invoice",
                  },
                ]
              : [],
          }),
        });
        return;
      }
      if (path.endsWith("/saved-filters")) {
        if (request.method() === "POST") {
          const body = request.postDataJSON() as Record<string, unknown>;
          state.filterBodies.push(body);
          await route.fulfill({
            json: envelope({
              filter: {
                displayName: body.displayName,
                filter: body.filter,
                filterNo: "FSF_FINANCE_0001",
                updatedAt: 1_786_406_400_000,
                version: 1,
                viewCode: body.viewCode,
              },
            }),
          });
          return;
        }
        const saved = state.filterBodies.at(-1);
        await route.fulfill({
          json: envelope({
            filters: saved
              ? [
                  {
                    displayName: saved.displayName,
                    filter: saved.filter,
                    filterNo: "FSF_FINANCE_0001",
                    updatedAt: 1_786_406_400_000,
                    version: 1,
                    viewCode: saved.viewCode,
                  },
                ]
              : [],
          }),
        });
        return;
      }
      if (request.method() === "POST" && path.endsWith("/exports")) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.exportBodies.push(body);
        await route.fulfill({
          json: envelope({
            export: {
              exportType: body.exportType,
              failureCode: "",
              requestNo: "FEX_FINANCE_0001",
              requestedAt: 1_786_406_400_000,
              reused: false,
              rowCount: 0,
              status: "queued",
            },
          }),
        });
        return;
      }
      if (
        request.method() === "POST" &&
        path.endsWith("/exports/FEX_FINANCE_0001/generation")
      ) {
        await route.fulfill({
          json: envelope({
            export: {
              expiresAt: 1_786_492_800_000,
              exportType: "invoice",
              failureCode: "",
              generatedAt: 1_786_406_400_000,
              requestNo: "FEX_FINANCE_0001",
              requestedAt: 1_786_406_400_000,
              reused: false,
              rowCount: 1,
              status: "ready",
            },
          }),
        });
        return;
      }
      if (
        request.method() === "POST" &&
        path.endsWith("/manual-receipts/proofs")
      ) {
        state.proofBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: envelope({
            proof: {
              expiresAt: 1_786_406_520_000,
              proofToken: "opaque-finance-proof",
            },
          }),
        });
        return;
      }
      if (request.method() === "POST" && path.endsWith("/manual-receipts")) {
        state.executeBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: envelope({
            adjustment: {
              ...adjustment(),
              adjustmentNo: "BAR_FINANCE_CREATED",
              reused: false,
            },
          }),
        });
        return;
      }
      if (path.endsWith("/overview")) {
        await route.fulfill({
          json: envelope({
            overview: {
              openInvoiceCount: 7,
              overdueInvoiceCount: 2,
              pendingAdjustmentCount: 1,
              receivables: [
                { currencyCode: "CNY", outstandingMinor: "900719925474099301" },
              ],
            },
          }),
        });
        return;
      }
      if (path.endsWith(`/invoices/${invoiceNo}`)) {
        await route.fulfill({
          json: envelope({
            detail: {
              invoice: invoice(),
              items: [
                {
                  descriptionEnUS: "High-memory server",
                  descriptionZhCN: "高内存云服务器",
                  invoiceItemNo: "IVI_FINANCE_0001",
                  lineNumber: 1,
                  totalAmountMinor: "900719925474099301",
                },
              ],
              nextItemCursor: "",
            },
          }),
        });
        return;
      }
      if (path.endsWith("/invoices")) {
        state.invoiceRequests.push(url);
        const rows = Array.from({ length: 11 }, (_, index) =>
          index === 0
            ? invoice()
            : { ...invoice(), invoiceNo: `INV_FINANCE_PAGE_${index}` },
        );
        const size = Number(url.searchParams.get("limit") || 10);
        const offset = (Number(url.searchParams.get("page") || 1) - 1) * size;
        await route.fulfill({
          json: envelope({
            invoices: rows.slice(offset, offset + size),
            nextCursor: "",
            total: rows.length,
          }),
        });
        return;
      }
      if (path.endsWith(`/transactions/${transactionNo}`)) {
        await route.fulfill({
          json: envelope({
            detail: {
              ...transaction(),
              entries: [
                {
                  accountTypeCode: "cash",
                  amountMinor: "8800",
                  directionCode: "debit",
                  entryNo: "BLE_1",
                  lineNumber: 1,
                },
                {
                  accountTypeCode: "revenue",
                  amountMinor: "8800",
                  directionCode: "credit",
                  entryNo: "BLE_2",
                  lineNumber: 2,
                },
              ],
              nextEntryCursor: "",
              postedBy: 1001,
              sourceReference: "INTERNAL_SOURCE_0001",
              sourceTypeCode: "manual_receipt",
            },
          }),
        });
        return;
      }
      if (path.endsWith("/transactions")) {
        state.transactionRequests.push(url);
        await route.fulfill({
          json: envelope({
            nextCursor: "",
            transactions: [transaction()],
            total: 1,
          }),
        });
        return;
      }
      if (path.endsWith(`/adjustments/${adjustmentNo}`)) {
        await route.fulfill({
          json: envelope({
            detail: {
              adjustment: adjustment(),
              evidenceDigest: "a".repeat(64),
              reviewerId: 2002,
              version: 1,
            },
          }),
        });
        return;
      }
      if (path.endsWith("/adjustments")) {
        await route.fulfill({
          json: envelope({
            adjustments: [adjustment()],
            nextCursor: "",
            total: 1,
          }),
        });
        return;
      }
      await route.fulfill({ status: 404, json: envelope({}) });
    },
  );
  return state;
}

export const financialInvoiceNo = invoiceNo;
