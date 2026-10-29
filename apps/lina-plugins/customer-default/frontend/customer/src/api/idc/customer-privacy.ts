import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-compliance';

export interface CustomerPrivacyConsent {
  actionCode: 'granted' | 'withdrawn';
  consentNo: string;
  consentTypeCode:
    | 'enterprise_verification'
    | 'sensitive_personal_information';
  decidedAt: number;
}

export interface CustomerPrivacyOverview {
  consents: CustomerPrivacyConsent[];
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
