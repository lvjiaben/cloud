import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type ContractStatus = 'completed' | 'failed' | 'processing';

type ContractItem = {
  amountMinor: string;
  canOpenSigning: boolean;
  canRecheck: boolean;
  contractNo: string;
  createdAt: number;
  currencyCode: string;
  languageCode: 'en-US' | 'zh-CN';
  orderNo: string;
  signingExpiresAt: number;
  statusCode: ContractStatus;
  templateName: string;
  version: number;
};

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: 'OK' });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installCustomerContractMock(
  page: Page,
  options: {
    locale?: 'en-US' | 'zh-CN';
    unsafeSigningLink?: boolean;
  } = {},
) {
  await mockCustomerAPIs(page, options);
  const signingRequests: string[] = [];
  const authoritativeChecks: string[] = [];
  const contracts: ContractItem[] = [
    {
      amountMinor: '128800',
      canOpenSigning: true,
      canRecheck: true,
      contractNo: 'CONTRACT-PROCESSING',
      createdAt: 1_786_320_000_000,
      currencyCode: 'CNY',
      languageCode: 'zh-CN',
      orderNo: 'ORDER-2026-001',
      signingExpiresAt: 1_786_924_800_000,
      statusCode: 'processing',
      templateName: '服务器采购合同',
      version: 3,
    },
    {
      amountMinor: '29900',
      canOpenSigning: true,
      canRecheck: true,
      contractNo: 'CONTRACT-RETRY',
      createdAt: 1_786_321_000_000,
      currencyCode: 'USD',
      languageCode: 'en-US',
      orderNo: 'ORDER-2026-002',
      signingExpiresAt: 1_786_924_800_000,
      statusCode: 'processing',
      templateName: 'Server Service Agreement',
      version: 2,
    },
    {
      amountMinor: '8800',
      canOpenSigning: false,
      canRecheck: false,
      contractNo: 'CONTRACT-TERMINAL-FAILED',
      createdAt: 1_786_322_000_000,
      currencyCode: 'CNY',
      languageCode: 'zh-CN',
      orderNo: 'ORDER-2026-003',
      signingExpiresAt: 1_786_924_800_000,
      statusCode: 'failed',
      templateName: '终态失败合同',
      version: 4,
    },
  ];

  await page
    .context()
    .route('https://sign.tencent.example/**', async (route) => {
      await route.fulfill({
        body: '<!doctype html><title>Tencent H5 signing</title>',
        contentType: 'text/html; charset=utf-8',
        status: 200,
      });
    });

  await page.route(
    '**/x/idc-compliance/api/v1/account/compliance/contracts**',
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const signingMatch = url.pathname.match(
        /\/contracts\/([^/]+)\/signing-links$/u,
      );
      const recheckMatch = url.pathname.match(
        /\/contracts\/([^/]+)\/authoritative-state$/u,
      );

      if (request.method() === 'POST' && signingMatch) {
        const contractNo = decodeURIComponent(signingMatch[1] || '');
        const contract = contracts.find(
          (item) => item.contractNo === contractNo,
        );
        if (!contract) {
          await route.abort();
          return;
        }
        signingRequests.push(contractNo);
        await fulfill(route, {
          item: {
            contract,
            expiresAt: Date.now() + 1_800_000,
            url: options.unsafeSigningLink
              ? `http://sign.tencent.example/h5/${encodeURIComponent(contractNo)}`
              : `https://sign.tencent.example/h5/${encodeURIComponent(contractNo)}?entry=${signingRequests.length}`,
          },
        });
        return;
      }

      if (request.method() === 'PUT' && recheckMatch) {
        const contractNo = decodeURIComponent(recheckMatch[1] || '');
        const contract = contracts.find(
          (item) => item.contractNo === contractNo,
        );
        if (!contract) {
          await route.abort();
          return;
        }
        authoritativeChecks.push(contractNo);
        contract.statusCode = 'completed';
        contract.canOpenSigning = false;
        contract.canRecheck = false;
        contract.version += 1;
        await fulfill(route, { item: contract });
        return;
      }

      if (
        request.method() === 'GET' &&
        url.pathname.endsWith('/account/compliance/contracts')
      ) {
        await fulfill(route, { items: contracts, nextCursor: '' });
        return;
      }

      await route.fallback();
    },
  );

  return { authoritativeChecks, signingRequests };
}
