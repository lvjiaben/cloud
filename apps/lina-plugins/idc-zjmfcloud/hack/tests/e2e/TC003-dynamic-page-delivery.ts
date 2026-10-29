import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect,test} from '../../../../../../hack/tests/fixtures/auth';
import {installDelivery} from '../support/dynamic-delivery';
const server={serverNo:`SVR-${'A'.repeat(32)}`,productNameZhCn:'商务云 4核8G',productNameEnUs:'Business Cloud',channelName:'华东云接口',poolName:'上海资源池',kind:'cloud_server',state:'ready',latestOperation:'idle',boundAt:1777000000000,updatedAt:1777000100000};
test.describe('TC-3 魔方云动态页面交付',()=>{
 test('TC-3b: 客户完整构建页面显示实际数据和详情，不显示内部编号',async({adminPage:page})=>{
  await installDelivery(page,'servers','customer/servers/index.html',{'account/plugin-servers':{items:[server],nextCursor:''},[`account/plugin-servers/${server.serverNo}`]:{item:server}});
  await expect(page.getByTestId('server-content')).toBeVisible();await expect(page.getByTestId('server-list')).toContainText('商务云 4核8G');await page.getByRole('button',{name:'查看详情',exact:true}).click();await expect(page.getByTestId('server-detail-grid')).toContainText('华东云接口');expect(await page.locator('body').innerText()).not.toContain(server.serverNo);
  const path=resolve(process.cwd(),'../../temp/plugin-delivery');await mkdir(path,{recursive:true});await page.screenshot({path:resolve(path,'cloud-customer.png'),fullPage:true});
 });
});
