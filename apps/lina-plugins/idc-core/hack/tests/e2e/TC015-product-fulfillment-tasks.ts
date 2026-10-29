import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProductFulfillmentPage } from "../pages/ProductFulfillmentPage";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { installProductFulfillmentMock } from "../support/product-fulfillment-mock";

import { installRefundManagementMock, managementRefundNo } from "../support/refund-management-mock";

const permissions = [
  "idc:admin:product-fulfillment:task:list",
  "idc:admin:product-fulfillment:task:query",
  "idc:admin:product-fulfillment:review-approve",
  "idc:admin:product-fulfillment:review-reject",
  "idc:admin:product-fulfillment:takeover",
  "idc:admin:product-fulfillment:manual-conclude",
  "idc:admin:product-fulfillment:recover",
];

test.describe("TC-15 商品履约任务安全操作", () => {
  test("TC-15ab: 核心冻结配置失败显示本地校验原因，仍需核对原开通记录", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,diagnosticCode:"resource_preparation_request_invalid",responseExcerpt:JSON.stringify({stage:"owner_setup",errorCode:"resource_preparation_request_invalid",businessRequestStarted:false})});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail()).toContainText("本地开通请求未通过校验，请管理员核对成交配置与开通记录。");
    await expect(fulfillment.detail().getByTestId("fulfillment-confirmed-not-sent")).toHaveCount(0);
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeDisabled();
    await fulfillment.screenshot("core-frozen-request-validation-diagnostic");expect(state.actionBodies).toHaveLength(0);
  });

  test("TC-15z: 本地资源任务校验失败可读，但诊断不能代替未受理证明", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,diagnosticCode:"resource_preparation_task_input_invalid",responseExcerpt:JSON.stringify({stage:"owner_setup",errorCode:"resource_preparation_task_input_invalid",businessRequestStarted:false})});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail()).toContainText("本地资源任务校验失败，尚未建立本次开通任务");
    const call=fulfillment.detail().getByTestId("fulfillment-call-failure");
    await expect(call).toContainText("接口执行环境准备失败");
    await expect(call).toContainText("该步骤在尝试发送业务请求之前失败");
    await expect(fulfillment.detail().getByTestId("fulfillment-confirmed-not-sent")).toHaveCount(0);
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeDisabled();
    await call.scrollIntoViewIfNeeded();await fulfillment.screenshot("local-task-validation-staff-diagnostic");expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15aa: 英文本地存储异常保留不确定性，不宣称没有发送", async ({adminPage,mainLayout}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,diagnosticCode:"resource_preparation_task_storage_failed",responseExcerpt:JSON.stringify({stage:"owner_setup",errorCode:"resource_preparation_task_storage_failed"})});
    await mainLayout.switchLanguage("English");
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();
    const drawer=adminPage.locator('[role="dialog"]:visible').last(),call=drawer.getByTestId("fulfillment-call-failure");
    await expect(drawer).toContainText("The local provisioning task could not be saved reliably.");
    await expect(call).toContainText("This record cannot establish whether the request was sent");
    await expect(drawer.getByTestId("fulfillment-confirmed-not-sent")).toHaveCount(0);
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeDisabled();
    await expect(drawer).not.toContainText("plugin.idc-core.");
    const withinWidth = () => drawer.evaluate(element => {
      const bounds=element.getBoundingClientRect();
      const cells=[...element.querySelectorAll('[data-testid="fulfillment-execution-diagnostic"] .ant-descriptions-item-content')];
      return cells.length===4 && element.scrollWidth <= element.clientWidth+1 && cells.every(cell => {
        const box=cell.getBoundingClientRect(), range=document.createRange();
        range.selectNodeContents(cell);
        return box.left>=bounds.left-1 && box.right<=bounds.right+1 && [...range.getClientRects()].every(rect => !rect.width || (rect.left>=bounds.left-1 && rect.right<=bounds.right+1));
      });
    });
    await expect.poll(withinWidth).toBe(true);
    await call.scrollIntoViewIfNeeded();await fulfillment.screenshot("local-task-storage-unconfirmed-english");
    await adminPage.setViewportSize({width:390,height:844});
    await expect(drawer.locator('.ant-descriptions').first().locator('tr').first().locator('th')).toHaveCount(1);
    await expect.poll(withinWidth).toBe(true);
    await expect.poll(() => adminPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await expect(drawer).toContainText("The local provisioning task could not be saved reliably.");
    await fulfillment.screenshot("local-task-storage-unconfirmed-english-mobile");expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15y: 英文明示已确认未发送，管理员处理入口可用", async ({adminPage,mainLayout}) => {
    await installCatalogAutomationMock(adminPage,{permissions:[...permissions,"idc:admin:billing:refund:review"]});
    const state=await installProductFulfillmentMock(adminPage,{recovery:true,diagnosticCode:"provider_request_not_sent"});
    await mainLayout.switchLanguage("English");const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();const drawer=adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer).toContainText("Local preparation failed; the resource owner confirmed no provisioning submission was attempted.");
    await expect(drawer.getByTestId("fulfillment-confirmed-not-sent")).toContainText("The system confirmed no provisioning submission was attempted");
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeEnabled();
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeInViewport();
    await expect(drawer).not.toContainText("This historical attempt did not retain API call details");
    await expect(drawer.getByRole("button",{name:"Review upstream result",exact:true})).toHaveCount(0);
    await fulfillment.screenshot("confirmed-not-sent-english");expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15x: 拥有方确认未发送后可修正重开或退款，不再要求人工确认未知", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage,{permissions:[...permissions,"idc:admin:billing:refund:review"]});
    const state=await installProductFulfillmentMock(adminPage,{recovery:true,diagnosticCode:"provider_request_not_sent",responseExcerpt:JSON.stringify({stage:"request_preparation",errorCode:"IDC_RESOURCE_PROVIDER_LOCAL_AUTH_POLICY",businessRequestStarted:false,responseReceived:false,businessRequestsAttempted:0})});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByTestId("fulfillment-confirmed-not-sent")).toContainText("系统已确认本次开通请求未发送");
    await expect(fulfillment.detail()).toContainText("管理员可修正上游配置后选择重新开通");
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeInViewport();
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeEnabled();
    await expect(fulfillment.detail().getByRole("button",{name:/退\s*款/u})).toBeEnabled();
    await expect(fulfillment.detail().getByRole("button",{name:"核对上游结果",exact:true})).toHaveCount(0);
    await fulfillment.screenshot("confirmed-not-sent-staff-choices");
    await fulfillment.openRecoveryAction();await expect(fulfillment.actionModal().getByRole("combobox",{name:"上游节点"})).toBeEnabled();expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("confirmed-not-sent-correction-form");
  });
  test("TC-15v: 无响应时展示中文发送前失败环节，不开放盲重开", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,responseExcerpt:JSON.stringify({stage:"request_preparation",errorCode:"request_preparation",detail:"Local request preparation failed before the business HTTP call",businessRequestStarted:false,responseReceived:false})});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    const call=fulfillment.detail().getByTestId("fulfillment-call-failure");
    await expect(call).toContainText("发送前配置或凭据校验未通过");
    await expect(call).toContainText("该步骤在尝试发送业务请求之前失败");
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeDisabled();
    await call.scrollIntoViewIfNeeded();await fulfillment.screenshot("call-failure-before-submission");
    expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15w: 英文交接失败说明不假称已发送或收到响应", async ({adminPage,mainLayout}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,responseExcerpt:JSON.stringify({stage:"provider_dispatch",errorCode:"provider_dispatch",detail:"The connector did not hand a business request to the resource owner"})});
    await mainLayout.switchLanguage("English");
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();
    const drawer=adminPage.locator('[role="dialog"]:visible').last();const call=drawer.getByTestId("fulfillment-call-failure");
    await expect(call).toContainText("The connector did not complete the provisioning request handoff");
    await expect(call).toContainText("This record cannot establish whether the request was sent");
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeDisabled();
    await call.scrollIntoViewIfNeeded();await fulfillment.screenshot("call-failure-handoff-en");expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15s: 核对未创建需要确认和说明，重试幂等并开放原单处理",async({adminPage})=>{
    await installCatalogAutomationMock(adminPage,{permissions:[...permissions,"idc:admin:billing:refund:review"]});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,verification:true,loseFirstActionResponse:true});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.screenshot("non-acceptance-unknown-detail");
    await fulfillment.detail().getByRole("button",{name:"核对上游结果",exact:true}).click();
    const modal=fulfillment.actionModal();
    await expect(modal).toContainText("确认上游未创建");
    await expect(modal.getByRole("button",{name:/确.*认/u})).toBeDisabled();
    await modal.getByRole("checkbox").check();
    await modal.getByLabel("处理说明").fill("短说明");
    await fulfillment.submitManualAction();await expect(modal).toContainText("请填写10至512个字的处理说明。");
    expect(state.actionBodies).toHaveLength(0);
    const note="已核对上游实例列表和开通任务记录，均未创建此订单对应资源。";
    await modal.getByLabel("处理说明").fill(note);
    await fulfillment.screenshot("non-acceptance-verification-form");
    await fulfillment.submitManualAction();await expect(modal).toBeVisible();
    await expect(modal.getByLabel("处理说明")).toHaveValue(note);
    await fulfillment.screenshot("non-acceptance-response-lost");
    await fulfillment.submitManualAction();await expect(modal).toBeHidden();
    expect(state.actionBodies).toHaveLength(2);expect(state.actionBodies[1]).toEqual(state.actionBodies[0]);
    expect(state.actionBodies[0]).toMatchObject({expectedVersion:3,confirmedNoCreation:true,note});
    await expect(fulfillment.detail().getByTestId("fulfillment-non-acceptance-history")).toContainText(note);
    await expect(fulfillment.detail()).toContainText("管理员已核对确认上游未创建实例或任务，原未知记录仍保留。");
    await expect(fulfillment.detail().getByRole("button",{name:"重新开通",exact:true})).toBeEnabled();
    await expect(fulfillment.detail().getByRole("button",{name:/退\s*款/u})).toBeEnabled();
    await expect(fulfillment.detail().getByRole("button",{name:"核对上游结果",exact:true})).toHaveCount(0);
    await fulfillment.screenshot("non-acceptance-verified-processing-choices");
  });
  test("TC-15t: 英文核对文案及独立恢复权限",async({adminPage,mainLayout})=>{
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,verification:true});
    await mainLayout.switchLanguage("English");
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();
    const drawer=adminPage.locator('[role="dialog"]:visible').last();
    await drawer.getByRole("button",{name:"Review upstream result",exact:true}).click();
    const modal=fulfillment.actionModal();
    await expect(modal).toContainText("Confirm upstream non-acceptance");
    await expect(modal.getByRole("checkbox")).toHaveAccessibleName("I checked upstream and confirmed there is no instance and no accepted or running provisioning task");
    await fulfillment.screenshot("non-acceptance-verification-en");
    expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15u: 无恢复权限不能人工确认未知订单",async({adminPage})=>{
    await installCatalogAutomationMock(adminPage,{permissions:permissions.filter(p=>!p.endsWith(":recover"))});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,verification:true});
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button",{name:"核对上游结果",exact:true})).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("non-acceptance-permission-denied");
  });
  test("TC-15q: 明确拒绝显示具体接口响应并可进入配置修正", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recovery:true,responseExcerpt:JSON.stringify({method:"POST",route:"/clouds",httpStatus:200,response:{status:400,msg:"镜像在当前节点未下载",_redactedFields:true}})});
    const fulfillment=new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();await fulfillment.openDetail("ORD-MOCK-1");
    const response=fulfillment.detail().getByTestId("fulfillment-http-response");
    await expect(response).toContainText("接口调用详情与脱敏响应");
    await expect(response).toContainText(/"httpStatus":\s*200/u);
    await expect(response).toContainText(/"status":\s*400/u);
    await expect(response).toContainText("镜像在当前节点未下载");
    await response.scrollIntoViewIfNeeded();
    await fulfillment.screenshot("fulfillment-http-response-rejection");
    await fulfillment.openRecoveryAction();
    await expect(fulfillment.actionModal().getByRole("combobox",{name:"上游节点"})).toBeEnabled();
    expect(state.actionBodies).toHaveLength(0);
  });
  test("TC-15r: 英文显示脱敏响应，异常回包仍不能盲重开", async ({adminPage,mainLayout}) => {
    await installCatalogAutomationMock(adminPage,{permissions});
    const state=await installProductFulfillmentMock(adminPage,{recoveryUnavailable:true,responseExcerpt:'{"httpStatus":502,"response":"Bad Gateway"}'});
    await mainLayout.switchLanguage("English");
    const fulfillment=new ProductFulfillmentPage(adminPage);await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();
    const drawer=adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer).toContainText("API call details and sanitized response");
    await expect(drawer.getByTestId("fulfillment-http-response")).toContainText("Bad Gateway");
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeDisabled();
    expect(state.actionBodies).toHaveLength(0);
    await drawer.getByTestId("fulfillment-http-response").scrollIntoViewIfNeeded();
    await fulfillment.screenshot("fulfillment-http-response-unknown-en");
  });
  test("TC-15o: 修正节点和镜像后原单开通，保留金额规格及修正记录", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage, {permissions});
    const state = await installProductFulfillmentMock(adminPage, {recovery: true});
    adminPage.setDefaultTimeout(15_000);
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto(); await fulfillment.openDetail("ORD-MOCK-1"); await fulfillment.openRecoveryAction();
    const modal = fulfillment.actionModal();
    await expect(modal).toContainText("原成交规格及金额不变");
    await expect(modal.getByTestId("recovery-paid-specifications")).toContainText("4096");
    await expect(modal.getByRole("combobox", {name:"上游节点"})).toBeEnabled();
    await fulfillment.selectRecoveryConfiguration("上游节点", "西安备用节点");
    await expect.poll(() => state.recoveryConfigurationRequests.at(-1)?.searchParams.get("recoveryLine")).toBe("6");
    await expect(modal.getByRole("combobox", {name:"系统镜像"})).toBeEnabled();
    await fulfillment.selectRecoveryConfiguration("系统镜像", "Ubuntu 24");
    await expect.poll(() => state.recoveryConfigurationRequests.at(-1)?.searchParams.get("recoveryOperatingSystem")).toBe("98");
    await fulfillment.fillRecoveryAction("已核对替换节点的镜像与可用资源，保持原成交规格。");
    await expect(modal.getByRole("combobox")).toHaveCount(5);
    await fulfillment.screenshot("recovery-correct-node-and-image");
    await fulfillment.submitManualAction();
    await expect(modal).toBeHidden();
    expect(state.actionBodies).toHaveLength(1);
    expect(state.actionBodies[0]).toMatchObject({expectedVersion:3,recoveryConfiguration:{region:"1",line:"6",operatingSystem:"98",lineLabel:"西安备用节点",operatingSystemLabel:"Ubuntu 24"}});
    expect(state.actionBodies[0]).not.toHaveProperty("amount");
    expect(state.actionBodies[0]?.recoveryConfiguration).not.toHaveProperty("cpu");
    await expect(fulfillment.detail().getByTestId("fulfillment-recovery-configurations")).toContainText("西安备用节点");
    await expect(fulfillment.detail().getByTestId("fulfillment-recovery-configurations")).toContainText("Ubuntu 24");
    await fulfillment.screenshot("recovery-corrected-configuration-history");
  });
  test("TC-15p: 上游配置读取失败时禁止提交，重试保留处理说明", async ({adminPage}) => {
    await installCatalogAutomationMock(adminPage, {permissions});
    const state = await installProductFulfillmentMock(adminPage, {recovery:true,failRecoveryConfiguration:true});
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto(); await fulfillment.openDetail("ORD-MOCK-1"); await fulfillment.openRecoveryAction();
    const modal = fulfillment.actionModal();
    await expect(modal).toContainText("无法读取上游配置");
    await expect(modal.getByRole("button", {name:/确.*认/u})).toBeDisabled();
    await modal.getByLabel("处理说明").fill("修正配置读取失败后重试，说明应完整保留。");
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("recovery-correction-unavailable");
    state.failRecoveryConfiguration = false;
    await modal.getByRole("button", {name:/重.*试/u}).click();
    await expect(modal.getByRole("combobox", {name:"对接上游"})).toBeEnabled();
    await expect(modal.getByLabel("处理说明")).toHaveValue("修正配置读取失败后重试，说明应完整保留。");
    expect(state.actionBodies).toHaveLength(0);
  });

  test("TC-15l: 新单未知记录显示缺失原因和禁用操作的具体说明", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, {permissions: [...permissions, "idc:admin:billing:refund:review"]});
    const state = await installProductFulfillmentMock(adminPage, {recoveryUnavailable: true});
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.screenshot("exception-diagnostic-list");
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail()).toContainText("这次执行未记录更具体的错误原因");
    await expect(fulfillment.detail()).toContainText("核对未确认的开通结果");
    await expect(fulfillment.detail()).toContainText("请先核对上游任务和实例记录");
    await expect(fulfillment.detail().getByRole("button", {name: "重新开通", exact: true})).toBeDisabled();
    await expect(fulfillment.detail().getByRole("button", {name: /退\s*款/u})).toBeDisabled();
    expect(state.actionBodies).toHaveLength(0);
    expect(state.recoveryConfigurationRequests).toHaveLength(0);
    await fulfillment.screenshot("exception-diagnostic-unknown");
  });
  test("TC-15m: 具体超时原因不会把未知结果变成可重开", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, {permissions});
    const state = await installProductFulfillmentMock(adminPage, {recoveryUnavailable: true, diagnosticCode: "idc_resource_provider_timeout"});
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto(); await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail()).toContainText("调用上游接口超时");
    await expect(fulfillment.detail()).not.toContainText("idc_resource_provider_timeout");
    await expect(fulfillment.detail()).not.toContainText("plugin.idc-core.fulfillment");
    await expect(fulfillment.detail().getByRole("button", {name: "重新开通", exact: true})).toBeDisabled();
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("exception-diagnostic-timeout");
  });
  test("TC-15n: 英文明确信息和原单处理入口可见", async ({ adminPage, mainLayout }) => {
    await installCatalogAutomationMock(adminPage, {permissions: [...permissions, "idc:admin:billing:refund:review"]});
    await installProductFulfillmentMock(adminPage, {recovery: true});
    await mainLayout.switchLanguage("English");
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button", {name: /detail/iu}).first().click();
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer).toContainText("The upstream explicitly rejected this request before acceptance");
    await expect(drawer.getByRole("button", {name: "Provision again", exact: true})).toBeEnabled();
    await expect(drawer.getByRole("button", {name: "Refund", exact: true})).toBeEnabled();
    await expect(drawer).not.toContainText("plugin.idc-core.fulfillment");
    await fulfillment.screenshot("exception-diagnostic-rejected-en");
    await drawer.getByRole("button", {name: "Provision again", exact: true}).click();
    const editor = adminPage.locator('[role="dialog"]:visible').filter({has: adminPage.getByRole("combobox", {name: "Upstream interface"})});
    await expect(editor.getByRole("combobox", {name: "Upstream node"})).toBeEnabled();
    await expect(editor.getByRole("combobox", {name: "System image"})).toBeEnabled();
    await expect(editor).not.toContainText("plugin.idc-core.fulfillment");
    await fulfillment.screenshot("recovery-configuration-en");
  });

  test("TC-15j: 管理员选择退款后进入原财务退款流程，不提交重新开通", async ({adminPage}) => {
    const refunds = await installRefundManagementMock(adminPage, {failedDelivery: true});
    await installCatalogAutomationMock(adminPage, {permissions: [...permissions, "idc:admin:billing:refund:review", "idc:admin:billing:refund:query", "idc:admin:billing:refund:list"]});
    const fulfillmentState = await installProductFulfillmentMock(adminPage, {recovery: true});
    const requests: Array<Record<string, unknown>> = [];
    await adminPage.route("**/admin/billing/failed-fulfillments/PFI-MOCK-1/refunds", async route => {
      requests.push(route.request().postDataJSON());
      await route.fulfill({json: {code: 0, data: {refund: {refundNo: managementRefundNo}}}});
    });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto(); await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button", {name: "重新开通", exact: true})).toBeVisible();
    await fulfillment.detail().getByRole("button", {name: /退\s*款/u}).click();
    const modal = adminPage.locator('[role="dialog"]:visible').filter({hasText: "申请退款"}).last();
    await expect(modal).toContainText("保留原付款记录");
    await expect(modal.getByLabel("处理原因")).toHaveCount(0);
    await modal.getByLabel("处理说明").fill("供应商已明确拒绝开通，管理员选择退款处理");
    await modal.getByRole("button", {name: /确.*认/u}).click();
    await expect(adminPage).toHaveURL(/tab=refunds.*refundNo=RFD_MANAGEMENT_0001/);
    await expect(adminPage.getByTestId("refund-detail-drawer-content")).toBeVisible();
    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({expectedVersion: 3, note: "供应商已明确拒绝开通，管理员选择退款处理"});
    expect(String(requests[0]?.idempotencyKey).length).toBeGreaterThanOrEqual(16);
    expect(fulfillmentState.actionBodies).toHaveLength(0);
    expect(refunds.detailRequests).toEqual([managementRefundNo]);
    await expect(adminPage.getByTestId("refund-detail-drawer-content")).not.toContainText("这项长期权益需要终止");
    await fulfillment.screenshot("failed-delivery-refund-workflow");
  });
  test("TC-15k: 英文失败详情可发现重新开通与退款入口", async ({adminPage, mainLayout}) => {
    await installCatalogAutomationMock(adminPage, {permissions: [...permissions, "idc:admin:billing:refund:review"]});
    await installProductFulfillmentMock(adminPage, {recovery: true});
    await mainLayout.switchLanguage("English");
    const fulfillment = new ProductFulfillmentPage(adminPage);
    // English assertions use the same detail drawer without Chinese funding labels.
    await fulfillment.goto();
    await fulfillment.pageRoot.getByRole("button",{name:/detail/iu}).first().click();
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer.getByRole("button",{name:"Provision again",exact:true})).toBeVisible();
    await expect(drawer.getByRole("button",{name:"Refund",exact:true})).toBeVisible();
    await expect(drawer).not.toContainText("plugin.idc-core.fulfillment");
    await fulfillment.screenshot("failed-delivery-staff-actions-en");
  });
  test("TC-15i: 人工待处理不伪装成仍在自动开通", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { manual: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("待人工处理");
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail()).toContainText("这笔订单需要人工核对或处理异常。");
    await expect(fulfillment.detail()).toContainText("付款记录保留，请勿重复开通或重复扣款。");
    await expect(fulfillment.detail()).not.toContainText("plugin.idc-core.fulfillment.status");
    await expect(fulfillment.detail().getByRole("button", { name: "重新开通", exact: true })).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("manual-attention-no-stuck-processing");
  });
  test("TC-15a: 六态列表和安全历史不显示内部供应商信息", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage);
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();

    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("待商家审核");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("证书");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("第 1 / 共 2 个");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("履约测试客户 · 139****8009 · #10009");
    await expect(fulfillment.row("ORD-MOCK-1")).not.toContainText("CUS_fulfillment_customer_0001");
    await fulfillment.pageRoot.getByRole("combobox", { name: "客户" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("履约测试客户 · 139****8009 · #10009", { exact: true })
      .click();
    await fulfillment.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("customerNo")).toBe("CUS_fulfillment_customer_0001");
    await fulfillment.openDetail("ORD-MOCK-1");
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer).toContainText("资金结清");
    await expect(drawer).toContainText("已确认结清");
    await expect(drawer).toContainText("履约任务已建立");
    await expect(drawer).toContainText("第 1 / 共 2 个");
    await expect(drawer).toContainText("履约测试客户 · 139****8009 · #10009");
    await expect(drawer).not.toContainText("CUS_fulfillment_customer_0001");
    await expect(drawer).not.toContainText(
      /PFI-MOCK|ITEM-MOCK|plugin_id|provider_secret|supplier_payload|evidence_digest|\{"/iu,
    );
    await fulfillment.screenshot("product-fulfillment-safe-detail");
  });

  test("TC-15b: 响应丢失后重试批准动作复用同一个幂等键", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, {
      loseFirstActionResponse: true,
    });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await drawer
      .getByRole("button", { name: /审核通过并开始履约|批准并开始履约/u })
      .click();
    const modal = adminPage
      .locator('[role="dialog"]')
      .filter({ has: adminPage.getByLabel("处理原因") });
    await modal.getByLabel("处理原因").click();
    await adminPage.locator(".ant-select-dropdown:visible").getByText("已核对", { exact: true }).click();
    await modal.getByRole("button", { name: /确\s*(认|定)/u }).click();
    await expect(
      adminPage.getByText("当前状态可能已经变化，本次操作未生效，请刷新后重试。"),
    ).toBeVisible();
    await expect(modal).toBeVisible();
    await modal.getByRole("button", { name: /确\s*(认|定)/u }).click();
    await expect(modal).toBeHidden();

    expect(state.actionBodies).toHaveLength(2);
    expect(state.actionBodies[0]?.idempotencyKey).toBeTruthy();
    expect(state.actionBodies[1]?.idempotencyKey).toBe(
      state.actionBodies[0]?.idempotencyKey,
    );
    expect(state.actionBodies[1]).toMatchObject({ note: "", reasonCode: "verified" });
    await fulfillment.screenshot("product-fulfillment-idempotent-retry");
  });

  test("TC-15c: 人工交付说明必填且响应丢失后重试保留结果", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { manual: true, loseFirstActionResponse: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.screenshot("manual-fulfillment-list");
    await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.openManualAction();
    await expect(fulfillment.actionModal()).toContainText("登记人工履约已完成");
    await expect(fulfillment.actionModal()).toContainText("按实际结果登记完成。");
    await fulfillment.fillManualAction("过短");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeVisible();
    await expect(fulfillment.actionModal()).toContainText("请填写10至512个字的处理说明。");
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("manual-fulfillment-validation");
    const note = "已完成域名注册，交付资料已发送给客户。";
    await fulfillment.actionModal().getByLabel("处理说明").fill(note);
    await fulfillment.screenshot("manual-fulfillment-form");
    await fulfillment.submitManualAction();
    await expect(adminPage.getByText("当前状态可能已经变化，本次操作未生效，请刷新后重试。")).toBeVisible();
    await expect(fulfillment.actionModal().getByLabel("处理说明")).toHaveValue(note);
    await fulfillment.screenshot("manual-fulfillment-retry");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeHidden();
    expect(state.actionBodies).toHaveLength(2);
    expect(state.actionBodies[1]).toEqual(state.actionBodies[0]);
    expect(state.actionBodies[1]).toMatchObject({ expectedVersion: 3, outcomeCode: "completed", note, reasonCode: "manual_fulfillment_completed" });
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("已完成");
    await expect(fulfillment.detail()).toContainText("人工履约已登记完成");
    await expect(fulfillment.detail()).toContainText(note);
    await expect(fulfillment.detail().getByRole("button", { name: "登记已完成", exact: true })).toHaveCount(0);
    await fulfillment.screenshot("manual-fulfillment-completed");
  });

  test("TC-15d: 无法交付保留原因且不冒充成功", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { manual: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.openManualAction(false);
    const note = "供应商域名库存不足，已通知客户办理退款。";
    await fulfillment.fillManualAction(note, false);
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeHidden();
    expect(state.actionBodies).toHaveLength(1);
    expect(state.actionBodies[0]).toMatchObject({ outcomeCode: "unable_to_complete", note, reasonCode: "manual_fulfillment_unable" });
    await expect(fulfillment.detail()).toContainText("人工履约已登记无法完成");
    await expect(fulfillment.detail()).toContainText(note);
    await fulfillment.screenshot("manual-fulfillment-unable");
  });

  test("TC-15e: 没有人工交付权限不显示写操作也不请求", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions: permissions.filter((permission) => !permission.endsWith(":manual-conclude")) });
    const state = await installProductFulfillmentMock(adminPage, { manual: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button", { name: /登记已完成|登记无法完成/u })).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("manual-fulfillment-readonly");
  });
  test("TC-15f: 恢复先验证说明，响应丢失重试保持同一命令", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { recovery: true, loseFirstActionResponse: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.screenshot("preparation-recovery-detail");
    await fulfillment.openRecoveryAction();
    await expect(fulfillment.actionModal()).toContainText("重新开通");
    await fulfillment.fillRecoveryAction("过短");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toContainText("请填写10至512个字的处理说明。");
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("preparation-recovery-note-validation");
    const note = "已修复接口配置并完成开通前检查。";
    await fulfillment.actionModal().getByLabel("处理说明").fill(note);
    await fulfillment.screenshot("preparation-recovery-form");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeVisible();
    await expect(adminPage.getByText("当前状态可能已经变化，本次操作未生效，请刷新后重试。")).toBeVisible();
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeHidden();
    expect(state.actionBodies).toHaveLength(2);
    expect(state.actionBodies[1]).toEqual(state.actionBodies[0]);
    expect(state.actionBodies[0]).toMatchObject({ expectedVersion: 3, reasonCode: "preparation_resolved", note });
    await expect(fulfillment.detail()).toContainText("已申请重新开通");
    await expect(fulfillment.detail()).toContainText(note);
    await expect(fulfillment.detail().getByRole("button", { name: "重新开通", exact: true })).toHaveCount(0);
    await fulfillment.screenshot("preparation-recovery-committed");
  });

  test("TC-15g: 独立恢复权限及不安全状态均隐藏恢复入口", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions: permissions.filter((value) => !value.endsWith(":recover")) });
    const state = await installProductFulfillmentMock(adminPage, { recovery: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button", { name: "重新开通", exact: true })).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("preparation-recovery-permission-denied");
  });

  test("TC-15h: 已受理或未知的任务不能通过恢复入口重放", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions: [...permissions, "idc:admin:billing:refund:review"] });
    const state = await installProductFulfillmentMock(adminPage, { recoveryUnavailable: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button", { name: "重新开通", exact: true })).toBeDisabled();
    await expect(fulfillment.detail().getByRole("button", {name: /退\s*款/u})).toBeDisabled();
    await expect(fulfillment.detail()).toContainText("暂不能重新开通或直接退款");
    await expect(fulfillment.detail()).toContainText("待人工处理");
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("preparation-recovery-unavailable");
  });

});
