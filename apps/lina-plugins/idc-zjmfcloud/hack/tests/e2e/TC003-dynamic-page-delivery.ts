import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect,test} from '../../../../../../hack/tests/fixtures/auth';
import {installDelivery} from '../support/dynamic-delivery';
const server={serverNo:`SVR-${'A'.repeat(32)}`,productNameZhCn:'商务云 4核8G',productNameEnUs:'Business Cloud',channelName:'华东云接口',poolName:'上海资源池',kind:'cloud_server',state:'ready',latestOperation:'idle',boundAt:1777000000000,updatedAt:1777000100000};
test.describe('TC-3 魔方云动态页面交付',()=>{
 test('TC-3a: 后台真实Vben表格、准确分页、只读详情和桥契约',async({adminPage:page})=>{
  await installDelivery(page,'server-management','admin/server-management/index.html',{'admin/resource/zjmfcloud-servers':{items:[server],total:1,nextCursor:''},[`admin/resource/zjmfcloud-servers/${server.serverNo}`]:{item:server}});
  await expect(page.locator('.vxe-grid')).toHaveCount(1);await expect(page.locator('.vxe-table--body-wrapper')).toContainText('商务云 4核8G');
  await expect(page.getByText('共 1 条',{exact:false})).toBeVisible();await page.getByRole('button',{name:'查看详情',exact:true}).click();await expect(page.getByText('服务器详情',{exact:true})).toBeVisible();await expect(page.getByRole('dialog').getByRole('textbox').first()).toHaveValue('商务云 4核8G');await expect.poll(async()=> (await page.getByRole('dialog').boundingBox())?.x ?? 1280).toBeLessThan(500);
  const requests=await page.evaluate(()=>(window as any).__deliveryRequests);expect(requests[0].payload).toEqual({method:'GET',path:'admin/resource/zjmfcloud-servers',params:{page:1,limit:20}});
  expect(requests.every((r:any)=>r.capability==='zjmfcloud-servers.admin.v1'&&r.payload.method==='GET')).toBe(true);
  const path=resolve(process.cwd(),'../../temp/plugin-delivery');await mkdir(path,{recursive:true});await page.screenshot({path:resolve(path,'cloud-admin.png'),fullPage:true});
 });
 test('TC-3b: 客户完整构建页面显示实际数据和详情，不显示内部编号',async({adminPage:page})=>{
  await installDelivery(page,'servers','customer/servers/index.html',{'account/plugin-servers':{items:[server],nextCursor:''},[`account/plugin-servers/${server.serverNo}`]:{item:server}});
  await expect(page.getByTestId('server-content')).toBeVisible();await expect(page.getByTestId('server-list')).toContainText('商务云 4核8G');await page.getByRole('button',{name:'查看详情',exact:true}).click();await expect(page.getByTestId('server-detail-grid')).toContainText('华东云接口');expect(await page.locator('body').innerText()).not.toContain(server.serverNo);
  const path=resolve(process.cwd(),'../../temp/plugin-delivery');await mkdir(path,{recursive:true});await page.screenshot({path:resolve(path,'cloud-customer.png'),fullPage:true});
 });
});
