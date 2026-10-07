import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const now = 1_786_480_800_000;

type InitialState =
  | 'draft'
  | 'pending'
  | 'rejected'
  | 'unverified'
  | 'verified';

type Material = {
  byteSize: number;
  materialNo: string;
  materialTypeCode: string;
  mediaType: string;
  sequenceNo: number;
  statusCode: string;
};

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data), status: 200 });
}

function profile() {
  return {
    expiresAt: 0,
    industryCode: 'IDC',
    legalName: 'Example Verified Technology Co., Ltd.',
    profileVersion: 7,
    registrationCountryCode: 'CN',
    registrationIdentifierMasked: '91************0Y46',
    verificationCaseNo: 'ent_verified_0001',
    verificationStatusCode: 'verified',
    verifiedAt: now - 86_400_000,
  };
}

function createCase(
  status: Exclude<InitialState, 'unverified'>,
  materials: Material[] = [],
) {
  return {
    caseNo: status === 'verified' ? 'ent_verified_0001' : 'ent_case_0001',
    decisionReasonCode: status === 'rejected' ? 'materials_incomplete' : '',
    expiresAt: 0,
    legalChangeRequestNo: '',
    materials,
    rejectedAt: status === 'rejected' ? now : 0,
    reverification: false,
    statusCode: status,
    submittedAt: ['pending', 'rejected', 'verified'].includes(status) ? now : 0,
    updatedAt: now,
    verifiedAt: status === 'verified' ? now : 0,
    version: 2,
  };
}

export interface EnterpriseVerificationMockState {
  consentRequests: Array<Record<string, unknown>>;
  draftRequests: Array<Record<string, unknown>>;
  legalChangeRequests: Array<Record<string, unknown>>;
  materialContentTypes: string[];
  materialRawBodies: string[];
  reservationRequests: Array<Record<string, unknown>>;
  submissionRequests: Array<Record<string, unknown>>;
}

