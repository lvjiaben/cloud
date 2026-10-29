import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { installResourceWorkspaceMenus } from "../support/resource-workspace-mock";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

// All records are test-local metadata. No real upstream request is made.
test("TC017a: 独立对接日志分页筛选、失败保留、权限拒绝清理与中文显示",async({adminPage})=>{
 const menus=await installResourceWorkspaceMenus(adminPage,["idc:admin:resource:api-log:list"]);
 expect(menus).toHaveLength(1);expect(menus[0]?.path).toBe("/idc/resource/api-logs");
 const state={failure:0,requests:[] as URL[]};
 await adminPage.route("**/x/idc-resource/api/v1/admin/resource/api-logs*",async route=>{
  const url=new URL(route.request().url());state.requests.push(url);
  if(state.failure){await route.fulfill({status:state.failure,json:{code:state.failure,message:"Test read failure"}});return;}
  const result=url.searchParams.get("result");
  const all=Array.from({length:61},(_,i)=>({id:61-i,channelNo:"CHN_test",channelName:"陕西测试接口",providerPluginId:"idc-zjmfcloud",methodCode:i%4===3?"operation.execute":i%2?"connection.verify":"operation.execute",succeeded:i%2===0,errorCode:i%4===3?"provider_operation_unknown":i%2?"provider_call_failed":"",durationMs:120,startedAt:1792380000000+i})).filter(item=>!result||item.succeeded===(result==="succeeded"));
  const page=Number(url.searchParams.get("page")||1);const size=Number(url.searchParams.get("limit")||25);
  await route.fulfill({json:{code:0,message:"success",data:{items:all.slice((page-1)*size,page*size),total:all.length}}});
 });
 await adminPage.goto(workspacePath("/idc/resource/api-logs"),{waitUntil:"domcontentloaded"});
 const root=adminPage.getByTestId("provider-api-logs-page");const rows=()=>root.locator(".vxe-table--main-wrapper .vxe-body--row");const pager=root.locator(".vxe-pager");
 await expect(rows()).toHaveCount(25);await expect(pager).toContainText("61");
 await expect(root).toContainText("执行服务器操作");await expect(root).toContainText("连接测试");await expect(root).toContainText("调用异常");await expect(root).toContainText("耗时（毫秒）");await expect(root).not.toContainText("plugin.idc-resource");
 const shot=async(name:string)=>{const now=new Date();const dir=resolve(process.cwd(),"../../temp",now.toISOString().slice(0,10).replaceAll("-",""));await mkdir(dir,{recursive:true});await adminPage.locator("#__app-loading__").waitFor({state:"hidden"});await adminPage.screenshot({path:resolve(dir,`${now.toISOString().slice(11,19).replaceAll(":","")}-${name}.png`)});};
 await expect(root).toContainText("连接器未确认结果，请查看开通任务中的最终诊断");
 await shot("provider-api-logs-loaded");
 await pager.locator(".vxe-pager--next-btn").click();await expect(pager.locator(".is--active")).toHaveText("2");expect(state.requests.at(-1)?.searchParams.get("page")).toBe("2");

 await root.locator(".ant-select").nth(1).click();await adminPage.locator(".ant-select-dropdown:visible").getByText("调用异常",{exact:true}).click();
 await root.getByRole("button",{name:/搜\s*索|查\s*询/}).click();await expect(rows()).toHaveCount(25);await expect(pager).toContainText("30");expect(state.requests.at(-1)?.searchParams.get("result")).toBe("failed");expect(state.requests.at(-1)?.searchParams.get("page")).toBe("1");await expect(root).toContainText("接口请求失败，请检查连接状态及任务详情");
 const refresh=()=>root.locator('.vxe-toolbar [title="刷新"]').click();
 state.failure=503;await refresh();await expect(adminPage.getByText("对接日志读取失败，请重试",{exact:true}).first()).toBeVisible();await expect(rows()).toHaveCount(25);await shot("provider-api-logs-failed");
 state.failure=403;await refresh();await expect(rows()).toHaveCount(0);await expect(pager).toContainText("0");await shot("provider-api-logs-denied");
 state.failure=0;await refresh();await expect(rows()).toHaveCount(25);await expect(pager).toContainText("30");await shot("provider-api-logs-recovered");
});
