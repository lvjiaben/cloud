// Exercise real writes and credential persistence without contacting supplier resources.
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { queryPgScalar } from '@host-tests/support/postgres';
import { test, expect } from '../support/catalog-auth';
import { AutomationConfigurationPage } from '../pages/AutomationConfigurationPage';
import { ProviderConfigurationPage } from '../../../../idc-resource/hack/tests/pages/ProviderConfigurationPage';

test.describe('TC-49 自动化接口真实操作', () => {
 test('TC-49a: 保存服务器密钥、测试连接、删除未使用的其他产品接口', async ({ adminPage }) => {
  test.setTimeout(120000);
  adminPage.setDefaultTimeout(15000);
  const api = adminPage.request;
  const token = await adminPage.evaluate(()=>{
   const key=Object.keys(localStorage).find(k=>k.endsWith('core-access'))!;
   return JSON.parse(localStorage.getItem(key)!).accessToken as string;
  });
  await adminPage.context().setExtraHTTPHeaders({Authorization:'Bearer '+token});
  const name = '接口回归-' + randomUUID().slice(0,8);
  const root = pluginApiPath('idc-resource','admin/resource/provider-configurations');
  let channel: any;
  let other: any;
  const center = new AutomationConfigurationPage(adminPage);
  try {
   await test.step('进入自动化接口页面', async()=>{await center.goto(); await center.openAutomation();});
   await expect(adminPage.getByTestId('provider-configuration-page')).toBeVisible();
   await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/225300-interface-entry.png')});
   const directory = await (await api.get(root)).json(); expect(directory.code).toBe(0);
   const candidate = directory.data.providerCandidates.find((item:any) => item.pluginId === 'idc-baota');
   expect(candidate).toBeTruthy();
   const view = new ProviderConfigurationPage(adminPage);
   await test.step('界面新增并保存服务器密钥', async()=>{await view.createChannel({connector: candidate.displayName + ' · ' + candidate.releaseVersion, name, endpoint:'https://supplier.example.invalid', secrets:{api_key:'tc049-test-only-secret'}});});
   const saved = await (await api.get(root,{params:{channelKeyword:name}})).json();
   expect(saved.code).toBe(0); channel=saved.data.channels.find((item:any)=>item.displayName===name);
   expect(channel).toBeTruthy(); expect(channel.secretFields.find((f:any)=>f.key==='api_key').ready).toBe(true);
   await expect(adminPage.getByRole('dialog',{name:'新增接口',exact:true})).toHaveCount(0);
   await expect(view.channelRow(name)).toBeVisible();
   await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/225000-interface-saved.png')});
   const verification = await (await api.post(root+'/channels/'+channel.channelNo+'/connection-verifications')).json();
   if (verification.code===0) expect(verification.data.verified).toBe(false);
   else expect(verification.message).not.toContain('审计');
   const providers = await (await api.get(pluginApiPath('idc-core','admin/catalog/automation/providers'))).json();
   const provider = providers.data.items.find((item:any)=>item.displayName==='云速 SSL 证书'); expect(provider).toBeTruthy();
   const created = await (await api.post(pluginApiPath('idc-core','admin/catalog/automation/interfaces'),{data:{displayName:name+'-SSL',selectionToken:provider.selectionToken,configuration:[{fieldKey:'endpoint',textValue:'https://ssl.example.invalid'},{fieldKey:'api_key',textValue:'tc049-key'},{fieldKey:'api_secret',textValue:'tc049-secret'}]}})).json();
   expect(created.code,created.message).toBe(0); other=created.data.interface;
   expect(other.interfaceInstanceNo).toMatch(/^pfi_[a-z0-9]+$/u);
   const refs = queryPgScalar("SELECT string_agg(quote_literal(s.secret_ref), ',') FROM plugin_idc_core_product_automation_interface_secret_reference s JOIN plugin_idc_core_product_automation_interface_configuration_revision r ON r.id=s.configuration_revision_id WHERE r.interface_instance_no='"+other.interfaceInstanceNo+"'");
   expect(refs).not.toBe('');
   await center.openProductInterfaces();
   const remove=adminPage.getByTestId('automation-interface-delete-'+other.interfaceInstanceNo);
   await adminPage.locator('input[name="namePrefix"]').fill(name+'-SSL');
   await adminPage.getByRole('button',{name:/搜.*索|查.*询/u}).click();
   await expect(remove).toBeVisible(); await expect(remove).toBeEnabled();
   await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/225100-interface-filtered.png')});
   const deleted=adminPage.waitForResponse(r=>r.request().method()==='DELETE' && r.url().includes(other.interfaceInstanceNo));
   await remove.click(); await adminPage.getByRole('button',{name:'确认删除',exact:true}).click();
   const result=await (await deleted).json(); expect(result.code,result.message).toBe(0);
   await expect(remove).toHaveCount(0);
   const list=await (await api.get(pluginApiPath('idc-core','admin/catalog/automation/interfaces'),{params:{namePrefix:name+'-SSL'}})).json();
   expect(list.data.total).toBe(0);
   expect(queryPgScalar("SELECT count(*) FROM plugin_idc_core_secret WHERE secret_no IN ("+refs+") AND status_code='revoked'")).toBe('2');
   expect(queryPgScalar("SELECT count(*) FROM plugin_idc_core_business_audit_event WHERE target_no='"+other.interfaceInstanceNo+"' AND operation_code='product-automation.interface.delete' AND result_code='succeeded'")).toBe('1');
   other=undefined;
   await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/225200-interface-deleted.png')});
  } finally {
   if(other) await api.delete(pluginApiPath('idc-core','admin/catalog/automation/interfaces/'+other.interfaceInstanceNo),{params:{expectedVersion:other.version}});
   if(!channel){const d=await(await api.get(root,{params:{channelKeyword:name}})).json();channel=d.data?.channels?.find((i:any)=>i.displayName===name);}
   if(channel){const clean=await(await api.put(root+'/channels/'+channel.channelNo,{data:{displayName:channel.displayName,endpoint:channel.endpoint,status:'deleted',version:channel.version}})).json();expect(clean.code,clean.message).toBe(0);}
  }
 });
});