export async function installEnterpriseVerificationMock(
  page: Page,
  options: {
    initialState?: InitialState;
    locale?: 'en-US' | 'zh-CN';
  } = {},
) {
  await mockCustomerAPIs(page, { locale: options.locale });
  // Enterprise verification first checks the signed-in customer type before
  // loading the compliance state. Keep that prerequisite in the fixture so
  // tests exercise the actual verification flow instead of the service-error
  // fallback caused by an empty generic response.
  await page.route(
    '**/x/idc-core/api/v1/account/customer/profile',
    async (route) => {
      await fulfill(route, {
        item: {
          countryCode: 'CN',
          customerCode: '10000',
          customerNo: 'CUS_00000000000000000000000000000001',
          displayName: '当前登录客户',
          identifierMasked: '913***********0000',
          legalNameMasked: '示例科技',
          preferredLocale: options.locale || 'zh-CN',
          status: 'active',
          type: 'enterprise',
          verification: 'verified',
          version: 1,
        },
      });
    },
  );
  const initialState = options.initialState || 'unverified';
  const calls: EnterpriseVerificationMockState = {
    consentRequests: [],
    draftRequests: [],
    legalChangeRequests: [],
    materialContentTypes: [],
    materialRawBodies: [],
    reservationRequests: [],
    submissionRequests: [],
  };
  let currentProfile = initialState === 'verified' ? profile() : null;
  let activeLegalChange: null | Record<string, unknown> = null;
  let currentCase =
    initialState === 'unverified' ? null : createCase(initialState);
  let sequence = 0;
  const root = '/x/idc-compliance/api/v1/account/compliance';

  function accountState() {
    return {
      activeLegalChange,
      case: currentCase,
      policyVersion: 'enterprise-notice-v1',
      profile: currentProfile,
    };
  }

  await page.route(
    '**/x/idc-compliance/api/v1/account/compliance/**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const method = request.method();

      if (path === `${root}/enterprise-verification` && method === 'GET') {
        await fulfill(route, { item: accountState() });
        return;
      }
      if (
        path === `${root}/enterprise-verification-consents` &&
        method === 'POST'
      ) {
        calls.consentRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await fulfill(route, {
          consentNo: 'enterprise_consent_0001',
          occurredAt: now,
          policyVersion: 'enterprise-notice-v1',
          reused: false,
        });
        return;
      }
      if (path === `${root}/enterprise-verifications` && method === 'POST') {
        const input = request.postDataJSON() as Record<string, unknown>;
        calls.draftRequests.push(input);
        currentCase = {
          ...createCase('draft'),
          caseNo: `ent_case_${calls.draftRequests.length.toString().padStart(4, '0')}`,
          legalChangeRequestNo: String(input.legalChangeRequestNo || ''),
          reverification: Boolean(input.legalChangeRequestNo),
        };
        await fulfill(route, {
          item: {
            caseNo: currentCase.caseNo,
            legalChangeRequestNo: currentCase.legalChangeRequestNo,
            reused: false,
            reverification: currentCase.reverification,
            statusCode: 'draft',
          },
        });
        return;
      }
      if (
        path === `${root}/enterprise-legal-change-requests` &&
        method === 'POST'
      ) {
        const input = request.postDataJSON() as Record<string, unknown>;
        calls.legalChangeRequests.push(input);
        activeLegalChange = {
          currentProfileVersion: 7,
          fieldCodes: input.fieldCodes,
          reasonCode: input.reasonCode,
          requestNo: 'LCR_change_0001',
          statusCode: 'submitted',
          submittedAt: now,
          verificationCaseNo: 'ent_verified_0001',
        };
        await fulfill(route, { item: activeLegalChange });
        return;
      }
      if (
        path.endsWith('/materials') &&
        path.includes('/enterprise-verifications/') &&
        method === 'POST'
      ) {
        const input = request.postDataJSON() as Record<string, unknown>;
        calls.reservationRequests.push(input);
        sequence += 1;
        await fulfill(route, {
          item: {
            materialNo: `material_${sequence}`,
            materialTypeCode: input.materialTypeCode,
            reused: false,
            statusCode: 'reserved',
          },
        });
        return;
      }
      if (
        path.includes('/enterprise-verifications/') &&
        path.includes('/materials/') &&
        path.endsWith('/content') &&
        method === 'PUT'
      ) {
        const materialNo = decodeURIComponent(path.split('/').at(-2) || '');
        const reservation = calls.reservationRequests[sequence - 1] || {};
        const raw = request.postDataBuffer()?.toString() || '';
        calls.materialContentTypes.push(
          request.headers()['content-type'] || '',
        );
        calls.materialRawBodies.push(raw);
        const material = {
          byteSize: Buffer.byteLength(raw),
          materialNo,
          materialTypeCode: String(reservation.materialTypeCode || ''),
          mediaType: request.headers()['content-type'] || '',
          sequenceNo: sequence,
          statusCode: 'available',
        };
        if (currentCase) {
          currentCase = {
            ...currentCase,
            materials: [
              ...currentCase.materials.filter(
                (item) => item.materialTypeCode !== material.materialTypeCode,
              ),
              material,
            ],
          };
        }
        await fulfill(route, { item: material });
        return;
      }
      if (
        path.endsWith('/submission') &&
        path.includes('/enterprise-verifications/') &&
        method === 'POST'
      ) {
        const input = request.postDataJSON() as Record<string, unknown>;
        calls.submissionRequests.push(input);
        if (currentCase) {
          currentCase = {
            ...currentCase,
            statusCode: 'pending',
            submittedAt: now,
            updatedAt: now,
          };
        }
        await fulfill(route, {
          item: {
            caseNo: currentCase?.caseNo,
            reused: false,
            reverification: currentCase?.reverification || false,
            statusCode: 'pending',
          },
        });
        return;
      }
      await route.fulfill({
        json: { code: 404, data: null, message: 'not found' },
        status: 404,
      });
    },
  );

  return calls;
}
