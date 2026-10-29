import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";

import { EnterpriseVerificationPage } from "../pages/EnterpriseVerificationPage";
import {
  enterpriseCaseNo,
  enterpriseMaterialNo,
  installEnterpriseCustomerSubmissionMock,
  installEnterpriseVerificationMock,
} from "../support/enterprise-verification-mock";

test.describe("TC-3 企业认证管理审核", () => {
  test.beforeEach(async ({ authenticatedPage: adminPage }, info) => {
    if (info.title.includes("TC-3f")) return;
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
  for (const materialOnly of [false, true]) test(`TC-3f: 材料留存随企业认证管理，策略更新和删除确认保留法律保全（${materialOnly ? "仅材料权限" : "包含企业列表权限"}）`, async ({ authenticatedPage: adminPage }) => {
    const staticUI = process.env.E2E_STATIC_UI_DIR;
    const authOrigin = process.env.E2E_AUTH_ORIGIN;
    if (authOrigin) {
      const auth = (await adminPage.context().storageState()).origins.find(origin => origin.origin === authOrigin);
      if (!auth) throw new Error("Requested reusable authentication origin is absent");
      await adminPage.addInitScript(entries => {
        for (const entry of entries) localStorage.setItem(entry.name, entry.value);
      }, auth.localStorage);
    }
    if (staticUI) await adminPage.route(url => url.pathname.startsWith("/admin/"), route => {
      const pathname = new URL(route.request().url()).pathname.replace(/^\/admin\//u, "");
      const file = resolve(staticUI, pathname);
      return route.fulfill({ path: file.startsWith(resolve(staticUI) + "/") && existsSync(file) ? file : resolve(staticUI, "index.html") });
    });
    await adminPage.addInitScript(() => {
      for (const key of Object.keys(localStorage)) if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
    });
    await adminPage.route("**/api/v1/i18n/runtime/messages?**", async route => {
      const response = await route.fetch(); const body = await response.json();
      const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
      const messages = JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/privacy.json`, import.meta.url), "utf8"));
      for (const [key,value] of Object.entries(messages)) {
        const segments=key.split(".");let target=body.data.messages;
        for (const segment of segments.slice(0,-1)) target=target[segment] ??= {};
        target[segments.at(-1)!]=value;
      }
      await route.fulfill({json:body});
    });
    await installEnterpriseVerificationMock(adminPage);
    let enterpriseQueries = 0;
    adminPage.on("request", request => {
      if (request.method() === "GET" && new URL(request.url()).pathname.endsWith("/admin/compliance/enterprise-verifications")) enterpriseQueries++;
    });
    // Keep the page acceptance independent of a shared administrator session.
    await adminPage.route("**/api/v1/user/info", route => route.fulfill({ json: { code: 0, message: "OK", data: {
      userId: 1, username: "material-retention-test", realName: "材料管理测试", avatar: "", roles: ["staff"], permissions: ["idc:admin:compliance:privacy:query", "idc:admin:compliance:privacy:configure", ...(materialOnly ? [] : ["idc:admin:compliance:enterprise-verification:list", "idc:admin:compliance:enterprise-verification:query"])], homePath: "/idc/compliance/enterprise-verifications", menus: [],
    } } }));
    await adminPage.route("**/api/v1/menus/all", route => route.fulfill({ json: { code: 0, message: "OK", data: { list: [{
      id: 90001, parentId: 0, name: "ComplianceMaterialTest", path: "/idc/compliance/enterprise-verifications", component: "#/views/system/plugin/dynamic-page", type: "M", meta: { title: "企业认证" },
    }] } } }));
    let retentionDays = 365;
    let deleted = false;
    let confirmed = false;
    let confirmationCalls = 0;
    let taskVersion = 1;
    let policyReads = 0;
    let releaseOldPolicy!: () => void;
    let policyReadStarted!: () => void;
    const oldPolicyGate = new Promise<void>(resolve => { releaseOldPolicy = resolve; });
    const firstPolicyRead = new Promise<void>(resolve => { policyReadStarted = resolve; });
    let held = false;
    const holdRequests: string[] = [];
    const task = { taskNo: "PDT_retention_test", dataTypeCode: "enterprise_verification_material", targetTypeCode: "enterprise_verification_material", targetNo: "MAT_retention_test", statusCode: "pending", dueAt: 1787040000000, holdReasonCode: "", version: 1 };
    await adminPage.route("**/x/idc-compliance/api/v1/admin/compliance/privacy/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      let data: unknown;
      if (url.pathname.endsWith("retention-policies")) {
        if (request.method() === "PUT") retentionDays = request.postDataJSON().retentionDays;
        const staleRead = request.method() === "GET" && policyReads++ === 0;
        if (staleRead) { policyReadStarted(); await oldPolicyGate; }
        const item = { policyNo: "PRP_retention_test", dataTypeCode: "enterprise_verification_material", retentionDays: staleRead ? 100 : retentionDays, version: 1, createdAt: 1787040000000 };
        data = request.method() === "PUT" ? { item } : { items: [item] };
      } else if (url.pathname.endsWith("retention-holds") || url.pathname.endsWith("retention-holds/releases")) {
        expect(request.postDataJSON()).toMatchObject({targetTypeCode: task.targetTypeCode, targetNo: task.targetNo});
        held = !url.pathname.endsWith("releases");
        taskVersion++;
        holdRequests.push(held ? "place" : "release");
        data = {};
      } else if (url.pathname.endsWith("confirmations")) {
        expect(request.postDataJSON().confirmed).toBe(true);
        expect(held).toBe(false);
        expect(request.postDataJSON().expectedVersion).toBe(taskVersion);
        confirmationCalls++;
        if (!confirmed) {
          confirmed = true;
          taskVersion++;
          await route.fulfill({ status: 503, json: { code: 51, message: "删除尚未完成，请继续已确认的处置", data: null } });
          return;
        }
        deleted = true;
        data = { item: { ...task, statusCode: "completed" } };
      } else data = { items: [{ ...task, version: taskVersion, statusCode: deleted ? "completed" : confirmed ? "confirmed" : held ? "blocked" : "pending", holdReasonCode:held ? "legal_hold" : "" }], total: 1 };
      await route.fulfill({ contentType: "application/json", body: JSON.stringify({ code: 0, data, message: "success" }) });
    });
    const enterprise = new EnterpriseVerificationPage(adminPage);
    await adminPage.goto(workspacePath("/idc/compliance/enterprise-verifications"), { waitUntil: "domcontentloaded" });
    await adminPage.getByTestId("material-retention-open").click({timeout:15000});
    const drawer = adminPage.getByRole("dialog").last();
    await firstPolicyRead;
    await drawer.getByRole("button", { name: /取\s*消/u }).click();
    await expect(drawer).toBeHidden();
    await adminPage.getByTestId("material-retention-open").click();
    const currentPolicy = drawer.locator("div.flex.items-center").filter({hasText:"企业认证材料"}).getByRole("spinbutton");
    await expect(currentPolicy).toHaveValue("365");
    const oldResponse = adminPage.waitForResponse(async response => response.request().method() === "GET" && new URL(response.url()).pathname.endsWith("retention-policies") && (await response.json()).data?.items?.[0]?.retentionDays === 100);
    releaseOldPolicy();
    await (await oldResponse).finished();
    await adminPage.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    await expect(currentPolicy).toHaveValue("365");
    await expect(adminPage.getByText("材料留存", { exact: true }).last()).toBeVisible();
    await expect(drawer.getByText("企业认证材料", { exact: true }).first()).toBeVisible();
    await expect(drawer.getByText("到期原件须经人工确认才删除", { exact: false })).toBeVisible();
    await drawer.evaluate(element => Promise.all(element.getAnimations().map(animation => animation.finished)));
    await enterprise.screenshot(materialOnly ? "material-retention-only-opened" : "material-retention-opened");
    const enterprisePolicy = drawer.locator("div.flex.items-center").filter({ hasText: "企业认证材料" });
    await enterprisePolicy.getByRole("spinbutton").fill("730");
    await enterprisePolicy.getByRole("button", { name: /保\s*存/u }).click();
    await adminPage.locator(".ant-popconfirm .ant-btn-primary").click();
    await expect.poll(() => retentionDays).toBe(730);
    await drawer.getByText("设置人工保全", { exact: true }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText("解除仅影响在此设置的人工保全");
    await adminPage.locator(".ant-modal-confirm .ant-btn-primary").click();
    await expect.poll(() => held).toBe(true);
    await expect(drawer.getByText("确认删除原件", { exact: true })).toHaveCount(0);
    await expect(drawer.getByText("法律保全", { exact: true })).toBeVisible();
    await expect(drawer).not.toContainText("plugin.idc-compliance");
    await enterprise.screenshot(materialOnly ? "material-retention-only-held" : "material-retention-held");
    await drawer.getByText("解除人工保全", { exact: true }).click();
    await adminPage.locator(".ant-modal-confirm .ant-btn-primary").click();
    await expect.poll(() => held).toBe(false);
    expect(holdRequests).toEqual(["place", "release"]);
    await drawer.getByText("确认删除原件", { exact: true }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText("法律保全");
    await adminPage.locator(".ant-modal-confirm .ant-btn-primary").click();
    await expect(drawer.getByText("已确认待完成", { exact: true })).toBeVisible();
    await expect(drawer.getByText("已确认的删除尚未完成，原件可能已移除。", { exact: false })).toBeVisible();
    await expect(drawer.getByText("设置人工保全", { exact: true })).toHaveCount(0);
    expect(deleted).toBe(false);
    expect(confirmationCalls).toBe(1);
    await enterprise.screenshot(materialOnly ? "material-retention-only-recoverable" : "material-retention-recoverable");
    await drawer.getByText("继续已确认删除", { exact: true }).click();
    await expect(adminPage.getByRole("dialog").last()).toContainText("不会重新审批或恢复原件");
    await adminPage.locator(".ant-modal-confirm .ant-btn-primary").click();
    await expect.poll(() => deleted).toBe(true);
    expect(confirmationCalls).toBe(2);
    await expect(drawer.getByText("已删除", { exact: true })).toBeVisible();
    await enterprise.screenshot(materialOnly ? "material-retention-only-confirmed" : "material-retention-confirmed");
    if (materialOnly) {
      expect(enterpriseQueries).toBe(0);
      await expect(enterprise.root.locator(".metric-rail")).toHaveCount(0);
      await expect(enterprise.root.locator(".enterprise-grid")).toHaveCount(0);
    } else expect(enterpriseQueries).toBeGreaterThan(0);
  });

});
