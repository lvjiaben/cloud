import { expect, test } from '@playwright/test';

import { EnterpriseVerificationPage } from '../pages/EnterpriseVerificationPage';
import { installEnterpriseVerificationMock } from '../support/enterprise-verification-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

const legalDetails = {
  legalName: '测试科技有限公司',
  registrationIdentifier: '91350211M000100Y46',
  representativeIdentityNumber: '110101199001010015',
  representativeName: '测试法人',
};

async function uploadRequiredEvidence(enterprise: EnterpriseVerificationPage) {
  await enterprise.upload(
    'business_license',
    'business-license-bytes',
    '测试科技有限公司营业执照.pdf',
  );
  await enterprise.upload(
    'legal_representative_id',
    'representative-id-bytes',
    '测试法人身份证.pdf',
  );
}

test.describe('TC005 客户企业认证与资料变更', () => {
  test('TC005a: 单独同意、加密材料上传和人工审核状态均由服务端闭环', async ({
    page,
  }) => {
    const calls = await installEnterpriseVerificationMock(page);
    const enterprise = new EnterpriseVerificationPage(page);
    await enterprise.login();
    await enterprise.open();

    await expect(enterprise.status).toHaveText('未认证');
    await expect(enterprise.root).toContainText('不发送给软件提供方');
    await enterprise.begin();
    await expect(enterprise.status).toHaveText('草稿');
    await enterprise.status.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(
      page,
      'customer-enterprise-verification-draft-zh-cn',
    );

    await enterprise.fillLegalDetails(legalDetails);
    await uploadRequiredEvidence(enterprise);
    await enterprise.submitButton.click();
    await expect(enterprise.status).toHaveText('待审核');
    await expect(enterprise.root).toContainText('正在人工审核');

    expect(calls.consentRequests).toHaveLength(1);
    expect(calls.consentRequests[0]).toMatchObject({
      accepted: true,
      locale: 'zh-CN',
      policyVersion: 'enterprise-notice-v1',
    });
    expect(calls.reservationRequests).toEqual([
      expect.objectContaining({ materialTypeCode: 'business_license' }),
      expect.objectContaining({
        materialTypeCode: 'legal_representative_id',
      }),
    ]);
    for (const request of calls.reservationRequests) {
      expect(request).not.toHaveProperty('fileName');
      expect(request).not.toHaveProperty('customerNo');
    }
    expect(calls.materialRawBodies).toEqual([
      'business-license-bytes',
      'representative-id-bytes',
    ]);
    expect(calls.submissionRequests[0]).toMatchObject({
      applicantRoleCode: 'legal_representative',
      legalName: legalDetails.legalName,
      registrationCountryCode: 'CN',
      registrationIdentifier: legalDetails.registrationIdentifier,
    });
    expect(calls.submissionRequests[0]).not.toHaveProperty('customerNo');

    const stored = await page.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    const storedText = stored.join('\n');
    expect(storedText).not.toContain(legalDetails.legalName);
    expect(storedText).not.toContain(legalDetails.registrationIdentifier);
    expect(storedText).not.toContain(legalDetails.representativeIdentityNumber);
    expect(storedText).not.toContain('测试科技有限公司营业执照.pdf');
    expect(storedText).not.toContain('material_1');
    await captureCustomerScreenshot(
      page,
      'customer-enterprise-verification-pending-zh-cn',
    );
  });

  test('TC005b: 审核退回会显示明确原因并允许修正后重新提交', async ({
    page,
  }) => {
    const calls = await installEnterpriseVerificationMock(page, {
      initialState: 'rejected',
    });
    const enterprise = new EnterpriseVerificationPage(page);
    await enterprise.login();
    await enterprise.open();

    await expect(enterprise.status).toHaveText('已退回');
    await expect(enterprise.root).toContainText('证明材料不完整');
    await captureCustomerScreenshot(
      page,
      'customer-enterprise-verification-rejected-zh-cn',
    );
    await enterprise.begin();
    await enterprise.fillLegalDetails(legalDetails);
    await uploadRequiredEvidence(enterprise);
    await enterprise.submitButton.click();
    await expect(enterprise.status).toHaveText('待审核');
    expect(calls.submissionRequests).toHaveLength(1);
  });

  test('TC005c: 认证后变更只先提交变更类别，旧认证资料继续有效', async ({
    page,
  }) => {
    const calls = await installEnterpriseVerificationMock(page, {
      initialState: 'verified',
    });
    const enterprise = new EnterpriseVerificationPage(page);
    await enterprise.login();
    await enterprise.open();

    await expect(enterprise.status).toHaveText('已认证');
    await expect(page.getByTestId('enterprise-current-profile')).toContainText(
      'Example Verified Technology Co., Ltd.',
    );
    await page
      .getByTestId('enterprise-change-fields')
      .getByText('企业名称')
      .click();
    await page.getByTestId('enterprise-change-reason').click();
    await page.getByText('企业名称变更', { exact: true }).click();
    await page.getByTestId('enterprise-create-change').click();

    expect(calls.legalChangeRequests).toEqual([
      expect.objectContaining({
        expectedProfileVersion: 7,
        fieldCodes: ['legal_name'],
        reasonCode: 'legal_name_change',
      }),
    ]);
    expect(calls.legalChangeRequests[0]).not.toHaveProperty('legalName');
    expect(calls.legalChangeRequests[0]).not.toHaveProperty(
      'registrationIdentifier',
    );
    await expect(page.getByTestId('enterprise-current-profile')).toContainText(
      'Example Verified Technology Co., Ltd.',
    );
    await enterprise.begin();
    await expect(page.getByTestId('enterprise-current-profile')).toContainText(
      'Example Verified Technology Co., Ltd.',
    );
    expect(calls.draftRequests[0]).toMatchObject({
      legalChangeRequestNo: 'LCR_change_0001',
    });
    await enterprise.status.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(
      page,
      'customer-enterprise-verification-change-draft-zh-cn',
    );
  });
});

test.describe('TC005 客户企业认证英文移动端', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC005d: 英文移动端完整展示本地处理边界且无横向溢出', async ({
    page,
  }) => {
    await installEnterpriseVerificationMock(page, { locale: 'en-US' });
    const enterprise = new EnterpriseVerificationPage(page);
    await enterprise.login();
    await enterprise.open();

    await expect(enterprise.root).toContainText('Business verification center');
    await expect(enterprise.root).toContainText(
      'It is not sent to the software provider',
    );
    await expect(enterprise.root).not.toContainText(
      'enterpriseVerification.notice.title',
    );
    expect(await enterprise.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(
      page,
      'customer-enterprise-verification-mobile-en-us',
    );
  });
});
