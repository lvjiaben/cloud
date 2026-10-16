import { customerTest as test } from "../support/customer-admin-session";
import { expect } from "../../../../../../hack/tests/fixtures/auth";
import { MainLayout } from "@host-tests/pages/MainLayout";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";

const customerNo="CUS_0123456789abcdef0123456789abcdef";
test.describe("TC-35 简单客户管理",()=>{

 test("TC-35a: 添加只需联系方式，失败保留内容及随机密码，成功显示同一密码",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);state.directFailures=1;
  const view=new CustomerManagementPage(adminPage);await view.goto();
  await expect(adminPage.getByTestId("customer-direct-open")).toHaveText(/^添\s*加$/u);await view.assertToolbarAlignment();
  await expect(adminPage.getByTestId("customer-tag-batch-open")).toBeDisabled();
  await view.selectCustomer(customerNo);
  await expect(adminPage.getByTestId("customer-tag-batch-open")).toBeEnabled();
  await view.selectCustomer(customerNo);
  await expect(adminPage.getByTestId("customer-tag-batch-open")).toBeDisabled();
  await view.assertToolbarAlignment();await view.screenshot("customer-toolbar-aligned");
  await expect(view.customerRow(customerNo)).toContainText("13800138000");await expect(view.customerRow(customerNo)).toContainText("customer@example.test");
  await view.openDirectCustomer();const dialog=adminPage.getByRole("dialog");
  await expect(dialog.locator("input")).toHaveCount(2);await expect(dialog).not.toContainText(/激活|原因|界面语言|初始密码/u);
  await view.createDirectCustomer({});expect(state.directRequests).toHaveLength(0);
  await view.createDirectCustomer({email:"new@example.test",phone:"13900139000"});await expect.poll(()=>state.directRequests.length).toBe(1);
  await expect(adminPage.getByTestId("customer-generated-password")).toHaveCount(0);await view.screenshot("customer-add-error");
  await view.createDirectCustomer({});await expect.poll(()=>state.directRequests.length).toBe(2);
  expect(state.directRequests[1]).toEqual(state.directRequests[0]);expect(state.directRequests[0]).toMatchObject({email:"new@example.test",phone:"13900139000"});
  await expect(adminPage.getByTestId("customer-generated-password")).toHaveText(String(state.directRequests[0]!.initialPassword));
  expect(String(state.directRequests[0]!.initialPassword)).toMatch(/^Aa7!.{20}$/);expect(state.activationRequests).toHaveLength(0);
  await view.screenshot("customer-added-password");
 });
 test("TC-35b: 重置只有确认，随机密码成功后展示",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openPasswordReset(customerNo);
  await expect(adminPage.locator(".ant-modal-confirm:visible input")).toHaveCount(0);await expect(adminPage.locator(".ant-modal-confirm:visible")).toContainText("13800138000");
  await view.confirmPasswordReset();await expect.poll(()=>state.passwordResetRequests.length).toBe(1);
  await expect(adminPage.getByTestId("customer-generated-password")).toHaveText(String(state.passwordResetRequests[0]!.newPassword));await view.screenshot("customer-reset-password");
 });
 test("TC-35c: 详情状态、分组和记录可操作，页签不改变宽度",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openOverview(customerNo);
  await expect(view.overviewDrawer.locator(".ant-card-head-wrapper")).toHaveCount(0);await expect(view.overviewDrawer).not.toContainText("运营足迹");
  expect(await view.copyCustomerNumber()).toBe("10000");
  const initial=await view.overviewDrawer.boundingBox();await view.toggleAccount();await expect.poll(()=>state.statusRequests.length).toBe(1);await expect(view.overviewDrawer.getByTestId("customer-ban-toggle")).toHaveText(/解\s*封/u);
  await view.toggleAccount();await expect.poll(()=>state.statusRequests.length).toBe(2);await expect(view.overviewDrawer.getByTestId("customer-ban-toggle")).toHaveText(/封\s*禁/u);
  await view.addGroup("新客户分组");await expect.poll(()=>state.segmentMutations.length).toBe(1);await expect(view.overviewDrawer).toContainText("新客户分组");
  await view.addGroup("电话联系", "tag");await expect.poll(()=>state.segmentMutations.length).toBe(2);await expect(view.overviewDrawer).toContainText("电话联系");
  await view.removeSegment("新客户分组");await expect.poll(()=>state.segmentMutations.length).toBe(3);expect(state.segmentMutations[2]).toMatchObject({kind:"group",removed:true});await expect(view.overviewDrawer).not.toContainText("新客户分组");
  await view.removeSegment("电话联系");await expect.poll(()=>state.segmentMutations.length).toBe(4);expect(state.segmentMutations[3]).toMatchObject({kind:"tag",removed:true});await expect(view.overviewDrawer).not.toContainText("电话联系");
  await view.openDetailTab("操作记录");await expect.poll(()=>state.auditRequests.length).toBe(1);await expect(view.overviewDrawer).toContainText("重置密码");
  const records=await view.overviewDrawer.boundingBox();expect(Math.abs(records!.width-initial!.width)).toBeLessThan(2);
  await view.screenshot("customer-detail-records");await view.openDetailTab("备注");await expect(view.overviewDrawer.getByRole("button",{name:"添加备注",exact:true})).toBeVisible();
  await view.addNote("客户希望电话联系");await expect.poll(()=>state.noteRequests.length).toBe(1);expect(state.noteRequests[0]).toMatchObject({category:"general",content:"客户希望电话联系"});await expect(view.overviewDrawer).toContainText("客户希望电话联系");
  expect((await view.overviewDrawer.boundingBox())!.width).toBeCloseTo(initial!.width,0);await view.screenshot("customer-detail-notes");
 });
 test("TC-35d: 英文窄屏入口与表单明确可读",async({adminPage})=>{
  await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await new MainLayout(adminPage).switchLanguage("English");await adminPage.setViewportSize({width:480,height:920});
  await expect(adminPage.getByTestId("customer-direct-open")).toHaveText("Add");await view.openDirectCustomer();await expect(adminPage.getByRole("dialog")).toContainText("Email");await expect(adminPage.getByRole("dialog")).toContainText("Phone");await expect(adminPage.getByRole("dialog")).not.toContainText("plugin.idc-core");await view.screenshot("customer-add-mobile-en");
 });
 test("TC-35e: 输入标签回车，取消不保存，失败保留并可重试",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.selectCustomer(customerNo);
  await view.openTagBatch();
  await expect(adminPage.getByTestId("customer-tag-batch-tags")).toContainText("输入标签后回车，或选择已有标签");
  await view.enterBatchTag("回车标签");await view.cancelTagBatch();expect(state.batchRequests).toHaveLength(0);expect(state.segmentMutations).toHaveLength(0);
  await view.openTagBatch();await view.enterBatchTag("回车标签");state.batchFailures=1;
  await view.confirmTagBatch();await expect.poll(()=>state.batchRequests.length).toBe(1);await expect(adminPage.getByRole("dialog")).toContainText("回车标签");await view.screenshot("customer-batch-tag-error");
  await view.confirmTagBatch();await expect.poll(()=>state.batchRequests.length).toBe(2);expect(state.batchRequests[1]).toEqual(state.batchRequests[0]);expect(state.batchRequests[0]).toMatchObject({tagNos:[],tagNames:["回车标签"]});await expect(adminPage.getByRole("dialog")).toBeHidden();
 });
 test("TC-35f: 详情回车添加后，下拉可直接复用，不出现新建表单",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openOverview(customerNo);
  await expect(adminPage.getByTestId("customer-tag-add")).toHaveCount(0);
  await view.addGroup("已输入标签","tag");await expect.poll(()=>state.segmentMutations.length).toBe(1);await expect(view.overviewDrawer).toContainText("已输入标签");
  await view.chooseExistingDetailTag("已输入标签");await expect.poll(()=>state.segmentMutations.length).toBe(2);
  expect(state.segmentMutations[1]).toMatchObject({kind:"tag",segmentNo:"TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"});expect(state.segmentMutations[1]?.name).toBeUndefined();
  await expect(view.overviewDrawer.locator(".segment-list .ant-tag").filter({hasText:"已输入标签"})).toHaveCount(1);
  await view.screenshot("customer-detail-inline-tags");
 });
});
