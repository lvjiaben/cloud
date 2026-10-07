import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
};

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

describe('enterprise verification customer APIs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the workflow inside the compliance account boundary', async () => {
    requestClient.get.mockResolvedValue({ item: { policyVersion: '1' } });
    requestClient.post
      .mockResolvedValueOnce({ consentNo: 'consent-1' })
      .mockResolvedValueOnce({ item: { caseNo: 'case-1' } })
      .mockResolvedValueOnce({ item: { requestNo: 'change-1' } })
      .mockResolvedValueOnce({ item: { materialNo: 'material/1' } })
      .mockResolvedValueOnce({ item: { statusCode: 'pending' } });
    requestClient.put.mockResolvedValue({
      item: { materialNo: 'material/1', statusCode: 'available' },
    });

    const api = await import('./enterprise-verification');
    const root = '/x/idc-compliance/api/v1/account/compliance';
    const file = new File(['pdf'], 'private-company-name.pdf', {
      type: 'application/pdf',
    });

    await api.getEnterpriseVerification();
    await api.createEnterpriseVerificationConsent({
      idempotencyKey: 'enterprise-consent-test-01',
      locale: 'zh-CN',
      policyVersion: '1',
    });
    await api.createEnterpriseVerificationDraft({
      consentNo: 'consent-1',
      idempotencyKey: 'enterprise-draft-test-01',
      legalChangeRequestNo: '',
    });
    await api.createEnterpriseLegalChange({
      expectedProfileVersion: 7,
      fieldCodes: ['legal_name'],
      idempotencyKey: 'enterprise-change-test-01',
      reasonCode: 'legal_name_change',
    });
    await api.uploadEnterpriseMaterial(
      'case/1',
      'business_license',
      file,
      'enterprise-material-test-01',
    );
    await api.submitEnterpriseVerification('case/1', {
      agentIdentityNumber: '',
      agentName: '',
      applicantRoleCode: 'legal_representative',
      idempotencyKey: 'enterprise-submit-test-01',
      industryCode: 'IDC',
      legalName: 'Example Company',
      legalRepresentativeIdentityNumber: '110101199001010015',
      legalRepresentativeName: 'Representative',
      materialNos: ['material/1'],
      registrationCountryCode: 'CN',
      registrationIdentifier: '91350211M000100Y46',
    });

    expect(requestClient.get).toHaveBeenCalledWith(
      `${root}/enterprise-verification`,
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      `${root}/enterprise-verification-consents`,
      expect.objectContaining({ accepted: true, policyVersion: '1' }),
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      4,
      `${root}/enterprise-verifications/case%2F1/materials`,
      {
        idempotencyKey: 'enterprise-material-test-01',
        materialTypeCode: 'business_license',
      },
      { silentErrorMessage: true },
    );
    expect(requestClient.put).toHaveBeenCalledWith(
      `${root}/enterprise-verifications/case%2F1/materials/material%2F1/content`,
      file,
      expect.objectContaining({
        headers: { 'Content-Type': 'application/pdf' },
      }),
    );
    expect(requestClient.put.mock.calls[0]).not.toContain(
      'private-company-name.pdf',
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      5,
      `${root}/enterprise-verifications/case%2F1/submission`,
      expect.objectContaining({
        legalName: 'Example Company',
        registrationIdentifier: '91350211M000100Y46',
      }),
      { silentErrorMessage: true },
    );
  });
});
