import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const now = 1_786_406_400_000;
export const creditInvoiceNo = 'INV_BALANCE_0001';
export const creditStatementNo = 'CRS_CREDIT_0001';

type Body = Record<string, unknown>;

export interface CustomerCreditMockState {
  rowCount: number;
  failLists: boolean;
  deniedDirectories: Partial<Record<string, 401 | 403>>;
  deniedRepayment: boolean;
  listRequests: Array<{ directory: string; page: number; limit: number }>;
  applicationBodies: Body[];
  creditPayments: string[];
  proofBodies: Body[];
  repaymentBodies: Body[];
  reserveBodies: Body[];
  uploadBodies: Uint8Array[];
}

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

function account(statusCode = 'active') {
  return {
    availableLimitMinor: '50000',
    billedPrincipalMinor: '8800',
    creditAccountNo: 'CRA_CREDIT_0001',
    currencyCode: 'CNY',
    delinquentPrincipalMinor: statusCode === 'frozen' ? '8800' : '0',
    freezeReasonCode: statusCode === 'frozen' ? 'statement_overdue' : undefined,
    statusCode,
    totalLimitMinor: '100000',
    unbilledPrincipalMinor: '0',
    validFrom: now - 86_400_000,
    validUntil: now + 365 * 86_400_000,
  };
}

export async function installCustomerCreditMock(
  page: Page,
  options: {
    baseMock?: boolean;
    locale?: 'en-US' | 'zh-CN';
    frozen?: boolean;
  } = {},
) {
  if (options.baseMock !== false) {
    await mockCustomerAPIs(page, options);
  }
  const state: CustomerCreditMockState = {
    rowCount: 1,
    failLists: false,
    deniedDirectories: {},
    deniedRepayment: false,
    listRequests: [],
    applicationBodies: [],
    creditPayments: [],
    proofBodies: [],
    repaymentBodies: [],
    reserveBodies: [],
    uploadBodies: [],
  };
  let applicationStatus = 'pending_review';
  let statementStatus = options.frozen ? 'overdue' : 'open';
  let statementOutstanding = '8800';

  await page.route(
    '**/x/idc-core/api/v1/account/billing/credit/**',
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const method = request.method();
      const directory = /^.*\/credit\/(applications|statements)$/u.exec(path)?.[1];
      const currentPage = Number(url.searchParams.get('page') || 1);
      const pageSize = Number(url.searchParams.get('limit') || 10);
      const scope = directory || (path.endsWith('/credit/overview') ? 'overview' : '');
      const denied = state.deniedDirectories[scope];
      if ((denied && method === 'GET') || (state.deniedRepayment && path.endsWith('/balance-repayment-proofs'))) {
        await route.fulfill({ status: denied || 403, json: { code: denied || 403, message: 'Access denied' } });
        return;
      }
      if (directory && method === 'GET') {
        state.listRequests.push({ directory, page: currentPage, limit: pageSize });
        if (state.failLists) {
          await route.fulfill({ status: 503, json: { code: 500, message: 'Credit data unavailable' } });
          return;
        }
      }
      let data: unknown;
      if (path.endsWith('/credit/overview') && method === 'GET') {
        data = {
          overview: {
            accounts: [account(options.frozen ? 'frozen' : 'active')],
            openStatementCount: statementStatus === 'paid' ? 0 : 1,
            overdueStatementCount: statementStatus === 'overdue' ? 1 : 0,
            pendingApplicationCount:
              applicationStatus === 'pending_review' ? 1 : 0,
          },
        };
      } else if (
        path.endsWith('/credit/evidence-uploads') &&
        method === 'POST'
      ) {
        const body = request.postDataJSON() as Body;
        state.reserveBodies.push(body);
        data = {
          expiresAt: now + 3_600_000,
          uploadNo: `CEU_CREDIT_${state.reserveBodies.length}`,
        };
      } else if (path.endsWith('/content') && method === 'PUT') {
        state.uploadBodies.push(
          new Uint8Array(request.postDataBuffer() ?? new Uint8Array()),
        );
        data = {
          fileName: 'bank-statement.pdf',
          mediaType: 'application/pdf',
          sizeBytes: state.uploadBodies.at(-1)?.byteLength ?? 0,
          uploadNo: 'CEU_CREDIT_1',
        };
      } else if (path.endsWith('/credit/applications')) {
        if (method === 'POST') {
          state.applicationBodies.push(request.postDataJSON() as Body);
          applicationStatus = 'pending_review';
        }
        data = {
          applications: [
            {
              applicationNo: 'CAP_CREDIT_0001',
              createdAt: now,
              currencyCode: 'CNY',
              requestedLimitMinor: '100000',
              requestedValidFrom: now,
              requestedValidUntil: now + 365 * 86_400_000,
              requiredApprovalCount: 2,
              reviewCount: 0,
              statusCode: applicationStatus,
            },
          ],
          nextCursor: '',
        };
        if (method === 'POST') {
          data = {
            application: (
              data as { applications: Array<Record<string, unknown>> }
            ).applications[0],
          };
        }
      } else if (
        path.endsWith('/balance-repayment-proofs') &&
        method === 'POST'
      ) {
        state.proofBodies.push(request.postDataJSON() as Body);
        data = { proof: { proofToken: 'opaque-credit-repayment-proof' } };
      } else if (path.endsWith('/balance-repayments') && method === 'POST') {
        state.repaymentBodies.push(request.postDataJSON() as Body);
        statementOutstanding = '0';
        statementStatus = 'paid';
        data = { repayment: { statusCode: 'succeeded' } };
      } else if (path.endsWith('/credit/statements') && method === 'GET') {
        data = {
          nextCursor: '',
          statements: [
            {
              availableBalanceMinor: '10000',
              canRepayFromBalance: statementStatus !== 'paid',
              currencyCode: 'CNY',
              outstandingAmountMinor: statementOutstanding,
              paidAmountMinor: statementStatus === 'paid' ? '8800' : '0',
              paymentDueAt: now - 86_400_000,
              statementNo: creditStatementNo,
              statusCode: statementStatus,
              totalAmountMinor: '8800',
            },
          ],
        };
      } else {
        await route.fulfill({ json: envelope({}), status: 404 });
        return;
      }
      if (directory && method === 'GET') {
        const response = data as Record<string, unknown>;
        const first = (response[directory] as Record<string, unknown>[])[0]!;
        const rowKey = directory === 'applications' ? 'applicationNo' : 'statementNo';
        const rows = Array.from({ length: state.rowCount }, (_, index) => ({ ...first, [rowKey]: index === 0 ? first[rowKey] : `${first[rowKey]}_${index + 1}` }));
        response.total = rows.length;
        response[directory] = rows.slice((currentPage - 1) * pageSize, currentPage * pageSize);
      }
      await route.fulfill({ json: envelope(data) });
    },
  );

  await page.route(
    `**/x/idc-core/api/v1/account/billing/invoices/${creditInvoiceNo}/credit-settlements`,
    async (route) => {
      state.creditPayments.push(creditInvoiceNo);
      await route.fulfill({
        json: envelope({
          settlement: {
            availableLimitMinor: '41200',
            creditAmountMinor: '8800',
            statusCode: 'settled',
          },
        }),
      });
    },
  );

  return state;
}
