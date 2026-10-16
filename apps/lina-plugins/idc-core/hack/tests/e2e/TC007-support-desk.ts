import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { SupportDeskPage } from "../pages/SupportDeskPage";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { CustomerAssistancePage } from "../pages/CustomerAssistancePage";
import { CustomerSupportPage } from "../pages/CustomerSupportPage";
import { installCustomerSupportListMock } from "../support/customer-support-list-mock";
import {
  installSupportDeskMock,
  supportTickets,
} from "../support/support-desk-mock";
import { execPgSQLStatements } from "@host-tests/support/postgres";

test.describe("TC007 后台工单工作台与客户工单", () => {
  test("TC007a: 全局合并使用真实候选与动作契约并保留历史", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.goto();
    await support.openTicket(supportTickets.global);

    await expect(support.pageRoot).toContainText("等待机房确认");
    await expect(support.pageRoot).toContainText("尚未设置处理时限。");
    await expect(support.pageRoot).toContainText("状态变更");
    await expect(support.pageRoot).toContainText("合并关系");
    await expect(support.pageRoot).toContainText("转交网络组继续处理");
    await expect(
      adminPage.getByTestId("support-action-global-merge"),
    ).toBeVisible();
    await expect(adminPage.getByTestId("support-action-merge")).toHaveCount(0);
    await expect(adminPage.getByTestId("support-action-close")).toHaveCount(0);
    await support.screenshot("support-global-merge-before");

    await adminPage.getByTestId("support-action-global-merge").click();
    await expect(
      adminPage.getByTestId("support-merge-candidates"),
    ).toContainText("选择授权范围内其他队列的工单");
    await expect(
      adminPage.getByTestId("support-merge-candidates"),
    ).toContainText("这里只显示你当前数据范围内、符合合并条件的工单。");
    await support.screenshot("support-global-merge-candidates");
    await adminPage
      .getByTestId(`support-merge-candidate-${supportTickets.sourceGlobal}`)
      .check();
    await adminPage
      .getByTestId("support-action-composer")
      .getByRole("textbox", { name: "原因" })
      .fill("确认为跨队列重复问题");
    await adminPage.getByTestId("support-action-submit").click();

    await expect.poll(() => state.mergeRequests.length).toBe(1);
    expect(state.candidateRequests[0]?.pathname).toContain(
      `/tickets/${supportTickets.global}/global-merge-candidates`,
    );
    expect(state.candidateRequests[0]?.searchParams.get("limit")).toBe("50");
    expect(state.mergeRequests[0]?.path).toContain(
      `/tickets/${supportTickets.global}/global-merges`,
    );
    expect(state.mergeRequests[0]?.body).toEqual({
      mainVersion: 7,
      reason: "确认为跨队列重复问题",
      sources: [{ ticketNo: supportTickets.sourceGlobal, version: 3 }],
    });
    await expect(support.pageRoot).toContainText(supportTickets.sourceGlobal);
    await expect(support.pageRoot).toContainText("工单已更新。");
    await support.screenshot("support-global-merge-after");
  });

  test("TC007b: 普通合并与只读工单严格跟随后端权限动作", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.goto();
    await support.openTicket(supportTickets.readonly);

    await expect(
      adminPage.locator('[data-testid^="support-action-"]'),
    ).toHaveCount(0);
    await support.openTicket(supportTickets.local);
    await expect(adminPage.getByTestId("support-action-merge")).toBeVisible();
    await expect(
      adminPage.getByTestId("support-action-global-merge"),
    ).toHaveCount(0);

    await mainLayout.switchLanguage("English");
    await expect(support.pageRoot).toContainText(
      "Waiting for data-center confirmation",
    );
    await expect(adminPage.getByTestId("support-action-merge")).toContainText(
      "Merge",
    );
    await adminPage.getByTestId("support-action-merge").click();
    await expect(
      adminPage.getByTestId("support-merge-candidates"),
    ).toContainText("Only tickets eligible for a regular merge are shown.");
    await adminPage
      .getByTestId(`support-merge-candidate-${supportTickets.sourceLocal}`)
      .check();
    await adminPage
      .getByTestId("support-action-composer")
      .getByRole("textbox", { name: "Reason" })
      .fill("Confirmed duplicate in the same support scope");
    await adminPage.getByTestId("support-action-submit").click();

    await expect.poll(() => state.mergeRequests.length).toBe(1);
    expect(state.candidateRequests[0]?.pathname).toContain(
      `/tickets/${supportTickets.local}/merge-candidates`,
    );
    expect(state.candidateRequests[0]?.searchParams.get("limit")).toBe("50");
    expect(state.mergeRequests[0]?.path).toContain(
      `/tickets/${supportTickets.local}/merges`,
    );
    expect(state.mergeRequests[0]?.body).toEqual({
      mainVersion: 5,
      reason: "Confirmed duplicate in the same support scope",
      sources: [{ ticketNo: supportTickets.sourceLocal, version: 4 }],
    });
    await expect(support.pageRoot).toContainText(supportTickets.sourceLocal);
    await expect(support.pageRoot).not.toContainText("support.");
    await support.screenshot("support-regular-merge-english");
  });

  test("TC007c: 部门分派和分类分流只提交真实配置契约", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.gotoSettings();

    await expect(support.settingsRoot).toContainText(
      "部门管归属，角色管权限。",
    );
    await expect(support.settingsRoot).toContainText("公共待领取队列");
    await expect(support.settingsRoot).not.toContainText("DEP_SUPPORT_CS");
    await support.screenshot("support-settings-routing-initial");

    await adminPage
      .getByTestId("support-settings-edit-department-DEP_SUPPORT_CS")
      .click();
    await expect(adminPage.getByTestId("support-settings-department-assignment")).toHaveCount(0);
    await expect(adminPage.getByTestId("support-organization-select")).toContainText("客户服务组织部");
    state.failNextSettingsWrite = true;
    await adminPage
      .getByTestId("support-settings-department-form")
      .getByRole("button", { name: "保存设置" })
      .click();
    await expect(support.settingsRoot).toContainText(
      "操作未完成，请刷新后重试。",
    );
    await expect(adminPage.getByTestId("support-organization-select")).toContainText("客户服务组织部");
    await support.screenshot("support-settings-write-conflict");
    await adminPage
      .getByTestId("support-settings-department-form")
      .getByRole("button", { name: "保存设置" })
      .click();

    await expect.poll(() => state.settingsRequests.length).toBeGreaterThan(0);
    const departmentUpdate = state.settingsRequests.find(
      (item) =>
        item.method === "PUT" &&
        item.path.endsWith("/settings/departments/DEP_SUPPORT_CS"),
    );
    expect(departmentUpdate?.body).toEqual({
      assignmentMode: "manual_claim",
      orgDeptId: 21,
      expectedVersion: 4,
      nameEnUS: "Customer support",
      nameZhCN: "客户支持部",
      parentDepartmentNo: "",
      status: "active",
    });

    await adminPage.getByRole("button", { name: "新建问题分类" }).click();
    const categoryForm = adminPage.getByTestId(
      "support-settings-category-form",
    );
    await categoryForm.getByLabel("中文名称").fill("未映射咨询");
    await categoryForm.getByLabel("英文名称").fill("Unmapped inquiry");
    await categoryForm.getByLabel("优先级").selectOption("high");
    await categoryForm.getByLabel("显示顺序").fill("20");
    await categoryForm.getByRole("button", { name: "保存设置" }).click();

    const categoryCreate = state.settingsRequests.find(
      (item) =>
        item.method === "POST" &&
        item.path.endsWith("/settings/issue-categories"),
    );
    expect(categoryCreate?.body).toEqual({
      departmentNo: "",
      nameEnUS: "Unmapped inquiry",
      nameZhCN: "未映射咨询",
      priority: "high",
      sortOrder: 20,
      status: "active",
    });
    await expect(support.settingsRoot).toContainText("未映射咨询");
    await expect(support.settingsRoot).toContainText("公共待领取队列");

    adminPage.once("dialog", (dialog) => dialog.accept());
    await support.settingsRoot
      .locator("article")
      .filter({ has: adminPage.getByText("未映射咨询", { exact: true }) })
      .last()
      .getByRole("button", { name: "停用" })
      .click();
    const categoryDisable = state.settingsRequests.find(
      (item) =>
        item.method === "DELETE" &&
        item.path.endsWith("/settings/issue-categories/CAT_SUPPORT_NEW_2"),
    );
    expect(categoryDisable?.body).toEqual({ expectedVersion: 1 });
    await support.screenshot("support-settings-routing");
  });

  test("TC007d: 状态、快捷回复、SLA 和关闭重开配置使用真实接口", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.gotoSettings();
    await support.openSettingsSection("writing");

    await adminPage.getByRole("button", { name: "新建自定义状态" }).click();
    const statusForm = adminPage.getByTestId("support-settings-status-form");
    await statusForm.getByLabel("中文名称").fill("等待备件");
    await statusForm.getByLabel("英文名称").fill("Waiting for parts");
    await statusForm
      .getByLabel("处理进度")
      .selectOption("waiting_upstream");
    await statusForm.getByLabel("显示顺序").fill("30");
    await statusForm.getByRole("button", { name: "保存设置" }).click();

    await adminPage.getByRole("button", { name: "新建快捷回复" }).click();
    const presetForm = adminPage.getByTestId("support-settings-preset-form");
    await presetForm.locator("select").first().selectOption("DEP_SUPPORT_CS");
    await presetForm.getByLabel("快捷回复标题").fill("等待备件说明");
    await presetForm
      .getByLabel("内容")
      .fill("我们正在等待备件到货，您仍可修改这段草稿。");
    await presetForm.getByLabel("显示顺序").fill("20");
    await presetForm.getByRole("button", { name: "保存设置" }).click();

    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "POST" && item.path.endsWith("/settings/statuses"),
      )?.body,
    ).toEqual({
      baseStage: "waiting_upstream",
      nameEnUS: "Waiting for parts",
      nameZhCN: "等待备件",
      sortOrder: 30,
      status: "active",
    });
    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "POST" &&
          item.path.endsWith("/settings/reply-presets"),
      )?.body,
    ).toEqual({
      body: "我们正在等待备件到货，您仍可修改这段草稿。",
      departmentNo: "DEP_SUPPORT_CS",
      locale: "zh-CN",
      sortOrder: 20,
      status: "active",
      title: "等待备件说明",
    });
    await support.screenshot("support-settings-writing-saved");

    await support.openSettingsSection("sla");
    await adminPage.getByRole("button", { name: "新建计时日历" }).click();
    const calendarForm = adminPage.getByTestId(
      "support-settings-calendar-form",
    );
    await calendarForm.getByLabel("计时日历").fill("中国工作日");
    await calendarForm.getByLabel("计时方式").selectOption("business_hours");
    await support.screenshot("support-settings-business-calendar-form");
    await calendarForm.getByRole("button", { name: "保存设置" }).click();

    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "POST" &&
          item.path.endsWith("/settings/sla-calendars"),
      )?.body,
    ).toEqual({
      mode: "business_hours",
      name: "中国工作日",
      status: "active",
      timezone: "Asia/Shanghai",
      workEndMinute: 1080,
      workStartMinute: 540,
      workdays: [1, 2, 3, 4, 5],
    });

    const holidayForm = adminPage.getByTestId("support-settings-holiday-form");
    await holidayForm.getByLabel("日期").fill("2026-10-02");
    await holidayForm.getByLabel("节假日名称").fill("国庆假期");
    await holidayForm.getByRole("button", { name: "保存设置" }).click();
    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "PUT" && item.path.endsWith("/holidays/2026-10-02"),
      )?.body,
    ).toEqual({ name: "国庆假期" });

    await adminPage.getByRole("button", { name: "新建时限规则" }).click();
    const ruleForm = adminPage.getByTestId("support-settings-rule-form");
    await ruleForm.locator("select").first().selectOption("DEP_SUPPORT_CS");
    await ruleForm.getByLabel("优先级").selectOption("high");
    await ruleForm.getByLabel("计时日历").selectOption("CAL_SUPPORT_24X7");
    await ruleForm.getByLabel("首次响应时限（分钟）").fill("20");
    await ruleForm.getByLabel("解决时限（分钟）").fill("180");
    await ruleForm.getByLabel("到期前多少分钟提醒").fill("10, 5");
    await ruleForm.getByRole("button", { name: "保存设置" }).click();

    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "POST" && item.path.endsWith("/settings/sla-rules"),
      )?.body,
    ).toEqual({
      calendarNo: "CAL_SUPPORT_24X7",
      categoryNo: "",
      departmentNo: "DEP_SUPPORT_CS",
      firstResponseMinutes: 20,
      nearDueMinutes: [10, 5],
      priority: "high",
      resolutionMinutes: 180,
      status: "active",
    });

    await support.openSettingsSection("automation");
    await adminPage.getByTestId("support-settings-auto-close").fill("5");
    await adminPage.getByTestId("support-settings-reopen").fill("9");
    await support.settingsRoot
      .getByRole("button", { name: "保存设置" })
      .click();
    expect(
      state.settingsRequests.find(
        (item) =>
          item.method === "PUT" &&
          item.path.endsWith("/settings/automation-policy"),
      )?.body,
    ).toEqual({ autoCloseDays: 5, expectedVersion: 4, reopenDays: 9 });
    await expect(
      adminPage.getByTestId("support-settings-auto-close"),
    ).toHaveValue("5");
    await expect(adminPage.getByTestId("support-settings-reopen")).toHaveValue(
      "9",
    );
    await support.screenshot("support-settings-automation-saved");

    await mainLayout.switchLanguage("English");
    await support.openSettingsSection("routing");
    await expect(support.settingsRoot).toContainText(
      "Organization departments and roles control support access.",
    );
    await expect(support.settingsRoot).not.toContainText("support.settings.");
    await support.screenshot("support-settings-english");
  });

  test("TC007e: 七类字段配置不要求手填技术编号且版本冲突保留输入", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.gotoSettings();
    await support.openSettingsSection("fields");

    for (const label of [
      "单行文本",
      "多行文本",
      "数字",
      "单选",
      "多选",
      "日期",
      "是 / 否",
    ]) {
      await expect(support.settingsRoot).toContainText(label);
    }
    await expect(support.settingsRoot).not.toContainText("FIELD_INTERNAL_");
    await expect(support.settingsRoot).toContainText(
      "修改名称、排序或停用字段，都不会改写工单里已经保存的内容。",
    );

    await adminPage.getByRole("button", { name: "新建工单字段" }).click();
    const form = adminPage.getByTestId("support-settings-field-form");
    await form.getByLabel("中文名称", { exact: true }).fill("客户影响范围");
    await form.getByLabel("英文名称", { exact: true }).fill("Customer impact");
    await form.getByLabel("字段类型").selectOption("single_select");
    await form.getByLabel("售前咨询").check();
    await adminPage.getByTestId("support-settings-add-field-option").click();
    const option = adminPage
      .getByTestId("support-settings-field-options")
      .locator(".form-grid")
      .first();
    await option.getByLabel("选项中文名称").fill("单台服务器");
    await option.getByLabel("选项英文名称").fill("One server");
    await form.getByRole("button", { name: "保存设置" }).click();

    const created = state.settingsRequests.find(
      (item) =>
        item.method === "POST" && item.path.endsWith("/settings/custom-fields"),
    );
    expect(created?.body).toMatchObject({
      appliesCustomer: true,
      appliesInternal: false,
      appliesPresales: true,
      fieldType: "single_select",
      nameEnUS: "Customer impact",
      nameZhCN: "客户影响范围",
      visibility: "customer_visible",
    });
    expect(created?.body.options).toEqual([
      {
        expectedVersion: 0,
        nameEnUS: "One server",
        nameZhCN: "单台服务器",
        optionNo: "",
        sortOrder: 0,
        status: "active",
      },
    ]);

    await adminPage
      .getByTestId("support-settings-edit-field-FIELD_INTERNAL_1")
      .click();
    await form.getByLabel("中文名称", { exact: true }).fill("冲突后仍保留");
    state.failNextSettingsWrite = true;
    await form.getByRole("button", { name: "保存设置" }).click();
    await expect(form.getByLabel("中文名称", { exact: true })).toHaveValue(
      "冲突后仍保留",
    );
    await expect(support.settingsRoot).toContainText(
      "字段未保存，当前输入仍在。请刷新核对是否有人刚刚修改过，再重试。",
    );
    await support.screenshot("support-custom-fields-conflict");
  });

  test("TC007f: 内部建单按七类真实定义提交并展示不可变字段快照", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.goto();

    await support.openTicket(supportTickets.global);
    await expect(
      adminPage.getByTestId("support-custom-field-snapshots"),
    ).toContainText("原始紧急程度");
    await expect(
      adminPage.getByTestId("support-custom-field-snapshots"),
    ).toContainText("高");
    await expect(
      adminPage.getByTestId("support-related-entitlement"),
    ).toContainText("裸金属服务器 · 创建时");
    await expect(
      adminPage.getByTestId("support-related-entitlement"),
    ).toContainText("裸金属服务器 · 已改名");
    await expect(
      adminPage.getByTestId("support-related-entitlement"),
    ).toContainText("已终止");
    await expect(support.pageRoot).not.toContainText("SERVICE_HIDDEN_FROM_UI");

    await support.openInternalTicketForm();
    const modal = adminPage.getByTestId("support-create-modal");
    await modal.getByLabel("主题").fill("七类字段验证");
    await modal.getByLabel("内容").fill("验证内部工单自定义字段提交。 ");
    await modal.getByLabel("问题分类").selectOption("CAT_SUPPORT_SERVER");
    await modal.getByLabel("部门").selectOption("DEP_SUPPORT_CS");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_1")
      .getByRole("textbox")
      .fill("单行值");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_2")
      .getByRole("textbox")
      .fill("多行值\n第二行");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_3")
      .getByRole("textbox")
      .fill("12.50");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_4")
      .getByRole("combobox")
      .selectOption("FIELD_INTERNAL_4_OPTION_A");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_5")
      .getByRole("listbox")
      .selectOption(["FIELD_INTERNAL_5_OPTION_A", "FIELD_INTERNAL_5_OPTION_B"]);
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_6")
      .locator('input[type="date"]')
      .fill("2026-08-15");
    await modal
      .getByTestId("support-internal-field-FIELD_INTERNAL_7")
      .getByRole("combobox")
      .selectOption("true");
    await modal.getByRole("button", { name: "提交" }).click();

    await expect.poll(() => state.createRequests.length).toBe(1);
    expect(state.createRequests[0]?.customFields).toEqual([
      { fieldNo: "FIELD_INTERNAL_1", textValue: "单行值" },
      { fieldNo: "FIELD_INTERNAL_2", textValue: "多行值\n第二行" },
      { fieldNo: "FIELD_INTERNAL_3", numberValue: "12.50" },
      { fieldNo: "FIELD_INTERNAL_4", optionNos: ["FIELD_INTERNAL_4_OPTION_A"] },
      {
        fieldNo: "FIELD_INTERNAL_5",
        optionNos: ["FIELD_INTERNAL_5_OPTION_A", "FIELD_INTERNAL_5_OPTION_B"],
      },
      { dateValue: "2026-08-15", fieldNo: "FIELD_INTERNAL_6" },
      { booleanValue: true, fieldNo: "FIELD_INTERNAL_7" },
    ]);
    expect(state.createRequests[0]).not.toHaveProperty("entitlementNo");
    await support.screenshot("support-internal-custom-fields");
  });

  test("TC007g: 自定义状态只使用服务端候选并按版本安全切换", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.goto();
    await support.openTicket(supportTickets.local);

    await expect(
      adminPage.getByTestId("support-action-status-transition"),
    ).toContainText("修改状态");
    await mainLayout.switchLanguage("English");
    await expect(
      adminPage.getByTestId("support-action-status-transition"),
    ).toContainText("Update status");

    await support.openStatusTransition();
    await expect(
      adminPage.getByTestId("support-status-transition-target"),
    ).toContainText("Force closed · Closed");
    await support.selectStatusTransition("STS_FORCE_CLOSED");
    await expect(
      adminPage
        .getByTestId("support-action-composer")
        .getByRole("textbox", { name: "Reason" }),
    ).toHaveAttribute("required", "");
    await support.submitAction();
    await expect.poll(() => state.statusTransitionRequests.length).toBe(0);

    await support.fillActionReason("Duplicate request confirmed by supervisor");
    await support.submitAction();
    await expect.poll(() => state.statusTransitionRequests.length).toBe(1);
    expect(state.statusTransitionRequests[0]).toEqual({
      customStatusNo: "STS_FORCE_CLOSED",
      reason: "Duplicate request confirmed by supervisor",
      ticketVersion: 5,
    });
    await expect(support.pageRoot).toContainText("Force closed");
    await expect(support.pageRoot).toContainText("Closed");
    await expect(
      adminPage.getByTestId("support-action-status-transition"),
    ).toHaveCount(0);
    await expect(support.pageRoot).toContainText(
      "The ticket has been updated.",
    );
    await support.screenshot("support-custom-status-transition");
  });

  test("TC007h: 状态切换冲突不伪造成功并保留当前输入", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    state.failNextStatusTransition = true;
    const support = new SupportDeskPage(adminPage);
    await support.goto();
    await support.openTicket(supportTickets.local);
    await support.openStatusTransition();
    await support.selectStatusTransition("STS_WAITING_CUSTOMER");
    await support.fillActionReason("等待客户补充现场信息");
    await support.submitAction();

    await expect.poll(() => state.statusTransitionRequests.length).toBe(1);
    expect(state.statusTransitionRequests[0]).toEqual({
      customStatusNo: "STS_WAITING_CUSTOMER",
      reason: "等待客户补充现场信息",
      ticketVersion: 5,
    });
    await expect(support.pageRoot).toContainText(
      "状态没有改变。请刷新工单后，从最新可选状态中重新选择。",
    );
    await expect(
      adminPage.getByTestId("support-status-transition-target"),
    ).toHaveValue("STS_WAITING_CUSTOMER");
    await expect(
      adminPage
        .getByTestId("support-action-composer")
        .getByRole("textbox", { name: "原因（选填）" }),
    ).toHaveValue("等待客户补充现场信息");
    await expect(support.pageRoot).toContainText("等待机房确认");
    await support.screenshot("support-custom-status-conflict");
  });

  test("TC007i: 上游转发只发送冻结的客户可见内容并人工处理回复和重试", async ({
    adminPage,
  }) => {
    const state = await installSupportDeskMock(adminPage);
    const support = new SupportDeskPage(adminPage);
    await support.goto();
    await support.openTicket(supportTickets.global);

    await expect(
      adminPage.getByTestId("support-upstream-forwards"),
    ).toContainText("上游转发记录");
    await expect(
      adminPage.getByTestId("support-upstream-forwards"),
    ).toContainText("上游明确未收到");
    await adminPage.getByTestId("support-action-upstream").click();
    await adminPage
      .getByTestId("support-selection-message-MSG_PUBLIC_1")
      .check();
    await adminPage
      .getByTestId("support-selection-attachment-ATT_PUBLIC_1")
      .check();
    await adminPage
      .getByRole("textbox", { name: "原因" })
      .fill("转发给上游核对线路");
    await adminPage.getByTestId("support-action-submit").click();
    await expect(
      adminPage.getByTestId("support-upstream-preview"),
    ).toContainText("服务器网络间歇中断。");
    await expect(
      adminPage.getByTestId("support-upstream-preview"),
    ).toContainText("network-log.txt");
    adminPage.once("dialog", (dialog) => dialog.accept());
    await adminPage.getByTestId("support-action-submit").click();
    await expect
      .poll(
        () =>
          state.upstreamRequests.filter((item) =>
            item.path.endsWith("/confirmations"),
          ).length,
      )
      .toBe(1);

    await adminPage.getByRole("button", { name: "查询上游回复" }).click();
    await expect
      .poll(
        () =>
          state.upstreamRequests.filter((item) =>
            item.path.endsWith("/reply-syncs"),
          ).length,
      )
      .toBe(1);
    await expect(support.pageRoot).toContainText("已同步 1 条新上游回复");

    await adminPage.getByRole("button", { name: "重试明确失败的发送" }).click();
    await adminPage
      .getByRole("textbox", { name: "原因" })
      .fill("上游明确返回未收到");
    adminPage.once("dialog", (dialog) => dialog.accept());
    await adminPage.getByTestId("support-action-submit").click();
    await expect
      .poll(
        () =>
          state.upstreamRequests.filter((item) =>
            item.path.endsWith("/retries"),
          ).length,
      )
      .toBe(1);

    await adminPage.getByRole("button", { name: "审核后公开给客户" }).click();
    await adminPage
      .getByRole("textbox", { name: "原因" })
      .fill("已核对回复内容可公开");
    await adminPage.getByTestId("support-action-submit").click();
    await expect
      .poll(
        () =>
          state.upstreamRequests.filter((item) =>
            item.path.endsWith("/publications"),
          ).length,
      )
      .toBe(1);
  });

  test("TC007j: 真实 PostgreSQL 工单可由工作台正常关闭", async ({
    adminPage,
  }) => {
    const ticketNo = `TKT_FB94_CLOSE_${Date.now()}`;
    const reason = "FB94 浏览器关闭工单回归";
    execPgSQLStatements([
      `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,assignee_user_id,created_by,updated_by)
        VALUES (0,'${ticketNo}','internal','internal','FB94 关闭工单回归','in_progress','normal',1,CURRENT_TIMESTAMP,1,1,1);`,
    ]);

    try {
      const support = new SupportDeskPage(adminPage);
      await support.goto();
      await support.openTicket(ticketNo);
      await adminPage.getByTestId("support-action-close").click();
      await support.fillActionReason(reason);
      await support.submitAction();

      await expect(support.pageRoot).toContainText("已关闭");
      await expect(support.pageRoot).not.toContainText(
        "操作未完成，请刷新后重试。",
      );
    } finally {
      execPgSQLStatements([
        `DELETE FROM plugin_idc_core_support_ticket_status_history WHERE tenant_id=0 AND ticket_no='${ticketNo}';`,
        `DELETE FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no='${ticketNo}';`,
      ]);
    }
  });

  test("TC007k: 客户工单使用 Vben 表格与搜索表单且真实查询和详情可用", async ({ adminPage }) => {
    const fixturePrefix = `TKT_${Date.now()}`;
    const ticketNumbers = Array.from({ length: 12 }, (_, index) => `${fixturePrefix}_${index + 1}`);
    const ticketValues = ticketNumbers.map((ticketNo, index) =>
      `(0,'${ticketNo}','customer','customer',
        (SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_code=10000),
        (SELECT category_no FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND status_code='active' ORDER BY id LIMIT 1),
        'Vben 工单列表验收 ${index + 1}','${["new", "in_progress", "waiting_customer", "resolved", "closed"][index % 5]}','normal',1,CURRENT_TIMESTAMP)`,
    );
    execPgSQLStatements([
      `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,category_no,subject,base_stage,priority_code,current_round,last_activity_at)
        VALUES ${ticketValues.join(",")} ON CONFLICT (ticket_no) DO NOTHING;`,
    ]);
    let popup: Awaited<ReturnType<CustomerManagementPage['startAssistance']>> | undefined;
    try {
      const customers = new CustomerManagementPage(adminPage);
      await customers.goto();
      await customers.openOverviewByPublicId("10000");
      popup = await customers.startAssistance();
      await new CustomerAssistancePage(popup).waitUntilReady();
      const support = new CustomerSupportPage(popup);
      const initial = await support.goto();
      expect(initial.code).toBe(0);
      const tickets = initial.data.tickets as Array<{ ticketNo: string; subject: string }>;
      expect(tickets.length).toBeGreaterThan(0);
      await expect(support.rows).toHaveCount(tickets.length);
      for (const label of ["关键词", "工单编号", "问题标题", "问题分类", "状态", "最近更新", "查看详情"]) {
        await expect(support.root).toContainText(label);
      }
      expect((await support.geometry()).bodyHeight).toBeGreaterThan(200);
      const geometry = await support.geometry();
      expect(geometry.pageOverflow).toBeLessThanOrEqual(1);
      expect(geometry.pagerBottom).toBeLessThanOrEqual(geometry.viewportHeight);
      await support.screenshot("customer-support-vben-live");

      const searched = await support.search(tickets[0]!.subject);
      expect(searched.ok()).toBeTruthy();
      const result = await searched.json();
      expect(result.code).toBe(0);
      expect(result.data.tickets.length).toBeGreaterThan(0);
      await expect(support.rows).toHaveCount(result.data.tickets.length);
      await expect(support.rows.first()).toContainText(tickets[0]!.subject);
      await support.screenshot("customer-support-vben-searched");
      await support.openDetail(tickets[0]!.ticketNo);
      await expect(popup.getByText(tickets[0]!.subject, { exact: true }).first()).toBeVisible();
      await support.goto();
      for (let attempt = 0; attempt < 2; attempt++) {
        const drawer = await support.openCreate();
        await expect(drawer).toContainText("问题标题");
        await expect(drawer).toContainText("问题分类");
        const bounds = await drawer.boundingBox();
        expect(bounds).not.toBeNull();
        expect(bounds!.width).toBeGreaterThan(600);
        expect(bounds!.x).toBeGreaterThanOrEqual(0);
        expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(popup.viewportSize()!.width + 1);
        await support.screenshot(`customer-support-create-${attempt + 1}`);
        await support.closeCreate();
      }
      await support.useEnglish();
      for (const label of ["Keyword", "Ticket number", "Subject", "Problem category", "Status", "Last updated", "View details"]) {
        await expect(support.root).toContainText(label);
      }
      await expect(support.root).not.toContainText("support.");
      await support.screenshot("customer-support-vben-english");
      await support.resizeMobile();
      await expect.poll(async () => (await support.geometry()).pageOverflow).toBeLessThanOrEqual(1);
      const mobile = await support.geometry();
      expect(mobile.pagerBottom).toBeLessThanOrEqual(mobile.viewportHeight);
      await support.screenshot("customer-support-vben-mobile");
    } finally {
      await popup?.close();
      execPgSQLStatements([
        `DELETE FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no IN (${ticketNumbers.map((ticketNo) => `'${ticketNo}'`).join(",")});`,
      ]);
    }
  });

  test("TC007l: 客户工单完整分页、每页条数、搜索重置和读取失败保持列表且可重试", async ({ adminPage }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");
    const popup = await customers.startAssistance();
    try {
      await new CustomerAssistancePage(popup).waitUntilReady();
      const state = await installCustomerSupportListMock(popup);
      const support = new CustomerSupportPage(popup);
      await support.goto();
      await expect(support.rows).toHaveCount(10);
      await expect(support.pager).toContainText("共 12 条记录");
      for (const selector of [".vxe-pager--home-btn", ".vxe-pager--jump-prev", ".vxe-pager--jump-next", ".vxe-pager--end-btn", ".vxe-select"]) {
        await expect(support.pager.locator(selector)).toBeVisible();
      }
      await support.nextPage();
      await expect(support.rows).toHaveCount(2);
      await expect(support.activePage()).toHaveText("2");
      expect(state.requests.at(-1)).toEqual({ cursor: "", page: "2", query: "", limit: "10" });
      await support.previousPage();
      await expect(support.rows).toHaveCount(10);
      await expect(support.activePage()).toHaveText("1");
      await support.goToPage(2);
      await expect(support.rows).toHaveCount(2);
      await support.changePageSize(20);
      await expect(support.rows).toHaveCount(12);
      await expect(support.activePage()).toHaveText("1");
      expect(state.requests.at(-1)).toEqual({ cursor: "", page: "1", query: "", limit: "20" });
      await support.changePageSize(10);
      await expect(support.rows).toHaveCount(10);
      await support.search("网络");
      await expect(support.rows).toHaveCount(1);
      await expect(support.rows).toContainText("网络连接需要协助");
      await support.reset();
      await expect(support.rows).toHaveCount(10);
      await support.search("不存在的工单");
      await expect(support.rows).toHaveCount(0);
      await expect(support.root).toContainText("还没有工单");
      await support.reset();
      await expect(support.rows).toHaveCount(10);

      state.failNext = true;
      await support.nextPage();
      await expect(support.root).toContainText("暂时无法读取工单，请稍后重试");
      await expect(support.rows).toHaveCount(10);
      await expect(support.activePage()).toHaveText("1");
      await support.screenshot("customer-support-vben-failure");
      await support.retry();
      await expect(support.rows).toHaveCount(2);
      await expect(support.activePage()).toHaveText("2");
      state.failNext = true;
      await support.search("网络");
      await expect(support.rows).toHaveCount(2);
      await support.retry();
      await expect(support.rows).toHaveCount(1);
      await expect(support.activePage()).toHaveText("1");
      expect(state.requests.at(-1)).toEqual({ cursor: "", page: "1", query: "网络", limit: "10" });
      await support.screenshot("customer-support-vben-recovered");
      await support.reset();
      await expect(support.rows).toHaveCount(10);
      state.denyNext = true;
      await support.nextPage();
      await expect(support.root).toContainText("暂时无法读取工单，请稍后重试");
      await expect(support.rows).toHaveCount(0);
      await expect(support.pager).toContainText("共 0 条记录");
      await support.screenshot("customer-support-permission-revoked");
    } finally {
      await popup.close();
    }
  });
});
