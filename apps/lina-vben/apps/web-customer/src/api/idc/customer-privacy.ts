import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-compliance';

export interface CustomerPrivacyNotice {
  assessmentNo: string;
  assessmentTypeCode: string;
  impact: string;
  necessity: string;
  purpose: string;
  recipients: string;
  retentionDays: number;
  retentionDescription: string;
  rightsEntry: string;
  storageLocation: string;
  version: number;
}

export interface CustomerPrivacyConsent {
  actionCode: 'granted' | 'withdrawn';
  consentNo: string;
  consentTypeCode:
    | 'enterprise_verification'
    | 'sensitive_personal_information';
  decidedAt: number;
}

export interface CustomerPrivacyRequest {
  completedAt: number;
  holdReasonCode: string;
  requestNo: string;
  requestTypeCode: 'delete' | 'export';
  statusCode: string;
  submittedAt: number;
  version: number;
}

export interface CustomerPrivacyOverview {
  consents: CustomerPrivacyConsent[];
  notices: CustomerPrivacyNotice[];
  requests: CustomerPrivacyRequest[];
}

function path(name = '') {
  return pluginApiPath(
    pluginID,
    `account/compliance/privacy${name ? `/${name}` : ''}`,
  );
}

export function getCustomerPrivacyOverview() {
  return requestClient.get<CustomerPrivacyOverview>(path());
}

export function withdrawCustomerPrivacyConsent(input: {
  consentTypeCode: CustomerPrivacyConsent['consentTypeCode'];
  idempotencyKey: string;
}) {
  return requestClient.post<{ consent: CustomerPrivacyConsent }>(
    path('consents/withdraw'),
    input,
  );
}

export function createCustomerPrivacyRequest(input: {
  idempotencyKey: string;
  requestTypeCode: CustomerPrivacyRequest['requestTypeCode'];
}) {
  return requestClient.post<{ request: CustomerPrivacyRequest }>(
    path('requests'),
    input,
  );
}
