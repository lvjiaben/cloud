import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect,test} from '../../../../../../hack/tests/fixtures/auth';
import {installDelivery} from '../support/dynamic-delivery';
test.describe('TC-4 迁移隔离页交付',()=>{
 test('TC-4a: 真实Vben预检表单和页面桥控制批次创建门禁',async({adminPage:page})=>{
  await installDelivery(page,'migration-checks','admin/migration/index.html',{'admin/migration/preflight':{supported:true,sourceSystem:'zjmf-finance',sourceVersion:'v10',blockers:[],warnings:[],capabilities:['orders']}});
  await expect(page.getByRole('button',{name:'创建批次',exact:true})).toBeDisabled();await page.getByRole('button',{name:'开始检查',exact:true}).click();
  await expect(page.getByText('来源信息',{exact:true})).toBeVisible();await page.getByRole('dialog').getByRole('textbox').first().fill('v10');await page.getByRole('button',{name:'确认',exact:true}).click();
  await expect(page.getByTestId('migration-result')).toContainText('迁移条件检查通过');await expect(page.getByRole('button',{name:'创建批次',exact:true})).toBeEnabled();await expect(page.getByRole('dialog')).toHaveCount(0);
  const requests=await page.evaluate(()=>(window as any).__deliveryRequests);expect(requests[0].payload.method).toBe('POST');expect(requests[0].payload.body.sourceVersion).toBe('v10');expect(requests[0].capability).toBe('idc-migration.admin.v1');
  const path=resolve(process.cwd(),'../../temp/plugin-delivery');await mkdir(path,{recursive:true});await page.screenshot({path:resolve(path,'migration-preflight.png'),fullPage:true});
 });
});
