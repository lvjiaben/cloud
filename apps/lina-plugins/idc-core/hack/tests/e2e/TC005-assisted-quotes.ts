import { customerTest as test } from "../support/customer-admin-session";
import { expect } from "../../../../../../hack/tests/fixtures/auth";
import { MainLayout } from "@host-tests/pages/MainLayout";
import { AssistedQuotesPage } from "../pages/AssistedQuotesPage";
import { installAssistedQuoteMock } from "../support/assisted-quote-mock";

test.describe("TC-5 客户管理协助下单", () => {

 test("TC-5a: 客户已选定、日期时间控件、配置及失败重试保持幂等", async ({adminPage}) => {
  const state=await installAssistedQuoteMock(adminPage,{createFailures:1});
  const view=new AssistedQuotesPage(adminPage); await view.goto();
  await expect(adminPage.getByTestId("customer-assist-CUS_ASSISTED_0001")).toHaveText("协助下单");
  await view.screenshot("assist-customer-list"); await view.openCreate();
  await expect(adminPage.getByTestId("assisted-create-selected-customer")).toContainText("协助报价客户");
  await expect(view.createDrawer).toContainText("付款截止时间");
  await view.checkDeadlinePicker();
  await expect(view.createDrawer).not.toContainText("审批"); await view.screenshot("assist-direct-empty");
  await view.submitCreate(); await expect(adminPage.getByTestId("assisted-create-error")).toBeVisible();
  expect(state.createBodies).toHaveLength(0); await view.fillCreate(); await view.screenshot("assist-direct-configured");
  await view.submitCreate(); await expect.poll(()=>state.createBodies.length).toBe(1);
  await expect(adminPage.getByTestId("assisted-create-error")).toBeVisible();
  await expect(view.createDrawer.getByLabel("主机名称",{exact:true})).toHaveValue("assisted-test-server");
  await view.screenshot("assist-direct-error"); await view.submitCreate(); await expect.poll(()=>state.createBodies.length).toBe(2);
  expect(state.createBodies[1]).toEqual(state.createBodies[0]);
  expect(state.createBodies[0]).toMatchObject({customerNo:"CUS_ASSISTED_0001",currencyCode:"USD",manualDiscountMinor:"1234",contractPolicyCode:"inherit",items:[{publicSlug:"assisted-server",quantity:2,purchaseCycles:3,values:[{fieldToken:"field-hostname",type:"single_line",textValue:"assisted-test-server"}]}]});
  expect(Number(state.createBodies[0]!.paymentDueAt)).toBeGreaterThan(Date.now());
  for (const field of ["statusCode","paid","validitySeconds","paymentTermCode"]) expect(state.createBodies[0]).not.toHaveProperty(field);
  await expect(view.createDrawer).toBeHidden(); await view.screenshot("assist-direct-success");
 });
 test("TC-5e: 添加商品实际增加第二个可配置商品",async({adminPage})=>{
  await installAssistedQuoteMock(adminPage);const view=new AssistedQuotesPage(adminPage);await view.goto();await view.openCreate();await view.addProduct();
  await expect(adminPage.getByTestId("assisted-item-product-1")).toBeVisible();
  await expect(adminPage.getByTestId("assisted-create-add-item")).toContainText("2/8");
  await view.select("assisted-item-product-1","高内存云服务器");
  await expect(view.createDrawer.getByLabel("主机名称",{exact:true})).toBeVisible();await view.screenshot("assist-added-product");
 });
 test("TC-5b: 有协助权限无需客户详情权限，没有协助权限隐藏按钮",async({adminPage})=>{
  await installAssistedQuoteMock(adminPage,{withoutCustomerQuery:true}); const view=new AssistedQuotesPage(adminPage);await view.goto();await view.openCreate();
  await expect(adminPage.getByRole("button",{name:"生成客户订单",exact:true})).toBeVisible();
  await installAssistedQuoteMock(adminPage,{withoutCreate:true});await view.goto();
  await expect(adminPage.getByTestId("customer-assist-CUS_ASSISTED_0001")).toHaveCount(0); await view.screenshot("assist-direct-no-permission");
 });
 test("TC-5c: 英文窄屏可完整填写和提交",async({adminPage})=>{
  const state=await installAssistedQuoteMock(adminPage);const view=new AssistedQuotesPage(adminPage);await view.goto();
  await new MainLayout(adminPage).switchLanguage("English");await adminPage.setViewportSize({width:480,height:920});await view.openCreate();
  await expect(view.createDrawer).toContainText("Payment deadline");await expect(view.createDrawer).not.toContainText("plugin.idc-core.assistedCreate.");
  await view.fillCreate(true);await view.screenshot("assist-direct-mobile-en");await view.submitCreate(true);
  await expect.poll(()=>state.createBodies.length).toBe(1);await expect(view.createDrawer).toBeHidden();
 });
 test("TC-5d: 客户私有必填文件阻止后台下单",async({adminPage})=>{
  const state=await installAssistedQuoteMock(adminPage,{file:"required"});const view=new AssistedQuotesPage(adminPage);await view.goto();await view.openCreate();await view.fillCreate();await view.submitCreate();
  await expect(adminPage.getByTestId("assisted-create-error")).toContainText("上传资料");expect(state.createBodies).toHaveLength(0);await view.screenshot("assist-direct-file-required");
 });
});
