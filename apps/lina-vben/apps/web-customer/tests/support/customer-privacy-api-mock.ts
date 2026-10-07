import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

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

export async function installCustomerPrivacyMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const english = options.locale === 'en-US';
  const state = {
    requests: [] as Array<Record<string, unknown>>,
    withdrawals: [] as Array<Record<string, unknown>>,
  };
  let consentAction = 'granted';
  const requests: Array<Record<string, unknown>> = [
    {
      completedAt: 0,
      holdReasonCode: 'tax',
      requestNo: 'PCR_CUSTOMER_001',
      requestTypeCode: 'delete',
      statusCode: 'retention_blocked',
      submittedAt: 1_787_020_000_000,
      version: 2,
    },
  ];

  await page.route(
    '**/x/idc-compliance/api/v1/account/compliance/privacy**',
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const body = (request.postDataJSON() ?? {}) as Record<string, unknown>;

      if (request.method() === 'GET' && url.pathname.endsWith('/privacy')) {
        await fulfill(route, {
          consents: [
            {
              actionCode: consentAction,
              consentNo: 'PVC_CUSTOMER_001',
              consentTypeCode: 'sensitive_personal_information',
              decidedAt: 1_787_020_000_000,
            },
          ],
          notices: [
            {
              assessmentNo: 'PIA_PERSONAL_001',
              assessmentTypeCode: 'personal_verification',
              impact: english
                ? 'Withdrawal stops new verification that depends on this consent.'
                : '撤回后，依赖这项授权的新认证会停止。',
              necessity: english
                ? 'Required for the merchant local compliance process.'
                : '用于完成当前商家的本地合规要求。',
              purpose: english
                ? 'Verify the account owner before the required business action.'
                : '在需要的业务操作前确认账户本人身份。',
              recipients: english
                ? 'The merchant and its configured local verification supplier.'
                : '当前商家及其配置的本地认证服务商。',
              retentionDays: 365,
              retentionDescription: english
                ? 'Uses the active merchant-configured policy.'
                : '使用当前商家配置并生效的保留规则。',
              rightsEntry: english
                ? 'Use this Privacy and Authorization page.'
                : '使用客户中心的“隐私与授权”页面。',
              storageLocation: english
                ? 'This installation and the configured local supplier.'
                : '当前安装及商家配置的本地服务商。',
              version: 3,
            },
          ],
          requests,
        });
        return;
      }
      if (
        request.method() === 'POST' &&
        url.pathname.endsWith('/privacy/consents/withdraw')
      ) {
        state.withdrawals.push(body);
        consentAction = 'withdrawn';
        await fulfill(route, {
          consent: {
            actionCode: consentAction,
            consentNo: 'PVC_CUSTOMER_002',
            consentTypeCode: body.consentTypeCode,
            decidedAt: Date.now(),
          },
        });
        return;
      }
      if (
        request.method() === 'POST' &&
        url.pathname.endsWith('/privacy/requests')
      ) {
        state.requests.push(body);
        requests.unshift({
          completedAt: 0,
          holdReasonCode: '',
          requestNo: `PCR_CUSTOMER_${state.requests.length + 1}`,
          requestTypeCode: body.requestTypeCode,
          statusCode: 'submitted',
          submittedAt: Date.now(),
          version: 1,
        });
        await fulfill(route, { request: requests[0] });
        return;
      }
      await route.fallback();
    },
  );
  return state;
}
