import { expect } from "../../../../../../hack/tests/fixtures/auth";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";
import { customerTest as test } from "../support/customer-admin-session";

test.describe("TC-13 后台添加可登录客户",()=>{
 test("TC-13a: 仅手机号即可添加活动客户，不发送邀请",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openDirectCustomer();await view.createDirectCustomer({phone:"13900139000"});
  await expect.poll(()=>state.directRequests.length).toBe(1);expect(state.directRequests[0]).toMatchObject({phone:"13900139000",phoneCountryCode:"+86"});expect(state.activationRequests).toHaveLength(0);
  await expect(adminPage.getByTestId("customer-generated-password")).toBeVisible();await view.closeGeneratedPassword();await expect(view.pageRoot).toContainText("13900139000");await view.screenshot("customer-phone-only");
 });
 test("TC-13b: 添加表单只有邮箱和手机两个输入",async({adminPage})=>{
  await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openDirectCustomer();const dialog=adminPage.getByRole("dialog");
  await expect(dialog.locator("input")).toHaveCount(2);await expect(dialog).toContainText("邮箱");await expect(dialog).toContainText("手机");await expect(dialog).not.toContainText(/界面语言|原因|临时密码/u);await view.screenshot("customer-direct-form");
 });
 test("TC-13c: 邮箱格式错误时保留表单且不请求创建",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openDirectCustomer();await view.createDirectCustomer({email:"invalid-email"});
  await expect(adminPage.getByTestId("customer-direct-email")).toHaveValue("invalid-email");await expect(adminPage.getByRole("dialog")).toBeVisible();expect(state.directRequests).toHaveLength(0);await view.screenshot("customer-invalid-email");
 });
 test("TC-13d: 取消重置不改变密码",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();await view.openPasswordReset("CUS_0123456789abcdef0123456789abcdef");
  await adminPage.locator(".ant-modal-confirm:visible").getByRole("button",{name:/取\s*消/u}).click();expect(state.passwordResetRequests).toHaveLength(0);await expect(adminPage.getByTestId("customer-generated-password")).toHaveCount(0);
 });
});
