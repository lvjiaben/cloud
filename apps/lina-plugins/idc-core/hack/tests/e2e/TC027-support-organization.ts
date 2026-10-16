import { expect, test } from "@host-tests/fixtures/auth";
import {
  execPgSQLStatements,
  queryPgScalar,
} from "@host-tests/support/postgres";
import { SupportOrganizationPage } from "../pages/SupportOrganizationPage";
import {
  installSupportDeskMock,
  supportTickets,
} from "../support/support-desk-mock";

test.describe("TC-27 组织部门工单队列", () => {
  test("TC-27d: 标记需跟进只记录部门事项，列表筛选使用真实字段", async ({
    adminPage,
  }) => {
    await installSupportDeskMock(adminPage);
    let escalation: Record<string, unknown> | undefined;
    let detail: any;
    await adminPage.route(
      `**/tickets/${supportTickets.global}`,
      async (route) => {
        if (!detail) {
          await route.fallback();
          return;
        }
        await route.fulfill({ json: { code: 0, data: { item: detail } } });
      },
    );
    await adminPage.route(
      `**/tickets/${supportTickets.global}/escalations`,
      async (route) => {
        escalation = route.request().postDataJSON();
        detail = {
          ...detail,
          ticket: { ...detail.ticket, followUp: true },
          escalations: [
            { createdAt: Date.now(), reason: escalation?.reason, toUserId: 0 },
          ],
        };
        await route.fulfill({ json: { code: 0, data: { item: detail } } });
      },
    );
    const view = new SupportOrganizationPage(adminPage);
    await view.goto();
    const read = adminPage.waitForResponse((response) =>
      response.url().endsWith(`/tickets/${supportTickets.global}`),
    );
    await view.openTicket(supportTickets.global);
    detail = (await (await read).json()).data.item;
    await view.followUp();
    await expect(
      adminPage.getByTestId("support-action-composer"),
    ).toContainText("不会指定个人");
    await view.submitFollowUp("需要持续确认线路恢复");
    await expect(view.pageRoot).toContainText("工单已更新。");
    expect(escalation).toEqual({
      ticketVersion: 7,
      reason: "需要持续确认线路恢复",
      toUserId: 0,
    });
    await expect(view.pageRoot).toContainText("需要持续确认线路恢复");
    await view.showFollowUpHistory("需要持续确认线路恢复");
    await view.screenshot("organization-follow-up-history");
    const filtered = adminPage.waitForRequest(
      (request) =>
        request.url().includes("/tickets?") &&
        request.url().includes("followUp=true"),
    );
    await view.filterFollowUp();
    expect(new URL((await filtered).url()).searchParams.get("followUp")).toBe(
      "true",
    );
    await expect(
      view.pageRoot.getByLabel("跟进状态", { exact: true }),
    ).toHaveValue("true");
    await view.screenshot("organization-follow-up");
  });

  test("TC-27c: 真实组织候选与队列新建、旧队列关联持久化", async ({
    adminPage,
  }) => {
    const stamp = Date.now();
    const orgName = `工单验收组织${stamp}`;
    const queueName = `工单验收队列${stamp}`;
    const legacyNo = `DEP_E2E_${stamp}`;
    const deptId = Number(
      queryPgScalar(
        `INSERT INTO plugin_org_core_dept(tenant_id,name,code,status,created_at,updated_at) VALUES(0,'${orgName}','E2E_${stamp}',1,NOW(),NOW()) RETURNING id;`,
      ),
    );
    execPgSQLStatements([
      `INSERT INTO plugin_idc_core_support_department(tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,org_dept_id) VALUES(0,'${legacyNo}','历史验收队列${stamp}','Legacy support validation','manual_claim','active',0);`,
    ]);
    try {
      const view = new SupportOrganizationPage(adminPage);
      await view.gotoSettings();
      await view.openOrganizations();
      const candidates = adminPage.waitForResponse(
        (response) =>
          response.url().includes("/support/organization-departments?") &&
          response.url().includes("keyword="),
      );
      await view.search(orgName);
      await candidates;
      await view.choose(orgName);
      await view.names(queueName, `Validation queue ${stamp}`);
      const created = adminPage.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().endsWith("/settings/departments"),
      );
      await view.save();
      const createResponse = await created;
      expect(createResponse.ok()).toBe(true);
      const createPayload = await createResponse.json();
      expect(createPayload.code).toBe(0);
      expect(createPayload.data.item.orgDeptId).toBe(deptId);
      expect(
        queryPgScalar(
          `SELECT org_dept_id FROM plugin_idc_core_support_department WHERE name_zh_cn='${queueName}'`,
        ),
      ).toBe(String(deptId));
      await expect(view.settingsRoot).toContainText(queueName);
      await view.edit(legacyNo);
      await expect(view.form()).toContainText("此历史队列尚未关联组织部门");
      await view.openOrganizations();
      await view.search(orgName);
      await view.choose(orgName);
      const updated = adminPage.waitForResponse(
        (response) =>
          response.request().method() === "PUT" &&
          response.url().endsWith(`/settings/departments/${legacyNo}`),
      );
      await view.save();
      const updatePayload = await (await updated).json();
      expect(updatePayload.code).toBe(0);
      expect(
        queryPgScalar(
          `SELECT org_dept_id FROM plugin_idc_core_support_department WHERE department_no='${legacyNo}'`,
        ),
      ).toBe(String(deptId));
      await view.gotoSettings();
      await view.edit(legacyNo);
      await expect(
        view.form().getByTestId("support-organization-select"),
      ).toContainText(orgName);
      await expect(view.form()).not.toContainText("尚未关联组织部门");
      await expect(
        view.form().getByText("使用中", { exact: true }),
      ).toBeVisible();
      await expect(
        view.form().getByText("已停用", { exact: true }),
      ).toBeVisible();
      await view.showDepartmentForm();
      await view.screenshot("organization-real-persisted");
    } finally {
      execPgSQLStatements([
        `DELETE FROM plugin_idc_core_support_department WHERE department_no='${legacyNo}' OR name_zh_cn='${queueName}';`,
        `DELETE FROM plugin_org_core_dept WHERE id=${deptId} AND code='E2E_${stamp}';`,
      ]);
    }
  });

  test("TC-27a: 旧队列关联必填、分页搜索与失败重试，保存仅人工领取", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    let fail = false;
    const requests: URL[] = [];
    await adminPage.route(
      "**/admin/support/organization-departments?*",
      async (route) => {
        const url = new URL(route.request().url());
        requests.push(url);
        if (fail) {
          await route.fulfill({
            status: 503,
            json: { code: 50, message: "Unavailable" },
          });
          return;
        }
        const keyword = url.searchParams.get("keyword") || "",
          page = Number(url.searchParams.get("page") || 1);
        const start = page === 2 ? 21 : 1;
        const items = keyword
          ? [{ deptId: 72, deptName: "财务处理组", deptCode: "FIN" }]
          : Array.from({ length: page === 2 ? 5 : 20 }, (_, i) => ({
              deptId: start + i,
              deptName: `组织部门${start + i}`,
              deptCode: `D${start + i}`,
            }));
        await route.fulfill({
          json: { code: 0, data: { items, total: keyword ? 1 : 25 } },
        });
      },
    );
    const view = new SupportOrganizationPage(adminPage);
    await view.gotoSettings();
    await expect(view.settingsRoot).toContainText("部门管归属，角色管权限。");
    await expect(view.settingsRoot).toContainText("角色和调部门会实时生效");
    await expect(view.settingsRoot).not.toContainText("员工业务资格");
    await view.edit("DEP_SUPPORT_NOC");
    await expect(view.form()).toContainText("此历史队列尚未关联组织部门");
    await expect(view.form()).not.toContainText("轮流分派");
    await view.save();
    await expect(view.form()).toContainText("请选择一个组织部门");
    expect(state.settingsRequests).toHaveLength(0);
    await view.openOrganizations();
    await expect(view.dropdown()).toContainText("共 25 个部门");
    await view.nextPage();
    await expect
      .poll(() => requests.at(-1)?.searchParams.get("page"))
      .toBe("2");
    await expect(
      view.dropdown().getByText("组织部门25", { exact: true }).last(),
    ).toBeVisible();
    await view.search("财务");
    await expect
      .poll(() => requests.at(-1)?.searchParams.get("keyword"))
      .toBe("财务");
    expect(requests.at(-1)?.searchParams.get("page")).toBe("1");
    await view.choose("财务处理组");
    await view.save();
    await expect.poll(() => state.settingsRequests.length).toBe(1);
    expect(state.settingsRequests[0]?.body).toMatchObject({
      orgDeptId: 72,
      assignmentMode: "manual_claim",
      expectedVersion: 2,
    });
    expect(state.settingsRequests[0]?.body).not.toHaveProperty("orgDeptName");
    await expect(view.settingsRoot).toContainText("工单设置已保存。");
    await view.screenshot("organization-linked");
    fail = true;
    await view.openOrganizations();
    await expect(view.dropdown()).toContainText("组织部门加载失败，请重试。");
    await view.screenshot("organization-candidates-failed");
    fail = false;
    await view.retryCandidates();
    await expect(view.dropdown()).toContainText("共 25 个部门");
    await view.choose("组织部门1");
    await view.names("售前咨询", "Presales");
    await view.save();
    await expect.poll(() => state.settingsRequests.length).toBe(2);
    expect(state.settingsRequests[1]?.method).toBe("POST");
    expect(state.settingsRequests[1]?.body).toMatchObject({
      orgDeptId: 1,
      assignmentMode: "manual_claim",
      nameZhCN: "售前咨询",
    });
  });

  test("TC-27b: 跨部门转派成功关闭旧详情，不再重读无权工单", async ({
    adminPage,
  }) => {
    await installSupportDeskMock(adminPage);
    let transferred = false,
      staleReads = 0;
    let failTransfer = true;
    let transferBody: Record<string, unknown> | undefined;
    await adminPage.route(
      `**/tickets/${supportTickets.global}`,
      async (route) => {
        if (transferred) {
          staleReads++;
          await route.fulfill({
            status: 403,
            json: { code: 50, message: "Forbidden" },
          });
          return;
        }
        await route.fallback();
      },
    );
    await adminPage.route(
      `**/tickets/${supportTickets.global}/transfers`,
      async (route) => {
        transferBody = route.request().postDataJSON();
        if (failTransfer) {
          await route.fulfill({
            status: 409,
            json: { code: 50, message: "Changed" },
          });
          return;
        }
        transferred = true;
        await route.fulfill({ json: { code: 0, data: {} } });
      },
    );
    const view = new SupportOrganizationPage(adminPage);
    await view.goto();
    await view.openTicket(supportTickets.global);
    await view.transfer();
    await expect(
      adminPage.getByTestId("support-action-composer"),
    ).toContainText("由该部门员工自行领取");
    await view.screenshot("organization-transfer-open");
    await view.transferTo("DEP_SUPPORT_CS", "由客户支持组继续处理");
    await expect(view.pageRoot).toContainText("操作未完成，请刷新后重试。");
    await expect(
      adminPage.getByTestId("support-action-composer"),
    ).toBeVisible();
    expect(transferred).toBe(false);
    await view.screenshot("organization-transfer-failed");
    failTransfer = false;
    await view.transferTo("DEP_SUPPORT_CS", "由客户支持组继续处理");
    await expect.poll(() => transferred).toBe(true);
    await expect(adminPage.getByTestId("support-action-composer")).toHaveCount(
      0,
    );
    await expect(view.pageRoot).toContainText("工单已更新。");
    expect(transferBody).toEqual({
      ticketVersion: 7,
      departmentNo: "DEP_SUPPORT_CS",
      reason: "由客户支持组继续处理",
    });
    expect(staleReads).toBe(0);
    await view.screenshot("organization-transfer-complete");
  });
});
