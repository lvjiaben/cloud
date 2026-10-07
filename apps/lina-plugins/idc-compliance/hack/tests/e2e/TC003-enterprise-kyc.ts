import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";

import { EnterpriseVerificationPage } from "../pages/EnterpriseVerificationPage";
import {
  enterpriseCaseNo,
  enterpriseMaterialNo,
  installEnterpriseCustomerSubmissionMock,
  installEnterpriseVerificationMock,
} from "../support/enterprise-verification-mock";

test.describe("TC-3 企业认证管理审核", () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, "idc-compliance");
  });

  test("TC-3a: 原件通过独立二进制接口私有上传，响应不返回存储地址或摘要", async ({
    adminPage,
  }) => {
    const state = await installEnterpriseCustomerSubmissionMock(adminPage);
    await adminPage.goto("/", { waitUntil: "domcontentloaded" });

    const result = await adminPage.evaluate(
      async ({ caseNo, materialNo }) => {
        const base =
          "/x/idc-compliance/api/v1/account/compliance/enterprise-verifications";
        const reserved = await fetch(`${base}/${caseNo}/materials`, {
          body: JSON.stringify({
            idempotencyKey: "enterprise-material-browser-0001",
            materialTypeCode: "business_license",
          }),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        }).then((response) => response.json());
        const uploaded = await fetch(
          `${base}/${caseNo}/materials/${materialNo}/content`,
          {
            body: new TextEncoder().encode("%PDF-1.4 private enterprise material"),
            headers: { "Content-Type": "application/octet-stream" },
            method: "PUT",
          },
        ).then((response) => response.json());
        return { reserved, uploaded };
      },
      { caseNo: enterpriseCaseNo, materialNo: enterpriseMaterialNo },
    );

    expect(result.reserved.data.item.materialNo).toBe(enterpriseMaterialNo);
    expect(result.uploaded.data.item.statusCode).toBe("available");
    expect(JSON.stringify(result)).not.toMatch(/private:\/\/|storage|sha256|digest/iu);
    expect(state.uploadedByteSizes[0]).toBeGreaterThan(20);
  });

  test("TC-3b: 脱敏详情、单次原件访问与服务端权威人工审核闭环", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installEnterpriseVerificationMock(adminPage);
    const enterprise = new EnterpriseVerificationPage(adminPage);
    await enterprise.goto();

    await expect(enterprise.root).toContainText("企业认证审核台");
    await expect(enterprise.root).toContainText("测******公司");
    await expect(enterprise.root).not.toContainText("91350211M000100Y46");
    const detail = await enterprise.openCase(enterpriseCaseNo);
    await expect(detail).toContainText("91************0Y46");
    await expect(detail).toContainText("单次下载");
    await enterprise.screenshot("enterprise-review-masked-detail-zh-cn");

    const download = adminPage.waitForEvent("download");
    await enterprise.downloadMaterial(enterpriseMaterialNo);
    await download;
    expect(state.accessRequests).toBe(1);
    expect(state.contentRequests).toEqual([
      { accessContext: "ra1.single-use.fixture" },
    ]);

    await enterprise.startReview();
    await expect(detail).toContainText("审核中");
    expect(state.decisions[0]?.action).toBe("review-starts");
    expect(state.decisions[0]?.body.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/u);

    await enterprise.approve();
    await expect(detail).toContainText("已认证");
    expect(state.decisions[1]?.action).toBe("approvals");
    await enterprise.screenshot("enterprise-review-approved-zh-cn");
  });

  test("TC-3b-i18n: 英文界面不暴露内部翻译键或原始法定资料", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("English");
    await installEnterpriseVerificationMock(adminPage);
    const enterprise = new EnterpriseVerificationPage(adminPage);
    await enterprise.goto();
    await expect(enterprise.root).toContainText("Enterprise verification desk");
    await expect(enterprise.root).not.toContainText(
      "plugin.idc-compliance",
    );
    await expect(enterprise.root).not.toContainText("91350211M000100Y46");
    await enterprise.screenshot("enterprise-review-queue-en-us");
  });

  test("TC-3b-reject: 驳回必须携带稳定原因并展示客户可处理状态", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    const state = await installEnterpriseVerificationMock(adminPage);
    const enterprise = new EnterpriseVerificationPage(adminPage);
    await enterprise.goto();
    const detail = await enterprise.openCase(enterpriseCaseNo);

    await enterprise.startReview();
    await expect(detail).toContainText("审核中");

    await enterprise.reject("必需材料不完整");
    await expect(enterprise.confirmationDialog()).toContainText(
      "所选稳定原因会展示给客户",
    );
    await enterprise.confirm();

    await expect(detail).toContainText("已驳回");
    expect(state.decisions[1]).toMatchObject({
      action: "rejections",
      body: { reasonCode: "materials_incomplete" },
    });
    expect(state.decisions[1]?.body.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/u);
    await enterprise.screenshot("enterprise-review-rejected-zh-cn");
  });

  test("TC-3c: 法定资料变更必须绑定变更申请并进入新的人工复核", async ({
    adminPage,
  }) => {
    const state = await installEnterpriseCustomerSubmissionMock(adminPage);
    await adminPage.goto("/", { waitUntil: "domcontentloaded" });
    const created = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-compliance/api/v1/account/compliance/enterprise-verifications",
        {
          body: JSON.stringify({
            consentNo: "consent-enterprise-change-0001",
            idempotencyKey: "enterprise-change-draft-browser-0001",
            legalChangeRequestNo: "LCR_20260818_0001",
          }),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        },
      );
      return response.json();
    });

    expect(created.data.item).toMatchObject({
      legalChangeRequestNo: "LCR_20260818_0001",
      reverification: true,
      statusCode: "draft",
    });
    expect(state.draftRequests).toEqual([
      {
        consentNo: "consent-enterprise-change-0001",
        idempotencyKey: "enterprise-change-draft-browser-0001",
        legalChangeRequestNo: "LCR_20260818_0001",
      },
    ]);
  });
});
