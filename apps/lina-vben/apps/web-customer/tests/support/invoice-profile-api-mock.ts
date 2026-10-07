import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type Profile = {
  bankAccountConfigured: boolean;
  bankAccountMasked: string;
  bankName: string;
  createdAt: number;
  invoiceTitle: string;
  isDefault: boolean;
  profileNo: string;
  registeredAddress: string;
  registeredPhone: string;
  statusCode: 'active' | 'archived';
  taxRegistrationNo: string;
  titleTypeCode: 'enterprise' | 'individual';
  updatedAt: number;
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

function masked(value: string) {
  return `****${value.slice(-4)}`;
}

export async function installInvoiceProfileMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  let sequence = 2;
  let profiles: Profile[] = [
    {
      bankAccountConfigured: true,
      bankAccountMasked: '****5678',
      bankName: '测试银行',
      createdAt: 1_786_320_000_000,
      invoiceTitle: '原默认企业',
      isDefault: true,
      profileNo: 'ivp-existing-enterprise',
      registeredAddress: '上海',
      registeredPhone: '021-12345678',
      statusCode: 'active',
      taxRegistrationNo: 'TAX-EXISTING',
      titleTypeCode: 'enterprise',
      updatedAt: 1_786_320_000_000,
      version: 1,
    },
    {
      bankAccountConfigured: false,
      bankAccountMasked: '',
      bankName: '',
      createdAt: 1_786_319_000_000,
      invoiceTitle: '个人备用抬头',
      isDefault: false,
      profileNo: 'ivp-existing-personal',
      registeredAddress: '',
      registeredPhone: '',
      statusCode: 'active',
      taxRegistrationNo: '',
      titleTypeCode: 'individual',
      updatedAt: 1_786_319_000_000,
      version: 1,
    },
  ];

  await page.route(
    '**/x/idc-core/api/v1/account/billing/invoice-profiles**',
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const segments = url.pathname.split('/').filter(Boolean);
      const profileIndex = segments.lastIndexOf('invoice-profiles');
      const profileNo = decodeURIComponent(segments[profileIndex + 1] || '');
      const action = segments[profileIndex + 2] || '';

      if (request.method() === 'GET') {
        await fulfill(route, { items: profiles, nextCursor: '' });
        return;
      }
      if (request.method() === 'POST' && !profileNo) {
        const input = request.postDataJSON() as Record<string, unknown>;
        sequence += 1;
        const bankAccount = String(input.bankAccount || '');
        const item: Profile = {
          bankAccountConfigured: bankAccount !== '',
          bankAccountMasked: bankAccount ? masked(bankAccount) : '',
          bankName: String(input.bankName || ''),
          createdAt: Date.now(),
          invoiceTitle: String(input.invoiceTitle || ''),
          isDefault: input.isDefault === true,
          profileNo: `ivp-created-${sequence}`,
          registeredAddress: String(input.registeredAddress || ''),
          registeredPhone: String(input.registeredPhone || ''),
          statusCode: 'active',
          taxRegistrationNo: String(input.taxRegistrationNo || ''),
          titleTypeCode: input.titleTypeCode as Profile['titleTypeCode'],
          updatedAt: Date.now(),
          version: 1,
        };
        if (item.isDefault) {
          profiles = profiles.map((profile) => ({ ...profile, isDefault: false }));
        }
        profiles = [item, ...profiles];
        await fulfill(route, { item });
        return;
      }
      const index = profiles.findIndex((profile) => profile.profileNo === profileNo);
      const current = profiles[index];
      if (!current) {
        await route.fulfill({ status: 404 });
        return;
      }
      if (request.method() === 'PUT') {
        const input = request.postDataJSON() as Record<string, unknown>;
        const bankAction = String(input.bankAccountAction || 'keep');
        const bankAccount = String(input.bankAccount || '');
        const next: Profile = {
          ...current,
          bankAccountConfigured:
            bankAction === 'clear'
              ? false
              : bankAction === 'replace'
                ? true
                : current.bankAccountConfigured,
          bankAccountMasked:
            bankAction === 'clear'
              ? ''
              : bankAction === 'replace'
                ? masked(bankAccount)
                : current.bankAccountMasked,
          bankName: String(input.bankName || ''),
          invoiceTitle: String(input.invoiceTitle || ''),
          registeredAddress: String(input.registeredAddress || ''),
          registeredPhone: String(input.registeredPhone || ''),
          taxRegistrationNo: String(input.taxRegistrationNo || ''),
          titleTypeCode: input.titleTypeCode as Profile['titleTypeCode'],
          updatedAt: Date.now(),
          version: current.version + 1,
        };
        profiles[index] = next;
        await fulfill(route, { item: next });
        return;
      }
      if (request.method() === 'POST' && action === 'default') {
        profiles = profiles.map((profile) => ({
          ...profile,
          isDefault: profile.profileNo === profileNo,
          version:
            profile.isDefault || profile.profileNo === profileNo
              ? profile.version + 1
              : profile.version,
        }));
        await fulfill(route, {
          item: profiles.find((profile) => profile.profileNo === profileNo),
        });
        return;
      }
      if (request.method() === 'POST' && action === 'deactivation') {
        const next: Profile = {
          ...current,
          isDefault: false,
          statusCode: 'archived',
          updatedAt: Date.now(),
          version: current.version + 1,
        };
        profiles[index] = next;
        await fulfill(route, { item: next });
        return;
      }
      await route.fallback();
    },
  );
}
