import { expect } from "../../../../../../hack/tests/fixtures/auth";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";
import { customerTest as test } from "../support/customer-admin-session";

test.describe("TC-9 后台统一添加入口",()=>{
 test("TC-9a: 不再显示激活邀请入口，添加只提交一次随机密码账户",async({adminPage})=>{
  const state=await installCustomerManagementMock(adminPage);const view=new CustomerManagementPage(adminPage);await view.goto();
  await expect(view.pageRoot.getByTestId("customer-activation-open")).toHaveCount(0);await expect(view.pageRoot.getByTestId("customer-direct-open")).toHaveCount(1);
  await view.openDirectCustomer();await view.createDirectCustomer({email:"buyer@example.test"});await expect.poll(()=>state.directRequests.length).toBe(1);
  expect(state.activationRequests).toHaveLength(0);expect(state.directRequests[0]).toMatchObject({email:"buyer@example.test",customerType:"individual"});
  await expect(adminPage.getByTestId("customer-generated-password")).toHaveText(String(state.directRequests[0]!.initialPassword));await view.closeGeneratedPassword();
  await expect(view.pageRoot).toContainText("buyer@example.test");await expect(view.pageRoot).not.toContainText("待激活");await view.screenshot("customer-unified-add");
 });
});
